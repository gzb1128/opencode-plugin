package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
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
		mcpManager: mcp.NewManager(paths.OpenCodeConfig, paths.PluginDataDir),
	}
}

type InstallOptions struct {
	MarketName string
	Version    string
	Scope      string
	Force      bool
	Disabled   bool
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
			Force:      opts.Force,
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

	key := fmt.Sprintf("%s@%s", resolved.Plugin.Name, opts.MarketName)

	if opts.Disabled {
		record := &config.InstallRecord{
			Scope:       opts.Scope,
			InstallPath: mat.Path,
			Version:     mat.Version,
			InstalledAt: time.Now(),
			Disabled:    true,
			DisabledAt:  time.Now(),
		}

		if err := i.configMgr.AddInstallRecord(key, record); err != nil {
			return fmt.Errorf("failed to record installation: %w", err)
		}

		fmt.Printf("✓ Updated disabled plugin: %s@%s\n", resolved.Plugin.Name, mat.Version)
		fmt.Printf("  From marketplace: %s\n", opts.MarketName)
		fmt.Printf("  Cache: %s\n", mat.Path)
		return nil
	}

	counts, err := i.linker.CreateSymlinksFromManifest(mat.Path, manifest, opts.Force)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to create symlinks: %v\n", err)
	}

	mcpCount, err := i.installMCP(mat.Path, resolved.Plugin.Name, opts.MarketName)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to install MCP servers: %v\n", err)
	}

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
	if counts != nil && counts.Skills > 0 {
		fmt.Printf("  Skills: %d\n", counts.Skills)
	}
	if counts != nil && counts.Commands > 0 {
		fmt.Printf("  Commands: %d\n", counts.Commands)
	}
	if counts != nil && counts.Agents > 0 {
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

func (i *Installer) installMCP(pluginPath, pluginName, marketName string) (int, error) {
	servers, err := i.mcpManager.GetMCPServers(pluginPath)
	if err != nil {
		return 0, err
	}

	if len(servers) == 0 {
		return 0, nil
	}

	if err := i.mcpManager.InstallMCPConfig(pluginPath, pluginName, marketName); err != nil {
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
			return fmt.Errorf("failed to stat %s: %w", name, err)
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
		needClone := false
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			needClone = true
		} else if !isRepoHealthy(cachePath) {
			fmt.Printf("  Cached repository is corrupted, re-cloning...\n")
			if err := os.RemoveAll(cachePath); err != nil {
				return nil, fmt.Errorf("failed to remove corrupted cache: %w", err)
			}
			needClone = true
		}
		if needClone {
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
	if plugin.DisplayName != "" {
		manifest["displayName"] = plugin.DisplayName
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

// Update downloads the latest version of an installed plugin and swaps it
// with the currently-installed version.
//
// 与 Remove+Install 序列不同，Update 先把新版本下载到 cache（最危险的网络步骤），
// 只有在下载成功之后才动旧版本的 symlinks / MCP / install record。如果下载失败，
// 旧版本完整保留，用户无需手动重装。
//
// 实现要点：
//   - Stage 1（materialize）：把旧 cache 目录 rename 成 .update-backup，然后让
//     materializePlugin 在原路径上重新 clone/copy。失败时 rename 回去即可回滚。
//   - Stage 2（swap）：删除旧 symlinks/MCP，建立新 symlinks/MCP，覆盖 install
//     record。这些都是本地文件操作，失败概率远低于网络。
//   - Stage 3（cleanup）：删除 .update-backup；如果新旧 cache 路径不同，也删旧路径。
//     最后调用 CleanupOldVersions 清理同 plugin 的其它历史版本。
func (i *Installer) Update(pluginName string, opts InstallOptions) error {
	// ===== Stage 0: Resolve plugin from marketplace（无副作用）=====
	markets, err := i.configMgr.LoadKnownMarkets()
	if err != nil {
		return fmt.Errorf("failed to load marketplaces: %w", err)
	}

	marketSources := make(map[string]marketplace.MarketSource)
	for name, src := range markets {
		marketSources[name] = marketplace.NewMarketSourceFromConfig(src)
	}

	resolved, err := i.marketMgr.ResolvePlugin(marketSources, pluginName, opts.MarketName)
	if err != nil {
		return err
	}
	opts.MarketName = resolved.MarketName
	key := fmt.Sprintf("%s@%s", resolved.Plugin.Name, opts.MarketName)

	// 记录旧状态，用于 swap / cleanup。记录不存在视作首次安装。
	var oldCachePath string
	if oldRecord, err := i.configMgr.GetInstallRecord(key); err == nil && oldRecord != nil {
		oldCachePath = oldRecord.InstallPath
	}

	// 在 backup rename 之前先把 oldCachePath 解析成 EvalSymlinks 形式。
	// RemoveSymlinks 依赖词法路径匹配（filepath.Rel），而 symlink target 创建时
	// 会经过 EvalSymlinks（例如 macOS 上 /var → /private/var）。如果在 rename 之
	// 后才取 EvalSymlinks 会失败，导致后续 RemoveSymlinks 匹配不到旧 symlink。
	// 所以这里在路径还存在时先解析好，留给 Stage 2 用。
	oldCachePathResolved := oldCachePath
	if oldCachePath != "" {
		if ev, err := filepath.EvalSymlinks(oldCachePath); err == nil {
			oldCachePathResolved = ev
		}
	}

	// ===== Stage 1: Materialize 新版本到 cache（最危险的网络步骤）=====
	// 如果旧 cache 目录存在，先 rename 成 .update-backup。这样：
	//   (a) 同路径场景（version 不变或 "latest"）：materialize 会发现 cache 不存在，
	//       重新 clone/copy 出全新内容；
	//   (b) 不同路径场景：旧路径被 backup，新路径未受影响，materialize 直接 clone 到新路径。
	// 如果 materialize 失败，把 backup rename 回去即可完整恢复旧版本。
	backupPath := ""
	if oldCachePath != "" {
		if _, statErr := os.Stat(oldCachePath); statErr == nil {
			backupPath = oldCachePath + ".update-backup"
			// 如果上一次失败的 update 残留了 backup，先清掉
			if _, leftoverErr := os.Stat(backupPath); leftoverErr == nil {
				os.RemoveAll(backupPath)
			}
			if err := os.Rename(oldCachePath, backupPath); err != nil {
				return fmt.Errorf("failed to back up existing cache before update: %w", err)
			}
		}
	}

	mat, err := i.materializePlugin(resolved, opts)
	if err != nil {
		// 回滚：把 backup 还原到原路径
		if backupPath != "" {
			if rbErr := os.Rename(backupPath, oldCachePath); rbErr != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Critical: failed to restore cache after failed update: %v\n", rbErr)
				fmt.Fprintf(os.Stderr, "⚠️  Backup left at: %s (manual recovery required)\n", backupPath)
			} else {
				fmt.Printf("✓ Rolled back to previous version (cache restored at %s)\n", oldCachePath)
			}
		}
		return fmt.Errorf("failed to materialize new version: %w", err)
	}

	// materialize 成功后，无论后续步骤是否成功，backup 都可以清理了——新 cache 已经
	// 落盘，旧的不再被引用。用 defer 兜底，避免遗漏。
	if backupPath != "" {
		defer func() {
			if rmErr := os.RemoveAll(backupPath); rmErr != nil {
				fmt.Printf("⚠️  Warning: failed to clean up update backup %s: %v\n", backupPath, rmErr)
			}
		}()
	}

	// 读取新 manifest，用于后续 symlink 创建
	var manifest map[string]interface{}
	if mat.ManifestPath != "" {
		manifest, _ = opencode.ReadManifest(mat.ManifestPath)
	}

	// ===== Stage 2: Swap（删旧 side-effects，建立新 side-effects）=====
	// 2a. 删旧 symlinks / MCP。这些指向 OLD 路径；即使新旧路径相同，文件集合也可能变化。
	//     用 oldCachePathResolved（已 EvalSymlinks）确保和 symlink target 词法一致。
	if oldCachePathResolved != "" {
		if _, err := i.linker.RemoveSymlinks(oldCachePathResolved); err != nil {
			fmt.Printf("⚠️  Warning: failed to remove old symlinks: %v\n", err)
		}
	}
	if err := i.mcpManager.UninstallMCPConfig(resolved.Plugin.Name); err != nil {
		fmt.Printf("⚠️  Warning: failed to remove old MCP config: %v\n", err)
	}

	// 2b. 建立新 state（disabled 模式下只写 record，不建 symlinks / MCP）
	var counts opencode.ComponentCounts
	var mcpCount int
	if !opts.Disabled {
		countsPtr, linkErr := i.linker.CreateSymlinksFromManifest(mat.Path, manifest, opts.Force)
		if linkErr != nil {
			fmt.Printf("⚠️  Warning: Failed to create symlinks: %v\n", linkErr)
		} else if countsPtr != nil {
			counts = *countsPtr
		}
		mcpCount, err = i.installMCP(mat.Path, resolved.Plugin.Name, opts.MarketName)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to install MCP servers: %v\n", err)
		}
	}

	record := &config.InstallRecord{
		Scope:       opts.Scope,
		InstallPath: mat.Path,
		Version:     mat.Version,
		InstalledAt: time.Now(),
	}
	if opts.Disabled {
		record.Disabled = true
		record.DisabledAt = time.Now()
	}
	if err := i.configMgr.AddInstallRecord(key, record); err != nil {
		return fmt.Errorf("failed to record updated installation: %w", err)
	}

	// ===== Stage 3: Cleanup 历史版本 cache =====
	// 新旧路径不同时，旧路径还在（不在 backup 里就是没被 rename），删掉。
	if oldCachePath != "" && oldCachePath != mat.Path {
		cacheDir := i.configMgr.GetPaths().CacheDir
		if isWithinDir(oldCachePath, cacheDir) {
			if err := os.RemoveAll(oldCachePath); err != nil {
				fmt.Printf("⚠️  Warning: failed to remove old cache %s: %v\n", oldCachePath, err)
			} else {
				fmt.Printf("✓ Removed old version cache: %s\n", oldCachePath)
			}
		}
	}
	// CleanupOldVersions 会扫描同 plugin 下所有未引用的版本目录，统一清理
	if err := i.CleanupOldVersions(mat.Path); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cache cleanup failed: %v\n", err)
	}

	// ===== 日志输出 =====
	if opts.Disabled {
		fmt.Printf("✓ Updated disabled plugin: %s@%s\n", resolved.Plugin.Name, mat.Version)
		fmt.Printf("  From marketplace: %s\n", opts.MarketName)
		fmt.Printf("  Cache: %s\n", mat.Path)
	} else {
		fmt.Printf("✓ Successfully updated plugin: %s@%s\n", resolved.Plugin.Name, mat.Version)
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
	}

	return nil
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

func (i *Installer) Disable(pluginName, marketName string) error {
	key := fmt.Sprintf("%s@%s", pluginName, marketName)
	record, err := i.configMgr.GetInstallRecord(key)
	if err != nil {
		return fmt.Errorf("plugin %s not found", key)
	}

	installPath := record.InstallPath
	cacheDir := i.configMgr.GetPaths().CacheDir
	if installPath != "" && !isWithinDir(installPath, cacheDir) {
		return fmt.Errorf("refusing to disable path %q outside cache directory %q", installPath, cacheDir)
	}

	if installPath != "" {
		if _, err := i.linker.RemoveSymlinks(installPath); err != nil {
			return fmt.Errorf("failed to remove symlinks: %w", err)
		}
	}

	if err := i.mcpManager.DisableMCPConfig(pluginName); err != nil {
		return fmt.Errorf("failed to disable MCP servers: %w", err)
	}

	if err := i.configMgr.MutateInstallRecord(key, func(r *config.InstallRecord) {
		if !r.Disabled {
			r.Disabled = true
			r.DisabledAt = time.Now()
		}
	}); err != nil {
		return fmt.Errorf("failed to update installation record: %w", err)
	}

	fmt.Printf("Disabled plugin: %s\n", key)
	return nil
}

func (i *Installer) Enable(pluginName, marketName string, force bool) error {
	key := fmt.Sprintf("%s@%s", pluginName, marketName)
	record, err := i.configMgr.GetInstallRecord(key)
	if err != nil {
		return fmt.Errorf("plugin %s not found", key)
	}

	installPath := record.InstallPath
	cacheDir := i.configMgr.GetPaths().CacheDir
	if installPath != "" && !isWithinDir(installPath, cacheDir) {
		return fmt.Errorf("refusing to enable path %q outside cache directory %q", installPath, cacheDir)
	}

	if installPath == "" {
		return fmt.Errorf("plugin %s has no install path recorded", key)
	}

	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin cache not found at %s, run 'opencode-plugin plugin update %s' or reinstall", installPath, key)
	}

	manifestPath := filepath.Join(installPath, ".claude-plugin", "plugin.json")
	var manifest map[string]interface{}
	if _, err := os.Stat(manifestPath); err == nil {
		manifest, _ = opencode.ReadManifest(manifestPath)
	}

	counts, err := i.linker.CreateSymlinksFromManifest(installPath, manifest, force)
	if err != nil {
		return fmt.Errorf("failed to create symlinks: %w", err)
	}

	if err := i.reinstallMCPIfNeeded(installPath, pluginName, marketName); err != nil {
		return fmt.Errorf("failed to enable MCP servers: %w", err)
	}

	if err := i.configMgr.MutateInstallRecord(key, func(r *config.InstallRecord) {
		r.Disabled = false
		r.DisabledAt = time.Time{}
	}); err != nil {
		return fmt.Errorf("failed to update installation record: %w", err)
	}

	fmt.Printf("Enabled plugin: %s\n", key)
	if counts.Skills > 0 {
		fmt.Printf("  Skills: %d\n", counts.Skills)
	}
	if counts.Commands > 0 {
		fmt.Printf("  Commands: %d\n", counts.Commands)
	}
	if counts.Agents > 0 {
		fmt.Printf("  Agents: %d\n", counts.Agents)
	}
	return nil
}

func (i *Installer) reinstallMCPIfNeeded(installPath, pluginName, marketName string) error {
	if err := i.mcpManager.EnableMCPConfig(pluginName); err != nil {
		return err
	}

	servers, err := i.mcpManager.GetMCPServers(installPath)
	if err != nil {
		fmt.Printf("Warning: failed to read MCP config from cache: %v\n", err)
		return nil
	}
	if len(servers) == 0 {
		return nil
	}

	return i.mcpManager.InstallMissingMCPConfig(installPath, pluginName, marketName, servers)
}

func (i *Installer) CleanupOldVersions(currentInstallPath string) error {
	cacheDir := i.configMgr.GetPaths().CacheDir
	if !isWithinDir(currentInstallPath, cacheDir) {
		return fmt.Errorf("path %q is not inside cache directory %q", currentInstallPath, cacheDir)
	}

	absCurrent, err := filepath.Abs(filepath.Clean(currentInstallPath))
	if err != nil {
		return fmt.Errorf("failed to resolve current install path: %w", err)
	}

	evalCurrent, err := filepath.EvalSymlinks(absCurrent)
	if err != nil {
		return fmt.Errorf("failed to evaluate symlinks for current install path: %w", err)
	}

	parent := filepath.Dir(evalCurrent)

	absCacheDir, _ := filepath.Abs(filepath.Clean(cacheDir))
	evalCacheDir, err := filepath.EvalSymlinks(absCacheDir)
	if err != nil {
		evalCacheDir = absCacheDir
	}

	sep := string(filepath.Separator)
	if !strings.HasPrefix(parent, evalCacheDir+sep) {
		return fmt.Errorf("parent directory %q is not inside cache directory %q", parent, cacheDir)
	}

	parentRel := parent[len(evalCacheDir)+1:]
	parts := strings.Split(parentRel, sep)
	if len(parts) != 2 {
		return fmt.Errorf("parent directory %q does not match expected cache/<market>/<plugin> shape", parent)
	}

	referenced, err := i.configMgr.GetAllInstallPaths()
	if err != nil {
		return fmt.Errorf("failed to load install paths: %w", err)
	}

	evalReferenced := make(map[string]bool)
	for p := range referenced {
		ep, err := filepath.Abs(filepath.Clean(p))
		if err != nil {
			continue
		}
		if ev, err := filepath.EvalSymlinks(ep); err == nil {
			ep = ev
		}
		evalReferenced[ep] = true
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read parent directory: %w", err)
	}

	var removed int
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		if !info.IsDir() {
			continue
		}

		entryPath := filepath.Join(parent, entry.Name())

		if entryPath == evalCurrent {
			continue
		}

		if evalReferenced[entryPath] {
			continue
		}

		if err := os.RemoveAll(entryPath); err != nil {
			fmt.Printf("⚠️  Failed to remove old cache %s: %v\n", entryPath, err)
			continue
		}
		removed++
	}

	if removed > 0 {
		fmt.Printf("✓ Cleaned up %d old cache version(s)\n", removed)
	}

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
	evalBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		return false
	}
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return strings.HasPrefix(evalPath, evalBase+sep)
	}
	if !os.IsNotExist(err) {
		return false
	}
	dir := absPath
	for len(dir) > len(absBase) {
		dir = filepath.Dir(dir)
		info, err := os.Lstat(dir)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return false
			}
			if !strings.HasPrefix(target, evalBase+sep) {
				return false
			}
		}
	}
	return true
}

func isRepoHealthy(path string) bool {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return false
	}
	if _, err := repo.Head(); err != nil {
		return false
	}
	return true
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
