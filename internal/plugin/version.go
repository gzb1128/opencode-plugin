package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/opencode/plugin-cli/internal/pathutil"
)

type PluginResolutionContext struct {
	MarketPath string
	PluginRoot string
}

type VersionResolver struct {
	gitClient *GitClient
}

type GitClient struct{}

func NewVersionResolver() *VersionResolver {
	return &VersionResolver{
		gitClient: &GitClient{},
	}
}

func (v *VersionResolver) Resolve(pluginPath string, requested string) (string, error) {
	if requested != "" && requested != "latest" {
		return requested, nil
	}

	pluginJSONPath := filepath.Join(pluginPath, ".claude-plugin", "plugin.json")
	version, err := v.readPluginJSONVersion(pluginJSONPath)
	if err == nil && version != "" && version != "latest" {
		return version, nil
	}

	sha, err := v.gitClient.GetCommitSHA(pluginPath)
	if err == nil && sha != "" {
		if len(sha) > 12 {
			return sha[:12], nil
		}
		return sha, nil
	}

	return "latest", nil
}

func (v *VersionResolver) readPluginJSONVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read plugin.json: %w", err)
	}

	var pluginData struct {
		Version string `json:"version"`
	}

	if err := json.Unmarshal(data, &pluginData); err != nil {
		return "", fmt.Errorf("failed to parse plugin.json: %w", err)
	}

	return pluginData.Version, nil
}

func (g *GitClient) GetCommitSHA(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", err
	}

	ref, err := repo.Head()
	if err != nil {
		return "", err
	}

	return ref.Hash().String(), nil
}

func (v *VersionResolver) GetPluginSourcePath(plugin *marketplace.Plugin, marketPath string) (string, error) {
	ctx := PluginResolutionContext{MarketPath: marketPath, PluginRoot: ""}
	return v.GetPluginSourcePathWithCtx(plugin, ctx)
}

func (v *VersionResolver) GetPluginSourcePathWithCtx(plugin *marketplace.Plugin, ctx PluginResolutionContext) (string, error) {
	src, ok := plugin.Source.(marketplace.PluginSource)
	if !ok {
		return "", fmt.Errorf("invalid plugin source format")
	}

	switch s := src.(type) {
	case *marketplace.LocalSource:
		base := ctx.MarketPath
		if ctx.PluginRoot != "" {
			base = filepath.Join(base, ctx.PluginRoot)
		}
		resolved := filepath.Join(base, s.Path)
		if ctx.MarketPath != "" {
			validated, err := pathutil.ResolvePathWithinBase(ctx.MarketPath, filepath.Join(func() string {
				if ctx.PluginRoot != "" {
					return ctx.PluginRoot + string(filepath.Separator) + s.Path
				}
				return s.Path
			}()))
			if err != nil {
				return "", fmt.Errorf("plugin source path escapes marketplace root: %w", err)
			}
			return validated, nil
		}
		return resolved, nil
	default:
		return "", fmt.Errorf("unsupported plugin source type: %s (plugin may need to be cloned first)", src.SourceType())
	}
}

func (v *VersionResolver) IsRemoteSource(plugin *marketplace.Plugin) bool {
	src, ok := plugin.Source.(marketplace.PluginSource)
	if !ok {
		return false
	}

	switch src.(type) {
	case *marketplace.GitHubSource, *marketplace.URLSource, *marketplace.GitSource, *marketplace.GitSubdirSource, *marketplace.NpmSource, *marketplace.PipSource:
		return true
	default:
		return false
	}
}

func (v *VersionResolver) CloneRemotePlugin(plugin *marketplace.Plugin, cachePath string) error {
	src, ok := plugin.Source.(marketplace.PluginSource)
	if !ok {
		return fmt.Errorf("invalid plugin source format")
	}
	return v.clonePluginSource(src, cachePath)
}

func (v *VersionResolver) clonePluginSource(src marketplace.PluginSource, cachePath string) error {
	switch s := src.(type) {
	case *marketplace.GitHubSource:
		gitURL := "https://github.com/" + s.Repo + ".git"
		return v.cloneGitSource(gitURL, s.Ref, s.SHA, cachePath)
	case *marketplace.URLSource:
		return v.cloneGitSource(s.URL, s.Ref, s.SHA, cachePath)
	case *marketplace.GitSource:
		return v.cloneGitSource(s.URL, s.Ref, s.SHA, cachePath)
	case *marketplace.GitSubdirSource:
		return v.cloneGitSubdirSource(s, cachePath)
	case *marketplace.NpmSource:
		return fmt.Errorf("npm source installation not yet implemented for package: %s", s.Package)
	case *marketplace.PipSource:
		return fmt.Errorf("pip source installation not yet implemented for package: %s", s.Package)
	default:
		return fmt.Errorf("source type '%s' is not a remote source", src.SourceType())
	}
}

func (v *VersionResolver) cloneGitSource(gitURL, ref, sha, cachePath string) error {
	if _, err := os.Stat(cachePath); err == nil {
		os.RemoveAll(cachePath)
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	_, err := git.PlainClone(cachePath, false, &git.CloneOptions{
		URL:               gitURL,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if sha != "" {
		if err := checkoutSHA(cachePath, sha); err != nil {
			return err
		}
	} else if ref != "" {
		if err := checkoutRef(cachePath, ref); err != nil {
			return err
		}
	}

	return nil
}

func (v *VersionResolver) cloneGitSubdirSource(src *marketplace.GitSubdirSource, cachePath string) error {
	gitURL := src.URL

	if _, err := os.Stat(cachePath); err == nil {
		os.RemoveAll(cachePath)
	}

	tempDir, err := os.MkdirTemp("", "opencode-plugin-clone-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	_, err = git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:               gitURL,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if src.SHA != "" {
		if err := checkoutSHA(tempDir, src.SHA); err != nil {
			return err
		}
	} else if src.Ref != "" {
		if err := checkoutRef(tempDir, src.Ref); err != nil {
			return err
		}
	}

	srcDir := filepath.Join(tempDir, src.SubPath)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("subdirectory '%s' not found in repository", src.SubPath)
	}

	if err := copyRecursive(srcDir, cachePath); err != nil {
		return fmt.Errorf("failed to copy plugin files: %w", err)
	}

	return nil
}

func checkoutSHA(repoPath, sha string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(sha))
	if err != nil {
		return fmt.Errorf("failed to resolve SHA %s: %w", sha, err)
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
		return fmt.Errorf("failed to checkout SHA %s: %w", sha, err)
	}
	return nil
}

func checkoutRef(repoPath, ref string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	branchErr := worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(ref)})
	if branchErr == nil {
		return nil
	}

	tagErr := worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewTagReferenceName(ref)})
	if tagErr == nil {
		return nil
	}

	return fmt.Errorf("failed to checkout ref %s: not found as branch (%v) or tag (%v)", ref, branchErr, tagErr)
}

func copyRecursive(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyRecursive(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

func (v *VersionResolver) GetAvailableVersions(pluginPath string) ([]string, error) {
	versions := []string{}

	pluginJSONPath := filepath.Join(pluginPath, ".claude-plugin", "plugin.json")
	version, err := v.readPluginJSONVersion(pluginJSONPath)
	if err == nil && version != "" {
		versions = append(versions, version)
	}

	repo, err := git.PlainOpen(pluginPath)
	if err == nil {
		tags, err := repo.Tags()
		if err == nil {
			tags.ForEach(func(ref *plumbing.Reference) error {
				tagName := ref.Name().Short()
				if strings.HasPrefix(tagName, "v") || strings.HasPrefix(tagName, "0") || strings.HasPrefix(tagName, "1") || strings.HasPrefix(tagName, "2") {
					versions = append(versions, tagName)
				}
				return nil
			})
		}
	}

	versions = append(versions, "latest")

	return versions, nil
}
