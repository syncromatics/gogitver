package git

import (
	stderrors "errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/coreos/go-semver/semver"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pkg/errors"
)

// BranchSettings contains flags that determine how branches are handled when calculating versions.
type BranchSettings struct {
	ForbidBehindMaster  bool
	ForbidBehindPrimary bool
	TrimBranchPrefix    bool
	IgnoreEnvVars       bool
	PrimaryBranch       string
}

type ambiguousPrimaryBranchError struct {
	message string
}

func (e *ambiguousPrimaryBranchError) Error() string {
	return e.message
}

// IsAmbiguousPrimaryBranchError returns true when primary branch auto-detection
// cannot safely choose between main and master.
func IsAmbiguousPrimaryBranchError(err error) bool {
	var target *ambiguousPrimaryBranchError
	return stderrors.As(err, &target)
}

type gitVersion struct {
	IsSolid bool
	Name    *semver.Version

	MajorBump bool
	MinorBump bool
	PatchBump bool
	Commit    string
}

// GetCurrentVersion returns the current version
func GetCurrentVersion(r *git.Repository, settings *Settings, branchSettings *BranchSettings, verbose bool) (version string, err error) {
	tag, ok := os.LookupEnv("TRAVIS_TAG")
	if !branchSettings.IgnoreEnvVars && ok && tag != "" { // If this is a tagged build in travis shortcircuit here
		version, err := parseTag(tag)
		if err != nil {
			return "", err
		}
		if verbose {
			log.Printf("Version determined using TRAVIS_TAG")
		}
		return version.String(), err
	}

	tagMap := make(map[string]string)

	// lightweight tags
	ltags, err := r.Tags()
	if err != nil {
		return "", errors.Wrap(err, "get tags failed")
	}

	err = ltags.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		tag := strings.Replace(name, "refs/tags/", "", -1)
		if verbose {
			log.Printf("Found lightweight tag %s for ref %s", tag, name)
		}
		tagMap[ref.Hash().String()] = tag
		return nil
	})

	// annotated tags
	tags, err := r.TagObjects()
	if err != nil {
		return "", errors.Wrap(err, "get tag objects failed")
	}

	err = tags.ForEach(func(ref *object.Tag) error {
		c, err := ref.Commit()
		if err != nil {
			return errors.Wrap(err, "get commit failed")
		}
		if verbose {
			log.Printf("Found tag %s", ref.Name)
		}
		tagMap[c.Hash.String()] = ref.Name
		return nil
	})
	if err != nil {
		return "", errors.Wrap(err, "GetCurrentVersion failed")
	}

	h, err := r.Head()
	if err != nil {
		return "", errors.Wrap(err, "GetCurrentVersion failed")
	}

	v, err := getVersion(r, h, tagMap, branchSettings, settings, verbose)
	if err != nil {
		return "", errors.Wrap(err, "GetCurrentVersion failed")
	}

	return v.String(), nil
}

// GetPrereleaseLabel returns the prerelease label for the current branch
func GetPrereleaseLabel(r *git.Repository, settings *Settings, branchSettings *BranchSettings) (result string, err error) {
	h, err := r.Head()
	if err != nil {
		return "", errors.Wrap(err, "GetCurrentVersion failed")
	}
	return getCurrentBranch(r, h, branchSettings)
}

// GetPrimaryBranchName returns the branch name used as the stable version baseline.
func GetPrimaryBranchName(r *git.Repository, branchSettings *BranchSettings) (string, error) {
	_, name, err := getPrimaryBranchReference(r, branchSettings)
	return name, err
}

func getVersion(r *git.Repository, h *plumbing.Reference, tagMap map[string]string, branchSettings *BranchSettings, settings *Settings, verbose bool) (version *semver.Version, err error) {
	currentBranch, err := getCurrentBranch(r, h, branchSettings)
	if err != nil {
		return nil, errors.Wrap(err, "getVersion failed")
	}
	if verbose {
		log.Printf("Current branch is %s", currentBranch)
	}

	primaryHead, primaryBranch, err := getPrimaryBranchReference(r, branchSettings)
	if err != nil {
		return nil, err
	}
	if verbose {
		log.Printf("Primary branch is %s", primaryBranch)
	}

	primaryCommit, err := r.CommitObject(primaryHead.Hash())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get primary branch commit from reference")
	}

	primaryWalker := newBranchWalker(r, primaryCommit, tagMap, settings, true, "", verbose)
	primaryVersion, err := primaryWalker.GetVersion()
	if err != nil {
		return nil, err
	}

	if h.Hash() == primaryHead.Hash() {
		return primaryVersion, nil
	}

	c, err := r.CommitObject(h.Hash())
	if err != nil {
		return nil, errors.Wrap(err, "getVersion failed")
	}

	walker := newBranchWalker(r, c, tagMap, settings, false, primaryHead.Hash().String(), verbose)
	versionMap, err := walker.GetVersionMap()
	if err != nil {
		return nil, err
	}

	var baseVersion *semver.Version
	index := len(versionMap) - 1
	if index == -1 {
		return nil, errors.Errorf("Cannot determine version in branch")
	}

	if versionMap[index].IsSolid {
		baseVersion = versionMap[index].Name
		index--
	} else {
		baseVersion = primaryVersion
	}

	if index < 0 {
		return baseVersion, nil
	}

	for ; index >= 0; index-- {
		v := versionMap[index]
		switch {
		case v.MajorBump:
			baseVersion.BumpMajor()
		case v.MinorBump:
			baseVersion.BumpMinor()
		case v.PatchBump:
			baseVersion.BumpPatch()
		}
	}

	shortHash := h.Hash().String()[:4]
	prerelease := fmt.Sprintf("%s-%d-%s", currentBranch, len(versionMap)-1, shortHash)
	baseVersion.PreRelease = semver.PreRelease(prerelease)

	if (branchSettings.ForbidBehindPrimary || branchSettings.ForbidBehindMaster) && baseVersion.LessThan(*primaryVersion) {
		return nil, errors.Errorf("Branch has calculated version '%s' whose version is less than primary branch '%s'", baseVersion, primaryVersion)
	}

	return baseVersion, nil
}

func getPrimaryBranchReference(r *git.Repository, branchSettings *BranchSettings) (*plumbing.Reference, string, error) {
	if branchSettings.PrimaryBranch != "" {
		ref, err := getBranchReference(r, branchSettings.PrimaryBranch)
		if err != nil {
			return nil, "", errors.Wrapf(err, "failed to get primary branch '%s'", branchSettings.PrimaryBranch)
		}
		return ref, branchSettings.PrimaryBranch, nil
	}

	mainRef, hasMain, err := getOptionalReference(r, plumbing.NewBranchReferenceName("main"))
	if err != nil {
		return nil, "", err
	}
	masterRef, hasMaster, err := getOptionalReference(r, plumbing.NewBranchReferenceName("master"))
	if err != nil {
		return nil, "", err
	}
	if hasMain && hasMaster {
		return nil, "", &ambiguousPrimaryBranchError{message: "found both refs/heads/main and refs/heads/master; specify --primary-branch to choose the version baseline"}
	}
	if hasMain {
		return mainRef, "main", nil
	}
	if hasMaster {
		return masterRef, "master", nil
	}

	originMainRef, hasOriginMain, err := getOptionalReference(r, plumbing.NewRemoteReferenceName("origin", "main"))
	if err != nil {
		return nil, "", err
	}
	originMasterRef, hasOriginMaster, err := getOptionalReference(r, plumbing.NewRemoteReferenceName("origin", "master"))
	if err != nil {
		return nil, "", err
	}
	if hasOriginMain && hasOriginMaster {
		return nil, "", &ambiguousPrimaryBranchError{message: "found both refs/remotes/origin/main and refs/remotes/origin/master; specify --primary-branch to choose the version baseline"}
	}
	if hasOriginMain {
		return originMainRef, "main", nil
	}
	if hasOriginMaster {
		return originMasterRef, "master", nil
	}

	return nil, "", errors.New("failed to get primary branch at refs/heads/main, refs/heads/master, refs/remotes/origin/main, or refs/remotes/origin/master")
}

func getOptionalReference(r *git.Repository, referenceName plumbing.ReferenceName) (*plumbing.Reference, bool, error) {
	ref, err := r.Reference(referenceName, false)
	if err == plumbing.ErrReferenceNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to read reference '%s'", referenceName)
	}

	return ref, true, nil
}

func getBranchReference(r *git.Repository, branchName string) (*plumbing.Reference, error) {
	ref, err := r.Reference(plumbing.NewBranchReferenceName(branchName), false)
	if err == nil {
		return ref, nil
	}

	ref, err = r.Reference(plumbing.NewRemoteReferenceName("origin", branchName), false)
	if err == nil {
		return ref, nil
	}

	return nil, err
}

func getCurrentBranch(r *git.Repository, h *plumbing.Reference, branchSettings *BranchSettings) (name string, err error) {
	branchName := ""

	if !branchSettings.IgnoreEnvVars {
		name, ok := os.LookupEnv("TRAVIS_PULL_REQUEST_BRANCH") // Travis
		if ok {
			branchName, err := cleanseBranchName(name, branchSettings.TrimBranchPrefix)
			if err != nil {
				return "", err
			}
			return branchName, nil
		}

		name, ok = os.LookupEnv("TRAVIS_BRANCH")
		if ok {
			branchName, err := cleanseBranchName(name, branchSettings.TrimBranchPrefix)
			if err != nil {
				return "", err
			}
			return branchName, nil
		}

		name, ok = os.LookupEnv("CI_COMMIT_REF_NAME") // GitLab
		if ok {
			branchName, err := cleanseBranchName(name, branchSettings.TrimBranchPrefix)
			if err != nil {
				return "", err
			}
			return branchName, nil
		}
	}

	refs, err := r.References()
	if err != nil {
		return "", err
	}
	err = refs.ForEach(func(r *plumbing.Reference) error {
		if r.Name().IsBranch() && r.Hash() == h.Hash() {
			branchName = r.Name().Short()
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	if branchName == "" {
		return "", fmt.Errorf("Cannot determine branch")
	}

	branch, err := cleanseBranchName(branchName, branchSettings.TrimBranchPrefix)
	if err != nil {
		return "", err
	}
	return branch, nil
}

func cleanseBranchName(name string, trimPrefix bool) (string, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		return "", err
	}

	branchName := reg.ReplaceAllString(name, "-")
	if !trimPrefix {
		return branchName, nil
	}

	reg, err = regexp.Compile("^(feature|hotfix)-")
	if err != nil {
		return "", err
	}

	branchName = reg.ReplaceAllString(branchName, "")
	return branchName, nil
}
