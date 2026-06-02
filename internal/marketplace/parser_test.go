package marketplace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMarketplaceIndex(t *testing.T) {
	marketplacePath := filepath.Join("..", "..", "test", "fixtures", "sample-marketplace", ".claude-plugin", "marketplace.json")

	mp, err := ParseMarketplaceIndex(marketplacePath)
	if err != nil {
		t.Fatalf("ParseMarketplaceIndex() error = %v", err)
	}

	if mp.Name != "test-marketplace" {
		t.Errorf("Name = %v, want test-marketplace", mp.Name)
	}

	if mp.Description != "Test marketplace for unit tests" {
		t.Errorf("Description = %v, want 'Test marketplace for unit tests'", mp.Description)
	}

	if mp.Owner == nil {
		t.Fatal("Owner is nil")
	}

	if mp.Owner.Name != "Test Org" {
		t.Errorf("Owner.Name = %v, want Test Org", mp.Owner.Name)
	}

	if !mp.ForceRemoveDeletedPlugins {
		t.Error("ForceRemoveDeletedPlugins should be true")
	}

	if mp.Metadata == nil {
		t.Fatal("Metadata is nil")
	}

	if mp.Metadata.Version != "1.0.0" {
		t.Errorf("Metadata.Version = %v, want 1.0.0", mp.Metadata.Version)
	}

	if len(mp.AllowCrossMarketplaceDeps) != 1 || mp.AllowCrossMarketplaceDeps[0] != "other-market" {
		t.Errorf("AllowCrossMarketplaceDeps = %v, want [other-market]", mp.AllowCrossMarketplaceDeps)
	}

	if len(mp.Plugins) != 6 {
		t.Fatalf("Plugins count = %v, want 6", len(mp.Plugins))
	}

	t.Run("local source", func(t *testing.T) {
		p := mp.Plugins[0]
		if p.Name != "test-plugin" {
			t.Errorf("Name = %v, want test-plugin", p.Name)
		}
		src, ok := p.Source.(*LocalSource)
		if !ok {
			t.Fatalf("Source type = %T, want *LocalSource", p.Source)
		}
		if src.Path != "./plugins/test-plugin" {
			t.Errorf("Path = %v, want ./plugins/test-plugin", src.Path)
		}
	})

	t.Run("github source", func(t *testing.T) {
		p := mp.Plugins[1]
		if p.Name != "external-plugin" {
			t.Errorf("Name = %v, want external-plugin", p.Name)
		}
		src, ok := p.Source.(*GitHubSource)
		if !ok {
			t.Fatalf("Source type = %T, want *GitHubSource", p.Source)
		}
		if src.Repo != "test/external-plugin" {
			t.Errorf("Repo = %v, want test/external-plugin", src.Repo)
		}
	})

	t.Run("npm source", func(t *testing.T) {
		p := mp.Plugins[2]
		if p.Name != "npm-plugin" {
			t.Errorf("Name = %v, want npm-plugin", p.Name)
		}
		src, ok := p.Source.(*NpmSource)
		if !ok {
			t.Fatalf("Source type = %T, want *NpmSource", p.Source)
		}
		if src.Package != "@org/test-plugin" {
			t.Errorf("Package = %v, want @org/test-plugin", src.Package)
		}
		if src.Version != "^1.0.0" {
			t.Errorf("Version = %v, want ^1.0.0", src.Version)
		}
	})

	t.Run("git source with ref", func(t *testing.T) {
		p := mp.Plugins[3]
		if p.Name != "git-plugin" {
			t.Errorf("Name = %v, want git-plugin", p.Name)
		}
		src, ok := p.Source.(*GitSource)
		if !ok {
			t.Fatalf("Source type = %T, want *GitSource", p.Source)
		}
		if src.URL != "https://gitlab.com/org/plugin.git" {
			t.Errorf("URL = %v, want https://gitlab.com/org/plugin.git", src.URL)
		}
		if src.Ref != "main" {
			t.Errorf("Ref = %v, want main", src.Ref)
		}
	})

	t.Run("git-subdir source with shorthand", func(t *testing.T) {
		p := mp.Plugins[4]
		if p.Name != "subdir-plugin" {
			t.Errorf("Name = %v, want subdir-plugin", p.Name)
		}
		src, ok := p.Source.(*GitSubdirSource)
		if !ok {
			t.Fatalf("Source type = %T, want *GitSubdirSource", p.Source)
		}
		if src.URL != "https://github.com/org/monorepo.git" {
			t.Errorf("URL = %v, want https://github.com/org/monorepo.git", src.URL)
		}
		if src.SubPath != "plugins/subdir-plugin" {
			t.Errorf("SubPath = %v, want plugins/subdir-plugin", src.SubPath)
		}
		if src.Ref != "v2.0.0" {
			t.Errorf("Ref = %v, want v2.0.0", src.Ref)
		}
	})

	t.Run("url source with sha", func(t *testing.T) {
		p := mp.Plugins[5]
		if p.Name != "url-plugin" {
			t.Errorf("Name = %v, want url-plugin", p.Name)
		}
		src, ok := p.Source.(*URLSource)
		if !ok {
			t.Fatalf("Source type = %T, want *URLSource", p.Source)
		}
		if src.URL != "https://github.com/org/plugin.git" {
			t.Errorf("URL = %v", src.URL)
		}
		if src.Ref != "v1.0.0" {
			t.Errorf("Ref = %v, want v1.0.0", src.Ref)
		}
		if src.SHA != "abc123def456789012345678901234567890abcd" {
			t.Errorf("SHA = %v", src.SHA)
		}
	})

	t.Run("github source with abbreviated sha", func(t *testing.T) {
		source, err := parsePluginSource(map[string]interface{}{
			"source": "github",
			"repo":   "org/plugin",
			"sha":    "abc123d",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		src, ok := source.(*GitHubSource)
		if !ok {
			t.Fatalf("Source type = %T, want *GitHubSource", source)
		}
		if src.SHA != "abc123d" {
			t.Errorf("SHA = %v, want abc123d", src.SHA)
		}
	})
}

func TestParseDependencies_StringForm(t *testing.T) {
	content := `{
  "name": "deps-market",
  "plugins": [
    {
      "name": "root",
      "description": "Root plugin",
      "source": "./plugins/root",
	      "dependencies": ["dep", "other@shared", "range@shared@^1.2.0", "dep@v2", "dep@2026"]
    }
  ]
}`

	tmpFile := filepath.Join(t.TempDir(), "deps-market.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	mp, err := ParseMarketplaceIndex(tmpFile)
	if err != nil {
		t.Fatalf("ParseMarketplaceIndex() error = %v", err)
	}

	if len(mp.Plugins) != 1 {
		t.Fatalf("Plugins count = %v, want 1", len(mp.Plugins))
	}

	deps := mp.Plugins[0].Dependencies
	want := []string{"dep", "other@shared", "range@shared", "dep@v2", "dep@2026"}
	if len(deps) != len(want) {
		t.Fatalf("Dependencies = %v, want %v", deps, want)
	}
	for i, d := range deps {
		if d != want[i] {
			t.Errorf("Dependencies[%d] = %q, want %q", i, d, want[i])
		}
	}
}

func TestParseDependencies_ObjectForm(t *testing.T) {
	content := `{
  "name": "obj-deps-market",
  "plugins": [
    {
      "name": "root",
      "description": "Root plugin",
      "source": "./plugins/root",
      "dependencies": [
        { "name": "dep" },
        { "name": "shared-dep", "marketplace": "shared" }
      ]
    }
  ]
}`

	tmpFile := filepath.Join(t.TempDir(), "obj-deps-market.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	mp, err := ParseMarketplaceIndex(tmpFile)
	if err != nil {
		t.Fatalf("ParseMarketplaceIndex() error = %v", err)
	}

	deps := mp.Plugins[0].Dependencies
	want := []string{"dep", "shared-dep@shared"}
	if len(deps) != len(want) {
		t.Fatalf("Dependencies = %v, want %v", deps, want)
	}
	for i, d := range deps {
		if d != want[i] {
			t.Errorf("Dependencies[%d] = %q, want %q", i, d, want[i])
		}
	}
}

func TestParseDependencyRef_Errors(t *testing.T) {
	tests := []struct {
		name    string
		raw     interface{}
		wantErr string
	}{
		{"number type", float64(42), "must be a string or object"},
		{"object missing name", map[string]interface{}{"marketplace": "m"}, "must have a 'name' field"},
		{"empty string", "", "must not be empty"},
		{"slash in name", "foo/bar", "must not contain '/'"},
		{"backslash in name", "foo\\bar", "must not contain '/'"},
		{"dot dot in name", "foo..bar", "must not contain '..'"},
		{"too many at segments", "a@b@c@d", "too many '@' segments"},
		{"three-part empty version", "a@b@", "version must not be empty"},
		{"three-part slash version", "a@b@../evil", "must not contain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDependencyRef(tt.raw)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestPluginRootMetadata(t *testing.T) {
	content := `{
  "name": "rooted-market",
  "metadata": { "pluginRoot": "packages" },
  "plugins": [
    {
      "name": "tool",
      "description": "Tool",
      "source": "./tool"
    }
  ]
}`

	tmpFile := filepath.Join(t.TempDir(), "rooted-market.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	mp, err := ParseMarketplaceIndex(tmpFile)
	if err != nil {
		t.Fatalf("ParseMarketplaceIndex() error = %v", err)
	}

	if mp.Metadata == nil || mp.Metadata.PluginRoot != "packages" {
		t.Fatalf("Metadata.PluginRoot = %v, want 'packages'", mp.Metadata)
	}

	p := mp.Plugins[0]
	src, ok := p.Source.(*LocalSource)
	if !ok {
		t.Fatalf("Source type = %T, want *LocalSource", p.Source)
	}
	if src.Path != "./tool" {
		t.Errorf("Path = %v, want ./tool", src.Path)
	}
}

func TestParsePluginSource_AllTypes(t *testing.T) {
	tests := []struct {
		name           string
		raw            interface{}
		wantType       string
		wantSourceType string
	}{
		{
			name:           "local string path",
			raw:            "./plugins/foo",
			wantType:       "*marketplace.LocalSource",
			wantSourceType: "local",
		},
		{
			name: "github object",
			raw: map[string]interface{}{
				"source": "github",
				"repo":   "owner/repo",
				"ref":    "main",
			},
			wantType:       "*marketplace.GitHubSource",
			wantSourceType: "github",
		},
		{
			name: "git object",
			raw: map[string]interface{}{
				"source": "git",
				"url":    "https://gitlab.com/repo.git",
				"ref":    "v1.0",
			},
			wantType:       "*marketplace.GitSource",
			wantSourceType: "git",
		},
		{
			name: "git-subdir object",
			raw: map[string]interface{}{
				"source": "git-subdir",
				"url":    "owner/repo",
				"path":   "plugins/foo",
				"ref":    "main",
			},
			wantType:       "*marketplace.GitSubdirSource",
			wantSourceType: "git-subdir",
		},
		{
			name: "url object",
			raw: map[string]interface{}{
				"source": "url",
				"url":    "https://github.com/repo.git",
			},
			wantType:       "*marketplace.URLSource",
			wantSourceType: "url",
		},
		{
			name: "npm object",
			raw: map[string]interface{}{
				"source":  "npm",
				"package": "@org/plugin",
				"version": "^1.0.0",
			},
			wantType:       "*marketplace.NpmSource",
			wantSourceType: "npm",
		},
		{
			name: "pip object",
			raw: map[string]interface{}{
				"source":  "pip",
				"package": "my-plugin",
				"version": ">=2.0.0",
			},
			wantType:       "*marketplace.PipSource",
			wantSourceType: "pip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := parsePluginSource(tt.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if src.SourceType() != tt.wantSourceType {
				t.Errorf("SourceType() = %v, want %v", src.SourceType(), tt.wantSourceType)
			}
		})
	}
}

func TestParsePluginSource_Errors(t *testing.T) {
	tests := []struct {
		name    string
		raw     interface{}
		wantErr string
	}{
		{
			name:    "invalid type (number)",
			raw:     float64(42),
			wantErr: "invalid source format",
		},
		{
			name: "missing source field",
			raw: map[string]interface{}{
				"repo": "owner/repo",
			},
			wantErr: "source must have a 'source' field",
		},
		{
			name: "unknown source type",
			raw: map[string]interface{}{
				"source": "unknown",
			},
			wantErr: "unsupported source type",
		},
		{
			name: "github missing repo",
			raw: map[string]interface{}{
				"source": "github",
			},
			wantErr: "github source must have a 'repo' field",
		},
		{
			name: "git missing url",
			raw: map[string]interface{}{
				"source": "git",
			},
			wantErr: "git source must have a 'url' field",
		},
		{
			name: "git-subdir missing url and repo",
			raw: map[string]interface{}{
				"source": "git-subdir",
				"path":   "plugins/foo",
			},
			wantErr: "git-subdir source must have a 'url' or 'repo' field",
		},
		{
			name: "git-subdir missing path",
			raw: map[string]interface{}{
				"source": "git-subdir",
				"url":    "owner/repo",
			},
			wantErr: "git-subdir source must have a 'path' field",
		},
		{
			name: "url missing url field",
			raw: map[string]interface{}{
				"source": "url",
			},
			wantErr: "url source must have a 'url' field",
		},
		{
			name: "npm missing package",
			raw: map[string]interface{}{
				"source": "npm",
			},
			wantErr: "npm source must have a 'package' field",
		},
		{
			name: "pip missing package",
			raw: map[string]interface{}{
				"source": "pip",
			},
			wantErr: "pip source must have a 'package' field",
		},
		{
			name: "invalid sha (too short)",
			raw: map[string]interface{}{
				"source": "github",
				"repo":   "owner/repo",
				"sha":    "abc123",
			},
			wantErr: "SHA must be a 7-40 character",
		},
		{
			name: "invalid sha (uppercase)",
			raw: map[string]interface{}{
				"source": "url",
				"url":    "https://example.com",
				"sha":    "ABC123DEF456789012345678901234567890ABCD",
			},
			wantErr: "SHA must be a 7-40 character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePluginSource(tt.raw)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseMarketplaceIndex_InvalidJSON(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(tmpFile, []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseMarketplaceIndex(tmpFile)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParseMarketplaceIndex_MissingName(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "missing-name.json")
	content := `{"description": "test"}`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseMarketplaceIndex(tmpFile)
	if err == nil {
		t.Error("expected error for missing name, got nil")
	}
}

func TestValidateMarketplaceName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "my-marketplace", false},
		{"empty", "", true},
		{"has spaces", "my marketplace", true},
		{"has slash", "my/market", true},
		{"has backslash", "my\\market", true},
		{"has dots", "my..market", true},
		{"dot only", ".", true},
		{"non-ascii", "my-marketplacé", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMarketplaceName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMarketplaceName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseMarketplaceIndex_PreservesDisplayName(t *testing.T) {
	content := `{
  "name": "display-market",
  "plugins": [
    {
      "name": "tool",
      "displayName": "Tool Pro",
      "description": "A tool plugin",
      "source": "./plugins/tool"
    },
    {
      "name": "plain",
      "description": "No display name",
      "source": "./plugins/plain"
    }
  ]
}`

	tmpFile := filepath.Join(t.TempDir(), "display-name-market.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	mp, err := ParseMarketplaceIndex(tmpFile)
	if err != nil {
		t.Fatalf("ParseMarketplaceIndex() error = %v", err)
	}

	if len(mp.Plugins) != 2 {
		t.Fatalf("Plugins count = %v, want 2", len(mp.Plugins))
	}

	if mp.Plugins[0].Name != "tool" {
		t.Errorf("Name = %v, want tool", mp.Plugins[0].Name)
	}
	if mp.Plugins[0].DisplayName != "Tool Pro" {
		t.Errorf("DisplayName = %v, want 'Tool Pro'", mp.Plugins[0].DisplayName)
	}

	if mp.Plugins[1].Name != "plain" {
		t.Errorf("Name = %v, want plain", mp.Plugins[1].Name)
	}
	if mp.Plugins[1].DisplayName != "" {
		t.Errorf("DisplayName = %v, want empty", mp.Plugins[1].DisplayName)
	}
}

func TestResolveGitSubdirURL(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]interface{}
		want string
	}{
		{
			name: "full url",
			raw:  map[string]interface{}{"url": "https://gitlab.com/repo.git"},
			want: "https://gitlab.com/repo.git",
		},
		{
			name: "repo shorthand via url field",
			raw:  map[string]interface{}{"url": "owner/repo"},
			want: "https://github.com/owner/repo.git",
		},
		{
			name: "repo field fallback",
			raw:  map[string]interface{}{"repo": "owner/repo"},
			want: "https://github.com/owner/repo.git",
		},
		{
			name: "ssh url preserved",
			raw:  map[string]interface{}{"url": "git@github.com:owner/repo.git"},
			want: "git@github.com:owner/repo.git",
		},
		{
			name: "empty returns empty",
			raw:  map[string]interface{}{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveGitSubdirURL(tt.raw)
			if got != tt.want {
				t.Errorf("resolveGitSubdirURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
