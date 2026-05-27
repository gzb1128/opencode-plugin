package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/opencode/plugin-cli/internal/mcp"
	"github.com/opencode/plugin-cli/internal/opencode"
)

type Installer struct {
	configMgr  *config.Manager
	resolver   *VersionResolver
	linker     *opencode.Linker
	marketMgr  *marketplace.Manager
	mcpManager *mcp.Manager
}

func NewInstaller(configMgr *config.Manager) *Installer {
	paths := configMgr.GetPaths()
	return &Installer{
		configMgr:  configMgr,
		resolver:   NewVersionResolver(),
		linker:     opencode.NewLinker(paths.AgentsDir),
		marketMgr:  marketplace.NewManager(paths.MarketsDir),
		mcpManager: mcp.NewManager(paths.OpenCodeConfig),
	}
}

type InstallOptions struct {
	MarketName string
	Version    string
	Scope      string
}

func (i *Installer) Install(pluginName string, opts InstallOptions) error {
	markets, err := i.configMgr.LoadKnownMarkets()
	if err != nil {
		return fmt.Errorf("failed to load marketplaces: %w", err)
	}

	plugin, marketSrc, marketName, err := i.findPlugin(markets, pluginName, opts.MarketName)
	if err != nil {
		return err
	}

	opts.MarketName = marketName
	marketPath := marketSrc.InstallLocation()

	isRemote := i.resolver.IsRemoteSource(plugin)
	var pluginPath string

	if isRemote {
		version, err := i.resolveRemoteVersion(plugin, opts.Version)
		if err != nil {
			return fmt.Errorf("failed to resolve version: %w", err)
		}

		cachePath := filepath.Join(
			i.configMgr.GetPaths().CacheDir,
			opts.MarketName,
			pluginName,
			version,
		)

		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			fmt.Printf("  Cloning plugin from remote repository...\n")
			if err := i.resolver.CloneRemotePlugin(plugin, cachePath); err != nil {
				return fmt.Errorf("failed to clone plugin: %w", err)
			}
		}

		pluginPath = cachePath

		counts, err := i.linker.CreateSymlinks(pluginPath)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to create symlinks: %v\n", err)
		}

		mcpCount, err := i.installMCP(pluginPath, pluginName)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to install MCP servers: %v\n", err)
		}

		key := fmt.Sprintf("%s@%s", pluginName, opts.MarketName)
		record := &config.InstallRecord{
			Scope:       opts.Scope,
			InstallPath: cachePath,
			Version:     version,
			InstalledAt: time.Now(),
		}

		if err := i.configMgr.AddInstallRecord(key, record); err != nil {
			return fmt.Errorf("failed to record installation: %w", err)
		}

		fmt.Printf("✓ Successfully installed plugin: %s@%s\n", pluginName, version)
		fmt.Printf("  From marketplace: %s\n", opts.MarketName)
		fmt.Printf("  Cache: %s\n", cachePath)
		fmt.Printf("  Skills: %d\n", counts.Skills)
		if mcpCount > 0 {
			fmt.Printf("  MCP Servers: %d\n", mcpCount)
		}

		return nil
	}

	pluginPath, err = i.resolver.GetPluginSourcePath(plugin, marketPath)
	if err != nil {
		return fmt.Errorf("failed to get plugin source path: %w", err)
	}

	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin directory not found: %s", pluginPath)
	}

	version, err := i.resolver.Resolve(pluginPath, opts.Version)
	if err != nil {
		return fmt.Errorf("failed to resolve version: %w", err)
	}

	cachePath := filepath.Join(
		i.configMgr.GetPaths().CacheDir,
		opts.MarketName,
		pluginName,
		version,
	)

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		if err := i.copyPluginToCache(pluginPath, cachePath); err != nil {
			return fmt.Errorf("failed to copy plugin to cache: %w", err)
		}
	}

	counts, err := i.linker.CreateSymlinks(cachePath)
	if err != nil {
		return fmt.Errorf("failed to create symlinks: %w", err)
	}

	mcpCount, err := i.installMCP(cachePath, pluginName)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to install MCP servers: %v\n", err)
	}

	key := fmt.Sprintf("%s@%s", pluginName, opts.MarketName)
	record := &config.InstallRecord{
		Scope:       opts.Scope,
		InstallPath: cachePath,
		Version:     version,
		InstalledAt: time.Now(),
	}

	if err := i.configMgr.AddInstallRecord(key, record); err != nil {
		return fmt.Errorf("failed to record installation: %w", err)
	}

	fmt.Printf("✓ Successfully installed plugin: %s@%s\n", pluginName, version)
	fmt.Printf("  From marketplace: %s\n", opts.MarketName)
	fmt.Printf("  Cache: %s\n", cachePath)
	fmt.Printf("  Skills: %d\n", counts.Skills)
	if mcpCount > 0 {
		fmt.Printf("  MCP Servers: %d\n", mcpCount)
	}

	return nil
}

func (i *Installer) resolveRemoteVersion(plugin *marketplace.Plugin, requested string) (string, error) {
	src, ok := plugin.Source.(marketplace.PluginSource)
	if !ok {
		if requested != "" && requested != "latest" {
			return requested, nil
		}
		return "latest", nil
	}

	var sha string
	switch s := src.(type) {
	case *marketplace.GitHubSource:
		sha = s.SHA
	case *marketplace.URLSource:
		sha = s.SHA
	case *marketplace.GitSource:
		sha = s.SHA
	case *marketplace.GitSubdirSource:
		sha = s.SHA
	}

	if sha != "" {
		if len(sha) > 12 {
			return sha[:12], nil
		}
		return sha, nil
	}

	if requested != "" && requested != "latest" {
		return requested, nil
	}

	return "latest", nil
}

func (i *Installer) installMCP(pluginPath, pluginName string) (int, error) {
	servers, err := i.mcpManager.GetMCPServers(pluginPath)
	if err != nil {
		return 0, err
	}

	if len(servers) == 0 {
		return 0, nil
	}

	if err := i.mcpManager.InstallMCPConfig(pluginPath, pluginName); err != nil {
		return 0, err
	}

	return len(servers), nil
}

func (i *Installer) findPlugin(markets map[string]map[string]interface{}, pluginName, marketName string) (*marketplace.Plugin, marketplace.MarketSource, string, error) {
	marketSources := make(map[string]marketplace.MarketSource)
	for name, src := range markets {
		marketSources[name] = marketplace.NewMarketSourceFromConfig(src)
	}

	plugin, ms, foundMarketName, err := i.marketMgr.FindPlugin(marketSources, pluginName, marketName)
	if err != nil {
		return nil, nil, "", err
	}

	return plugin, ms, foundMarketName, nil
}

func (i *Installer) copyPluginToCache(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	skipItems := map[string]bool{
		".git": true,
	}

	for _, entry := range entries {
		name := entry.Name()

		if skipItems[name] {
			continue
		}

		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		if entry.IsDir() {
			if err := i.copyDir(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy directory %s: %w", name, err)
			}
		} else {
			if err := i.copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", name, err)
			}
		}
	}

	return nil
}

func (i *Installer) copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := i.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := i.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func (i *Installer) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func (i *Installer) Remove(pluginName, marketName string) error {
	key := fmt.Sprintf("%s@%s", pluginName, marketName)
	record, err := i.configMgr.GetInstallRecord(key)
	if err != nil {
		return fmt.Errorf("plugin %s not found", key)
	}

	count, err := i.linker.RemoveSymlinks(record.InstallPath)
	if err != nil {
		fmt.Printf("⚠️  Error removing symlinks: %v\n", err)
	}

	if err := i.mcpManager.UninstallMCPConfig(pluginName); err != nil {
		fmt.Printf("⚠️  Warning: Failed to uninstall MCP servers: %v\n", err)
	}

	if err := os.RemoveAll(record.InstallPath); err != nil {
		fmt.Printf("⚠️  Failed to remove cache: %v\n", err)
	} else {
		fmt.Printf("✓ Removed cache: %s\n", record.InstallPath)
	}

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
