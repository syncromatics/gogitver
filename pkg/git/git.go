package git

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// BranchSettings contains flags that determine how branches are handled when calculating versions.
type BranchSettings struct {
	ForbidBehindMaster bool
	TrimBranchPrefix   bool
	IgnoreEnvVars      bool
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

func getVersion(r *git.Repository, h *plumbing.Reference, tagMap map[string]string, branchSettings *BranchSettings, settings *Settings, verbose bool) (version *semver.Version, err error) {
	currentBranch, err := getCurrentBranch(r, h, branchSettings)
	if err != nil {
		return nil, errors.Wrap(err, "getVersion failed")
	}
	if verbose {
		log.Printf("Current branch is %s", currentBranch)
	}

	masterHead, err := r.Reference("refs/heads/master", false)
	if err != nil {
		masterHead, err = r.Reference("refs/remotes/origin/master", false) // TODO: This needs test coverage
		if err != nil {
			return nil, errors.Wrap(err, "failed to get master branch at 'refs/heads/master, 'refs/remotes/origin/master'")
		}
	}

	masterCommit, err := r.CommitObject(masterHead.Hash())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get master commit from reference")
	}

	masterWalker := newBranchWalker(r, masterCommit, tagMap, settings, true, "", verbose)
	masterVersion, err := masterWalker.GetVersion()
	if err != nil {
		return nil, err
	}

	if h.Hash() == masterHead.Hash() {
		return masterVersion, nil
	}

	c, err := r.CommitObject(h.Hash())
	if err != nil {
		return nil, errors.Wrap(err, "getVersion failed")
	}

	walker := newBranchWalker(r, c, tagMap, settings, false, masterHead.Hash().String(), verbose)
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
		baseVersion = masterVersion
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

	if branchSettings.ForbidBehindMaster && baseVersion.LessThan(*masterVersion) {
		return nil, errors.Errorf("Branch has calculated version '%s' whose version is less than master '%s'", baseVersion, masterVersion)
	}

	return baseVersion, nil
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
