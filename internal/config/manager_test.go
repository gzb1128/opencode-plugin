package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManager_KnownMarkets(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &Paths{
		BaseDir:       tmpDir,
		MarketsDir:    filepath.Join(tmpDir, "markets"),
		CacheDir:      filepath.Join(tmpDir, "cache"),
		KnownMarkets:  filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	for _, dir := range []string{paths.MarketsDir, paths.CacheDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	manager := &Manager{paths: paths}

	markets, err := manager.LoadKnownMarkets()
	if err != nil {
		t.Fatalf("LoadKnownMarkets() error = %v", err)
	}

	if len(markets) != 0 {
		t.Errorf("LoadKnownMarkets() expected empty, got %d markets", len(markets))
	}

	marketSrc := map[string]interface{}{
		"source":          "github",
		"repo":            "test/marketplace",
		"url":             "https://github.com/test/marketplace.git",
		"installLocation": filepath.Join(tmpDir, "markets", "test"),
	}

	if err := manager.AddKnownMarket("test", marketSrc); err != nil {
		t.Fatalf("AddKnownMarket() error = %v", err)
	}

	markets, err = manager.LoadKnownMarkets()
	if err != nil {
		t.Fatalf("LoadKnownMarkets() error = %v", err)
	}

	if len(markets) != 1 {
		t.Errorf("LoadKnownMarkets() expected 1 market, got %d", len(markets))
	}

	if _, ok := markets["test"]; !ok {
		t.Error("LoadKnownMarkets() expected market 'test'")
	}

	if err := manager.RemoveKnownMarket("test"); err != nil {
		t.Fatalf("RemoveKnownMarket() error = %v", err)
	}

	markets, err = manager.LoadKnownMarkets()
	if err != nil {
		t.Fatalf("LoadKnownMarkets() error = %v", err)
	}

	if len(markets) != 0 {
		t.Errorf("LoadKnownMarkets() expected 0 markets, got %d", len(markets))
	}
}

func TestManager_InstalledPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &Paths{
		BaseDir:       tmpDir,
		MarketsDir:    filepath.Join(tmpDir, "markets"),
		CacheDir:      filepath.Join(tmpDir, "cache"),
		KnownMarkets:  filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	manager := &Manager{paths: paths}

	installed, err := manager.LoadInstalledPlugins()
	if err != nil {
		t.Fatalf("LoadInstalledPlugins() error = %v", err)
	}

	if installed.Version != 2 {
		t.Errorf("LoadInstalledPlugins() version = %v, want 2", installed.Version)
	}

	if len(installed.Plugins) != 0 {
		t.Errorf("LoadInstalledPlugins() expected empty, got %d plugins", len(installed.Plugins))
	}

	record := &InstallRecord{
		Scope:       "user",
		InstallPath: "/tmp/cache/test-plugin/1.0.0",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		LastUpdated: time.Now(),
	}

	if err := manager.AddInstallRecord("test-plugin@test-market", record); err != nil {
		t.Fatalf("AddInstallRecord() error = %v", err)
	}

	loaded, err := manager.GetInstallRecord("test-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord() error = %v", err)
	}

	if loaded.Version != "1.0.0" {
		t.Errorf("GetInstallRecord() version = %v, want 1.0.0", loaded.Version)
	}

	if err := manager.RemoveInstallRecord("test-plugin@test-market"); err != nil {
		t.Fatalf("RemoveInstallRecord() error = %v", err)
	}

	_, err = manager.GetInstallRecord("test-plugin@test-market")
	if err == nil {
		t.Error("GetInstallRecord() expected error after removal")
	}
}
