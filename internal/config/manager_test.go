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

func TestManager_UpdateInstallRecord(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &Paths{
		BaseDir:       tmpDir,
		MarketsDir:    filepath.Join(tmpDir, "markets"),
		CacheDir:      filepath.Join(tmpDir, "cache"),
		KnownMarkets:  filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	manager := &Manager{paths: paths}

	record := &InstallRecord{
		Scope:       "user",
		InstallPath: "/tmp/cache/test-plugin/1.0.0",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
	}

	if err := manager.AddInstallRecord("test-plugin@test-market", record); err != nil {
		t.Fatalf("AddInstallRecord() error = %v", err)
	}

	now := time.Now()
	record.Disabled = true
	record.DisabledAt = now

	if err := manager.UpdateInstallRecord("test-plugin@test-market", record); err != nil {
		t.Fatalf("UpdateInstallRecord() error = %v", err)
	}

	loaded, err := manager.GetInstallRecord("test-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord() error = %v", err)
	}

	if !loaded.Disabled {
		t.Error("Expected Disabled to be true after update")
	}

	if loaded.DisabledAt.IsZero() {
		t.Error("Expected DisabledAt to be set after update")
	}

	record.Disabled = false
	record.DisabledAt = time.Time{}

	if err := manager.UpdateInstallRecord("test-plugin@test-market", record); err != nil {
		t.Fatalf("UpdateInstallRecord() error = %v", err)
	}

	loaded, err = manager.GetInstallRecord("test-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord() error = %v", err)
	}

	if loaded.Disabled {
		t.Error("Expected Disabled to be false after re-enable")
	}
}

func TestInstallRecord_DisabledBackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &Paths{
		BaseDir:       tmpDir,
		MarketsDir:    filepath.Join(tmpDir, "markets"),
		CacheDir:      filepath.Join(tmpDir, "cache"),
		KnownMarkets:  filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	manager := &Manager{paths: paths}

	oldFormat := `{
		"version": 2,
		"plugins": {
			"old-plugin@test-market": [
				{
					"scope": "user",
					"installPath": "/tmp/cache/old-plugin/1.0.0",
					"version": "1.0.0",
					"installedAt": "2026-01-01T00:00:00Z"
				}
			]
		}
	}`

	if err := os.WriteFile(paths.InstalledFile, []byte(oldFormat), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	record, err := manager.GetInstallRecord("old-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord() error = %v", err)
	}

	if record.Disabled {
		t.Error("Expected Disabled to default to false for old-format records")
	}

	if !record.DisabledAt.IsZero() {
		t.Error("Expected DisabledAt to be zero for old-format records")
	}
}

func TestManager_MutateInstallRecord(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &Paths{
		BaseDir:       tmpDir,
		MarketsDir:    filepath.Join(tmpDir, "markets"),
		CacheDir:      filepath.Join(tmpDir, "cache"),
		KnownMarkets:  filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	manager := &Manager{paths: paths}

	record := &InstallRecord{
		Scope:       "user",
		InstallPath: "/tmp/cache/test-plugin/1.0.0",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
	}
	manager.AddInstallRecord("test-plugin@test-market", record)

	if err := manager.MutateInstallRecord("test-plugin@test-market", func(r *InstallRecord) {
		r.Disabled = true
		r.DisabledAt = time.Now()
	}); err != nil {
		t.Fatalf("MutateInstallRecord() error = %v", err)
	}

	loaded, err := manager.GetInstallRecord("test-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord() error = %v", err)
	}
	if !loaded.Disabled {
		t.Error("Expected Disabled to be true after MutateInstallRecord")
	}
}

func TestManager_MutateInstallRecord_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &Paths{
		BaseDir:       tmpDir,
		MarketsDir:    filepath.Join(tmpDir, "markets"),
		CacheDir:      filepath.Join(tmpDir, "cache"),
		KnownMarkets:  filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	manager := &Manager{paths: paths}

	err := manager.MutateInstallRecord("nonexistent@test-market", func(r *InstallRecord) {
		r.Disabled = true
	})
	if err == nil {
		t.Error("Expected error for nonexistent record")
	}
}
