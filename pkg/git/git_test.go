package git_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err)

	// Assert
	assert.Equal(t, "2.2.3", version)
}

func Test_ShouldCalculateVersionFromCommitsInMain(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	commitMultiple(t, worktree, "Initial commit")

	err := worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/main"),
	})
	require.NoError(t, err)

	err = repository.Storer.RemoveReference(plumbing.ReferenceName("refs/heads/master"))
	require.NoError(t, err)

	commitMultiple(t, worktree,
		"(+semver: breaking) This is a major commit\n",
		"(+semver: minor) This is a minor commit\n",
		"(+semver: patch) This is a patch commit\n",
	)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, "1.1.1", version)
}

func Test_ShouldFailWhenMainAndMasterBothExistWithoutExplicitPrimaryBranch(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	commitMultiple(t, worktree, "Initial commit")

	err := worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/main"),
	})
	require.NoError(t, err)

	commitMultiple(t, worktree, "(+semver: minor) main branch commit\n")

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	_, err = igit.GetCurrentVersion(repository, settings, branchSettings, false)

	// Assert
	require.ErrorContains(t, err, "found both 'main' and 'master' branches")
}

func Test_ShouldUseExplicitPrimaryBranch(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	commitMultiple(t, worktree, "Initial commit")

	err := worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/main"),
	})
	require.NoError(t, err)

	commitMultiple(t, worktree, "(+semver: major) main branch commit\n")

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/master"),
	})
	require.NoError(t, err)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
		PrimaryBranch: "master",
	}

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, "0.0.1", version)
}

func Test_ShouldUseOriginMainWhenLocalPrimaryBranchIsMissing(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	primaryHash := commitMultiple(t, worktree, "Initial commit")

	err := repository.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/remotes/origin/main"), primaryHash))
	require.NoError(t, err)

	err = worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/a-branch"),
	})
	require.NoError(t, err)

	hash := commitMultiple(t, worktree, "(+semver: patch) branch commit\n")

	err = repository.Storer.RemoveReference(plumbing.ReferenceName("refs/heads/master"))
	require.NoError(t, err)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	require.NoError(t, err)

	// Assert
	shortHash := hash.String()[0:4]
	expected := fmt.Sprintf("0.0.2-a-branch-0-%s", shortHash)
	assert.Equal(t, expected, version)
}

func Test_ShouldForbidBranchBehindPrimary(t *testing.T) {
	// Arrange
	repository := repositoryWithBranchBehindPrimary(t)
	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		ForbidBehindPrimary: true,
		IgnoreEnvVars:       true,
	}

	// Act
	_, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)

	// Assert
	require.ErrorContains(t, err, "whose version is less than primary branch")
}

func Test_ShouldForbidBranchBehindPrimaryWithMasterCompatibilityFlag(t *testing.T) {
	// Arrange
	repository := repositoryWithBranchBehindPrimary(t)
	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		ForbidBehindMaster: true,
		IgnoreEnvVars:      true,
	}

	// Act
	_, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)

	// Assert
	require.ErrorContains(t, err, "whose version is less than primary branch")
}

func Test_ShouldFailWhenOriginMainAndOriginMasterBothExistWithoutExplicitPrimaryBranch(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	primaryHash := commitMultiple(t, worktree, "Initial commit")

	err := repository.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/remotes/origin/main"), primaryHash))
	require.NoError(t, err)
	err = repository.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/remotes/origin/master"), primaryHash))
	require.NoError(t, err)

	err = worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/a-branch"),
	})
	require.NoError(t, err)

	commitMultiple(t, worktree, "(+semver: patch) branch commit\n")

	err = repository.Storer.RemoveReference(plumbing.ReferenceName("refs/heads/master"))
	require.NoError(t, err)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	_, err = igit.GetCurrentVersion(repository, settings, branchSettings, false)

	// Assert
	require.ErrorContains(t, err, "found both 'main' and 'master' branches")
}

func Test_ShouldFailWhenLocalMasterAndOriginMainBothExistWithoutExplicitPrimaryBranch(t *testing.T) {
	// Arrange: a local master and a remote main are both present. Ambiguity is
	// name-based across local and remote, so this must not silently prefer local.
	repository, worktree := initRepository(t)

	primaryHash := commitMultiple(t, worktree, "Initial commit")

	err := repository.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/remotes/origin/main"), primaryHash))
	require.NoError(t, err)

	err = worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/a-branch"),
	})
	require.NoError(t, err)

	commitMultiple(t, worktree, "(+semver: patch) branch commit\n")

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	_, err = igit.GetCurrentVersion(repository, settings, branchSettings, false)

	// Assert
	require.ErrorContains(t, err, "found both 'main' and 'master' branches")
}

func Test_ShouldCalculateVersionFromCommitsInBranch(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	commitMultiple(t, worktree, "Initial commit")

	err := worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/a-branch"),
	})
	require.NoError(t, err)

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
	require.NoError(t, err)

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
	require.NoError(t, err)

	branchHash := commitMultiple(t, worktree,
		"(+semver: major)\n",
		"(+semver: minor)\n",
		"(+semver: patch)\n",
		"some text\n",
	)

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/master"),
	})
	require.NoError(t, err)

	masterHash, err = commitMerge(t, worktree, "merged a-branch\n", masterHash, branchHash)
	require.NoError(t, err)

	err = worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/another-branch"),
	})
	require.NoError(t, err)

	branchHash = commitMultiple(t, worktree,
		"(+semver: minor)\n",
		"(+semver: patch)\n",
	)

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/master"),
	})
	require.NoError(t, err)

	masterHash, err = commitMerge(t, worktree, "merged another-branch\n", masterHash, branchHash)
	require.NoError(t, err)

	err = worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/yet-another-branch"),
	})
	require.NoError(t, err)

	branchHash = commitMultiple(t, worktree,
		"(+semver: patch)\n",
		"(+semver: patch)\n",
		"(+semver: patch)\n",
	)

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/master"),
	})
	require.NoError(t, err)

	_, err = commitMerge(t, worktree, "merged yet-another-branch\n", masterHash, branchHash)
	require.NoError(t, err)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	require.NoError(t, err)

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
	require.NoError(t, err)

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
	require.NoError(t, err)

	// Assert
	assert.Equal(t, "1.3.1", version)
}

func Test_ShouldFailToCalculateVersionFromImproperlyNamedLightweightTag(t *testing.T) { // TODO: Submit a PR that allows this condition to exist; ignore unparsable tags
	// Arrange
	repository, worktree := initRepository(t)

	hash := commitMultiple(t, worktree, "Initial commit")

	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/an-arbitrary-tag-name"), hash)
	err := repository.Storer.SetReference(ref)
	require.NoError(t, err)

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	_, err = igit.GetCurrentVersion(repository, settings, branchSettings, false)

	// Assert
	require.Error(t, err)
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
	require.NoError(t, err)

	target, err := repository.Storer.SetEncodedObject(tagObj)
	require.NoError(t, err)

	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/5.6.7"), target)
	err = repository.Storer.SetReference(ref)
	require.NoError(t, err)

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
	require.NoError(t, err)

	// Assert
	assert.Equal(t, "5.7.1", version)
}

func Test_ShouldCalculateVersionFromTravisTag(t *testing.T) {
	// Arrange
	repository, worktree := initRepository(t)

	commitMultiple(t, worktree, "Initial commit")

	settings := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{}
	t.Setenv("TRAVIS_TAG", "v1.2.3")

	// Act
	version, err := igit.GetCurrentVersion(repository, settings, branchSettings, false)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, "1.2.3", version)
}

func initRepository(t *testing.T) (*git.Repository, *git.Worktree) {
	t.Helper()

	fs := memfs.New()
	storage := memory.NewStorage()

	repository, err := git.Init(storage, fs)
	require.NoError(t, err)

	worktree, err := repository.Worktree()
	require.NoError(t, err)

	return repository, worktree
}

func commitMultiple(t *testing.T, worktree *git.Worktree, messages ...string) plumbing.Hash {
	t.Helper()

	var hash plumbing.Hash
	var err error
	for i, msg := range messages {
		fileName := fmt.Sprintf("commit-%d-%d.txt", time.Now().UnixNano(), i)
		err = util.WriteFile(worktree.Filesystem, fileName, []byte(msg), 0644)
		require.NoError(t, err)

		_, err = worktree.Add(fileName)
		require.NoError(t, err)

		hash, err = worktree.Commit(msg, &git.CommitOptions{Author: defaultSignature()})
		require.NoError(t, err)
	}

	return hash
}

func commitMerge(t *testing.T, worktree *git.Worktree, message string, parents ...plumbing.Hash) (plumbing.Hash, error) {
	t.Helper()

	fileName := fmt.Sprintf("merge-%d.txt", time.Now().UnixNano())
	err := util.WriteFile(worktree.Filesystem, fileName, []byte(message), 0644)
	require.NoError(t, err)

	_, err = worktree.Add(fileName)
	require.NoError(t, err)

	return worktree.Commit(message, &git.CommitOptions{
		Author:  defaultSignature(),
		Parents: parents,
	})
}

func repositoryWithBranchBehindPrimary(t *testing.T) *git.Repository {
	t.Helper()

	repository, worktree := initRepository(t)

	initialHash := commitMultiple(t, worktree, "Initial commit")
	err := repository.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/v0.0.1"), initialHash))
	require.NoError(t, err)

	err = worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.ReferenceName("refs/heads/a-branch"),
	})
	require.NoError(t, err)
	commitMultiple(t, worktree, "branch work\n")

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/master"),
	})
	require.NoError(t, err)
	commitMultiple(t, worktree, "(+semver: minor) primary branch work\n")

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/a-branch"),
	})
	require.NoError(t, err)

	return repository
}

func TestTrimBranchPrefix(t *testing.T) {
	r := getSingleBranchCommit("feature/should-be-trimmed", t)
	s := igit.GetDefaultSettings()
	label, err := igit.GetPrereleaseLabel(r, s, &igit.BranchSettings{
		IgnoreEnvVars:    true,
		TrimBranchPrefix: true,
	})
	require.NoError(t, err)

	assert.Equal(t, "should-be-trimmed", label)
}

func Test_ShouldResolveMainAsPrimaryBranchForLabelSuppression(t *testing.T) {
	r := getSingleBranchCommit("main", t)
	err := r.Storer.RemoveReference(plumbing.ReferenceName("refs/heads/master"))
	require.NoError(t, err)

	s := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	label, err := igit.GetPrereleaseLabel(r, s, branchSettings)
	require.NoError(t, err)

	primaryBranch, err := igit.GetPrimaryBranchName(r, branchSettings)
	require.NoError(t, err)
	if label == primaryBranch {
		label = ""
	}

	assert.Equal(t, "", label)
}

func Test_LabelSuppressionKeepsBranchLabelWhenPrimaryBranchMissing(t *testing.T) {
	// Arrange: a lone feature branch with no main/master to act as a baseline.
	r := getSingleBranchCommit("feature", t)
	err := r.Storer.RemoveReference(plumbing.ReferenceName("refs/heads/master"))
	require.NoError(t, err)

	s := igit.GetDefaultSettings()
	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	label, err := igit.GetPrereleaseLabel(r, s, branchSettings)
	require.NoError(t, err)

	// Act: this mirrors the prerelease command's suppression logic.
	primaryBranch, err := igit.GetPrimaryBranchName(r, branchSettings)

	// Assert: a missing primary branch is an ordinary (non-ambiguous) error, so
	// the command continues and the branch label is preserved rather than blanked.
	require.Error(t, err)
	require.False(t, igit.IsAmbiguousPrimaryBranchError(err))
	if err == nil && label == primaryBranch {
		label = ""
	}
	assert.Equal(t, "feature", label)
}

func Test_LabelSuppressionDetectsAmbiguousPrimaryBranch(t *testing.T) {
	// Arrange: both main and master exist, so the baseline is ambiguous.
	r := getSingleBranchCommit("main", t)

	branchSettings := &igit.BranchSettings{
		IgnoreEnvVars: true,
	}

	// Act
	_, err := igit.GetPrimaryBranchName(r, branchSettings)

	// Assert: the prerelease command treats this as fatal (it panics) rather than
	// silently suppressing the label.
	require.Error(t, err)
	assert.True(t, igit.IsAmbiguousPrimaryBranchError(err))
}

func Test_IsAmbiguousPrimaryBranchErrorUnwrapsWrappedErrors(t *testing.T) {
	r := getSingleBranchCommit("main", t)

	_, err := igit.GetPrimaryBranchName(r, &igit.BranchSettings{IgnoreEnvVars: true})
	require.Error(t, err)

	wrapped := fmt.Errorf("getVersion failed: %w", err)
	assert.True(t, igit.IsAmbiguousPrimaryBranchError(wrapped))
}

func TestCleanseBranchName(t *testing.T) {
	r := getSingleBranchCommit("author's-branch", t)
	s := igit.GetDefaultSettings()
	label, err := igit.GetPrereleaseLabel(r, s, &igit.BranchSettings{
		IgnoreEnvVars: true,
	})
	require.NoError(t, err)

	assert.Equal(t, "author-s-branch", label)
}

func getSingleBranchCommit(branchName string, t *testing.T) *git.Repository {
	t.Helper()

	fs := memfs.New()
	storage := memory.NewStorage()

	r, err := git.Init(storage, fs)
	require.NoError(t, err)

	w, err := r.Worktree()
	require.NoError(t, err)

	err = util.WriteFile(fs, "foo", []byte("foo"), 0644)
	require.NoError(t, err)
	_, err = w.Add("foo")
	require.NoError(t, err)
	_, err = w.Commit("foo\n", &git.CommitOptions{Author: defaultSignature()})
	require.NoError(t, err)

	ref := fmt.Sprintf("refs/heads/%s", branchName)
	b := plumbing.ReferenceName(ref)
	err = w.Checkout(&git.CheckoutOptions{
		Create: true,
		Force:  false,
		Branch: b,
	})
	require.NoError(t, err)

	err = util.WriteFile(fs, "foo2", []byte("foo"), 0644)
	require.NoError(t, err)
	_, err = w.Add("foo2")
	require.NoError(t, err)
	_, err = w.Commit("(+semver: major) This is a major commit\n", &git.CommitOptions{Author: defaultSignature()})
	require.NoError(t, err)

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
