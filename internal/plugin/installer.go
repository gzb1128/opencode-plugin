package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/opencode/plugin-cli/internal/opencode"
)

type Installer struct {
	configMgr *config.Manager
	resolver  *VersionResolver
	linker    *opencode.Linker
	marketMgr *marketplace.Manager
}

func NewInstaller(configMgr *config.Manager) *Installer {
	paths := configMgr.GetPaths()
	return &Installer{
		configMgr: configMgr,
		resolver:  NewVersionResolver(),
		linker:    opencode.NewLinker(paths.OpenCodeConfig),
		marketMgr: marketplace.NewManager(paths.MarketsDir),
	}
}

type InstallOptions struct {
	MarketName string
	Version    string
	Scope      string // "user" or "project"
}

func (i *Installer) Install(pluginName string, opts InstallOptions) error {
	// Load known markets
	markets, err := i.configMgr.LoadKnownMarkets()
	if err != nil {
		return fmt.Errorf("failed to load marketplaces: %w", err)
	}

	// Find plugin in marketplaces
	plugin, marketSrc, err := i.findPlugin(markets, pluginName, opts.MarketName)
	if err != nil {
		return err
	}

	// Get marketplace install location
	marketPath, ok := marketSrc["installLocation"].(string)
	if !ok {
		return fmt.Errorf("marketplace install location not found")
	}

	// Get plugin source path
	pluginPath, err := i.resolver.GetPluginSourcePath(plugin, marketPath)
	if err != nil {
		return fmt.Errorf("failed to get plugin source path: %w", err)
	}

	// Check if plugin directory exists
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin directory not found: %s", pluginPath)
	}

	// Resolve version
	version, err := i.resolver.Resolve(pluginPath, opts.Version)
	if err != nil {
		return fmt.Errorf("failed to resolve version: %w", err)
	}

	// Determine cache path
	cachePath := filepath.Join(
		i.configMgr.GetPaths().CacheDir,
		opts.MarketName,
		pluginName,
		version,
	)

	// Copy to cache (if not already there)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		if err := i.copyPluginToCache(pluginPath, cachePath); err != nil {
			return fmt.Errorf("failed to copy plugin to cache: %w", err)
		}
	}

	// Create symlinks
	counts, err := i.linker.CreateSymlinks(cachePath)
	if err != nil {
		return fmt.Errorf("failed to create symlinks: %w", err)
	}

	// Record installation
	key := fmt.Sprintf("%s@%s", pluginName, opts.MarketName)
	record := &config.InstallRecord{
		Scope:       opts.Scope,
		InstallPath: cachePath,
		Version:     version,
		InstalledAt: time.Now(),
		LastUpdated: time.Now(),
	}

	if err := i.configMgr.AddInstallRecord(key, record); err != nil {
		return fmt.Errorf("failed to record installation: %w", err)
	}

	// Print success message
	fmt.Printf("✓ Successfully installed plugin: %s@%s\n", pluginName, version)
	fmt.Printf("  From marketplace: %s\n", opts.MarketName)
	fmt.Printf("  Cache: %s\n", cachePath)
	fmt.Printf("  Skills: %d, Commands: %d, Agents: %d\n", counts.Skills, counts.Commands, counts.Agents)

	return nil
}

func (i *Installer) findPlugin(markets map[string]map[string]interface{}, pluginName, marketName string) (*marketplace.Plugin, map[string]interface{}, error) {
	// Convert markets to the format expected by marketplace.Manager
	marketSources := make(map[string]marketplace.MarketSource)
	for name, src := range markets {
		ms := marketplace.MarketSource{}
		if v, ok := src["source"].(string); ok {
			ms.Type = v
		}
		if v, ok := src["repo"].(string); ok {
			ms.Repo = v
		}
		if v, ok := src["url"].(string); ok {
			ms.URL = v
		}
		if v, ok := src["path"].(string); ok {
			ms.Path = v
		}
		if v, ok := src["installLocation"].(string); ok {
			ms.InstallLocation = v
		}
		marketSources[name] = ms
	}

	plugin, ms, err := i.marketMgr.FindPlugin(marketSources, pluginName, marketName)
	if err != nil {
		return nil, nil, err
	}

	// Convert back to map
	result := map[string]interface{}{
		"source":          ms.Type,
		"repo":            ms.Repo,
		"url":             ms.URL,
		"path":            ms.Path,
		"installLocation": ms.InstallLocation,
	}

	return plugin, result, nil
}

func (i *Installer) copyPluginToCache(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// Copy directories
	components := []string{"skills", "commands", "agents", ".claude-plugin"}
	for _, comp := range components {
		srcPath := filepath.Join(src, comp)
		dstPath := filepath.Join(dst, comp)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}

		if err := i.copyDir(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to copy %s: %w", comp, err)
		}
	}

	return nil
}

func (i *Installer) copyDir(src, dst string) error {
	return os.Rename(src, dst)
}

func (i *Installer) Remove(pluginName string, marketName string) error {
	// Load installation record
	key := fmt.Sprintf("%s@%s", pluginName, marketName)
	record, err := i.configMgr.GetInstallRecord(key)
	if err != nil {
		return fmt.Errorf("plugin %s not found", key)
	}

	// Remove symlinks
	count, err := i.linker.RemoveSymlinks(record.InstallPath)
	if err != nil {
		fmt.Printf("⚠️  Error removing symlinks: %v\n", err)
	}

	// Remove cache
	if err := os.RemoveAll(record.InstallPath); err != nil {
		fmt.Printf("⚠️  Failed to remove cache: %v\n", err)
	} else {
		fmt.Printf("✓ Removed cache: %s\n", record.InstallPath)
	}

	// Remove installation record
	if err := i.configMgr.RemoveInstallRecord(key); err != nil {
		return fmt.Errorf("failed to remove installation record: %w", err)
	}

	fmt.Printf("✓ Successfully removed plugin: %s (%d symlinks removed)\n", pluginName, count)

	return nil
}

func (i *Installer) List() (map[string][]config.InstallRecord, error) {
	installed, err := i.configMgr.LoadInstalledPlugins()
	if err != nil {
		return nil, err
	}

	return installed.Plugins, nil
}
