package marketplace

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
	gotIndexPath, err := MarketSourceIndexPath(source)
	if err != nil {
		t.Fatalf("MarketSourceIndexPath() error = %v", err)
	}
	if gotIndexPath != indexPath {
		t.Fatalf("index path = %q, want %q", gotIndexPath, indexPath)
	}
}

func TestManagerAddURLMarketSource(t *testing.T) {
	responseBody := `{
  "name": "remote-json",
  "plugins": [
    {
      "name": "remote-plugin",
      "description": "Remote plugin",
      "source": "./plugins/remote-plugin"
    }
  ]
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, responseBody)
	}))
	defer server.Close()

	marketsDir := t.TempDir()
	mgr := NewManager(marketsDir)

	mp, source, err := mgr.Add("remote-json", server.URL+"/marketplace.json")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if mp.Name != "remote-json" {
		t.Fatalf("marketplace name = %q, want remote-json", mp.Name)
	}
	if source.SourceType() != "url" {
		t.Fatalf("source type = %q, want url", source.SourceType())
	}

	installLoc := source.InstallLocation()
	if installLoc == "" {
		t.Fatal("install location is empty")
	}
	if _, err := os.Stat(installLoc); os.IsNotExist(err) {
		t.Fatalf("cached file does not exist at %s", installLoc)
	}

	indexPath, err := MarketSourceIndexPath(source)
	if err != nil {
		t.Fatalf("MarketSourceIndexPath() error = %v", err)
	}
	if indexPath != installLoc {
		t.Fatalf("index path = %q, want %q", indexPath, installLoc)
	}

	absMarketsDir, _ := filepath.Abs(filepath.Clean(marketsDir))
	if !filepath.IsAbs(indexPath) {
		t.Fatalf("index path should be absolute: %s", indexPath)
	}
	if indexPath == absMarketsDir {
		t.Fatal("index path should not be the markets directory itself")
	}
}

func TestManagerAddURLWithHeaders(t *testing.T) {
	receivedAuth := ""
	responseBody := `{
  "name": "header-market",
  "plugins": [
    {
      "name": "header-plugin",
      "description": "Plugin with auth",
      "source": "./plugins/header-plugin"
    }
  ]
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, responseBody)
	}))
	defer server.Close()

	marketsDir := t.TempDir()
	mgr := NewManager(marketsDir)

	source := &URLMarketSource{
		URL:     server.URL + "/marketplace.json",
		Headers: map[string]string{"Authorization": "Bearer test-token"},
	}

	mp, resultSource, err := mgr.AddSource("header-market", source)
	if err != nil {
		t.Fatalf("AddSource() error = %v", err)
	}

	if mp.Name != "header-market" {
		t.Fatalf("marketplace name = %q, want header-market", mp.Name)
	}
	if receivedAuth != "Bearer test-token" {
		t.Fatalf("received Authorization = %q, want Bearer test-token", receivedAuth)
	}

	urlSource, ok := resultSource.(*URLMarketSource)
	if !ok {
		t.Fatalf("expected *URLMarketSource, got %T", resultSource)
	}
	if urlSource.Headers["Authorization"] != "Bearer test-token" {
		t.Fatalf("headers not preserved: %v", urlSource.Headers)
	}
}

func TestManagerRemoveSource(t *testing.T) {
	t.Run("URL marketplace removes cached file", func(t *testing.T) {
		marketsDir := t.TempDir()
		mgr := NewManager(marketsDir)

		responseBody := `{"name":"rm-test","plugins":[{"name":"p1","description":"p","source":"./p"}]}`
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, responseBody)
		}))
		defer server.Close()

		_, source, err := mgr.Add("rm-test", server.URL+"/marketplace.json")
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		cachePath := source.InstallLocation()
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			t.Fatal("cache file should exist before removal")
		}

		if err := mgr.RemoveSource("rm-test", source); err != nil {
			t.Fatalf("RemoveSource() error = %v", err)
		}
		if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
			t.Fatal("cache file should be removed")
		}
	})

	t.Run("local marketplace does not remove user path", func(t *testing.T) {
		marketsDir := t.TempDir()
		mgr := NewManager(marketsDir)

		userPath := t.TempDir()
		indexPath := filepath.Join(userPath, ".claude-plugin", "marketplace.json")
		os.MkdirAll(filepath.Dir(indexPath), 0755)
		os.WriteFile(indexPath, []byte(`{"name":"local","plugins":[]}`), 0644)

		_, source, err := mgr.Add("local-test", userPath)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		if err := mgr.RemoveSource("local-test", source); err != nil {
			t.Fatalf("RemoveSource() error = %v", err)
		}
		if _, err := os.Stat(userPath); os.IsNotExist(err) {
			t.Fatal("user path should NOT be removed for local market")
		}
	})

	t.Run("file marketplace does not remove user path", func(t *testing.T) {
		marketsDir := t.TempDir()
		mgr := NewManager(marketsDir)

		userDir := t.TempDir()
		indexPath := filepath.Join(userDir, "marketplace.json")
		os.WriteFile(indexPath, []byte(`{"name":"ftest","plugins":[]}`), 0644)

		_, source, err := mgr.Add("file-test", indexPath)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		if err := mgr.RemoveSource("file-test", source); err != nil {
			t.Fatalf("RemoveSource() error = %v", err)
		}
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			t.Fatal("user file should NOT be removed for file market")
		}
	})
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
