package marketplace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

func TestCloneOrPullWithOptions_ExistingRepoWithBranchRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: go-git file transport depends on system git version compatibility")
	}
	srcDir := t.TempDir()
	srcRepo, err := git.PlainInit(srcDir, false)
	if err != nil {
		t.Fatal(err)
	}
	srcWt, _ := srcRepo.Worktree()
	os.WriteFile(filepath.Join(srcDir, "f1.txt"), []byte("v1"), 0644)
	srcWt.Add("f1.txt")
	srcWt.Commit("v1", &git.CommitOptions{})

	client := NewGitClient()
	client.timeout = 10 * time.Second

	cloneDir := t.TempDir()
	err = client.CloneOrPullWithOptions(srcDir, cloneDir, CloneOptions{Ref: "master"})
	if err != nil {
		t.Fatalf("first clone with ref: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cloneDir, "f1.txt"))
	if err != nil || string(data) != "v1" {
		t.Fatalf("after first clone: %q, want v1", string(data))
	}

	os.WriteFile(filepath.Join(srcDir, "f2.txt"), []byte("v2"), 0644)
	srcWt.Add("f2.txt")
	srcWt.Commit("v2", &git.CommitOptions{})

	err = client.CloneOrPullWithOptions(srcDir, cloneDir, CloneOptions{Ref: "master"})
	if err != nil {
		t.Fatalf("update existing clone with branch ref: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cloneDir, "f2.txt")); err != nil {
		t.Errorf("f2.txt not present after update: %v", err)
	}
}

func TestCloneOrPullWithOptions_ExistingRepoWithTagRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: go-git file transport depends on system git version compatibility")
	}
	srcDir := t.TempDir()
	srcRepo, err := git.PlainInit(srcDir, false)
	if err != nil {
		t.Fatal(err)
	}
	srcWt, _ := srcRepo.Worktree()
	os.WriteFile(filepath.Join(srcDir, "f1.txt"), []byte("v1"), 0644)
	srcWt.Add("f1.txt")
	commitV1, _ := srcWt.Commit("v1", &git.CommitOptions{})

	srcRepo.CreateTag("v1.0.0", commitV1, nil)

	os.WriteFile(filepath.Join(srcDir, "f2.txt"), []byte("v2"), 0644)
	srcWt.Add("f2.txt")
	srcWt.Commit("v2", &git.CommitOptions{})

	client := NewGitClient()
	client.timeout = 10 * time.Second

	cloneDir := t.TempDir()
	err = client.CloneOrPullWithOptions(srcDir, cloneDir, CloneOptions{Ref: "v1.0.0"})
	if err != nil {
		t.Fatalf("clone with tag ref: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cloneDir, "f1.txt"))
	if err != nil || string(data) != "v1" {
		t.Fatalf("after tag clone: %q, want v1", string(data))
	}

	if _, err := os.Stat(filepath.Join(cloneDir, "f2.txt")); err == nil {
		t.Error("f2.txt should not exist when checked out at tag v1.0.0")
	}
}

func TestFetchRef_TagOnlyRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: go-git file transport depends on system git version compatibility")
	}
	srcDir := t.TempDir()
	srcRepo, _ := git.PlainInit(srcDir, false)
	srcWt, _ := srcRepo.Worktree()
	os.WriteFile(filepath.Join(srcDir, "f.txt"), []byte("tag"), 0644)
	srcWt.Add("f.txt")
	commit, _ := srcWt.Commit("tagged", &git.CommitOptions{})
	srcRepo.CreateTag("release", commit, nil)

	cloneDir := t.TempDir()
	git.PlainClone(cloneDir, false, &git.CloneOptions{URL: srcDir})

	cloneRepo, _ := git.PlainOpen(cloneDir)
	remote, _ := cloneRepo.Remote("origin")

	client := &GitClient{timeout: 10 * time.Second}
	err := client.fetchRef(cloneRepo, remote.Config().URLs[0], "release")
	if err != nil {
		t.Fatalf("fetchRef for tag-only ref 'release': %v", err)
	}

	err = client.checkoutRemoteRef(cloneDir, "release")
	if err != nil {
		t.Fatalf("checkoutRemoteRef for 'release': %v", err)
	}
}

func TestFetchRef_NonexistentRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: go-git file transport depends on system git version compatibility")
	}
	srcDir := t.TempDir()
	srcRepo, _ := git.PlainInit(srcDir, false)
	srcWt, _ := srcRepo.Worktree()
	os.WriteFile(filepath.Join(srcDir, "f.txt"), []byte("x"), 0644)
	srcWt.Add("f.txt")
	srcWt.Commit("init", &git.CommitOptions{})

	_, err := srcRepo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{srcDir}})
	if err != nil {
		t.Fatal(err)
	}

	err = (&GitClient{timeout: 10 * time.Second}).fetchRef(srcRepo, srcDir, "nonexistent-branch")
	if err == nil {
		t.Error("expected error for nonexistent ref")
	}
}

func TestCloneOrPullWithOptions_DirtyWorkTreeReset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: go-git file transport depends on system git version compatibility")
	}

	srcDir := t.TempDir()
	srcRepo, err := git.PlainInit(srcDir, false)
	if err != nil {
		t.Fatal(err)
	}
	srcWt, _ := srcRepo.Worktree()
	os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("original"), 0644)
	srcWt.Add("data.txt")
	srcWt.Commit("initial", &git.CommitOptions{})

	client := NewGitClient()
	client.timeout = 10 * time.Second

	cloneDir := t.TempDir()
	if err := client.CloneOrPullWithOptions(srcDir, cloneDir, CloneOptions{}); err != nil {
		t.Fatalf("initial clone: %v", err)
	}

	os.WriteFile(filepath.Join(cloneDir, "data.txt"), []byte("dirty local change"), 0644)

	os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("updated upstream"), 0644)
	srcWt.Add("data.txt")
	srcWt.Commit("update data.txt", &git.CommitOptions{})

	if err := client.CloneOrPullWithOptions(srcDir, cloneDir, CloneOptions{}); err != nil {
		t.Fatalf("pull with dirty tree: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cloneDir, "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "updated upstream" {
		t.Errorf("after pull, data.txt = %q, want %q", string(data), "updated upstream")
	}
}

func TestPull_DirtyWorkTreeReset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: go-git file transport depends on system git version compatibility")
	}

	srcDir := t.TempDir()
	srcRepo, err := git.PlainInit(srcDir, false)
	if err != nil {
		t.Fatal(err)
	}
	srcWt, _ := srcRepo.Worktree()
	os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("original"), 0644)
	srcWt.Add("data.txt")
	srcWt.Commit("initial", &git.CommitOptions{})

	client := NewGitClient()
	client.timeout = 10 * time.Second

	cloneDir := t.TempDir()
	if err := client.CloneOrPullWithOptions(srcDir, cloneDir, CloneOptions{}); err != nil {
		t.Fatalf("initial clone: %v", err)
	}

	os.WriteFile(filepath.Join(cloneDir, "data.txt"), []byte("dirty local change"), 0644)

	os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("updated upstream"), 0644)
	srcWt.Add("data.txt")
	srcWt.Commit("update data.txt", &git.CommitOptions{})

	if err := client.Pull(cloneDir); err != nil {
		t.Fatalf("Pull with dirty tree: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cloneDir, "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "updated upstream" {
		t.Errorf("after Pull, data.txt = %q, want %q", string(data), "updated upstream")
	}
}

func TestCheckout_DirtyWorkTree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: go-git file transport depends on system git version compatibility")
	}

	srcDir := t.TempDir()
	srcRepo, err := git.PlainInit(srcDir, false)
	if err != nil {
		t.Fatal(err)
	}
	srcWt, _ := srcRepo.Worktree()
	os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("v1"), 0644)
	srcWt.Add("data.txt")
	commitV1, _ := srcWt.Commit("v1", &git.CommitOptions{})
	srcRepo.CreateTag("v1", commitV1, nil)

	os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("v2"), 0644)
	srcWt.Add("data.txt")
	srcWt.Commit("v2", &git.CommitOptions{})

	client := NewGitClient()
	client.timeout = 10 * time.Second

	cloneDir := t.TempDir()
	if err := client.CloneOrPullWithOptions(srcDir, cloneDir, CloneOptions{}); err != nil {
		t.Fatalf("initial clone: %v", err)
	}

	os.WriteFile(filepath.Join(cloneDir, "data.txt"), []byte("dirty"), 0644)

	if err := client.Checkout(cloneDir, "v1"); err != nil {
		t.Fatalf("Checkout with dirty tree: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cloneDir, "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v1" {
		t.Errorf("after Checkout, data.txt = %q, want %q", string(data), "v1")
	}
}

func TestCloneOrPullWithOptions_DirtyWorkTreeWithRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: go-git file transport depends on system git version compatibility")
	}

	srcDir := t.TempDir()
	srcRepo, err := git.PlainInit(srcDir, false)
	if err != nil {
		t.Fatal(err)
	}
	srcWt, _ := srcRepo.Worktree()
	os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("v1"), 0644)
	srcWt.Add("data.txt")
	commitV1, _ := srcWt.Commit("v1", &git.CommitOptions{})
	srcRepo.CreateTag("v1", commitV1, nil)

	os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("v2"), 0644)
	srcWt.Add("data.txt")
	srcWt.Commit("v2", &git.CommitOptions{})

	client := NewGitClient()
	client.timeout = 10 * time.Second

	cloneDir := t.TempDir()
	if err := client.CloneOrPullWithOptions(srcDir, cloneDir, CloneOptions{Ref: "master"}); err != nil {
		t.Fatalf("initial clone: %v", err)
	}

	os.WriteFile(filepath.Join(cloneDir, "data.txt"), []byte("dirty"), 0644)

	if err := client.CloneOrPullWithOptions(srcDir, cloneDir, CloneOptions{Ref: "v1"}); err != nil {
		t.Fatalf("update with ref and dirty tree: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cloneDir, "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v1" {
		t.Errorf("after ref checkout with dirty tree, data.txt = %q, want %q", string(data), "v1")
	}
}
