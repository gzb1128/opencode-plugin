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
)

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
	// If version is specified, use it
	if requested != "" && requested != "latest" {
		return requested, nil
	}

	// Try to read plugin.json version
	pluginJSONPath := filepath.Join(pluginPath, ".claude-plugin", "plugin.json")
	version, err := v.readPluginJSONVersion(pluginJSONPath)
	if err == nil && version != "" && version != "latest" {
		return version, nil
	}

	// Try to get git SHA
	sha, err := v.gitClient.GetCommitSHA(pluginPath)
	if err == nil && sha != "" {
		// Use first 12 characters of SHA
		if len(sha) > 12 {
			return sha[:12], nil
		}
		return sha, nil
	}

	// Fallback to "latest"
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
	switch src := plugin.Source.(type) {
	case string:
		return filepath.Join(marketPath, src), nil

	case marketplace.PluginSource:
		switch src.Type {
		case string(marketplace.SourceTypeLocal):
			return filepath.Join(marketPath, src.Path), nil
		default:
			return "", fmt.Errorf("unsupported plugin source type: %s (plugin may need to be cloned first)", src.Type)
		}

	case map[string]interface{}:
		sourceType, _ := src["source"].(string)
		switch sourceType {
		case "url", "github":
			return v.getRemoteSourceURL(src)
		case "git-subdir":
			return v.getGitSubdirSource(src)
		case "":
			if path, ok := src["path"].(string); ok {
				return filepath.Join(marketPath, path), nil
			}
		}
		return "", fmt.Errorf("invalid plugin source format")

	default:
		return "", fmt.Errorf("invalid plugin source format")
	}
}

func (v *VersionResolver) getRemoteSourceURL(src map[string]interface{}) (string, error) {
	url, _ := src["url"].(string)
	if url == "" {
		repo, _ := src["repo"].(string)
		if repo != "" {
			url = "https://github.com/" + repo + ".git"
		}
	}
	if url == "" {
		return "", fmt.Errorf("plugin source has no URL or repo")
	}
	return url, nil
}

func (v *VersionResolver) getGitSubdirSource(src map[string]interface{}) (string, error) {
	url, _ := src["url"].(string)
	if url == "" {
		return "", fmt.Errorf("git-subdir source has no URL")
	}
	return url, nil
}

// IsRemoteSource checks if a plugin needs to be cloned from a remote repository
func (v *VersionResolver) IsRemoteSource(plugin *marketplace.Plugin) bool {
	switch src := plugin.Source.(type) {
	case marketplace.PluginSource:
		switch src.Type {
		case "url", "github", "git-subdir":
			return true
		}
	case map[string]interface{}:
		sourceType, _ := src["source"].(string)
		switch sourceType {
		case "url", "github", "git-subdir":
			return true
		}
	}
	return false
}

// CloneRemotePlugin clones a remote plugin source to cache directory
func (v *VersionResolver) CloneRemotePlugin(plugin *marketplace.Plugin, cachePath string) error {
	switch src := plugin.Source.(type) {
	case marketplace.PluginSource:
		return v.clonePluginSource(&src, cachePath)
	case map[string]interface{}:
		ps := toPluginSource(src)
		return v.clonePluginSource(ps, cachePath)
	default:
		return fmt.Errorf("invalid plugin source format")
	}
}

func toPluginSource(src map[string]interface{}) *marketplace.PluginSource {
	ps := &marketplace.PluginSource{Type: src["source"].(string)}
	if v, ok := src["url"].(string); ok {
		ps.URL = v
	}
	if v, ok := src["repo"].(string); ok {
		ps.Repo = v
		if ps.URL == "" {
			ps.URL = "https://github.com/" + v + ".git"
		}
	}
	if v, ok := src["path"].(string); ok {
		ps.SubPath = v
	}
	if v, ok := src["ref"].(string); ok {
		ps.Ref = v
	}
	if v, ok := src["sha"].(string); ok {
		ps.SHA = v
	}
	return ps
}

func (v *VersionResolver) clonePluginSource(src *marketplace.PluginSource, cachePath string) error {
	switch src.Type {
	case "url", "github":
		return v.cloneURLSource(src, cachePath)
	case "git-subdir":
		return v.cloneGitSubdirSource(src, cachePath)
	default:
		return fmt.Errorf("source type '%s' is not a remote source", src.Type)
	}
}

func (v *VersionResolver) cloneURLSource(src *marketplace.PluginSource, cachePath string) error {
	gitURL := src.URL
	if gitURL == "" && src.Repo != "" {
		gitURL = "https://github.com/" + src.Repo + ".git"
	}
	if gitURL == "" {
		return fmt.Errorf("plugin source has no URL or repo")
	}

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

	if src.SHA != "" {
		repo, err := git.PlainOpen(cachePath)
		if err != nil {
			return fmt.Errorf("failed to open repository: %w", err)
		}
		worktree, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("failed to get worktree: %w", err)
		}
		hash, err := repo.ResolveRevision(plumbing.Revision(src.SHA))
		if err != nil {
			return fmt.Errorf("failed to resolve SHA %s: %w", src.SHA, err)
		}
		if err := worktree.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
			return fmt.Errorf("failed to checkout SHA %s: %w", src.SHA, err)
		}
	}

	return nil
}

func (v *VersionResolver) cloneGitSubdirSource(src *marketplace.PluginSource, cachePath string) error {
	gitURL := src.URL
	if gitURL == "" && src.Repo != "" {
		gitURL = "https://github.com/" + src.Repo + ".git"
	}
	if gitURL == "" {
		return fmt.Errorf("git-subdir source requires 'url'")
	}
	subPath := src.SubPath

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
		repo, err := git.PlainOpen(tempDir)
		if err != nil {
			return fmt.Errorf("failed to open repository: %w", err)
		}
		worktree, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("failed to get worktree: %w", err)
		}
		hash, err := repo.ResolveRevision(plumbing.Revision(src.SHA))
		if err != nil {
			return fmt.Errorf("failed to resolve SHA %s: %w", src.SHA, err)
		}
		if err := worktree.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
			return fmt.Errorf("failed to checkout SHA %s: %w", src.SHA, err)
		}
	} else if src.Ref != "" {
		repo, err := git.PlainOpen(tempDir)
		if err != nil {
			return fmt.Errorf("failed to open repository: %w", err)
		}
		worktree, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("failed to get worktree: %w", err)
		}
		if err := worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(src.Ref)}); err != nil {
			return fmt.Errorf("failed to checkout ref %s: %w", src.Ref, err)
		}
	}

	srcDir := filepath.Join(tempDir, subPath)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("subdirectory '%s' not found in repository", subPath)
	}

	if err := copyRecursive(srcDir, cachePath); err != nil {
		return fmt.Errorf("failed to copy plugin files: %w", err)
	}

	return nil
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

	// Check plugin.json version
	pluginJSONPath := filepath.Join(pluginPath, ".claude-plugin", "plugin.json")
	version, err := v.readPluginJSONVersion(pluginJSONPath)
	if err == nil && version != "" {
		versions = append(versions, version)
	}

	// Check git tags
	repo, err := git.PlainOpen(pluginPath)
	if err == nil {
		tags, err := repo.Tags()
		if err == nil {
			tags.ForEach(func(ref *plumbing.Reference) error {
				tagName := ref.Name().Short()
				// Only include version-like tags (e.g., v1.0.0, 1.0.0)
				if strings.HasPrefix(tagName, "v") || strings.HasPrefix(tagName, "0") || strings.HasPrefix(tagName, "1") || strings.HasPrefix(tagName, "2") {
					versions = append(versions, tagName)
				}
				return nil
			})
		}
	}

	// Always add "latest" as an option
	versions = append(versions, "latest")

	return versions, nil
}
