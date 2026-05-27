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

func (m *Manager) Add(name, url string) (*Marketplace, MarketSource, error) {
	source, err := ParseMarketplaceSource(url)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse marketplace source: %w", err)
	}

	marketDir := filepath.Join(m.marketsDir, name)

	switch s := source.(type) {
	case *GitHubMarketSource, *GitMarketSource:
		cloneURL := GetMarketSourceURL(source)
		if err := m.gitClient.CloneOrPull(cloneURL, marketDir); err != nil {
			return nil, nil, fmt.Errorf("failed to clone/pull repository: %w", err)
		}

	case *URLMarketSource:
		return nil, nil, fmt.Errorf("JSON URL marketplace not yet implemented")

	case *LocalMarketSource, *DirectoryMarketSource:
		marketDir = GetMarketSourcePath(source)

	case *FileMarketSource:
		marketDir = marketplaceRootForIndexPath(s.Path)

	default:
		return nil, nil, fmt.Errorf("unsupported source type: %s", source.SourceType())
	}

	source.SetInstallLocation(marketDir)

	indexPath := MarketSourceIndexPath(source)
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("marketplace.json not found at %s", indexPath)
	}

	marketplace, err := ParseMarketplaceIndex(indexPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse marketplace.json: %w", err)
	}

	return marketplace, source, nil
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

func (m *Manager) FindPlugin(markets map[string]MarketSource, pluginName, marketName string) (*Plugin, MarketSource, string, error) {
	if marketName != "" {
		market, ok := markets[marketName]
		if !ok {
			return nil, nil, "", fmt.Errorf("marketplace %s not found", marketName)
		}

		indexPath := MarketSourceIndexPath(market)
		marketplace, err := ParseMarketplaceIndex(indexPath)
		if err != nil {
			return nil, nil, "", err
		}

		for _, plugin := range marketplace.Plugins {
			if plugin.Name == pluginName {
				return &plugin, market, marketName, nil
			}
		}

		return nil, nil, "", fmt.Errorf("plugin %s not found in marketplace %s", pluginName, marketName)
	}

	for mName, market := range markets {
		indexPath := MarketSourceIndexPath(market)
		marketplace, err := ParseMarketplaceIndex(indexPath)
		if err != nil {
			continue
		}

		for _, plugin := range marketplace.Plugins {
			if plugin.Name == pluginName {
				return &plugin, market, mName, nil
			}
		}
	}

	return nil, nil, "", fmt.Errorf("plugin %s not found in any marketplace", pluginName)
}

func (m *Manager) Remove(name string) error {
	paths := m.marketsDir
	marketDir := filepath.Join(paths, name)

	if _, err := os.Stat(marketDir); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(marketDir)
}

func MarketSourceIndexPath(source MarketSource) string {
	if fileSource, ok := source.(*FileMarketSource); ok && fileSource.Path != "" {
		return fileSource.Path
	}
	return filepath.Join(source.InstallLocation(), ".claude-plugin", "marketplace.json")
}

func marketplaceRootForIndexPath(indexPath string) string {
	dir := filepath.Dir(indexPath)
	if filepath.Base(dir) == ".claude-plugin" {
		return filepath.Dir(dir)
	}
	return dir
}
