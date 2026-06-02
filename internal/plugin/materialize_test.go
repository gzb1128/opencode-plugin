package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
)

func newTestConfigManager(t *testing.T) *config.Manager {
	t.Helper()
	tmpDir := t.TempDir()
	paths := config.TestEnvironment(tmpDir).Paths()
	os.MkdirAll(paths.BaseDir, 0755)
	os.MkdirAll(paths.CacheDir, 0755)
	return config.NewManagerWithPath(paths)
}

func TestMaterializePlugin_LocalWithFallbackManifest(t *testing.T) {
	tmpDir := t.TempDir()
	marketPath := filepath.Join(tmpDir, "market")
	pluginSrcPath := filepath.Join(marketPath, "plugins", "tool")
	os.MkdirAll(pluginSrcPath, 0755)
	os.WriteFile(filepath.Join(pluginSrcPath, "skill.md"), []byte("# Skill"), 0644)

	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "tool",
      "description": "Tool plugin",
      "version": "2.0.0",
      "source": "./plugins/tool",
      "skills": ["skill1"],
      "commands": ["cmd1"],
      "agents": ["agent1"],
      "mcpServers": {
        "svc": { "command": "node", "args": ["server.js"] }
      },
      "hooks": { "preInstall": "script.sh" },
      "outputStyles": ["dark"]
    }
  ]
}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0644)

	mp, err := marketplace.ParseMarketplaceIndex(indexPath)
	if err != nil {
		t.Fatalf("ParseMarketplaceIndex() error = %v", err)
	}

	src := &marketplace.FileMarketSource{}
	src.SetInstallLocation(marketPath)

	resolved := &marketplace.ResolvedPlugin{
		Plugin:      &mp.Plugins[0],
		Market:      src,
		MarketName:  "test-market",
		Marketplace: mp,
	}

	mgr := newTestConfigManager(t)

	installer := &Installer{
		resolver:  NewVersionResolver(),
		configMgr: mgr,
	}

	mat, err := installer.materializePlugin(resolved, InstallOptions{MarketName: "test-market"})
	if err != nil {
		t.Fatalf("materializePlugin() error = %v", err)
	}

	if mat.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", mat.Version)
	}

	manifestData, err := os.ReadFile(mat.ManifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	if manifest["name"] != "tool" {
		t.Errorf("manifest name = %v, want tool", manifest["name"])
	}
	if manifest["version"] != "2.0.0" {
		t.Errorf("manifest version = %v, want 2.0.0", manifest["version"])
	}
	if _, ok := manifest["mcpServers"]; !ok {
		t.Error("manifest should contain mcpServers")
	}
	if _, ok := manifest["skills"]; !ok {
		t.Error("manifest should contain skills")
	}
	if _, ok := manifest["commands"]; !ok {
		t.Error("manifest should contain commands")
	}
	if _, ok := manifest["agents"]; !ok {
		t.Error("manifest should contain agents")
	}
	if _, ok := manifest["hooks"]; ok {
		t.Error("manifest should NOT contain deferred field hooks")
	}
	if _, ok := manifest["outputStyles"]; ok {
		t.Error("manifest should NOT contain deferred field outputStyles")
	}
}

func TestMaterializePlugin_ExistingManifest(t *testing.T) {
	tmpDir := t.TempDir()
	marketPath := filepath.Join(tmpDir, "market")
	pluginSrcPath := filepath.Join(marketPath, "plugins", "tool")
	manifestDir := filepath.Join(pluginSrcPath, ".claude-plugin")
	os.MkdirAll(manifestDir, 0755)
	os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"tool","version":"3.0.0"}`), 0644)

	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "tool",
      "description": "Tool plugin",
      "source": "./plugins/tool"
    }
  ]
}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0644)

	mp, err := marketplace.ParseMarketplaceIndex(indexPath)
	if err != nil {
		t.Fatalf("ParseMarketplaceIndex() error = %v", err)
	}

	src := &marketplace.FileMarketSource{}
	src.SetInstallLocation(marketPath)

	resolved := &marketplace.ResolvedPlugin{
		Plugin:      &mp.Plugins[0],
		Market:      src,
		MarketName:  "test-market",
		Marketplace: mp,
	}

	mgr := newTestConfigManager(t)

	installer := &Installer{
		resolver:  NewVersionResolver(),
		configMgr: mgr,
	}

	mat, err := installer.materializePlugin(resolved, InstallOptions{MarketName: "test-market"})
	if err != nil {
		t.Fatalf("materializePlugin() error = %v", err)
	}

	if mat.Version != "3.0.0" {
		t.Errorf("Version = %q, want 3.0.0", mat.Version)
	}
}

func TestMaterializePlugin_FallbackManifestIncludesDisplayName(t *testing.T) {
	tmpDir := t.TempDir()
	marketPath := filepath.Join(tmpDir, "market")
	pluginSrcPath := filepath.Join(marketPath, "plugins", "tool")
	os.MkdirAll(pluginSrcPath, 0755)

	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "tool",
      "displayName": "Tool Pro",
      "description": "A tool plugin",
      "version": "1.0.0",
      "source": "./plugins/tool"
    }
  ]
}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0644)

	mp, err := marketplace.ParseMarketplaceIndex(indexPath)
	if err != nil {
		t.Fatalf("ParseMarketplaceIndex() error = %v", err)
	}

	src := &marketplace.FileMarketSource{}
	src.SetInstallLocation(marketPath)

	resolved := &marketplace.ResolvedPlugin{
		Plugin:      &mp.Plugins[0],
		Market:      src,
		MarketName:  "test-market",
		Marketplace: mp,
	}

	mgr := newTestConfigManager(t)

	installer := &Installer{
		resolver:  NewVersionResolver(),
		configMgr: mgr,
	}

	mat, err := installer.materializePlugin(resolved, InstallOptions{MarketName: "test-market"})
	if err != nil {
		t.Fatalf("materializePlugin() error = %v", err)
	}

	manifestData, err := os.ReadFile(mat.ManifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	if manifest["name"] != "tool" {
		t.Errorf("manifest name = %v, want tool", manifest["name"])
	}
	if manifest["displayName"] != "Tool Pro" {
		t.Errorf("manifest displayName = %v, want 'Tool Pro'", manifest["displayName"])
	}
}

func TestMaterializePlugin_FallbackManifestOmitsDisplayNameWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	marketPath := filepath.Join(tmpDir, "market")
	pluginSrcPath := filepath.Join(marketPath, "plugins", "tool")
	os.MkdirAll(pluginSrcPath, 0755)

	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "tool",
      "description": "A tool plugin",
      "source": "./plugins/tool"
    }
  ]
}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0644)

	mp, err := marketplace.ParseMarketplaceIndex(indexPath)
	if err != nil {
		t.Fatalf("ParseMarketplaceIndex() error = %v", err)
	}

	src := &marketplace.FileMarketSource{}
	src.SetInstallLocation(marketPath)

	resolved := &marketplace.ResolvedPlugin{
		Plugin:      &mp.Plugins[0],
		Market:      src,
		MarketName:  "test-market",
		Marketplace: mp,
	}

	mgr := newTestConfigManager(t)

	installer := &Installer{
		resolver:  NewVersionResolver(),
		configMgr: mgr,
	}

	mat, err := installer.materializePlugin(resolved, InstallOptions{MarketName: "test-market"})
	if err != nil {
		t.Fatalf("materializePlugin() error = %v", err)
	}

	manifestData, err := os.ReadFile(mat.ManifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	if _, ok := manifest["displayName"]; ok {
		t.Error("manifest should NOT contain displayName when not set")
	}
}
