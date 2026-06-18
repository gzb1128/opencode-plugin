package marketplace

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

type CloneOptions struct {
	Ref string
}

type GitClient struct {
	timeout time.Duration
}

func NewGitClient() *GitClient {
	return &GitClient{
		timeout: 60 * time.Second,
	}
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
		Hash:  *hash,
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("failed to checkout %s: %w", ref, err)
	}

	return nil
}

func (g *GitClient) checkoutRemoteRef(repoPath, ref string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	candidates := []plumbing.Revision{
		plumbing.Revision(fmt.Sprintf("refs/remotes/origin/%s", ref)),
		plumbing.Revision(fmt.Sprintf("refs/tags/%s", ref)),
		plumbing.Revision(ref),
	}

	var lastErr error
	for _, rev := range candidates {
		hash, err := repo.ResolveRevision(rev)
		if err != nil {
			lastErr = err
			continue
		}
		if err := worktree.Checkout(&git.CheckoutOptions{Hash: *hash, Force: true}); err != nil {
			return fmt.Errorf("failed to checkout %s: %w", ref, err)
		}
		return nil
	}

	return fmt.Errorf("failed to resolve ref %s: %w", ref, lastErr)
}

func (g *GitClient) IsGitRepo(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

func (g *GitClient) CloneOrPullWithOptions(url, path string, opts CloneOptions) error {
	if !g.IsGitRepo(path) {
		cloneOpts := &git.CloneOptions{
			URL:               url,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		}
		if opts.Ref != "" {
			refName := opts.Ref
			if !strings.HasPrefix(refName, "refs/") {
				refName = plumbing.NewBranchReferenceName(refName).String()
			}
			cloneOpts.ReferenceName = plumbing.ReferenceName(refName)
		}

		_, err := git.PlainClone(path, false, cloneOpts)
		if err != nil {
			if opts.Ref != "" {
				// 第一次 PlainClone 可能在 path 下残留 .git/，必须先清空再重试，
				// 否则 go-git 要求目标目录为空，第二次 clone 也会失败。
				if rmErr := os.RemoveAll(path); rmErr != nil {
					return fmt.Errorf("failed to clone repository: %w (cleanup of partial clone also failed: %v)", err, rmErr)
				}
				_, err2 := git.PlainClone(path, false, &git.CloneOptions{
					URL:               url,
					RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
				})
				if err2 != nil {
					return fmt.Errorf("failed to clone repository: %w (retry without ref also failed: %v)", err, err2)
				}
				if checkoutErr := g.Checkout(path, opts.Ref); checkoutErr != nil {
					return fmt.Errorf("cloned but failed to checkout ref %s: %w", opts.Ref, checkoutErr)
				}
				return nil
			}
			return fmt.Errorf("failed to clone repository: %w", err)
		}
		return nil
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	if opts.Ref != "" {
		wt, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("failed to get worktree: %w", err)
		}
		if err := wt.Reset(&git.ResetOptions{Mode: git.HardReset}); err != nil {
			return fmt.Errorf("failed to reset worktree before fetch: %w", err)
		}
		if err := g.fetchRef(repo, url, opts.Ref); err != nil {
			return fmt.Errorf("failed to fetch ref %s: %w", opts.Ref, err)
		}
		return g.checkoutRemoteRef(path, opts.Ref)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if err := worktree.Reset(&git.ResetOptions{Mode: git.HardReset}); err != nil {
		return fmt.Errorf("failed to reset worktree before pull: %w", err)
	}

	err = worktree.Pull(&git.PullOptions{
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull: %w", err)
	}

	return nil
}

func (g *GitClient) fetchRef(repo *git.Repository, remoteURL, ref string) error {
	remotes, err := repo.Remotes()
	if err != nil {
		return err
	}

	var remote *git.Remote
	for _, r := range remotes {
		if r.Config().Name == "origin" {
			remote = r
			break
		}
	}
	if remote == nil && len(remotes) > 0 {
		remote = remotes[0]
	}
	if remote == nil {
		return fmt.Errorf("no remote found in repository")
	}

	refSpecs := []config.RefSpec{
		config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", ref, ref)),
		config.RefSpec(fmt.Sprintf("+refs/tags/%s:refs/tags/%s", ref, ref)),
	}

	var fetched bool
	for _, rs := range refSpecs {
		err = remote.Fetch(&git.FetchOptions{
			RefSpecs: []config.RefSpec{rs},
		})
		if err == nil || err == git.NoErrAlreadyUpToDate {
			fetched = true
		}
	}
	if !fetched {
		return fmt.Errorf("ref %s not found as branch or tag: %w", ref, err)
	}
	return nil
}
