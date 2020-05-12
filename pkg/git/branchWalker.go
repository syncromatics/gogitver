package git

import (
	"log"
	"regexp"
	"strings"

	"gopkg.in/src-d/go-git.v4/plumbing"

	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type branchWalker struct {
	repository *git.Repository
	head       *object.Commit
	tagMap     map[string]string
	settings   *Settings
	isMaster   bool
	endHash    string
	verbose    bool

	visited            map[string]bool
	commitsToReconcile map[string]*gitVersion
}

type versionHolder struct {
	versionMap []*gitVersion
}

func newBranchWalker(repository *git.Repository, head *object.Commit, tagMap map[string]string, settings *Settings, isMaster bool, endHash string, verbose bool) *branchWalker {
	return &branchWalker{
		repository:         repository,
		head:               head,
		settings:           settings,
		tagMap:             tagMap,
		isMaster:           isMaster,
		endHash:            endHash,
		visited:            make(map[string]bool),
		commitsToReconcile: make(map[string]*gitVersion),
		verbose:            verbose,
	}
}

func (b *branchWalker) GetVersion() (*semver.Version, error) {
	versionMap, err := b.GetVersionMap()
	if err != nil {
		return nil, err
	}

	var baseVersion *semver.Version
	index := len(versionMap) - 1
	v := versionMap[index]
	if v.IsSolid {
		baseVersion = v.Name
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

	if b.verbose {
		log.Printf("[%s] %s", v.Commit, baseVersion.String())
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
		default: // every commit in master has at least a patch bump
			baseVersion.BumpPatch()
		}
		if b.verbose {
			log.Printf("[%s] %s", v.Commit, baseVersion.String())
		}
	}

	return baseVersion, nil
}

func (b *branchWalker) GetVersionMap() ([]*gitVersion, error) {
	versionMap := versionHolder{
		versionMap: []*gitVersion{},
	}

	err := b.walkVersion(b.head, &versionMap, false)
	if err != nil {
		return nil, err
	}

	if b.isMaster {
		for hash, version := range b.commitsToReconcile {
			err = b.reconcileCommit(hash, version)
			if err != nil {
				return nil, err
			}
		}
	}

	return versionMap.versionMap, nil
}

func (b *branchWalker) walkVersion(ref *object.Commit, version *versionHolder, tilVisited bool) error {
	if _, visited := b.visited[ref.Hash.String()]; tilVisited && visited {
		return nil
	}

	b.visited[ref.Hash.String()] = true

	tag, ok := b.tagMap[ref.Hash.String()]
	if ok {
		tagVersion, err := parseTag(tag)
		if err != nil {
			return err
		}
		version.versionMap = append(version.versionMap, &gitVersion{IsSolid: true, Name: tagVersion, Commit: ref.Hash.String()})
		return nil
	}

	parents := ref.NumParents()
	if parents > 1 {
		versionToReconcile := gitVersion{IsSolid: false, Commit: ref.Hash.String()}
		version.versionMap = append(version.versionMap, &versionToReconcile)

		b.commitsToReconcile[ref.Hash.String()] = &versionToReconcile
		return b.checkWalkParent(ref, version, tilVisited)
	}

	matched, err := regexp.MatchString(b.settings.MajorPattern, ref.Message)
	if err != nil {
		return err
	}
	if matched {
		version.versionMap = append(version.versionMap, &gitVersion{IsSolid: false, MajorBump: true, Commit: ref.Hash.String()})
		return b.checkWalkParent(ref, version, tilVisited)
	}

	matched, err = regexp.MatchString(b.settings.MinorPattern, ref.Message)
	if err != nil {
		return err
	}
	if matched {
		version.versionMap = append(version.versionMap, &gitVersion{IsSolid: false, MinorBump: true, Commit: ref.Hash.String()})
		return b.checkWalkParent(ref, version, tilVisited)
	}

	matched, err = regexp.MatchString(b.settings.PatchPattern, ref.Message)
	if err != nil {
		return err
	}
	if matched {
		version.versionMap = append(version.versionMap, &gitVersion{IsSolid: false, PatchBump: true, Commit: ref.Hash.String()})
		return b.checkWalkParent(ref, version, tilVisited)
	}

	version.versionMap = append(version.versionMap, &gitVersion{IsSolid: false, Commit: ref.Hash.String()})
	return b.checkWalkParent(ref, version, tilVisited)
}

func (b *branchWalker) checkWalkParent(ref *object.Commit, version *versionHolder, tilVisited bool) error {
	parents := ref.NumParents()
	if parents == 0 {
		return nil
	}

	parent, err := ref.Parent(0)
	if err != nil {
		return nil
	}

	if parent.Hash.String() == b.endHash {
		return nil
	}

	return b.walkVersion(parent, version, tilVisited)
}

func (b *branchWalker) reconcileCommit(hash string, version *gitVersion) error {
	commit, err := b.repository.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return errors.Wrap(err, "failed to get commit in reconcile")
	}

	numParents := commit.NumParents()
	if numParents <= 1 {
		return nil
	}

	versionMap := versionHolder{
		versionMap: []*gitVersion{},
	}
	for i := 1; i < numParents; i++ {
		parentToWalk, err := commit.Parent(i)
		if err != nil {
			return errors.Wrap(err, "failed to get parent in reconcile")
		}

		err = b.walkVersion(parentToWalk, &versionMap, true)
		if err != nil {
			return err
		}
	}

	var hasMajor, hasMinor bool
	for _, bump := range versionMap.versionMap {
		if bump.MajorBump {
			hasMajor = true
		}
		if bump.MinorBump {
			hasMinor = true
		}
	}

	if hasMajor {
		version.MajorBump = true
	} else if hasMinor {
		version.MinorBump = true
	} else {
		version.PatchBump = true
	}

	return nil
}

func parseTag(tag string) (*semver.Version, error) { // TODO: Support ignoring invalid semver tags
	trimmedTag := strings.TrimPrefix(tag, "v")
	version, err := semver.NewVersion(trimmedTag)
	if err != nil {
		return nil, err
	}

	return version, nil
}
