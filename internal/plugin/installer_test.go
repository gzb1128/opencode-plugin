package plugin

import (
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
		mcpManager: mcp.NewManager(paths.OpenCodeConfig),
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
