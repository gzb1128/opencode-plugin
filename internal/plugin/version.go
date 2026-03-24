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
		// Relative path from market root
		return filepath.Join(marketPath, src), nil

	case marketplace.PluginSource:
		// PluginSource object
		switch src.Type {
		case string(marketplace.SourceTypeLocal):
			return filepath.Join(marketPath, src.Path), nil
		default:
			return "", fmt.Errorf("unsupported plugin source type: %s (plugin may need to be cloned first)", src.Type)
		}

	case map[string]interface{}:
		// Raw map from JSON
		sourceType, _ := src["source"].(string)
		switch sourceType {
		case "github", "git", "git-subdir":
			// These need to be cloned first, return error
			return "", fmt.Errorf("plugin source type '%s' requires cloning, use 'market update' first", sourceType)
		case "url":
			// URL source
			return "", fmt.Errorf("plugin source type 'url' is not supported yet")
		case "":
			// Relative path
			if path, ok := src["path"].(string); ok {
				return filepath.Join(marketPath, path), nil
			}
		}
		return "", fmt.Errorf("invalid plugin source format")

	default:
		return "", fmt.Errorf("invalid plugin source format")
	}
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
