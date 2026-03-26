package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
)

type Manager struct {
	marketsDir string
	gitClient  *GitClient
}

func NewManager(marketsDir string) *Manager {
	return &Manager{
		marketsDir: marketsDir,
		gitClient:  NewGitClient(),
	}
}

func (m *Manager) Add(name, url string) (*Marketplace, error) {
	source, err := ParseMarketplaceSource(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse marketplace source: %w", err)
	}

	marketDir := filepath.Join(m.marketsDir, name)

	switch source.Type {
	case string(SourceTypeGitHub), string(SourceTypeGit):
		if err := m.gitClient.CloneOrPull(source.URL, marketDir); err != nil {
			return nil, fmt.Errorf("failed to clone/pull repository: %w", err)
		}

	case string(SourceTypeJSONURL):
		return nil, fmt.Errorf("JSON URL marketplace not yet implemented")

	case string(SourceTypeLocal):
		marketDir = source.Path

	default:
		return nil, fmt.Errorf("unsupported source type: %s", source.Type)
	}

	indexPath := filepath.Join(marketDir, ".claude-plugin", "marketplace.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("marketplace.json not found at %s", indexPath)
	}

	marketplace, err := ParseMarketplaceIndex(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse marketplace.json: %w", err)
	}

	return marketplace, nil
}

func (m *Manager) Get(marketDir string) (*Marketplace, error) {
	indexPath := filepath.Join(marketDir, ".claude-plugin", "marketplace.json")
	return ParseMarketplaceIndex(indexPath)
}

func (m *Manager) List(marketDirs map[string]string) (map[string]*Marketplace, error) {
	result := make(map[string]*Marketplace)
	for name, dir := range marketDirs {
		marketplace, err := m.Get(dir)
		if err != nil {
			result[name] = nil
			continue
		}
		result[name] = marketplace
	}
	return result, nil
}

func (m *Manager) Update(marketDir string) error {
	if err := m.gitClient.Pull(marketDir); err != nil {
		return fmt.Errorf("failed to update marketplace: %w", err)
	}

	indexPath := filepath.Join(marketDir, ".claude-plugin", "marketplace.json")
	if _, err := ParseMarketplaceIndex(indexPath); err != nil {
		return fmt.Errorf("failed to parse updated marketplace.json: %w", err)
	}

	return nil
}

func (m *Manager) FindPlugin(markets map[string]MarketSource, pluginName, marketName string) (*Plugin, *MarketSource, string, error) {
	if marketName != "" {
		market, ok := markets[marketName]
		if !ok {
			return nil, nil, "", fmt.Errorf("marketplace %s not found", marketName)
		}

		indexPath := filepath.Join(market.InstallLocation, ".claude-plugin", "marketplace.json")
		marketplace, err := ParseMarketplaceIndex(indexPath)
		if err != nil {
			return nil, nil, "", err
		}

		for _, plugin := range marketplace.Plugins {
			if plugin.Name == pluginName {
				return &plugin, &market, marketName, nil
			}
		}

		return nil, nil, "", fmt.Errorf("plugin %s not found in marketplace %s", pluginName, marketName)
	}

	for mName, market := range markets {
		indexPath := filepath.Join(market.InstallLocation, ".claude-plugin", "marketplace.json")
		marketplace, err := ParseMarketplaceIndex(indexPath)
		if err != nil {
			continue
		}

		for _, plugin := range marketplace.Plugins {
			if plugin.Name == pluginName {
				return &plugin, &market, mName, nil
			}
		}
	}

	return nil, nil, "", fmt.Errorf("plugin %s not found in any marketplace", pluginName)
}

func (m *Manager) Remove(name string) error {
	// This is a helper method that removes a marketplace directory
	// The actual config removal is handled by the caller
	paths := m.marketsDir
	marketDir := filepath.Join(paths, name)

	if _, err := os.Stat(marketDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to remove
	}

	return os.RemoveAll(marketDir)
}
