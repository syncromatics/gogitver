package git_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-billy.v4/util"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/memory"

	igit "github.com/syncromatics/gogitver/pkg/git"
)

func Test_ShouldCalculateVersionFromCommitsInMaster(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	commitMultiple(t, worktree,
		"(+semver: breaking) This is a major commit\n",
		"(+semver: major) This is also a major commit\n",
		"(+semver: feature) This is a minor commit\n",
		"(+semver: minor) This is also a minor commit\n",
		"(+semver: fix) This is a patch commit\n",
		"(+semver: patch) This is also a patch commit\n",
		"This is also a patch commit\n",
	)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	assert.Nil(t, err)

	// Assert
	assert.Equal(t, "2.2.3", version)
}

func Test_ShouldCalculateVersionFromCommitsInBranch(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	commitMultiple(t, worktree, "Initial commit")

	err := worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/a-branch"),
	})
	assert.Nil(t, err)

	hash := commitMultiple(t, worktree,
		"(+semver: major)\n",
		"(+semver: minor)\n",
		"(+semver: patch)\n",
		"some text\n",
	)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	assert.Nil(t, err)

	// Assert
	shortHash := hash.String()[0:4]
	expected := fmt.Sprintf("1.1.1-a-branch-3-%s", shortHash)
	assert.Equal(t, expected, version)
}

func Test_ShouldCalculateVersionFromCommitsInMasterWithMergeCommits(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	masterHash := commitMultiple(t, worktree, "Initial commit")

	err := worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/a-branch"),
	})
	assert.Nil(t, err)

	branchHash := commitMultiple(t, worktree,
		"(+semver: major)\n",
		"(+semver: minor)\n",
		"(+semver: patch)\n",
		"some text\n",
	)

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/master"),
	})
	assert.Nil(t, err)

	masterHash, err = worktree.Commit("merged a-branch\n", &git.CommitOptions{
		Author: defaultSignature(),
		Parents: []plumbing.Hash{
			masterHash,
			branchHash,
		},
	})
	assert.Nil(t, err)

	err = worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/another-branch"),
	})
	assert.Nil(t, err)

	branchHash = commitMultiple(t, worktree,
		"(+semver: minor)\n",
		"(+semver: patch)\n",
	)

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/master"),
	})
	assert.Nil(t, err)

	masterHash, err = worktree.Commit("merged another-branch\n", &git.CommitOptions{
		Author: defaultSignature(),
		Parents: []plumbing.Hash{
			masterHash,
			branchHash,
		},
	})
	assert.Nil(t, err)

	err = worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/yet-another-branch"),
	})
	assert.Nil(t, err)

	branchHash = commitMultiple(t, worktree,
		"(+semver: patch)\n",
		"(+semver: patch)\n",
		"(+semver: patch)\n",
	)

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/master"),
	})
	assert.Nil(t, err)

	_, err = worktree.Commit("merged yet-another-branch\n", &git.CommitOptions{
		Author: defaultSignature(),
		Parents: []plumbing.Hash{
			masterHash,
			branchHash,
		},
	})
	assert.Nil(t, err)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	assert.Nil(t, err)

	// Assert
	assert.Equal(t, "1.1.1", version)
}

func Test_ShouldCalculateVersionFromLightweightTag(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	hash := commitMultiple(t, worktree,
		"(+semver: breaking)\n",
		"(+semver: breaking)\n",
	)

	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/v1.2.3"), hash)
	err := repository.Storer.SetReference(ref)
	assert.Nil(t, err)

	commitMultiple(t, worktree,
		"(+semver: minor)\n",
		"(+semver: patch)\n",
	)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	assert.Nil(t, err)

	// Assert
	assert.Equal(t, "1.3.1", version)
}

func Test_ShouldFailToCalculateVersionFromImproperlyNamedLightweightTag(t *testing.T) { // TODO: Submit a PR that allows this condition to exist; ignore unparsable tags
	// Arrange
	repository, worktree := initRepository(t)

	hash := commitMultiple(t, worktree, "Initial commit")

	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/an-arbitrary-tag-name"), hash)
	err := repository.Storer.SetReference(ref)
	assert.Nil(t, err)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	_, err = igit.GetCurrentVersion(repository, settings, branchSettings, false)

	// Assert
	assert.NotNil(t, err)
}

func Test_ShouldCalculateVersionFromAnnotatedTag(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	hash := commitMultiple(t, worktree,
		"(+semver: breaking)\n",
		"(+semver: breaking)\n",
	)

	commitObj, err := object.GetObject(repository.Storer, hash)
	tag := object.Tag{
		Name:       "5.6.7",
		Message:    "not important",
		TargetType: commitObj.Type(),
		Target:     hash,
	}
	tagObj := repository.Storer.NewEncodedObject()
	err = tag.Encode(tagObj)
	assert.Nil(t, err)

	target, err := repository.Storer.SetEncodedObject(tagObj)
	assert.Nil(t, err)

	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/5.6.7"), target)
	err = repository.Storer.SetReference(ref)
	assert.Nil(t, err)

	commitMultiple(t, worktree,
		"(+semver: minor)\n",
		"(+semver: patch)\n",
	)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	assert.Nil(t, err)

	// Assert
	assert.Equal(t, "5.7.1", version)
}

func Test_ShouldCalculateVersionFromTravisTag(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	commitMultiple(t, worktree, "Initial commit")

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{}
	os.Setenv("TRAVIS_TAG", "v1.2.3")

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	assert.Nil(t, err)

	// Assert
	assert.Equal(t, "1.2.3", version)
}

func initRepository(t *testing.T) (*git.Repository, *git.Worktree) {
	fs := memfs.New()
	storage := memory.NewStorage()

	repository, err := git.Init(storage, fs)
	assert.Nil(t, err)

	worktree, err := repository.Worktree()
	assert.Nil(t, err)

	return repository, worktree
}

func commitMultiple(t *testing.T, worktree *git.Worktree, messages ...string) plumbing.Hash {
	var hash plumbing.Hash
	var err error
	for _, msg := range messages {
		hash, err = worktree.Commit(msg, &git.CommitOptions{Author: defaultSignature()})
		assert.Nil(t, err)
	}

	return hash
}

func TestTrimBranchPrefix(t *testing.T) {
	r := getSingleBranchCommit("feature/should-be-trimmed", t)
	s := igit.GetDefaultSettings()
	label, err := igit.GetPrereleaseLabel(r, s, &igit.BranchSettings{
		IgnoreEnvVars:    true,
		TrimBranchPrefix: true,
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	assert.Equal(t, "should-be-trimmed", label)
}

func TestCleanseBranchName(t *testing.T) {
	r := getSingleBranchCommit("author's-branch", t)
	s := igit.GetDefaultSettings()
	label, err := igit.GetPrereleaseLabel(r, s, &igit.BranchSettings{
		IgnoreEnvVars: true,
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	assert.Equal(t, "author-s-branch", label)
}

func getSingleBranchCommit(branchName string, t *testing.T) *git.Repository {
	fs := memfs.New()
	storage := memory.NewStorage()

	r, err := git.Init(storage, fs)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	w, err := r.Worktree()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	util.WriteFile(fs, "foo", []byte("foo"), 0644)
	_, err = w.Add("foo")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	_, err = w.Commit("foo\n", &git.CommitOptions{Author: defaultSignature()})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	ref := fmt.Sprintf("refs/heads/%s", branchName)
	b := plumbing.ReferenceName(ref)
	w.Checkout(&git.CheckoutOptions{
		Create: true,
		Force:  false,
		Branch: b,
	})

	util.WriteFile(fs, "foo2", []byte("foo"), 0644)
	_, err = w.Add("foo2")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	_, err = w.Commit("(+semver: major) This is a major commit\n", &git.CommitOptions{Author: defaultSignature()})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	return r
}

func defaultSignature() *object.Signature {
	when, _ := time.Parse(object.DateFormat, "Thu May 04 00:03:43 2017 +0200")
	return &object.Signature{
		Name:  "foo",
		Email: "foo@foo.foo",
		When:  when,
	}
}
