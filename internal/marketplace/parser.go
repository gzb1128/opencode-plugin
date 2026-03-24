package marketplace

import (
	"encoding/json"
	"fmt"
	"os"
)

func ParseMarketplaceIndex(path string) (*Marketplace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read marketplace.json: %w", err)
	}

	var marketplace Marketplace
	if err := json.Unmarshal(data, &marketplace); err != nil {
		return nil, fmt.Errorf("failed to parse marketplace.json: %w", err)
	}

	if marketplace.Name == "" {
		return nil, fmt.Errorf("marketplace.json must have a 'name' field")
	}

	for i, plugin := range marketplace.Plugins {
		if err := parsePluginSource(&plugin); err != nil {
			return nil, fmt.Errorf("failed to parse source for plugin %s: %w", plugin.Name, err)
		}
		marketplace.Plugins[i] = plugin
	}

	return &marketplace, nil
}

func parsePluginSource(plugin *Plugin) error {
	switch v := plugin.Source.(type) {
	case string:
		plugin.Source = PluginSource{
			Type: string(SourceTypeLocal),
			Path: v,
		}
	case map[string]interface{}:
		sourceType, ok := v["source"].(string)
		if !ok {
			return fmt.Errorf("source must have a 'source' field")
		}

		ps := PluginSource{Type: sourceType}

		switch sourceType {
		case string(SourceTypeGitHub):
			if repo, ok := v["repo"].(string); ok {
				ps.Repo = repo
				ps.URL = fmt.Sprintf("https://github.com/%s.git", repo)
			}
		case string(SourceTypeGit), "url":
			if url, ok := v["url"].(string); ok {
				ps.URL = url
			}
			if sha, ok := v["sha"].(string); ok {
				ps.SHA = sha
			}
		case "git-subdir":
			if url, ok := v["url"].(string); ok {
				ps.URL = url
			}
			if path, ok := v["path"].(string); ok {
				ps.SubPath = path
			}
			if ref, ok := v["ref"].(string); ok {
				ps.Ref = ref
			}
			if sha, ok := v["sha"].(string); ok {
				ps.SHA = sha
			}
		}

		plugin.Source = ps
	default:
		return fmt.Errorf("invalid source format")
	}

	return nil
}
