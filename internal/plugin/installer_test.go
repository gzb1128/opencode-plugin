package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
