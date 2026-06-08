package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/opencode/plugin-cli/internal/mcp"
	"github.com/opencode/plugin-cli/internal/opencode"
)

func setupInstallerTest(t *testing.T) (*Installer, string) {
	t.Helper()

	tmpDir := t.TempDir()
	paths := config.TestEnvironment(tmpDir).Paths()
	os.MkdirAll(paths.BaseDir, 0755)
	os.MkdirAll(paths.MarketsDir, 0755)
	os.MkdirAll(paths.CacheDir, 0755)
	os.MkdirAll(paths.AgentsDir, 0755)

	mgr := config.NewManagerWithPath(paths)

	installer := &Installer{
		resolver:   NewVersionResolver(),
		configMgr:  mgr,
		linker:     opencode.NewLinker(paths.AgentsDir),
		marketMgr:  marketplace.NewManager(paths.MarketsDir),
		mcpManager: mcp.NewManager(paths.OpenCodeConfig, paths.PluginDataDir),
	}

	return installer, paths.BaseDir
}

func setupInstalledPlugin(t *testing.T, installer *Installer, pluginName, marketName string) string {
	t.Helper()
	paths := installer.configMgr.GetPaths()

	cachePath := filepath.Join(paths.CacheDir, marketName, pluginName, "1.0.0")
	skillsDir := filepath.Join(cachePath, "skills")
	os.MkdirAll(skillsDir, 0755)
	os.WriteFile(filepath.Join(skillsDir, "test-skill.md"), []byte("# Test Skill"), 0644)

	manifestDir := filepath.Join(cachePath, ".claude-plugin")
	os.MkdirAll(manifestDir, 0755)
	os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"`+pluginName+`","version":"1.0.0"}`), 0644)

	key := fmt.Sprintf("%s@%s", pluginName, marketName)
	record := &config.InstallRecord{
		Scope:       "user",
		InstallPath: cachePath,
		Version:     "1.0.0",
		InstalledAt: time.Now(),
	}
	installer.configMgr.AddInstallRecord(key, record)

	manifest, _ := opencode.ReadManifest(filepath.Join(cachePath, ".claude-plugin", "plugin.json"))
	installer.linker.CreateSymlinksFromManifest(cachePath, manifest, false)

	return cachePath
}

func TestInstaller_Disable(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	cachePath := setupInstalledPlugin(t, installer, "test-plugin", "test-market")
	paths := installer.configMgr.GetPaths()

	symlinkPath := filepath.Join(paths.AgentsDir, "skills", "test-skill.md")
	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		t.Fatalf("symlink should exist before disable")
	}

	if err := installer.Disable("test-plugin", "test-market"); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("symlink should be removed after disable")
	}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("cache directory should still exist after disable")
	}

	record, err := installer.configMgr.GetInstallRecord("test-plugin@test-market")
	if err != nil {
		t.Fatalf("install record should still exist: %v", err)
	}
	if !record.Disabled {
		t.Error("record should be marked as disabled")
	}
}

func TestInstaller_Disable_Idempotent(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	setupInstalledPlugin(t, installer, "test-plugin", "test-market")

	if err := installer.Disable("test-plugin", "test-market"); err != nil {
		t.Fatalf("first disable failed: %v", err)
	}

	if err := installer.Disable("test-plugin", "test-market"); err != nil {
		t.Fatalf("second disable (idempotent) failed: %v", err)
	}

	record, _ := installer.configMgr.GetInstallRecord("test-plugin@test-market")
	if !record.Disabled {
		t.Error("record should still be disabled after idempotent retry")
	}
}

func TestInstaller_Enable(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()
	setupInstalledPlugin(t, installer, "test-plugin", "test-market")

	if err := installer.Disable("test-plugin", "test-market"); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	symlinkPath := filepath.Join(paths.AgentsDir, "skills", "test-skill.md")
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("symlink should not exist after disable")
	}

	if err := installer.Enable("test-plugin", "test-market", false); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}

	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		t.Error("symlink should exist after enable")
	}

	record, err := installer.configMgr.GetInstallRecord("test-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord() error = %v", err)
	}
	if record.Disabled {
		t.Error("record should not be disabled after enable")
	}
}

func TestInstaller_Enable_PreservesExistingMCPConfig(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()
	cachePath := setupInstalledPlugin(t, installer, "test-plugin", "test-market")

	manifestPath := filepath.Join(cachePath, ".claude-plugin", "plugin.json")
	manifest := `{
		"name": "test-plugin",
		"version": "1.0.0",
		"mcpServers": {
			"server": {
				"command": "node",
				"args": ["default.js"]
			}
		}
	}`
	os.WriteFile(manifestPath, []byte(manifest), 0644)

	installer.linker.RemoveSymlinks(cachePath)
	installer.configMgr.MutateInstallRecord("test-plugin@test-market", func(record *config.InstallRecord) {
		record.Disabled = true
		record.DisabledAt = time.Now()
	})

	existingContent := `{
		"mcp": {
			"test-plugin.server": {
				"type": "local",
				"command": ["node", "custom.js"],
				"enabled": false,
				"environment": {"USER_EDIT": "yes"}
			}
		}
	}`
	os.MkdirAll(paths.OpenCodeConfig, 0755)
	os.WriteFile(filepath.Join(paths.OpenCodeConfig, "opencode.json"), []byte(existingContent), 0644)

	if err := installer.Enable("test-plugin", "test-market", false); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(paths.OpenCodeConfig, "opencode.json"))
	var config map[string]json.RawMessage
	json.Unmarshal(data, &config)
	var mcpConfig map[string]mcp.OpenCodeMCPServer
	json.Unmarshal(config["mcp"], &mcpConfig)

	server := mcpConfig["test-plugin.server"]
	if !server.Enabled {
		t.Fatal("expected MCP server to be enabled")
	}
	if len(server.Command) != 2 || server.Command[1] != "custom.js" {
		t.Fatalf("expected user-modified command to be preserved, got %v", server.Command)
	}
	if server.Environment["USER_EDIT"] != "yes" {
		t.Fatal("expected user-modified environment to be preserved")
	}
}

func TestInstaller_Enable_Idempotent(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	setupInstalledPlugin(t, installer, "test-plugin", "test-market")

	installer.Disable("test-plugin", "test-market")

	if err := installer.Enable("test-plugin", "test-market", false); err != nil {
		t.Fatalf("first enable failed: %v", err)
	}

	if err := installer.Enable("test-plugin", "test-market", false); err != nil {
		t.Fatalf("second enable (idempotent) failed: %v", err)
	}

	record, _ := installer.configMgr.GetInstallRecord("test-plugin@test-market")
	if record.Disabled {
		t.Error("record should still be enabled after idempotent retry")
	}
}

func TestInstaller_Enable_AfterPartialDisable(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()
	cachePath := setupInstalledPlugin(t, installer, "test-plugin", "test-market")

	symlinkPath := filepath.Join(paths.AgentsDir, "skills", "test-skill.md")

	key := "test-plugin@test-market"
	installer.configMgr.MutateInstallRecord(key, func(record *config.InstallRecord) {
		record.Disabled = true
		record.DisabledAt = time.Now()
	})

	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		t.Fatal("symlink should still exist (only record was changed)")
	}

	if err := installer.Enable("test-plugin", "test-market", false); err != nil {
		t.Fatalf("Enable after partial disable failed: %v", err)
	}

	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		t.Error("symlink should still exist after enable (was never removed)")
	}

	record, _ := installer.configMgr.GetInstallRecord(key)
	if record.Disabled {
		t.Error("record should be enabled now")
	}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("cache should still exist")
	}
}

func TestInstaller_Disable_NotInstalled(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	err := installer.Disable("nonexistent", "market")
	if err == nil {
		t.Error("expected error when disabling non-installed plugin")
	}
}

func TestInstaller_Enable_NotInstalled(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	err := installer.Enable("nonexistent", "market", false)
	if err == nil {
		t.Error("expected error when enabling non-installed plugin")
	}
}

func TestInstaller_Enable_CacheMissing(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	key := "missing@test-market"
	record := &config.InstallRecord{
		Scope:       "user",
		InstallPath: "/nonexistent/path",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		Disabled:    true,
	}
	installer.configMgr.AddInstallRecord(key, record)

	err := installer.Enable("missing", "test-market", false)
	if err == nil {
		t.Error("expected error when enabling plugin with missing cache")
	}
}

func TestInstall_WithDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	marketPath := filepath.Join(tmpDir, "market")
	depSrcPath := filepath.Join(marketPath, "plugins", "dep")
	rootSrcPath := filepath.Join(marketPath, "plugins", "root")
	os.MkdirAll(filepath.Join(depSrcPath, "skills"), 0755)
	os.MkdirAll(filepath.Join(rootSrcPath, "skills"), 0755)
	os.WriteFile(filepath.Join(depSrcPath, "skills", "dep-skill.md"), []byte("# Dep Skill"), 0644)
	os.WriteFile(filepath.Join(rootSrcPath, "skills", "root-skill.md"), []byte("# Root Skill"), 0644)

	depManifestDir := filepath.Join(depSrcPath, ".claude-plugin")
	os.MkdirAll(depManifestDir, 0755)
	os.WriteFile(filepath.Join(depManifestDir, "plugin.json"), []byte(`{"name":"dep","version":"1.0.0"}`), 0644)

	rootManifestDir := filepath.Join(rootSrcPath, ".claude-plugin")
	os.MkdirAll(rootManifestDir, 0755)
	os.WriteFile(filepath.Join(rootManifestDir, "plugin.json"), []byte(`{"name":"root","version":"2.0.0"}`), 0644)

	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "dep",
      "description": "Dependency plugin",
      "source": "./plugins/dep"
    },
    {
      "name": "root",
      "description": "Root plugin with dep",
      "source": "./plugins/root",
      "dependencies": ["dep"]
    }
  ]
}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0755)

	installer, _ := setupInstallerTest(t)

	src := &marketplace.LocalMarketSource{Path: marketPath}
	src.SetInstallLocation(marketPath)

	if err := installer.configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(src)); err != nil {
		t.Fatalf("AddKnownMarket error: %v", err)
	}

	if err := installer.Install("root", InstallOptions{MarketName: "test-market", Scope: "user"}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	depRecord, err := installer.configMgr.GetInstallRecord("dep@test-market")
	if err != nil {
		t.Fatalf("dep not installed: %v", err)
	}
	if depRecord.Version != "1.0.0" {
		t.Errorf("dep version = %q, want 1.0.0", depRecord.Version)
	}

	rootRecord, err := installer.configMgr.GetInstallRecord("root@test-market")
	if err != nil {
		t.Fatalf("root not installed: %v", err)
	}
	if rootRecord.Version != "2.0.0" {
		t.Errorf("root version = %q, want 2.0.0", rootRecord.Version)
	}

	installer.configMgr.RemoveInstallRecord("dep@test-market")
	installer.configMgr.RemoveInstallRecord("root@test-market")
}

func TestInstall_DependencyAlreadyInstalled(t *testing.T) {
	tmpDir := t.TempDir()

	marketPath := filepath.Join(tmpDir, "market")
	depSrcPath := filepath.Join(marketPath, "plugins", "dep")
	rootSrcPath := filepath.Join(marketPath, "plugins", "root")
	os.MkdirAll(filepath.Join(depSrcPath, "skills"), 0755)
	os.MkdirAll(filepath.Join(rootSrcPath, "skills"), 0755)
	os.WriteFile(filepath.Join(depSrcPath, "skills", "dep-skill.md"), []byte("# Dep Skill"), 0644)
	os.WriteFile(filepath.Join(rootSrcPath, "skills", "root-skill.md"), []byte("# Root Skill"), 0644)

	depManifestDir := filepath.Join(depSrcPath, ".claude-plugin")
	os.MkdirAll(depManifestDir, 0755)
	os.WriteFile(filepath.Join(depManifestDir, "plugin.json"), []byte(`{"name":"dep","version":"1.0.0"}`), 0644)

	rootManifestDir := filepath.Join(rootSrcPath, ".claude-plugin")
	os.MkdirAll(rootManifestDir, 0755)
	os.WriteFile(filepath.Join(rootManifestDir, "plugin.json"), []byte(`{"name":"root","version":"2.0.0"}`), 0644)

	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "dep",
      "description": "Dependency plugin",
      "source": "./plugins/dep"
    },
    {
      "name": "root",
      "description": "Root plugin with dep",
      "source": "./plugins/root",
      "dependencies": ["dep"]
    }
  ]
}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0755)

	installer, _ := setupInstallerTest(t)

	src := &marketplace.LocalMarketSource{Path: marketPath}
	src.SetInstallLocation(marketPath)

	if err := installer.configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(src)); err != nil {
		t.Fatalf("AddKnownMarket error: %v", err)
	}

	if err := installer.Install("dep", InstallOptions{MarketName: "test-market", Scope: "user"}); err != nil {
		t.Fatalf("first install of dep failed: %v", err)
	}

	if err := installer.Install("root", InstallOptions{MarketName: "test-market", Scope: "user"}); err != nil {
		t.Fatalf("Install root with dep already installed failed: %v", err)
	}

	rootRecord, err := installer.configMgr.GetInstallRecord("root@test-market")
	if err != nil {
		t.Fatalf("root not installed: %v", err)
	}
	if rootRecord.Version != "2.0.0" {
		t.Errorf("root version = %q, want 2.0.0", rootRecord.Version)
	}

	installer.configMgr.RemoveInstallRecord("dep@test-market")
	installer.configMgr.RemoveInstallRecord("root@test-market")
}

func TestCleanupOldVersions_RemovesUnreferencedSibling(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()
	cacheDir := paths.CacheDir

	currentPath := filepath.Join(cacheDir, "market", "plugin", "v2")
	oldPath := filepath.Join(cacheDir, "market", "plugin", "v1")
	os.MkdirAll(currentPath, 0755)
	os.MkdirAll(oldPath, 0755)

	installer.configMgr.AddInstallRecord("plugin@market", &config.InstallRecord{
		InstallPath: currentPath,
		Version:     "v2",
	})

	if err := installer.CleanupOldVersions(currentPath); err != nil {
		t.Fatalf("CleanupOldVersions() error = %v", err)
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old version should have been removed")
	}

	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		t.Error("current version should still exist")
	}
}

func TestCleanupOldVersions_KeepsCurrentVersion(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()
	cacheDir := paths.CacheDir

	currentPath := filepath.Join(cacheDir, "market", "plugin", "v2")
	os.MkdirAll(currentPath, 0755)

	installer.configMgr.AddInstallRecord("plugin@market", &config.InstallRecord{
		InstallPath: currentPath,
		Version:     "v2",
	})

	if err := installer.CleanupOldVersions(currentPath); err != nil {
		t.Fatalf("CleanupOldVersions() error = %v", err)
	}

	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		t.Error("current version should still exist")
	}
}

func TestCleanupOldVersions_KeepsReferencedSibling(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()
	cacheDir := paths.CacheDir

	currentPath := filepath.Join(cacheDir, "market", "plugin", "v2")
	referencedPath := filepath.Join(cacheDir, "market", "plugin", "v1")
	os.MkdirAll(currentPath, 0755)
	os.MkdirAll(referencedPath, 0755)

	installer.configMgr.AddInstallRecord("plugin@market", &config.InstallRecord{
		InstallPath: currentPath,
		Version:     "v2",
	})
	installer.configMgr.AddInstallRecord("other-plugin@market", &config.InstallRecord{
		InstallPath: referencedPath,
		Version:     "v1",
	})

	if err := installer.CleanupOldVersions(currentPath); err != nil {
		t.Fatalf("CleanupOldVersions() error = %v", err)
	}

	if _, err := os.Stat(referencedPath); os.IsNotExist(err) {
		t.Error("referenced sibling should not be removed")
	}
}

func TestCleanupOldVersions_RefusesPathOutsideCache(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	outsidePath := filepath.Join(t.TempDir(), "plugin", "v1")
	os.MkdirAll(outsidePath, 0755)

	err := installer.CleanupOldVersions(outsidePath)
	if err == nil {
		t.Fatal("expected error for path outside cache directory")
	}
}

func TestCleanupOldVersions_RefusesMalformedPathInsideCache(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()

	shallowPath := filepath.Join(paths.CacheDir, "market")
	os.MkdirAll(shallowPath, 0755)

	err := installer.CleanupOldVersions(shallowPath)
	if err == nil {
		t.Fatal("expected error for malformed path (not deep enough)")
	}
}

func TestCleanupOldVersions_SkipsSymlinkSiblings(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()
	cacheDir := paths.CacheDir

	currentPath := filepath.Join(cacheDir, "market", "plugin", "v2")
	linkTarget := t.TempDir()
	linkPath := filepath.Join(cacheDir, "market", "plugin", "link-version")
	os.MkdirAll(currentPath, 0755)
	os.Symlink(linkTarget, linkPath)

	installer.configMgr.AddInstallRecord("plugin@market", &config.InstallRecord{
		InstallPath: currentPath,
		Version:     "v2",
	})

	if err := installer.CleanupOldVersions(currentPath); err != nil {
		t.Fatalf("CleanupOldVersions() error = %v", err)
	}

	if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
		t.Error("symlink sibling should not be removed")
	}
}

func TestCleanupOldVersions_HandlesMissingDirectory(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()
	cacheDir := paths.CacheDir

	missingPath := filepath.Join(cacheDir, "market", "plugin", "v1")

	err := installer.CleanupOldVersions(missingPath)
	if err == nil {
		t.Fatal("expected error for nonexistent current install path")
	}
}

func TestGenerateFallbackManifest_IncludesDisplayName(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	cachePath := filepath.Join(t.TempDir(), "cache", "tool@market", "v1")
	os.MkdirAll(cachePath, 0755)

	plugin := &marketplace.Plugin{
		Name:        "tool",
		DisplayName: "Tool Pro",
		Description: "A tool plugin",
	}

	if err := installer.generateFallbackManifest(plugin, cachePath); err != nil {
		t.Fatalf("generateFallbackManifest() error = %v", err)
	}

	manifestPath := filepath.Join(cachePath, ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	if manifest["name"] != "tool" {
		t.Errorf("name = %v, want tool", manifest["name"])
	}
	if manifest["displayName"] != "Tool Pro" {
		t.Errorf("displayName = %v, want 'Tool Pro'", manifest["displayName"])
	}
}

func TestGenerateFallbackManifest_SkipsDisplayNameWhenEmpty(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	cachePath := filepath.Join(t.TempDir(), "cache", "plain@market", "v1")
	os.MkdirAll(cachePath, 0755)

	plugin := &marketplace.Plugin{
		Name:        "plain",
		Description: "A plain plugin",
	}

	if err := installer.generateFallbackManifest(plugin, cachePath); err != nil {
		t.Fatalf("generateFallbackManifest() error = %v", err)
	}

	manifestPath := filepath.Join(cachePath, ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	if _, ok := manifest["displayName"]; ok {
		t.Error("displayName should not be present when empty")
	}
}

func TestInstall_DisplayNameDoesNotAffectPluginID(t *testing.T) {
	tmpDir := t.TempDir()

	marketPath := filepath.Join(tmpDir, "market")
	pluginSrcPath := filepath.Join(marketPath, "plugins", "tool")
	os.MkdirAll(filepath.Join(pluginSrcPath, "skills"), 0755)
	os.WriteFile(filepath.Join(pluginSrcPath, "skills", "tool-skill.md"), []byte("# Tool Skill"), 0644)

	manifestDir := filepath.Join(pluginSrcPath, ".claude-plugin")
	os.MkdirAll(manifestDir, 0755)
	os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"tool","version":"1.0.0"}`), 0644)

	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "tool",
      "displayName": "Tool Pro",
      "description": "A tool plugin",
      "source": "./plugins/tool"
    }
  ]
}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0755)

	installer, _ := setupInstallerTest(t)

	src := &marketplace.LocalMarketSource{Path: marketPath}
	src.SetInstallLocation(marketPath)

	if err := installer.configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(src)); err != nil {
		t.Fatalf("AddKnownMarket error: %v", err)
	}

	if err := installer.Install("tool", InstallOptions{MarketName: "test-market", Scope: "user"}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	record, err := installer.configMgr.GetInstallRecord("tool@test-market")
	if err != nil {
		t.Fatalf("plugin should be installed as 'tool@test-market': %v", err)
	}
	if record.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", record.Version)
	}

	installer.configMgr.RemoveInstallRecord("tool@test-market")
}

func TestInstall_Disabled_SkipsSymlinksAndMCP(t *testing.T) {
	tmpDir := t.TempDir()

	marketPath := filepath.Join(tmpDir, "market")
	pluginSrcPath := filepath.Join(marketPath, "plugins", "my-plugin")
	os.MkdirAll(filepath.Join(pluginSrcPath, "skills"), 0755)
	os.WriteFile(filepath.Join(pluginSrcPath, "skills", "my-skill.md"), []byte("# My Skill"), 0644)

	manifestDir := filepath.Join(pluginSrcPath, ".claude-plugin")
	os.MkdirAll(manifestDir, 0755)
	os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"my-plugin","version":"1.0.0"}`), 0644)

	marketJSON := `{
		"name": "test-market",
		"plugins": [
			{
				"name": "my-plugin",
				"description": "Test",
				"source": "./plugins/my-plugin"
			}
		]
	}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0755)

	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()

	src := &marketplace.LocalMarketSource{Path: marketPath}
	src.SetInstallLocation(marketPath)

	if err := installer.configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(src)); err != nil {
		t.Fatalf("AddKnownMarket error: %v", err)
	}

	if err := installer.Install("my-plugin", InstallOptions{MarketName: "test-market", Scope: "user", Disabled: true}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	record, err := installer.configMgr.GetInstallRecord("my-plugin@test-market")
	if err != nil {
		t.Fatalf("record should exist: %v", err)
	}
	if !record.Disabled {
		t.Error("record should be disabled")
	}

	symlinkPath := filepath.Join(paths.AgentsDir, "skills", "my-skill.md")
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("symlink should NOT exist for disabled install")
	}

	installer.configMgr.RemoveInstallRecord("my-plugin@test-market")
}

func TestInstall_DisabledDependencyNotReenabled(t *testing.T) {
	tmpDir := t.TempDir()

	marketPath := filepath.Join(tmpDir, "market")
	depSrcPath := filepath.Join(marketPath, "plugins", "dep")
	rootSrcPath := filepath.Join(marketPath, "plugins", "root")
	os.MkdirAll(filepath.Join(depSrcPath, "skills"), 0755)
	os.MkdirAll(filepath.Join(rootSrcPath, "skills"), 0755)
	os.WriteFile(filepath.Join(depSrcPath, "skills", "dep-skill.md"), []byte("# Dep Skill"), 0644)
	os.WriteFile(filepath.Join(rootSrcPath, "skills", "root-skill.md"), []byte("# Root Skill"), 0644)

	depManifestDir := filepath.Join(depSrcPath, ".claude-plugin")
	os.MkdirAll(depManifestDir, 0755)
	os.WriteFile(filepath.Join(depManifestDir, "plugin.json"), []byte(`{"name":"dep","version":"1.0.0"}`), 0644)

	rootManifestDir := filepath.Join(rootSrcPath, ".claude-plugin")
	os.MkdirAll(rootManifestDir, 0755)
	os.WriteFile(filepath.Join(rootManifestDir, "plugin.json"), []byte(`{"name":"root","version":"2.0.0"}`), 0644)

	marketJSON := `{
		"name": "test-market",
		"plugins": [
			{
				"name": "dep",
				"description": "Dependency plugin",
				"source": "./plugins/dep"
			},
			{
				"name": "root",
				"description": "Root plugin with dep",
				"source": "./plugins/root",
				"dependencies": ["dep"]
			}
		]
	}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0755)

	installer, _ := setupInstallerTest(t)

	src := &marketplace.LocalMarketSource{Path: marketPath}
	src.SetInstallLocation(marketPath)

	if err := installer.configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(src)); err != nil {
		t.Fatalf("AddKnownMarket error: %v", err)
	}

	if err := installer.Install("dep", InstallOptions{MarketName: "test-market", Scope: "user"}); err != nil {
		t.Fatalf("install dep failed: %v", err)
	}

	if err := installer.Disable("dep", "test-market"); err != nil {
		t.Fatalf("disable dep failed: %v", err)
	}

	if err := installer.Install("root", InstallOptions{MarketName: "test-market", Scope: "user"}); err != nil {
		t.Fatalf("install root failed: %v", err)
	}

	depRecord, err := installer.configMgr.GetInstallRecord("dep@test-market")
	if err != nil {
		t.Fatalf("dep record missing: %v", err)
	}
	if !depRecord.Disabled {
		t.Error("dep should remain disabled after installing root")
	}

	installer.configMgr.RemoveInstallRecord("dep@test-market")
	installer.configMgr.RemoveInstallRecord("root@test-market")
}

func TestIsWithinDir(t *testing.T) {
	cacheDir := t.TempDir()
	innerDir := filepath.Join(cacheDir, "plugin@market", "v1")
	os.MkdirAll(innerDir, 0755)

	t.Run("path inside cache dir", func(t *testing.T) {
		if !isWithinDir(innerDir, cacheDir) {
			t.Error("expected inner dir to be within cache")
		}
	})

	t.Run("path outside cache dir rejected", func(t *testing.T) {
		if isWithinDir(t.TempDir(), cacheDir) {
			t.Error("external dir should be rejected")
		}
	})

	t.Run("cache dir itself rejected", func(t *testing.T) {
		if isWithinDir(cacheDir, cacheDir) {
			t.Error("cache dir itself should be rejected (must be a child)")
		}
	})

	t.Run("symlink pointing outside cache rejected", func(t *testing.T) {
		outsideDir := t.TempDir()
		linkPath := filepath.Join(cacheDir, "escape")
		os.Symlink(outsideDir, linkPath)
		if isWithinDir(linkPath, cacheDir) {
			t.Error("symlink pointing outside cache should be rejected")
		}
	})

	t.Run("nonexistent path lexically inside cache allowed", func(t *testing.T) {
		deletedPath := filepath.Join(cacheDir, "deleted-plugin", "v1")
		if !isWithinDir(deletedPath, cacheDir) {
			t.Error("nonexistent path lexically inside cache should be allowed")
		}
	})

	t.Run("nonexistent path under symlink parent pointing outside rejected", func(t *testing.T) {
		outsideDir := t.TempDir()
		linkPath := filepath.Join(cacheDir, "link-outside")
		os.Symlink(outsideDir, linkPath)
		missingPath := filepath.Join(linkPath, "missing", "v1")
		if isWithinDir(missingPath, cacheDir) {
			t.Error("nonexistent path under symlink escaping cache should be rejected")
		}
	})
}
