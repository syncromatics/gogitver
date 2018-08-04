package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/coreos/go-semver/semver"
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

func main() {
	tagMap := make(map[string]string)

	r, err := git.PlainOpen(".")
	if err != nil {
		panic(err)
	}

	tags, err := r.Tags()
	if err != nil {
		panic(err)
	}

	err = tags.ForEach(func(ref *plumbing.Reference) error {
		c, err := r.CommitObject(ref.Hash())
		if err != nil {
			return err
		}
		tagMap[c.Hash.String()] = ref.Name().Short()
		return nil
	})
	if err != nil {
		panic(err)
	}

	h, err := r.Head()
	if err != nil {
		panic(err)
	}

	v, err := getVersion(r, h, tagMap)
	if err != nil {
		panic(err)
	}

	fmt.Println(v)
}

func getVersion(r *git.Repository, h *plumbing.Reference, tagMap map[string]string) (version *semver.Version, err error) {
	currentBranch, err := getCurrentBranch(r, h)
	if err != nil {
		panic(err)
	}

	masterHead, err := r.Reference("refs/heads/master", false)
	if err != nil {
		panic(err)
	}

	masterVersion, err := getMasterVersion(r, masterHead, tagMap)
	if err != nil {
		panic(err)
	}

	if h.Hash() == masterHead.Hash() {
		return masterVersion, nil
	}

	c, err := r.CommitObject(h.Hash())
	if err != nil {
		return nil, err
	}

	versionMap := []gitVersion{}
	err = walkVersion(r, c, tagMap, &versionMap, masterHead.Hash().String())
	if err != nil {
		return nil, err
	}

	var baseVersion *semver.Version
	index := len(versionMap) - 1
	if index == -1 {
		return nil, fmt.Errorf("Cannot determine version in branch")
	}

	if versionMap[index].IsSolid {
		if versionMap[index].Name.LessThan(*masterVersion) {
			return nil, fmt.Errorf("Branch has tag '%s' whose version is less than master '%s'", versionMap[index].Name, masterVersion)
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

	baseVersion.PreRelease = semver.PreRelease(currentBranch)

	return baseVersion, nil
}

func getMasterVersion(r *git.Repository, masterHead *plumbing.Reference, tagMap map[string]string) (version *semver.Version, err error) {
	versionMap := []gitVersion{}

	c, err := r.CommitObject(masterHead.Hash())
	if err != nil {
		return nil, err
	}
	err = walkVersion(r, c, tagMap, &versionMap, "")
	if err != nil {
		return nil, err
	}

	var baseVersion *semver.Version
	index := len(versionMap) - 1
	if versionMap[index].IsSolid {
		baseVersion = versionMap[index].Name
		index--
	} else {
		baseVersion, err = semver.NewVersion("0.0.0")
		if err != nil {
			return nil, err
		}
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

	return baseVersion, nil
}

func walkVersion(r *git.Repository, ref *object.Commit, tagMap map[string]string, versionMap *[]gitVersion, endHash string) error {
	tag, ok := tagMap[ref.Hash.String()]
	if ok {
		tagVersion, err := semver.NewVersion(tag)
		if err != nil {
			return err
		}
		*versionMap = append(*versionMap, gitVersion{IsSolid: true, Name: tagVersion})
		return nil
	}

	matched, err := regexp.MatchString("(\\+semver: major)|(\\+semver: breaking)", ref.Message)
	if err != nil {
		return err
	}
	if matched {
		*versionMap = append(*versionMap, gitVersion{IsSolid: false, MajorBump: true})
		return checkWalkParent(r, ref, tagMap, versionMap, endHash)
	}

	matched, err = regexp.MatchString("(\\+semver: minor)|(\\+semver: feature)", ref.Message)
	if err != nil {
		return err
	}
	if matched {
		*versionMap = append(*versionMap, gitVersion{IsSolid: false, MinorBump: true})
		return checkWalkParent(r, ref, tagMap, versionMap, endHash)
	}

	*versionMap = append(*versionMap, gitVersion{IsSolid: false, PatchBump: true})

	return checkWalkParent(r, ref, tagMap, versionMap, endHash)
}

func checkWalkParent(r *git.Repository, ref *object.Commit, tagMap map[string]string, versionMap *[]gitVersion, endHash string) error {
	if ref.NumParents() == 0 {
		return nil
	}

	parent, err := ref.Parent(0)
	if err != nil {
		return nil
	}

	if parent.Hash.String() == endHash {
		return nil
	}

	return walkVersion(r, parent, tagMap, versionMap, endHash)
}

func getCurrentBranch(r *git.Repository, h *plumbing.Reference) (name string, err error) {
	branchName := ""

	refs, err := r.References()
	if err != nil {
		panic(err)
	}
	err = refs.ForEach(func(r *plumbing.Reference) error {
		if r.Name().IsBranch() && r.Hash() == h.Hash() {
			branchName = r.Name().Short()
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	if branchName == "" {
		return "", fmt.Errorf("Cannot determine branch")
	}

	branchName = strings.Replace(branchName, "/", "-", -1)
	return branchName, nil
}
