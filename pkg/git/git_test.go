package git_test

import (
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-billy.v4/util"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/memory"

	igit "github.com/annymsmthd/gogitver/pkg/git"
)

func TestUseLightweightTagForVersionAnchor(t *testing.T) {
	fs := memfs.New()
	storage := memory.NewStorage()

	r, err := git.Init(storage, fs)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	w, err := r.Worktree()
	util.WriteFile(fs, "foo", []byte("foo"), 0644)

	_, err = w.Add("foo")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	hash, err := w.Commit("foo\n", &git.CommitOptions{Author: defaultSignature()})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	util.WriteFile(fs, "foo2", []byte("foo"), 0644)

	_, err = w.Add("foo2")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	hash, err = w.Commit("(+semver: major) This is a major commit\n", &git.CommitOptions{Author: defaultSignature()})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	_, err = createTag(r, hash, "1.5.0", false)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	s := igit.GetDefaultSettings()
	version, err := igit.GetCurrentVersion(r, s, true, false, false)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	assert.Equal(t, "1.5.0", version)
}

func TestUseAnnotatedTagForVersionAnchor(t *testing.T) {
	fs := memfs.New()
	storage := memory.NewStorage()

	r, err := git.Init(storage, fs)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	w, err := r.Worktree()
	util.WriteFile(fs, "foo", []byte("foo"), 0644)

	_, err = w.Add("foo")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	hash, err := w.Commit("foo\n", &git.CommitOptions{Author: defaultSignature()})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	util.WriteFile(fs, "foo2", []byte("foo"), 0644)

	_, err = w.Add("foo2")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	hash, err = w.Commit("(+semver: major) This is a major commit\n", &git.CommitOptions{Author: defaultSignature()})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	_, err = createTag(r, hash, "1.5.0", true)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	s := igit.GetDefaultSettings()
	version, err := igit.GetCurrentVersion(r, s, true, false, false)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	assert.Equal(t, "1.5.0", version)
}

func TestTrimBranchPrefix(t *testing.T) {
	r := getSingleBranchCommit("feature/should-be-trimmed", t)
	s := igit.GetDefaultSettings()
	label, err := igit.GetPrereleaseLabel(r, s, true)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	assert.Equal(t, "should-be-trimmed", label)
}

func TestCleanseBranchName(t *testing.T) {
	r := getSingleBranchCommit("author's-branch", t)
	s := igit.GetDefaultSettings()
	label, err := igit.GetPrereleaseLabel(r, s, true)
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
		Force: false,
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

// this method will be available in go-git after 4.7.0
func createTag(r *git.Repository, h plumbing.Hash, name string, annotated bool) (plumbing.Hash, error) {
	rawobj, err := object.GetObject(r.Storer, h)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	rname := plumbing.ReferenceName(path.Join("refs", "tags", name))

	var target plumbing.Hash
	if annotated {
		tag := object.Tag{
			Name:       name,
			Message:    "",
			TargetType: rawobj.Type(),
			Target:     h,
		}
		obj := r.Storer.NewEncodedObject()
		if err := tag.Encode(obj); err != nil {
			return plumbing.ZeroHash, err
		}

		target, err = r.Storer.SetEncodedObject(obj)
		if err := tag.Encode(obj); err != nil {
			return plumbing.ZeroHash, err
		}
	} else {
		target = h
	}

	ref := plumbing.NewHashReference(rname, target)
	if err = r.Storer.SetReference(ref); err != nil {
		return plumbing.ZeroHash, err
	}

	return ref.Hash(), nil
}
