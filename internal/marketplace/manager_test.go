package marketplace

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

func TestMarketSourceIndexPath_CustomPath(t *testing.T) {
	t.Run("custom path resolves within install location", func(t *testing.T) {
		tmpDir := t.TempDir()
		source := &GitMarketSource{
			URL:  "/fake/repo.git",
			Path: "catalog/.claude-plugin/marketplace.json",
		}
		source.SetInstallLocation(tmpDir)

		got, err := MarketSourceIndexPath(source)
		if err != nil {
			t.Fatalf("MarketSourceIndexPath() error = %v", err)
		}

		want := filepath.Join(tmpDir, "catalog", ".claude-plugin", "marketplace.json")
		if got != want {
			t.Errorf("MarketSourceIndexPath() = %q, want %q", got, want)
		}
	})

	t.Run("path traversal returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		source := &GitMarketSource{
			URL:  "/fake/repo.git",
			Path: "../outside/marketplace.json",
		}
		source.SetInstallLocation(tmpDir)

		_, err := MarketSourceIndexPath(source)
		if err == nil {
			t.Fatal("expected error for path traversal, got nil")
		}
	})

	t.Run("absolute path returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		source := &GitMarketSource{
			URL:  "/fake/repo.git",
			Path: "/tmp/marketplace.json",
		}
		source.SetInstallLocation(tmpDir)

		_, err := MarketSourceIndexPath(source)
		if err == nil {
			t.Fatal("expected error for absolute path, got nil")
		}
	})

	t.Run("symlink escape returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		outsideDir := t.TempDir()

		subDir := filepath.Join(tmpDir, "sub")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}

		linkPath := filepath.Join(subDir, "escape")
		if err := os.Symlink(outsideDir, linkPath); err != nil {
			t.Fatal(err)
		}

		source := &GitMarketSource{
			URL:  "/fake/repo.git",
			Path: "sub/escape/marketplace.json",
		}
		source.SetInstallLocation(tmpDir)

		_, err := MarketSourceIndexPath(source)
		if err == nil {
			t.Fatal("expected error for symlink escape, got nil")
		}
	})

	t.Run("empty path uses default", func(t *testing.T) {
		tmpDir := t.TempDir()
		source := &GitMarketSource{
			URL: "/fake/repo.git",
		}
		source.SetInstallLocation(tmpDir)

		got, err := MarketSourceIndexPath(source)
		if err != nil {
			t.Fatalf("MarketSourceIndexPath() error = %v", err)
		}

		want := filepath.Join(tmpDir, ".claude-plugin", "marketplace.json")
		if got != want {
			t.Errorf("MarketSourceIndexPath() = %q, want %q", got, want)
		}
	})

	t.Run("GitHub source with custom path", func(t *testing.T) {
		tmpDir := t.TempDir()
		source := &GitHubMarketSource{
			Repo: "owner/repo",
			Path: "custom/marketplace.json",
		}
		source.SetInstallLocation(tmpDir)

		got, err := MarketSourceIndexPath(source)
		if err != nil {
			t.Fatalf("MarketSourceIndexPath() error = %v", err)
		}

		want := filepath.Join(tmpDir, "custom", "marketplace.json")
		if got != want {
			t.Errorf("MarketSourceIndexPath() = %q, want %q", got, want)
		}
	})
}

func initTestGitRepo(t *testing.T, dir string, branchName, marketplaceName string) {
	t.Helper()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	indexPath := filepath.Join(dir, ".claude-plugin", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	data := fmt.Sprintf(`{"name":"%s","plugins":[]}`, marketplaceName)
	if err := os.WriteFile(indexPath, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write marketplace: %v", err)
	}

	_, err = wt.Add(".")
	if err != nil {
		t.Fatalf("failed to add files: %v", err)
	}
	_, err = wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	if branchName != "" {
		headRef, err := repo.Head()
		if err != nil {
			t.Fatalf("failed to get HEAD: %v", err)
		}
		defaultBranch := headRef.Name().Short()
		if defaultBranch != branchName {
			ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/"+branchName), headRef.Hash())
			if err := repo.Storer.SetReference(ref); err != nil {
				t.Fatalf("failed to create branch %s: %v", branchName, err)
			}
			if err := wt.Checkout(&git.CheckoutOptions{
				Branch: plumbing.ReferenceName("refs/heads/" + branchName),
			}); err != nil {
				t.Fatalf("failed to checkout branch %s: %v", branchName, err)
			}
		}
	}
}

func TestManagerAddSource_GitWithRef(t *testing.T) {
	repoDir := t.TempDir()
	initTestGitRepo(t, repoDir, "", "main-market")

	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	headRef, err := repo.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}
	featureRef := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/feature"), headRef.Hash())
	if err := repo.Storer.SetReference(featureRef); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/feature"),
	}); err != nil {
		t.Fatalf("failed to checkout feature: %v", err)
	}

	featureIndexPath := filepath.Join(repoDir, ".claude-plugin", "marketplace.json")
	featureData := `{"name":"feature-market","plugins":[]}`
	if err := os.WriteFile(featureIndexPath, []byte(featureData), 0644); err != nil {
		t.Fatalf("failed to write feature marketplace: %v", err)
	}

	_, err = wt.Add(".")
	if err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	_, err = wt.Commit("feature commit", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	marketsDir := t.TempDir()
	mgr := NewManager(marketsDir)

	source := &GitMarketSource{
		URL: repoDir,
		Ref: "feature",
	}

	mp, _, err := mgr.AddSource("feature-test", source)
	if err != nil {
		t.Fatalf("AddSource() error = %v", err)
	}

	if mp.Name != "feature-market" {
		t.Errorf("marketplace name = %q, want %q", mp.Name, "feature-market")
	}
}

func TestManagerAddSource_GitWithCustomPath(t *testing.T) {
	repoDir := t.TempDir()
	initTestGitRepo(t, repoDir, "main", "path-market")

	customIndexPath := filepath.Join(repoDir, "catalog", ".claude-plugin", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(customIndexPath), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	customData := `{"name":"custom-path-market","plugins":[]}`
	if err := os.WriteFile(customIndexPath, []byte(customData), 0644); err != nil {
		t.Fatalf("failed to write custom marketplace: %v", err)
	}

	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}
	_, err = wt.Add(".")
	if err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	_, err = wt.Commit("add custom path", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	marketsDir := t.TempDir()
	mgr := NewManager(marketsDir)

	source := &GitMarketSource{
		URL:  repoDir,
		Path: "catalog/.claude-plugin/marketplace.json",
	}

	mp, resultSource, err := mgr.AddSource("custom-path-test", source)
	if err != nil {
		t.Fatalf("AddSource() error = %v", err)
	}

	if mp.Name != "custom-path-market" {
		t.Errorf("marketplace name = %q, want %q", mp.Name, "custom-path-market")
	}

	indexPath, err := MarketSourceIndexPath(resultSource)
	if err != nil {
		t.Fatalf("MarketSourceIndexPath() error = %v", err)
	}

	expected := filepath.Join(resultSource.InstallLocation(), "catalog", ".claude-plugin", "marketplace.json")
	resolvedExpected, _ := filepath.EvalSymlinks(filepath.Dir(expected))
	if resolvedExpected != "" {
		expected = filepath.Join(resolvedExpected, filepath.Base(expected))
	}
	if indexPath != expected {
		t.Errorf("index path = %q, want %q", indexPath, expected)
	}
}

func TestManagerAddSource_SparsePathsNoError(t *testing.T) {
	repoDir := t.TempDir()
	initTestGitRepo(t, repoDir, "main", "sparse-market")

	marketsDir := t.TempDir()
	mgr := NewManager(marketsDir)

	source := &GitMarketSource{
		URL:         repoDir,
		SparsePaths: []string{".claude-plugin", "plugins"},
	}

	mp, _, err := mgr.AddSource("sparse-test", source)
	if err != nil {
		t.Fatalf("AddSource() with sparsePaths error = %v", err)
	}

	if mp.Name != "sparse-market" {
		t.Errorf("marketplace name = %q, want %q", mp.Name, "sparse-market")
	}
}

func TestGetMarketSourceManifestPath(t *testing.T) {
	t.Run("GitHub with path", func(t *testing.T) {
		src := &GitHubMarketSource{Path: "custom/marketplace.json"}
		if got := GetMarketSourceManifestPath(src); got != "custom/marketplace.json" {
			t.Errorf("got %q, want %q", got, "custom/marketplace.json")
		}
	})
	t.Run("Git with path", func(t *testing.T) {
		src := &GitMarketSource{Path: "other/marketplace.json"}
		if got := GetMarketSourceManifestPath(src); got != "other/marketplace.json" {
			t.Errorf("got %q, want %q", got, "other/marketplace.json")
		}
	})
	t.Run("Git without path", func(t *testing.T) {
		src := &GitMarketSource{}
		if got := GetMarketSourceManifestPath(src); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("other source types return empty", func(t *testing.T) {
		src := &URLMarketSource{}
		if got := GetMarketSourceManifestPath(src); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestGetMarketSourceRef(t *testing.T) {
	t.Run("GitHub with ref", func(t *testing.T) {
		src := &GitHubMarketSource{Ref: "main"}
		if got := GetMarketSourceRef(src); got != "main" {
			t.Errorf("got %q, want %q", got, "main")
		}
	})
	t.Run("Git with ref", func(t *testing.T) {
		src := &GitMarketSource{Ref: "feature"}
		if got := GetMarketSourceRef(src); got != "feature" {
			t.Errorf("got %q, want %q", got, "feature")
		}
	})
	t.Run("Git without ref", func(t *testing.T) {
		src := &GitMarketSource{}
		if got := GetMarketSourceRef(src); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestIsWithinMarketsDir(t *testing.T) {
	marketsDir := t.TempDir()
	innerDir := filepath.Join(marketsDir, "inner")
	os.MkdirAll(innerDir, 0755)

	t.Run("path inside marketsDir", func(t *testing.T) {
		if !isWithinMarketsDir(innerDir, marketsDir) {
			t.Error("expected inner dir to be within marketsDir")
		}
	})

	t.Run("path is marketsDir itself rejected", func(t *testing.T) {
		if isWithinMarketsDir(marketsDir, marketsDir) {
			t.Error("marketsDir itself should be rejected")
		}
	})

	t.Run("path outside marketsDir rejected", func(t *testing.T) {
		if isWithinMarketsDir(t.TempDir(), marketsDir) {
			t.Error("external dir should be rejected")
		}
	})

	t.Run("symlink pointing outside marketsDir rejected", func(t *testing.T) {
		outsideDir := t.TempDir()
		linkPath := filepath.Join(marketsDir, "escape-link")
		os.Symlink(outsideDir, linkPath)
		if isWithinMarketsDir(linkPath, marketsDir) {
			t.Error("symlink pointing outside marketsDir should be rejected")
		}
	})

	t.Run("symlink pointing inside marketsDir allowed", func(t *testing.T) {
		targetDir := filepath.Join(marketsDir, "real-target")
		os.MkdirAll(targetDir, 0755)
		linkPath := filepath.Join(marketsDir, "good-link")
		os.Symlink(targetDir, linkPath)
		if !isWithinMarketsDir(linkPath, marketsDir) {
			t.Error("symlink pointing inside marketsDir should be allowed")
		}
	})
}
