package marketplace

import (
	"fmt"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type GitClient struct {
	timeout time.Duration
}

func NewGitClient() *GitClient {
	return &GitClient{
		timeout: 60 * time.Second,
	}
}

func (g *GitClient) Clone(url, path string) error {
	_, err := git.PlainClone(path, false, &git.CloneOptions{
		URL:               url,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})

	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	return nil
}

func (g *GitClient) GetLatestCommitSHA(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return ref.Hash().String(), nil
}

func (g *GitClient) Checkout(repoPath, ref string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return fmt.Errorf("failed to resolve revision %s: %w", ref, err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})
	if err != nil {
		return fmt.Errorf("failed to checkout %s: %w", ref, err)
	}

	return nil
}

func (g *GitClient) Pull(repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	err = worktree.Pull(&git.PullOptions{
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull: %w", err)
	}

	return nil
}

func (g *GitClient) IsGitRepo(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

func (g *GitClient) CloneOrPull(url, path string) error {
	if g.IsGitRepo(path) {
		return g.Pull(path)
	}
	return g.Clone(url, path)
}

func (g *GitClient) FetchSubDir(url, subPath, ref, targetPath string) error {
	tempDir, err := os.MkdirTemp("", "opencode-plugin-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := g.Clone(url, tempDir); err != nil {
		return err
	}

	if ref != "" {
		if err := g.Checkout(tempDir, ref); err != nil {
			return err
		}
	}

	srcPath := fmt.Sprintf("%s/%s", tempDir, subPath)
	if err := copyDir(srcPath, targetPath); err != nil {
		return fmt.Errorf("failed to copy subdirectory: %w", err)
	}

	return nil
}

func copyDir(src, dst string) error {
	return os.Rename(src, dst)
}
