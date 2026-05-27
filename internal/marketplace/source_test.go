package marketplace

import (
	"os"
	"path/filepath"
	"strings"
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
			wantType: "git",
		},
		{
			name:     "GitHub HTTPS URL (classified as git)",
			url:      "https://github.com/opencode/plugins-official.git",
			wantType: "git",
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
		src, ok := result.(*GitMarketSource)
		if !ok {
			t.Fatalf("expected *GitMarketSource, got %T", result)
		}
		if src.URL != "git@github.com:opencode/plugins-official.git" {
			t.Errorf("URL = %v, want git@github.com:opencode/plugins-official.git", src.URL)
		}
	})

	t.Run("HTTPS preserves original clone URL", func(t *testing.T) {
		result, err := ParseMarketplaceSource("https://github.com/opencode/plugins-official.git")
		if err != nil {
			t.Fatal(err)
		}
		src, ok := result.(*GitMarketSource)
		if !ok {
			t.Fatalf("expected *GitMarketSource, got %T", result)
		}
		if src.URL != "https://github.com/opencode/plugins-official.git" {
			t.Errorf("URL = %v, want https://github.com/opencode/plugins-official.git", src.URL)
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

func TestParseMarketplaceSource_InputParity(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantType   string
		assertFunc func(t *testing.T, src MarketSource)
	}{
		{
			name:     "GitLab SSH deploy key",
			input:    "deploy@gitlab.com:group/project.git",
			wantType: "git",
			assertFunc: func(t *testing.T, src MarketSource) {
				s := src.(*GitMarketSource)
				if s.URL != "deploy@gitlab.com:group/project.git" {
					t.Errorf("URL = %v, want deploy@gitlab.com:group/project.git", s.URL)
				}
			},
		},
		{
			name:     "GitHub SSH with org prefix and ref",
			input:    "org-123456@github.com:owner/repo.git#release",
			wantType: "git",
			assertFunc: func(t *testing.T, src MarketSource) {
				s := src.(*GitMarketSource)
				if s.URL != "org-123456@github.com:owner/repo.git" {
					t.Errorf("URL = %v, want org-123456@github.com:owner/repo.git", s.URL)
				}
				if s.Ref != "release" {
					t.Errorf("Ref = %v, want release", s.Ref)
				}
			},
		},
		{
			name:     "GitHub shorthand with hash ref",
			input:    "owner/repo#main",
			wantType: "github",
			assertFunc: func(t *testing.T, src MarketSource) {
				s := src.(*GitHubMarketSource)
				if s.Repo != "owner/repo" {
					t.Errorf("Repo = %v, want owner/repo", s.Repo)
				}
				if s.Ref != "main" {
					t.Errorf("Ref = %v, want main", s.Ref)
				}
			},
		},
		{
			name:     "GitHub shorthand with at ref",
			input:    "owner/repo@v1.0.0",
			wantType: "github",
			assertFunc: func(t *testing.T, src MarketSource) {
				s := src.(*GitHubMarketSource)
				if s.Repo != "owner/repo" {
					t.Errorf("Repo = %v, want owner/repo", s.Repo)
				}
				if s.Ref != "v1.0.0" {
					t.Errorf("Ref = %v, want v1.0.0", s.Ref)
				}
			},
		},
		{
			name:     "GitHub HTTPS URL with hash ref",
			input:    "https://github.com/owner/repo#main",
			wantType: "git",
			assertFunc: func(t *testing.T, src MarketSource) {
				s := src.(*GitMarketSource)
				if s.URL != "https://github.com/owner/repo.git" {
					t.Errorf("URL = %v, want https://github.com/owner/repo.git", s.URL)
				}
				if s.Ref != "main" {
					t.Errorf("Ref = %v, want main", s.Ref)
				}
			},
		},
		{
			name:     "Azure DevOps HTTPS URL with hash ref",
			input:    "https://dev.azure.com/org/proj/_git/repo#main",
			wantType: "git",
			assertFunc: func(t *testing.T, src MarketSource) {
				s := src.(*GitMarketSource)
				if s.URL != "https://dev.azure.com/org/proj/_git/repo" {
					t.Errorf("URL = %v, want https://dev.azure.com/org/proj/_git/repo", s.URL)
				}
				if s.Ref != "main" {
					t.Errorf("Ref = %v, want main", s.Ref)
				}
			},
		},
		{
			name:     "home-relative path",
			input:    "~/marketplaces/company",
			wantType: "directory",
			assertFunc: func(t *testing.T, src MarketSource) {
				s := src.(*DirectoryMarketSource)
				home, _ := os.UserHomeDir()
				expected := filepath.Join(home, "marketplaces", "company")
				if s.Path != expected {
					t.Errorf("Path = %v, want %v", s.Path, expected)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "home-relative path" {
				home, _ := os.UserHomeDir()
				dir := filepath.Join(home, "marketplaces", "company")
				os.MkdirAll(dir, 0755)
				defer os.RemoveAll(filepath.Join(home, "marketplaces"))
			}

			result, err := ParseMarketplaceSource(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.SourceType() != tt.wantType {
				t.Errorf("SourceType() = %v, want %v", result.SourceType(), tt.wantType)
			}
			if tt.assertFunc != nil {
				tt.assertFunc(t, result)
			}
		})
	}
}

func TestSplitSourceRef(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantBase string
		wantRef  string
	}{
		{"hash ref", "owner/repo#main", "owner/repo", "main"},
		{"at ref github shorthand", "owner/repo@v1.0.0", "owner/repo", "v1.0.0"},
		{"SSH URL with hash ref", "git@github.com:owner/repo.git#release", "git@github.com:owner/repo.git", "release"},
		{"SSH URL no ref", "git@github.com:owner/repo.git", "git@github.com:owner/repo.git", ""},
		{"HTTPS URL with hash ref", "https://github.com/owner/repo#main", "https://github.com/owner/repo", "main"},
		{"deploy SSH URL no split at", "deploy@gitlab.com:group/project.git", "deploy@gitlab.com:group/project.git", ""},
		{"deploy SSH URL with hash ref", "deploy@gitlab.com:group/project.git#dev", "deploy@gitlab.com:group/project.git", "dev"},
		{"no ref", "owner/repo", "owner/repo", ""},
		{"whitespace trimmed", " owner/repo#main ", "owner/repo", "main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref := splitSourceRef(tt.input)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if ref != tt.wantRef {
				t.Errorf("ref = %q, want %q", ref, tt.wantRef)
			}
		})
	}
}

func TestParseMarketplaceSource_GitHubShorthandSSH(t *testing.T) {
	t.Run("git@github.com:owner/repo.git#release", func(t *testing.T) {
		result, err := ParseMarketplaceSource("git@github.com:owner/repo.git#release")
		if err != nil {
			t.Fatal(err)
		}
		src, ok := result.(*GitMarketSource)
		if !ok {
			t.Fatalf("expected *GitMarketSource, got %T", result)
		}
		if src.URL != "git@github.com:owner/repo.git" {
			t.Errorf("URL = %v, want git@github.com:owner/repo.git", src.URL)
		}
		if src.Ref != "release" {
			t.Errorf("Ref = %v, want release", src.Ref)
		}
	})
}

func TestParseMarketplaceSource_HomePathExpansion(t *testing.T) {
	t.Run("nonexistent home path is error", func(t *testing.T) {
		input := "~/nonexistent_dir_for_testing_12345"
		_, err := ParseMarketplaceSource(input)
		if err == nil {
			t.Error("expected error for nonexistent home-relative path")
		}
		if !strings.Contains(err.Error(), "unsupported") {
			t.Errorf("expected unsupported format error, got: %v", err)
		}
	})
}
