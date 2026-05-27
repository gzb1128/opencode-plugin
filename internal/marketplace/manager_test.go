package marketplace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerAddFileMarketSource(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, ".claude-plugin", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		t.Fatalf("failed to create marketplace metadata dir: %v", err)
	}
	writeTestMarketplaceIndex(t, indexPath)

	mgr := NewManager(t.TempDir())
	mp, source, err := mgr.Add("file-market", indexPath)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if mp.Name != "file-market" {
		t.Fatalf("marketplace name = %q, want file-market", mp.Name)
	}
	if source.SourceType() != string(SourceTypeFile) {
		t.Fatalf("source type = %q, want file", source.SourceType())
	}
	if source.InstallLocation() != root {
		t.Fatalf("install location = %q, want %q", source.InstallLocation(), root)
	}

	plugin, foundSource, foundMarket, err := mgr.FindPlugin(map[string]MarketSource{
		"file-market": source,
	}, "local-plugin", "file-market")
	if err != nil {
		t.Fatalf("FindPlugin() error = %v", err)
	}
	if plugin.Name != "local-plugin" {
		t.Fatalf("plugin name = %q, want local-plugin", plugin.Name)
	}
	if foundSource != source {
		t.Fatal("FindPlugin() returned a different market source")
	}
	if foundMarket != "file-market" {
		t.Fatalf("market name = %q, want file-market", foundMarket)
	}
}

func TestManagerAddRootMarketplaceFile(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, "marketplace.json")
	writeTestMarketplaceIndex(t, indexPath)

	mgr := NewManager(t.TempDir())
	_, source, err := mgr.Add("root-file-market", indexPath)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if source.InstallLocation() != root {
		t.Fatalf("install location = %q, want %q", source.InstallLocation(), root)
	}
	if MarketSourceIndexPath(source) != indexPath {
		t.Fatalf("index path = %q, want %q", MarketSourceIndexPath(source), indexPath)
	}
}

func writeTestMarketplaceIndex(t *testing.T, path string) {
	t.Helper()

	data := []byte(`{
  "name": "file-market",
  "plugins": [
    {
      "name": "local-plugin",
      "description": "Local plugin",
      "source": "./plugins/local-plugin"
    }
  ]
}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write marketplace index: %v", err)
	}
}
