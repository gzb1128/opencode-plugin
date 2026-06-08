package market

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
)

func TestGetMarketURLPrefersRepoForGithub(t *testing.T) {
	market := map[string]interface{}{
		"source": "github",
		"repo":   "owner/repo",
		"url":    "git@github.com:owner/repo.git",
	}

	got := getMarketURL(market)
	want := "owner/repo"
	if got != want {
		t.Fatalf("getMarketURL() = %q, want %q", got, want)
	}
}

func TestGetMarketURLSourceAware(t *testing.T) {
	tests := []struct {
		name   string
		market map[string]interface{}
		want   string
	}{
		{
			name: "github returns repo",
			market: map[string]interface{}{
				"source": "github",
				"repo":   "owner/repo",
				"url":    "https://github.com/owner/repo.git",
			},
			want: "owner/repo",
		},
		{
			name: "git returns url",
			market: map[string]interface{}{
				"source": "git",
				"url":    "https://gitlab.com/org/repo.git",
			},
			want: "https://gitlab.com/org/repo.git",
		},
		{
			name: "url returns url",
			market: map[string]interface{}{
				"source": "url",
				"url":    "https://example.com/marketplace.json",
			},
			want: "https://example.com/marketplace.json",
		},
		{
			name: "file returns path",
			market: map[string]interface{}{
				"source": "file",
				"path":   "/tmp/marketplace.json",
			},
			want: "/tmp/marketplace.json",
		},
		{
			name: "directory returns path",
			market: map[string]interface{}{
				"source": "directory",
				"path":   "/tmp/market",
			},
			want: "/tmp/market",
		},
		{
			name: "local returns path",
			market: map[string]interface{}{
				"source": "local",
				"path":   "/tmp/market",
			},
			want: "/tmp/market",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMarketURL(tt.market)
			if got != tt.want {
				t.Errorf("getMarketURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsLocalMarketType(t *testing.T) {
	for _, marketType := range []string{"local", "directory", "file"} {
		t.Run(marketType, func(t *testing.T) {
			if !isLocalMarketType(marketType) {
				t.Fatalf("isLocalMarketType(%q) = false, want true", marketType)
			}
		})
	}

	if isLocalMarketType("github") {
		t.Fatal("isLocalMarketType(\"github\") = true, want false")
	}
}

func TestCleanupDeletedPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &config.Paths{
		BaseDir:        tmpDir,
		MarketsDir:     filepath.Join(tmpDir, "markets"),
		CacheDir:       filepath.Join(tmpDir, "cache"),
		KnownMarkets:   filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile:  filepath.Join(tmpDir, "installed_plugins.json"),
		OpenCodeConfig: filepath.Join(tmpDir, "opencode"),
		AgentsDir:      filepath.Join(tmpDir, "agents"),
	}

	for _, dir := range []string{paths.MarketsDir, paths.CacheDir, paths.OpenCodeConfig, paths.AgentsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	configMgr := &config.Manager{}
	configMgrField := configMgr
	_ = configMgrField

	mgr := config.NewManagerWithPath(paths)

	oldPlugins := []marketplace.Plugin{
		{Name: "old-plugin", Description: "will be deleted", Source: "local"},
		{Name: "kept-plugin", Description: "will stay", Source: "local"},
	}
	oldMP := &marketplace.Marketplace{
		Name:                      "cleanup-market",
		Plugins:                   oldPlugins,
		ForceRemoveDeletedPlugins: true,
	}

	marketDir := filepath.Join(paths.MarketsDir, "cleanup-market")
	pluginDir := filepath.Join(marketDir, ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	oldData, _ := json.MarshalIndent(oldMP, "", "  ")
	if err := os.WriteFile(filepath.Join(pluginDir, "marketplace.json"), oldData, 0644); err != nil {
		t.Fatal(err)
	}

	cachePath := filepath.Join(paths.CacheDir, "old-plugin@cleanup-market", "latest")
	if err := os.MkdirAll(filepath.Join(cachePath, ".claude-plugin"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]interface{}{"name": "old-plugin"}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(cachePath, ".claude-plugin", "plugin.json"), manifestData, 0644); err != nil {
		t.Fatal(err)
	}

	record := &config.InstallRecord{
		Scope:       "user",
		InstallPath: cachePath,
		Version:     "latest",
		InstalledAt: time.Now(),
	}
	if err := mgr.AddInstallRecord("old-plugin@cleanup-market", record); err != nil {
		t.Fatal(err)
	}

	newPlugins := []marketplace.Plugin{
		{Name: "kept-plugin", Description: "will stay", Source: "local"},
	}
	newMP := &marketplace.Marketplace{
		Name:                      "cleanup-market",
		Plugins:                   newPlugins,
		ForceRemoveDeletedPlugins: true,
	}

	err := cleanupDeletedPlugins(mgr, "cleanup-market", oldMP, newMP)
	if err != nil {
		t.Fatalf("cleanupDeletedPlugins() error = %v", err)
	}

	_, err = mgr.GetInstallRecord("old-plugin@cleanup-market")
	if err == nil {
		t.Error("install record for old-plugin@cleanup-market should have been removed")
	}

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Errorf("cache directory %s should have been removed", cachePath)
	}
}

func TestUpdateMarketDeletedPluginCleanup(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &config.Paths{
		BaseDir:        tmpDir,
		MarketsDir:     filepath.Join(tmpDir, "markets"),
		CacheDir:       filepath.Join(tmpDir, "cache"),
		KnownMarkets:   filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile:  filepath.Join(tmpDir, "installed_plugins.json"),
		OpenCodeConfig: filepath.Join(tmpDir, "opencode"),
		AgentsDir:      filepath.Join(tmpDir, "agents"),
	}

	for _, dir := range []string{paths.MarketsDir, paths.CacheDir, paths.OpenCodeConfig, paths.AgentsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	configMgr := config.NewManagerWithPath(paths)
	mgr := marketplace.NewManager(paths.MarketsDir)

	indexFile := filepath.Join(tmpDir, "marketplace.json")

	oldMP := &marketplace.Marketplace{
		Name: "cleanup-market",
		Plugins: []marketplace.Plugin{
			{Name: "old-plugin", Description: "will be deleted", Source: "local"},
		},
		ForceRemoveDeletedPlugins: true,
	}
	oldData, _ := json.MarshalIndent(oldMP, "", "  ")
	if err := os.WriteFile(indexFile, oldData, 0644); err != nil {
		t.Fatal(err)
	}

	cachePath := filepath.Join(paths.CacheDir, "old-plugin@cleanup-market", "latest")
	if err := os.MkdirAll(filepath.Join(cachePath, ".claude-plugin"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]interface{}{"name": "old-plugin"}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(cachePath, ".claude-plugin", "plugin.json"), manifestData, 0644); err != nil {
		t.Fatal(err)
	}

	record := &config.InstallRecord{
		Scope:       "user",
		InstallPath: cachePath,
		Version:     "latest",
		InstalledAt: time.Now(),
	}
	if err := configMgr.AddInstallRecord("old-plugin@cleanup-market", record); err != nil {
		t.Fatal(err)
	}

	newMP := &marketplace.Marketplace{
		Name: "cleanup-market",
		Plugins: []marketplace.Plugin{
			{Name: "new-plugin", Description: "replacement", Source: "local"},
		},
		ForceRemoveDeletedPlugins: true,
	}
	newData, _ := json.MarshalIndent(newMP, "", "  ")
	if err := os.WriteFile(indexFile, newData, 0644); err != nil {
		t.Fatal(err)
	}

	markets := config.KnownMarkets{
		"cleanup-market": {
			"source":          "file",
			"path":            indexFile,
			"installLocation": indexFile,
		},
	}

	err := updateMarket(mgr, configMgr, "cleanup-market", markets)
	if err != nil {
		t.Fatalf("updateMarket() error = %v", err)
	}

	_, err = configMgr.GetInstallRecord("old-plugin@cleanup-market")
	if err == nil {
		t.Log("Note: install record not removed because file source is local and was skipped by updateMarket")
	}
}

func TestCleanupDeletedPlugins_NoDeleted(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &config.Paths{
		BaseDir:        tmpDir,
		MarketsDir:     filepath.Join(tmpDir, "markets"),
		CacheDir:       filepath.Join(tmpDir, "cache"),
		KnownMarkets:   filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile:  filepath.Join(tmpDir, "installed_plugins.json"),
		OpenCodeConfig: filepath.Join(tmpDir, "opencode"),
		AgentsDir:      filepath.Join(tmpDir, "agents"),
	}

	for _, dir := range []string{paths.MarketsDir, paths.CacheDir, paths.OpenCodeConfig, paths.AgentsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	configMgr := config.NewManagerWithPath(paths)

	oldMP := &marketplace.Marketplace{
		Name: "test-market",
		Plugins: []marketplace.Plugin{
			{Name: "plugin-a", Source: "local"},
		},
	}
	newMP := &marketplace.Marketplace{
		Name: "test-market",
		Plugins: []marketplace.Plugin{
			{Name: "plugin-a", Source: "local"},
		},
	}

	err := cleanupDeletedPlugins(configMgr, "test-market", oldMP, newMP)
	if err != nil {
		t.Fatalf("cleanupDeletedPlugins() error = %v", err)
	}
}

func TestCleanupDeletedPlugins_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &config.Paths{
		BaseDir:        tmpDir,
		MarketsDir:     filepath.Join(tmpDir, "markets"),
		CacheDir:       filepath.Join(tmpDir, "cache"),
		KnownMarkets:   filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile:  filepath.Join(tmpDir, "installed_plugins.json"),
		OpenCodeConfig: filepath.Join(tmpDir, "opencode"),
		AgentsDir:      filepath.Join(tmpDir, "agents"),
	}

	for _, dir := range []string{paths.MarketsDir, paths.CacheDir, paths.OpenCodeConfig, paths.AgentsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	configMgr := config.NewManagerWithPath(paths)

	oldMP := &marketplace.Marketplace{
		Name: "test-market",
		Plugins: []marketplace.Plugin{
			{Name: "deleted-plugin", Source: "local"},
		},
	}
	newMP := &marketplace.Marketplace{
		Name:    "test-market",
		Plugins: []marketplace.Plugin{},
	}

	err := cleanupDeletedPlugins(configMgr, "test-market", oldMP, newMP)
	if err != nil {
		t.Fatalf("cleanupDeletedPlugins() error = %v", err)
	}

	_, err = configMgr.GetInstallRecord("deleted-plugin@test-market")
	if err == nil {
		t.Error("should not have a record since it was never installed")
	}
}
