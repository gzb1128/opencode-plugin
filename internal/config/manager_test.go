package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestManager(t *testing.T) (*Manager, *Paths) {
	t.Helper()
	tmpDir := t.TempDir()
	paths := &Paths{
		BaseDir:       tmpDir,
		MarketsDir:    filepath.Join(tmpDir, "markets"),
		CacheDir:      filepath.Join(tmpDir, "cache"),
		KnownMarkets:  filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile: filepath.Join(tmpDir, "installed_plugins.json"),
	}
	return &Manager{paths: paths}, paths
}

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

func TestInstallRecord_DisabledBackwardCompatibility(t *testing.T) {
	manager, paths := setupTestManager(t)

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
	manager, _ := setupTestManager(t)

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
	manager, _ := setupTestManager(t)

	err := manager.MutateInstallRecord("nonexistent@test-market", func(r *InstallRecord) {
		r.Disabled = true
	})
	if err == nil {
		t.Error("Expected error for nonexistent record")
	}
}

func TestInstallRecord_EnabledSerializationOmitsDisabledAt(t *testing.T) {
	record := InstallRecord{
		Scope:       "user",
		InstallPath: "/tmp/cache/test-plugin/1.0.0",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		Disabled:    false,
	}

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	output := string(data)
	if !strings.Contains(output, `"disabled":false`) {
		t.Errorf("Expected disabled=false to be serialized, got %s", output)
	}
	if strings.Contains(output, "disabledAt") {
		t.Errorf("Expected disabledAt to be omitted for enabled records, got %s", output)
	}
}

func TestManager_MutateInstallRecord_SkipsWriteOnNoChange(t *testing.T) {
	manager, paths := setupTestManager(t)

	record := &InstallRecord{
		Scope:       "user",
		InstallPath: "/tmp/cache/test-plugin/1.0.0",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		Disabled:    true,
		DisabledAt:  time.Now(),
	}
	manager.AddInstallRecord("test-plugin@test-market", record)

	dataBefore, _ := os.ReadFile(paths.InstalledFile)

	if err := manager.MutateInstallRecord("test-plugin@test-market", func(r *InstallRecord) {
	}); err != nil {
		t.Fatalf("MutateInstallRecord() error = %v", err)
	}

	dataAfter, _ := os.ReadFile(paths.InstalledFile)

	if string(dataBefore) != string(dataAfter) {
		t.Error("file should not be rewritten when mutation makes no change")
	}
}
