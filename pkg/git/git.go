package git

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type gitVersion struct {
	IsSolid bool
	Name    *semver.Version

	MajorBump bool
	MinorBump bool
	PatchBump bool
}

// GetCurrentVersion returns the current version
func GetCurrentVersion(r *git.Repository, settings *Settings, ignoreTravisTag bool, forbidBehindMaster bool, trimPrefix bool) (version string, err error) {
	tag, ok := os.LookupEnv("TRAVIS_TAG")
	if !ignoreTravisTag && ok && tag != "" { // If this is a tagged build in travis shortcircuit here
		version, err := semver.NewVersion(tag)
		if err != nil {
			return "", err
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
		tagMap[ref.Hash().String()] = strings.Replace(name, "refs/tags/", "", -1)
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

	v, err := getVersion(r, h, tagMap, forbidBehindMaster, trimPrefix, settings)
	if err != nil {
		return "", errors.Wrap(err, "GetCurrentVersion failed")
	}

	return v.String(), nil
}

// GetPrereleaseLabel returns the prerelease label for the current branch
func GetPrereleaseLabel(r *git.Repository, settings *Settings, trimPrefix bool) (result string, err error) {
	h, err := r.Head()
	if err != nil {
		return "", errors.Wrap(err, "GetCurrentVersion failed")
	}
	return getCurrentBranch(r, h, trimPrefix)
}

func getVersion(r *git.Repository, h *plumbing.Reference, tagMap map[string]string, forbidBehindMaster bool, trimPrefix bool, settings *Settings) (version *semver.Version, err error) {
	currentBranch, err := getCurrentBranch(r, h, trimPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "getVersion failed")
	}

	masterHead, err := r.Reference("refs/heads/master", false)
	if err != nil {
		masterHead, err = r.Reference("refs/remotes/origin/master", false)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get master branch at 'refs/heads/master, 'refs/remotes/origin/master'")
		}
	}

	masterCommit, err := r.CommitObject(masterHead.Hash())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get master commit from reference")
	}

	masterWalker := newBranchWalker(r, masterCommit, tagMap, settings, true, "")
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

	walker := newBranchWalker(r, c, tagMap, settings, false, masterHead.Hash().String())
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

	if forbidBehindMaster && baseVersion.LessThan(*masterVersion) {
		return nil, errors.Errorf("Branch has calculated version '%s' whose version is less than master '%s'", baseVersion, masterVersion)
	}

	return baseVersion, nil
}

func getCurrentBranch(r *git.Repository, h *plumbing.Reference, trimPrefix bool) (name string, err error) {
	branchName := ""

	name, ok := os.LookupEnv("TRAVIS_PULL_REQUEST_BRANCH") // Travis
	if ok {
		branchName, err := cleanseBranchName(name, trimPrefix)
		if err != nil {
			return "", err
		}
		return branchName, nil
	}

	name, ok = os.LookupEnv("TRAVIS_BRANCH")
	if ok {
		branchName, err := cleanseBranchName(name, trimPrefix)
		if err != nil {
			return "", err
		}
		return branchName, nil
	}

	name, ok = os.LookupEnv("CI_COMMIT_REF_NAME") // GitLab
	if ok {
		branchName, err := cleanseBranchName(name, trimPrefix)
		if err != nil {
			return "", err
		}
		return branchName, nil
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

	branch, err := cleanseBranchName(branchName, trimPrefix)
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
