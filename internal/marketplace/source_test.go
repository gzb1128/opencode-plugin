package marketplace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMarketplaceSource(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantType string
		setup    func() string // returns cleanup path
	}{
		{
			name:     "GitHub shorthand format",
			url:      "opencode/plugins-official",
			wantType: "github",
		},
		{
			name:     "GitHub SSH format",
			url:      "git@github.com:opencode/plugins-official.git",
			wantType: "github",
		},
		{
			name:     "GitHub HTTPS URL (classified as github)",
			url:      "https://github.com/opencode/plugins-official.git",
			wantType: "github",
		},
		{
			name:     "Git HTTPS URL (non-GitHub)",
			url:      "https://gitlab.com/opencode/plugins-official.git",
			wantType: "git",
		},
		{
			name:     "Git SSH URL (non-GitHub)",
			url:      "git@gitlab.com:opencode/plugins-official.git",
			wantType: "git",
		},
		{
			name:     "marketplace.json URL",
			url:      "https://example.com/marketplace.json",
			wantType: "url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseMarketplaceSource(tt.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.SourceType() != tt.wantType {
				t.Errorf("SourceType() = %v, want %v", result.SourceType(), tt.wantType)
			}
		})
	}

	t.Run("local directory path", func(t *testing.T) {
		tmpDir := filepath.Join(t.TempDir(), "test-market")
		os.MkdirAll(tmpDir, 0755)

		result, err := ParseMarketplaceSource(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.SourceType() != "directory" {
			t.Errorf("SourceType() = %v, want directory", result.SourceType())
		}
	})

	t.Run("local file path", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "marketplace.json")
		os.WriteFile(tmpFile, []byte("{}"), 0644)

		result, err := ParseMarketplaceSource(tmpFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.SourceType() != "file" {
			t.Errorf("SourceType() = %v, want file", result.SourceType())
		}
	})

	t.Run("Empty URL", func(t *testing.T) {
		_, err := ParseMarketplaceSource("")
		if err == nil {
			t.Error("expected error for empty URL")
		}
	})

	t.Run("Invalid format", func(t *testing.T) {
		_, err := ParseMarketplaceSource("not-a-valid-format")
		if err == nil {
			t.Error("expected error for invalid format")
		}
	})
}

func TestParseMarketplaceSource_GitHubFields(t *testing.T) {
	t.Run("shorthand extracts repo", func(t *testing.T) {
		result, err := ParseMarketplaceSource("opencode/plugins-official")
		if err != nil {
			t.Fatal(err)
		}
		src, ok := result.(*GitHubMarketSource)
		if !ok {
			t.Fatalf("expected *GitHubMarketSource, got %T", result)
		}
		if src.Repo != "opencode/plugins-official" {
			t.Errorf("Repo = %v, want opencode/plugins-official", src.Repo)
		}
	})

	t.Run("SSH extracts repo", func(t *testing.T) {
		result, err := ParseMarketplaceSource("git@github.com:opencode/plugins-official.git")
		if err != nil {
			t.Fatal(err)
		}
		src, ok := result.(*GitHubMarketSource)
		if !ok {
			t.Fatalf("expected *GitHubMarketSource, got %T", result)
		}
		if src.Repo != "opencode/plugins-official" {
			t.Errorf("Repo = %v, want opencode/plugins-official", src.Repo)
		}
		if got := GetMarketSourceURL(src); got != "git@github.com:opencode/plugins-official.git" {
			t.Errorf("GetMarketSourceURL() = %v, want original SSH URL", got)
		}
	})

	t.Run("HTTPS preserves original clone URL", func(t *testing.T) {
		result, err := ParseMarketplaceSource("https://github.com/opencode/plugins-official.git")
		if err != nil {
			t.Fatal(err)
		}
		src, ok := result.(*GitHubMarketSource)
		if !ok {
			t.Fatalf("expected *GitHubMarketSource, got %T", result)
		}
		if got := GetMarketSourceURL(src); got != "https://github.com/opencode/plugins-official.git" {
			t.Errorf("GetMarketSourceURL() = %v, want original HTTPS URL", got)
		}
	})
}

func TestParseMarketplaceSource_GitFields(t *testing.T) {
	result, err := ParseMarketplaceSource("https://gitlab.com/org/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	src, ok := result.(*GitMarketSource)
	if !ok {
		t.Fatalf("expected *GitMarketSource, got %T", result)
	}
	if src.URL != "https://gitlab.com/org/repo.git" {
		t.Errorf("URL = %v", src.URL)
	}
}

func TestMarketSourceInterface(t *testing.T) {
	t.Run("GitHubMarketSource implements interface", func(t *testing.T) {
		var _ MarketSource = &GitHubMarketSource{Repo: "owner/repo"}
	})

	t.Run("GitMarketSource implements interface", func(t *testing.T) {
		var _ MarketSource = &GitMarketSource{URL: "https://example.com"}
	})

	t.Run("URLMarketSource implements interface", func(t *testing.T) {
		var _ MarketSource = &URLMarketSource{URL: "https://example.com/marketplace.json"}
	})

	t.Run("LocalMarketSource implements interface", func(t *testing.T) {
		var _ MarketSource = &LocalMarketSource{Path: "/tmp/market"}
	})

	t.Run("FileMarketSource implements interface", func(t *testing.T) {
		var _ MarketSource = &FileMarketSource{Path: "/tmp/market.json"}
	})

	t.Run("DirectoryMarketSource implements interface", func(t *testing.T) {
		var _ MarketSource = &DirectoryMarketSource{Path: "/tmp/market"}
	})
}

func TestMarketSourceInstallLocation(t *testing.T) {
	src := &GitHubMarketSource{Repo: "owner/repo"}
	src.SetInstallLocation("/path/to/install")

	if src.InstallLocation() != "/path/to/install" {
		t.Errorf("InstallLocation() = %v, want /path/to/install", src.InstallLocation())
	}
}

func TestMarketSourceToConfig(t *testing.T) {
	t.Run("GitHubMarketSource", func(t *testing.T) {
		src := &GitHubMarketSource{Repo: "owner/repo", Ref: "main"}
		src.SetInstallLocation("/install/path")

		cfg := MarketSourceToConfig(src)
		if cfg["source"] != "github" {
			t.Errorf("source = %v, want github", cfg["source"])
		}
		if cfg["repo"] != "owner/repo" {
			t.Errorf("repo = %v, want owner/repo", cfg["repo"])
		}
		if cfg["ref"] != "main" {
			t.Errorf("ref = %v, want main", cfg["ref"])
		}
		if cfg["installLocation"] != "/install/path" {
			t.Errorf("installLocation = %v", cfg["installLocation"])
		}
	})

	t.Run("GitMarketSource with sparsePaths", func(t *testing.T) {
		src := &GitMarketSource{
			URL:         "https://gitlab.com/repo.git",
			SparsePaths: []string{".claude-plugin", "plugins"},
		}

		cfg := MarketSourceToConfig(src)
		if cfg["source"] != "git" {
			t.Errorf("source = %v, want git", cfg["source"])
		}
		sp, ok := cfg["sparsePaths"].([]string)
		if !ok || len(sp) != 2 {
			t.Errorf("sparsePaths = %v, want 2 items", cfg["sparsePaths"])
		}
	})

	t.Run("URLMarketSource with headers", func(t *testing.T) {
		src := &URLMarketSource{
			URL:     "https://example.com/marketplace.json",
			Headers: map[string]string{"Authorization": "Bearer token"},
		}

		cfg := MarketSourceToConfig(src)
		if cfg["source"] != "url" {
			t.Errorf("source = %v, want url", cfg["source"])
		}
		headers, ok := cfg["headers"].(map[string]string)
		if !ok || headers["Authorization"] != "Bearer token" {
			t.Errorf("headers = %v", cfg["headers"])
		}
	})

	t.Run("LocalMarketSource", func(t *testing.T) {
		src := &LocalMarketSource{Path: "/tmp/market"}

		cfg := MarketSourceToConfig(src)
		if cfg["source"] != "local" {
			t.Errorf("source = %v, want local", cfg["source"])
		}
		if cfg["path"] != "/tmp/market" {
			t.Errorf("path = %v, want /tmp/market", cfg["path"])
		}
	})
}

func TestNewMarketSourceFromConfig(t *testing.T) {
	t.Run("github from config", func(t *testing.T) {
		cfg := map[string]interface{}{
			"source":          "github",
			"repo":            "owner/repo",
			"url":             "git@github.com:owner/repo.git",
			"ref":             "main",
			"installLocation": "/install/path",
		}
		ms := NewMarketSourceFromConfig(cfg)

		src, ok := ms.(*GitHubMarketSource)
		if !ok {
			t.Fatalf("expected *GitHubMarketSource, got %T", ms)
		}
		if src.Repo != "owner/repo" {
			t.Errorf("Repo = %v", src.Repo)
		}
		if src.URL != "git@github.com:owner/repo.git" {
			t.Errorf("URL = %v", src.URL)
		}
		if src.Ref != "main" {
			t.Errorf("Ref = %v", src.Ref)
		}
		if src.InstallLocation() != "/install/path" {
			t.Errorf("InstallLocation = %v", src.InstallLocation())
		}
	})

	t.Run("git with sparsePaths from config", func(t *testing.T) {
		cfg := map[string]interface{}{
			"source":      "git",
			"url":         "https://gitlab.com/repo.git",
			"sparsePaths": []interface{}{".claude-plugin", "plugins"},
		}
		ms := NewMarketSourceFromConfig(cfg)

		src, ok := ms.(*GitMarketSource)
		if !ok {
			t.Fatalf("expected *GitMarketSource, got %T", ms)
		}
		if src.URL != "https://gitlab.com/repo.git" {
			t.Errorf("URL = %v", src.URL)
		}
		if len(src.SparsePaths) != 2 {
			t.Errorf("SparsePaths len = %v, want 2", len(src.SparsePaths))
		}
	})

	t.Run("url with headers from config", func(t *testing.T) {
		cfg := map[string]interface{}{
			"source": "url",
			"url":    "https://example.com/marketplace.json",
			"headers": map[string]interface{}{
				"Authorization": "Bearer token",
			},
		}
		ms := NewMarketSourceFromConfig(cfg)

		src, ok := ms.(*URLMarketSource)
		if !ok {
			t.Fatalf("expected *URLMarketSource, got %T", ms)
		}
		if src.Headers["Authorization"] != "Bearer token" {
			t.Errorf("Headers = %v", src.Headers)
		}
	})

	t.Run("local from config", func(t *testing.T) {
		cfg := map[string]interface{}{
			"source": "local",
			"path":   "/tmp/market",
		}
		ms := NewMarketSourceFromConfig(cfg)

		src, ok := ms.(*LocalMarketSource)
		if !ok {
			t.Fatalf("expected *LocalMarketSource, got %T", ms)
		}
		if src.Path != "/tmp/market" {
			t.Errorf("Path = %v", src.Path)
		}
	})

	t.Run("unknown source falls back to local", func(t *testing.T) {
		cfg := map[string]interface{}{
			"source": "unknown",
		}
		ms := NewMarketSourceFromConfig(cfg)

		if ms.SourceType() != "local" {
			t.Errorf("SourceType() = %v, want local for unknown", ms.SourceType())
		}
	})
}

func TestRoundTripConfig(t *testing.T) {
	sources := []MarketSource{
		&GitHubMarketSource{Repo: "owner/repo", Ref: "main"},
		&GitMarketSource{URL: "https://gitlab.com/repo.git", SparsePaths: []string{"plugins"}},
		&URLMarketSource{URL: "https://example.com/marketplace.json", Headers: map[string]string{"Auth": "token"}},
		&LocalMarketSource{Path: "/tmp/market"},
	}

	for _, original := range sources {
		original.SetInstallLocation("/install/path")
		t.Run(original.SourceType()+" roundtrip", func(t *testing.T) {
			cfg := MarketSourceToConfig(original)
			restored := NewMarketSourceFromConfig(cfg)

			if restored.SourceType() != original.SourceType() {
				t.Errorf("SourceType mismatch: %v vs %v", restored.SourceType(), original.SourceType())
			}
			if restored.InstallLocation() != original.InstallLocation() {
				t.Errorf("InstallLocation mismatch: %v vs %v", restored.InstallLocation(), original.InstallLocation())
			}
			switch want := original.(type) {
			case *GitMarketSource:
				got := restored.(*GitMarketSource)
				if len(got.SparsePaths) != len(want.SparsePaths) {
					t.Errorf("SparsePaths len mismatch: %v vs %v", got.SparsePaths, want.SparsePaths)
				}
			case *URLMarketSource:
				got := restored.(*URLMarketSource)
				if got.Headers["Auth"] != "token" {
					t.Errorf("Headers mismatch: %v", got.Headers)
				}
			}
		})
	}
}

func TestPluginSourceInterface(t *testing.T) {
	sources := []PluginSource{
		&LocalSource{Path: "./plugins/foo"},
		&GitHubSource{Repo: "owner/repo", Ref: "main"},
		&GitSource{URL: "https://gitlab.com/repo.git"},
		&GitSubdirSource{URL: "https://github.com/repo.git", SubPath: "plugins/foo"},
		&URLSource{URL: "https://github.com/repo.git"},
		&NpmSource{Package: "@org/plugin", Version: "^1.0.0"},
		&PipSource{Package: "my-plugin", Version: ">=2.0.0"},
	}

	for _, src := range sources {
		t.Run(src.SourceType()+" implements PluginSource", func(t *testing.T) {
			var _ PluginSource = src
		})
	}
}
