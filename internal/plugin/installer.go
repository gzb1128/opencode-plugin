package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/opencode/plugin-cli/internal/mcp"
	"github.com/opencode/plugin-cli/internal/opencode"
	"github.com/opencode/plugin-cli/internal/pathutil"
)

type MaterializedPlugin struct {
	Path         string
	Version      string
	ManifestPath string
}

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

	marketSources := make(map[string]marketplace.MarketSource)
	for name, src := range markets {
		marketSources[name] = marketplace.NewMarketSourceFromConfig(src)
	}

	rootResolved, err := i.marketMgr.ResolvePlugin(marketSources, pluginName, opts.MarketName)
	if err != nil {
		return err
	}

	opts.MarketName = rootResolved.MarketName
	rootID := fmt.Sprintf("%s@%s", rootResolved.Plugin.Name, rootResolved.MarketName)

	installed, err := i.configMgr.LoadInstalledPlugins()
	if err != nil {
		return fmt.Errorf("failed to load installed plugins: %w", err)
	}
	alreadyInstalled := make(map[string]bool)
	for key := range installed.Plugins {
		alreadyInstalled[key] = true
	}

	resolvedMap := map[string]*marketplace.ResolvedPlugin{rootID: rootResolved}
	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		if rp, ok := resolvedMap[id]; ok {
			return rp, nil
		}
		parts := strings.SplitN(id, "@", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid plugin id: %s", id)
		}
		rp, err := i.marketMgr.ResolvePlugin(marketSources, parts[0], parts[1])
		if err != nil {
			return nil, err
		}
		resolvedMap[id] = rp
		return rp, nil
	}

	allowedCross := make(map[string]bool)
	if rootResolved.Marketplace != nil {
		for _, m := range rootResolved.Marketplace.AllowCrossMarketplaceDeps {
			allowedCross[m] = true
		}
	}

	result, err := ResolveDependencyClosure(rootID, lookup, alreadyInstalled, allowedCross)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	for _, id := range result.Closure {
		if id == rootID {
			continue
		}
		rp := resolvedMap[id]
		parts := strings.SplitN(id, "@", 2)
		depOpts := InstallOptions{
			MarketName: parts[1],
			Scope:      opts.Scope,
		}
		if err := i.installOneResolvedPlugin(rp, depOpts); err != nil {
			return fmt.Errorf("failed to install dependency %s: %w", id, err)
		}
	}

	return i.installOneResolvedPlugin(rootResolved, opts)
}

func (i *Installer) installOneResolvedPlugin(resolved *marketplace.ResolvedPlugin, opts InstallOptions) error {
	mat, err := i.materializePlugin(resolved, opts)
	if err != nil {
		return err
	}

	var manifest map[string]interface{}
	if mat.ManifestPath != "" {
		manifest, _ = opencode.ReadManifest(mat.ManifestPath)
	}

	counts, err := i.linker.CreateSymlinksFromManifest(mat.Path, manifest)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to create symlinks: %v\n", err)
	}

	mcpCount, err := i.installMCP(mat.Path, resolved.Plugin.Name)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to install MCP servers: %v\n", err)
	}

	key := fmt.Sprintf("%s@%s", resolved.Plugin.Name, opts.MarketName)
	record := &config.InstallRecord{
		Scope:       opts.Scope,
		InstallPath: mat.Path,
		Version:     mat.Version,
		InstalledAt: time.Now(),
	}

	if err := i.configMgr.AddInstallRecord(key, record); err != nil {
		return fmt.Errorf("failed to record installation: %w", err)
	}

	fmt.Printf("✓ Successfully installed plugin: %s@%s\n", resolved.Plugin.Name, mat.Version)
	fmt.Printf("  From marketplace: %s\n", opts.MarketName)
	fmt.Printf("  Cache: %s\n", mat.Path)
	if counts.Skills > 0 {
		fmt.Printf("  Skills: %d\n", counts.Skills)
	}
	if counts.Commands > 0 {
		fmt.Printf("  Commands: %d\n", counts.Commands)
	}
	if counts.Agents > 0 {
		fmt.Printf("  Agents: %d\n", counts.Agents)
	}
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
	var npmSrc *marketplace.NpmSource
	switch s := src.(type) {
	case *marketplace.GitHubSource:
		sha = s.SHA
	case *marketplace.URLSource:
		sha = s.SHA
	case *marketplace.GitSource:
		sha = s.SHA
	case *marketplace.GitSubdirSource:
		sha = s.SHA
	case *marketplace.NpmSource:
		npmSrc = s
	}

	if sha != "" {
		if len(sha) > 12 {
			return sha[:12], nil
		}
		return sha, nil
	}

	if npmSrc != nil {
		if requested != "" && requested != "latest" {
			return requested, nil
		}
		if npmSrc.Version != "" {
			return npmSrc.Version, nil
		}
		if isLocalPath(npmSrc.Package) {
			ver, err := readPackageVersionFromJSON(npmSrc.Package)
			if err == nil && ver != "" {
				return ver, nil
			}
		}
		return "latest", nil
	}

	if requested != "" && requested != "latest" {
		return requested, nil
	}

	return "latest", nil
}

func readPackageVersionFromJSON(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", fmt.Errorf("failed to read package.json: %w", err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("failed to parse package.json: %w", err)
	}
	return pkg.Version, nil
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
	return copyDirTree(src, dst, map[string]bool{".git": true})
}

func (i *Installer) copyDir(src, dst string) error {
	return copyDirTree(src, dst, nil)
}

func copyDirTree(src, dst string, skip map[string]bool) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if skip != nil && skip[name] {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		if info.IsDir() {
			if err := copyDirTree(srcPath, dstPath, skip); err != nil {
				return err
			}
		} else {
			if err := copyFilePreserveMode(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFilePreserveMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func (i *Installer) materializePlugin(resolved *marketplace.ResolvedPlugin, opts InstallOptions) (*MaterializedPlugin, error) {
	marketPath := resolved.Market.InstallLocation()
	plugin := resolved.Plugin

	var pluginRoot string
	if resolved.Marketplace != nil && resolved.Marketplace.Metadata != nil {
		pluginRoot = resolved.Marketplace.Metadata.PluginRoot
	}

	ctx := PluginResolutionContext{MarketPath: marketPath, PluginRoot: pluginRoot}

	isRemote := i.resolver.IsRemoteSource(plugin)
	var sourcePath string
	var version string

	if isRemote {
		var err error
		version, err = i.resolveRemoteVersion(plugin, opts.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve version: %w", err)
		}
	} else {
		var err error
		sourcePath, err = i.resolver.GetPluginSourcePathWithCtx(plugin, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get plugin source path: %w", err)
		}

		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("plugin directory not found: %s", sourcePath)
		}

		version, err = i.resolver.Resolve(sourcePath, opts.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve version: %w", err)
		}

		if version == "latest" && plugin.Version != "" {
			version = plugin.Version
		}
	}

	pluginID := fmt.Sprintf("%s@%s", plugin.Name, opts.MarketName)
	cachePath, err := pathutil.SafePluginCachePath(i.configMgr.GetPaths().CacheDir, pluginID, version)
	if err != nil {
		return nil, fmt.Errorf("failed to compute cache path: %w", err)
	}

	manifestPath := filepath.Join(cachePath, ".claude-plugin", "plugin.json")

	if isRemote {
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			fmt.Printf("  Cloning plugin from remote repository...\n")
			if err := i.resolver.CloneRemotePlugin(plugin, cachePath); err != nil {
				return nil, fmt.Errorf("failed to clone plugin: %w", err)
			}
		}
	} else {
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			if err := i.copyPluginToCache(sourcePath, cachePath); err != nil {
				return nil, fmt.Errorf("failed to copy plugin to cache: %w", err)
			}
		}
	}

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		if err := i.generateFallbackManifest(plugin, cachePath); err != nil {
			fmt.Printf("⚠️  Warning: Failed to generate fallback manifest: %v\n", err)
		}
	}

	return &MaterializedPlugin{
		Path:         cachePath,
		Version:      version,
		ManifestPath: manifestPath,
	}, nil
}

func (i *Installer) generateFallbackManifest(plugin *marketplace.Plugin, cachePath string) error {
	manifestDir := filepath.Join(cachePath, ".claude-plugin")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	manifest := map[string]interface{}{
		"name":        plugin.Name,
		"description": plugin.Description,
	}
	if plugin.Version != "" {
		manifest["version"] = plugin.Version
	}
	if plugin.Author != nil {
		manifest["author"] = plugin.Author
	}
	if plugin.Homepage != "" {
		manifest["homepage"] = plugin.Homepage
	}
	if plugin.Repository != "" {
		manifest["repository"] = plugin.Repository
	}
	if plugin.License != "" {
		manifest["license"] = plugin.License
	}
	if len(plugin.Keywords) > 0 {
		manifest["keywords"] = plugin.Keywords
	}
	if len(plugin.Dependencies) > 0 {
		manifest["dependencies"] = plugin.Dependencies
	}
	if plugin.Skills != nil {
		manifest["skills"] = plugin.Skills
	}
	if plugin.Commands != nil {
		manifest["commands"] = plugin.Commands
	}
	if plugin.Agents != nil {
		manifest["agents"] = plugin.Agents
	}
	if len(plugin.MCPServersRaw) > 0 {
		manifest["mcpServers"] = json.RawMessage(plugin.MCPServersRaw)
	}

	deferredFields := []struct {
		name string
		raw  json.RawMessage
	}{
		{"hooks", plugin.HooksRaw},
		{"outputStyles", plugin.OutputStylesRaw},
		{"channels", plugin.ChannelsRaw},
		{"lspServers", plugin.LSPServersRaw},
		{"userConfig", plugin.UserConfigRaw},
	}
	for _, df := range deferredFields {
		if len(df.raw) > 0 {
			fmt.Printf("⚠️  Warning: field %q is deferred and will be skipped in fallback manifest for plugin %s\n", df.name, plugin.Name)
		}
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(manifestDir, "plugin.json")
	return os.WriteFile(manifestPath, append(data, '\n'), 0644)
}

func (i *Installer) Remove(pluginName, marketName string) error {
	key := fmt.Sprintf("%s@%s", pluginName, marketName)
	record, err := i.configMgr.GetInstallRecord(key)
	if err != nil {
		return fmt.Errorf("plugin %s not found", key)
	}

	installPath := record.InstallPath
	cacheDir := i.configMgr.GetPaths().CacheDir
	if !isWithinDir(installPath, cacheDir) {
		return fmt.Errorf("refusing to remove path %q outside cache directory %q", installPath, cacheDir)
	}

	count, err := i.linker.RemoveSymlinks(installPath)
	if err != nil {
		fmt.Printf("⚠️  Error removing symlinks: %v\n", err)
	}

	if err := i.mcpManager.UninstallMCPConfig(pluginName); err != nil {
		fmt.Printf("⚠️  Warning: Failed to uninstall MCP servers: %v\n", err)
	}

	if err := os.RemoveAll(installPath); err != nil {
		fmt.Printf("⚠️  Failed to remove cache: %v\n", err)
	} else {
		fmt.Printf("✓ Removed cache: %s\n", installPath)
	}

	if err := i.configMgr.RemoveInstallRecord(key); err != nil {
		return fmt.Errorf("failed to remove installation record: %w", err)
	}

	fmt.Printf("✓ Successfully removed plugin: %s (%d symlinks removed)\n", pluginName, count)

	return nil
}

func isWithinDir(path, base string) bool {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	absBase, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return false
	}
	sep := string(filepath.Separator)
	if !strings.HasPrefix(absPath, absBase+sep) {
		return false
	}
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return false
	}
	evalBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		return false
	}
	return strings.HasPrefix(evalPath, evalBase+sep)
}

func (i *Installer) List() (map[string][]config.InstallRecord, error) {
	installed, err := i.configMgr.LoadInstalledPlugins()
	if err != nil {
		return nil, err
	}

	return installed.Plugins, nil
}

func (i *Installer) ListInstalledByMarket(marketName string) ([]string, error) {
	installed, err := i.configMgr.LoadInstalledPlugins()
	if err != nil {
		return nil, err
	}

	var pluginNames []string
	suffix := "@" + marketName
	for key := range installed.Plugins {
		if strings.HasSuffix(key, suffix) {
			pluginName := strings.TrimSuffix(key, suffix)
			pluginNames = append(pluginNames, pluginName)
		}
	}
	return pluginNames, nil
}
