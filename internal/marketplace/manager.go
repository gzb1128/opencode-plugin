package marketplace

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/opencode/plugin-cli/internal/pathutil"
)

type ResolvedPlugin struct {
	Plugin      *Plugin
	Market      MarketSource
	MarketName  string
	Marketplace *Marketplace
}

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

	return m.AddSource(name, source)
}

func (m *Manager) AddSource(name string, source MarketSource) (*Marketplace, MarketSource, error) {
	var marketDir string

	switch s := source.(type) {
	case *GitHubMarketSource, *GitMarketSource:
		safeDir, err := pathutil.SafeMarketplaceCachePath(m.marketsDir, name, "")
		if err != nil {
			return nil, nil, fmt.Errorf("invalid marketplace name %q: %w", name, err)
		}
		if source.InstallLocation() != "" && isWithinMarketsDir(source.InstallLocation(), m.marketsDir) {
			marketDir = source.InstallLocation()
		} else {
			marketDir = safeDir
		}
		cloneURL := GetMarketSourceURL(source)
		opts := CloneOptions{
			Ref: GetMarketSourceRef(source),
		}
		if err := m.gitClient.CloneOrPullWithOptions(cloneURL, marketDir, opts); err != nil {
			return nil, nil, fmt.Errorf("failed to clone/pull repository: %w", err)
		}

	case *URLMarketSource:
		cachedPath, err := m.cacheMarketplaceFromURL(name, s)
		if err != nil {
			return nil, nil, err
		}
		marketDir = cachedPath

	case *LocalMarketSource, *DirectoryMarketSource:
		marketDir = GetMarketSourcePath(source)

	case *FileMarketSource:
		marketDir = marketplaceRootForIndexPath(s.Path)

	default:
		return nil, nil, fmt.Errorf("unsupported source type: %s", source.SourceType())
	}

	source.SetInstallLocation(marketDir)

	indexPath, err := MarketSourceIndexPath(source)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve index path: %w", err)
	}
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("marketplace.json not found at %s", indexPath)
	}

	marketplace, err := ParseMarketplaceIndex(indexPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse marketplace.json: %w", err)
	}

	return marketplace, source, nil
}

func (m *Manager) cacheMarketplaceFromURL(name string, source *URLMarketSource) (string, error) {
	if err := os.MkdirAll(m.marketsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create markets directory: %w", err)
	}

	cachePath, err := pathutil.SafeMarketplaceCachePath(m.marketsDir, name, ".json")
	if err != nil {
		return "", fmt.Errorf("failed to compute cache path: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", source.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range source.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch marketplace: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	// marketplace.json 通常只有几 KB；给 32MB 上限防止误指 / 恶意大文件导致 OOM。
	const maxMarketplaceBytes = 32 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMarketplaceBytes+1))
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(body)) > maxMarketplaceBytes {
		return "", fmt.Errorf("marketplace response exceeds %d bytes (likely wrong URL)", maxMarketplaceBytes)
	}

	tmpFile, err := os.CreateTemp(m.marketsDir, ".marketplace-tmp-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write cache: %w", err)
	}
	tmpFile.Close()

	if _, err := ParseMarketplaceIndex(tmpPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("invalid marketplace content: %w", err)
	}

	if err := os.Rename(tmpPath, cachePath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write cache: %w", err)
	}

	source.SetInstallLocation(cachePath)
	return cachePath, nil
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

		indexPath, err := MarketSourceIndexPath(market)
		if err != nil {
			return nil, nil, "", err
		}
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
		indexPath, err := MarketSourceIndexPath(market)
		if err != nil {
			continue
		}
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

func (m *Manager) ResolvePlugin(markets map[string]MarketSource, pluginName, marketName string) (*ResolvedPlugin, error) {
	if marketName != "" {
		market, ok := markets[marketName]
		if !ok {
			return nil, fmt.Errorf("marketplace %s not found", marketName)
		}

		indexPath, err := MarketSourceIndexPath(market)
		if err != nil {
			return nil, err
		}
		marketplace, err := ParseMarketplaceIndex(indexPath)
		if err != nil {
			return nil, err
		}

		for i := range marketplace.Plugins {
			if marketplace.Plugins[i].Name == pluginName {
				return &ResolvedPlugin{
					Plugin:      &marketplace.Plugins[i],
					Market:      market,
					MarketName:  marketName,
					Marketplace: marketplace,
				}, nil
			}
		}

		return nil, fmt.Errorf("plugin %s not found in marketplace %s", pluginName, marketName)
	}

	for mName, market := range markets {
		indexPath, err := MarketSourceIndexPath(market)
		if err != nil {
			continue
		}
		marketplace, err := ParseMarketplaceIndex(indexPath)
		if err != nil {
			continue
		}

		for i := range marketplace.Plugins {
			if marketplace.Plugins[i].Name == pluginName {
				return &ResolvedPlugin{
					Plugin:      &marketplace.Plugins[i],
					Market:      market,
					MarketName:  mName,
					Marketplace: marketplace,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("plugin %s not found in any marketplace", pluginName)
}

func (m *Manager) Remove(name string) error {
	safeDir, err := pathutil.SafeMarketplaceCachePath(m.marketsDir, name, "")
	if err != nil {
		return nil
	}
	marketDir := safeDir

	if _, err := os.Stat(marketDir); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(marketDir)
}

func (m *Manager) RemoveSource(name string, source MarketSource) error {
	switch source.(type) {
	case *GitHubMarketSource, *GitMarketSource:
		cachePath, err := pathutil.SafeMarketplaceCachePath(m.marketsDir, name, "")
		if err != nil {
			return m.Remove(name)
		}
		if info, err := os.Stat(cachePath); err == nil && info.IsDir() {
			return os.RemoveAll(cachePath)
		}
		return m.Remove(name)
	case *URLMarketSource:
		loc := source.InstallLocation()
		if loc == "" {
			return nil
		}
		if !isWithinMarketsDir(loc, m.marketsDir) {
			return nil
		}
		if info, err := os.Stat(loc); err == nil && !info.IsDir() {
			return os.Remove(loc)
		}
		return nil
	case *LocalMarketSource, *FileMarketSource, *DirectoryMarketSource:
		return nil
	default:
		return m.Remove(name)
	}
}

// isWithinMarketsDir delegates to pathutil.IsWithinDir.
// 之前 plugin 包和这里各有一份近乎相同的实现，已经统一。
func isWithinMarketsDir(path, marketsDir string) bool {
	return pathutil.IsWithinDir(path, marketsDir)
}

func MarketSourceIndexPath(source MarketSource) (string, error) {
	switch s := source.(type) {
	case *FileMarketSource:
		return s.Path, nil
	case *URLMarketSource:
		if s.InstallLocation() != "" {
			return s.InstallLocation(), nil
		}
		return "", fmt.Errorf("URL market source has no install location")
	default:
		customPath := GetMarketSourceManifestPath(source)
		if customPath != "" {
			return pathutil.ResolvePathWithinBase(source.InstallLocation(), customPath)
		}
		return pathutil.ResolvePathWithinBase(source.InstallLocation(), ".claude-plugin"+string(filepath.Separator)+"marketplace.json")
	}
}

func marketplaceRootForIndexPath(indexPath string) string {
	dir := filepath.Dir(indexPath)
	if filepath.Base(dir) == ".claude-plugin" {
		return filepath.Dir(dir)
	}
	return dir
}
