package git

import (
	"fmt"
	"os"
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
func GetCurrentVersion(r *git.Repository, settings *Settings, ignoreTravisTag bool) (version string, err error) {
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

	v, err := getVersion(r, h, tagMap, settings)
	if err != nil {
		return "", errors.Wrap(err, "GetCurrentVersion failed")
	}

	return v.String(), nil
}

func getVersion(r *git.Repository, h *plumbing.Reference, tagMap map[string]string, settings *Settings) (version *semver.Version, err error) {
	currentBranch, err := getCurrentBranch(r, h)
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
		if versionMap[index].Name.LessThan(*masterVersion) {
			return nil, errors.Errorf("Branch has tag '%s' whose version is less than master '%s'", versionMap[index].Name, masterVersion)
		}
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

	return baseVersion, nil
}

func getCurrentBranch(r *git.Repository, h *plumbing.Reference) (name string, err error) {
	branchName := ""

	name, ok := os.LookupEnv("TRAVIS_PULL_REQUEST_BRANCH")
	if ok {
		return cleanseBranchName(name), nil
	}

	name, ok = os.LookupEnv("TRAVIS_BRANCH")
	if ok {
		return cleanseBranchName(name), nil
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

	return cleanseBranchName(branchName), nil
}

func cleanseBranchName(name string) string {
	return strings.Replace(name, "/", "-", -1)
}
