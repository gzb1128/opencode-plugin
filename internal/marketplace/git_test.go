package marketplace

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/opencode/plugin-cli/internal/gitutil"
)

func TestRunOnce_SingleAttempt(t *testing.T) {
	client := NewGitClient()
	attempts := 0
	err := client.git.RunOnce(func() error {
		attempts++
		return fmt.Errorf("connection reset by peer")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (RunOnce must not retry)", attempts)
	}
}

func TestCloneOrPullWithOptions_UsesGitHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("not a git server"))
	}))
	t.Cleanup(server.Close)

	client := &GitClient{git: gitutil.NewClient(gitutil.Options{Timeout: 20 * time.Millisecond, Attempts: 1})}

	started := time.Now()
	err := client.CloneOrPullWithOptions(server.URL+"/market.git", filepath.Join(t.TempDir(), "clone"), CloneOptions{})
	if err == nil {
		t.Fatal("expected clone timeout")
	}
	if elapsed := time.Since(started); elapsed >= 150*time.Millisecond {
		t.Fatalf("CloneOrPullWithOptions took %s, want timeout before server responds", elapsed)
	}
}

func newTestGitClient(timeout time.Duration) *GitClient {
	return &GitClient{git: gitutil.NewClient(gitutil.Options{Timeout: timeout})}
}

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

	client := newTestGitClient(10 * time.Second)

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

	client := newTestGitClient(10 * time.Second)

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

	client := newTestGitClient(10 * time.Second)
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

	err = newTestGitClient(10*time.Second).fetchRef(srcRepo, srcDir, "nonexistent-branch")
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

	client := newTestGitClient(10 * time.Second)

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

	client := newTestGitClient(10 * time.Second)

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

	client := newTestGitClient(10 * time.Second)

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

func TestCloneOrPullWithOptions_FallbackCloneWithoutRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: go-git file transport depends on system git version compatibility")
	}
	srcDir := t.TempDir()
	srcRepo, err := git.PlainInit(srcDir, false)
	if err != nil {
		t.Fatal(err)
	}
	srcWt, _ := srcRepo.Worktree()
	if err := os.WriteFile(filepath.Join(srcDir, "real.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := srcWt.Add("real.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := srcWt.Commit("init", &git.CommitOptions{}); err != nil {
		t.Fatal(err)
	}

	client := newTestGitClient(10 * time.Second)
	cloneDir := t.TempDir()

	// 用不存在的 ref clone：第一次 clone-with-ref 会失败，
	// 走 fallback 路径（clone-without-ref → Checkout）。
	// Checkout 不存在的 ref 也会失败，但 clone 本身应该成功——
	// 这证明 fallback clone 确实执行了。
	err = client.CloneOrPullWithOptions(srcDir, cloneDir, CloneOptions{Ref: "nonexistent"})
	if err == nil {
		t.Fatal("expected error (checkout of nonexistent ref should fail)")
	}

	// clone 应该成功了（fallback 路径执行了），文件应该存在
	data, readErr := os.ReadFile(filepath.Join(cloneDir, "real.txt"))
	if readErr != nil {
		t.Fatalf("fallback clone should have placed files, but read failed: %v", readErr)
	}
	if string(data) != "data" {
		t.Errorf("real.txt = %q, want %q", string(data), "data")
	}
}
