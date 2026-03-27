package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode/plugin-cli/internal/marketplace"
)

// --- IsRemoteSource ---

func TestIsRemoteSource(t *testing.T) {
	resolver := NewVersionResolver()

	t.Run("url type via PluginSource", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "url", URL: "https://example.com/foo.git"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for url type")
		}
	})

	t.Run("github type via PluginSource", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "github", Repo: "owner/repo"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for github type")
		}
	})

	t.Run("git-subdir type via PluginSource", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "git-subdir", URL: "https://example.com/repo.git"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for git-subdir type")
		}
	})

	t.Run("local type via PluginSource", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "local", Path: "./plugins/foo"}}
		if resolver.IsRemoteSource(&p) {
			t.Fatal("local type should not be remote")
		}
	})

	t.Run("url type via raw map", func(t *testing.T) {
		p := marketplace.Plugin{Source: map[string]interface{}{"source": "url", "url": "https://example.com/foo.git"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for url map type")
		}
	})

	t.Run("git-subdir type via raw map", func(t *testing.T) {
		p := marketplace.Plugin{Source: map[string]interface{}{"source": "git-subdir", "url": "owner/repo"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for git-subdir map type")
		}
	})

	t.Run("relative string source", func(t *testing.T) {
		p := marketplace.Plugin{Source: "./plugins/foo"}
		if resolver.IsRemoteSource(&p) {
			t.Fatal("relative path should not be remote")
		}
	})
}

// --- GetPluginSourcePath ---

func TestGetPluginSourcePath(t *testing.T) {
	resolver := NewVersionResolver()
	marketPath := filepath.Join(t.TempDir(), "markets", "test")

	t.Run("relative string source", func(t *testing.T) {
		p := marketplace.Plugin{Source: "./plugins/my-plugin"}
		path, err := resolver.GetPluginSourcePath(&p, marketPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(marketPath, "plugins", "my-plugin")
		if path != expected {
			t.Errorf("expected %s, got %s", expected, path)
		}
	})

	t.Run("local PluginSource", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "local", Path: "./plugins/foo"}}
		path, err := resolver.GetPluginSourcePath(&p, marketPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(marketPath, "plugins", "foo")
		if path != expected {
			t.Errorf("expected %s, got %s", expected, path)
		}
	})

	t.Run("remote url PluginSource returns error", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "url", URL: "https://example.com"}}
		_, err := resolver.GetPluginSourcePath(&p, marketPath)
		if err == nil {
			t.Fatal("expected error for remote source")
		}
	})

	t.Run("raw map url type returns URL", func(t *testing.T) {
		p := marketplace.Plugin{Source: map[string]interface{}{"source": "url", "url": "https://example.com/foo.git"}}
		path, err := resolver.GetPluginSourcePath(&p, marketPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "https://example.com/foo.git" {
			t.Errorf("expected https://example.com/foo.git, got %s", path)
		}
	})

	t.Run("raw map url type with repo shorthand", func(t *testing.T) {
		p := marketplace.Plugin{Source: map[string]interface{}{"source": "url", "repo": "owner/repo"}}
		path, err := resolver.GetPluginSourcePath(&p, marketPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "https://github.com/owner/repo.git" {
			t.Errorf("expected https://github.com/owner/repo.git, got %s", path)
		}
	})

	t.Run("raw map git-subdir type returns URL", func(t *testing.T) {
		p := marketplace.Plugin{Source: map[string]interface{}{"source": "git-subdir", "url": "https://example.com/repo.git"}}
		path, err := resolver.GetPluginSourcePath(&p, marketPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "https://example.com/repo.git" {
			t.Errorf("expected https://example.com/repo.git, got %s", path)
		}
	})

	t.Run("raw map empty path fallback", func(t *testing.T) {
		p := marketplace.Plugin{Source: map[string]interface{}{"path": "./plugins/foo"}}
		path, err := resolver.GetPluginSourcePath(&p, marketPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(marketPath, "plugins", "foo")
		if path != expected {
			t.Errorf("expected %s, got %s", expected, path)
		}
	})
}

// --- toPluginSource ---

func TestToPluginSource(t *testing.T) {
	t.Run("url source with sha", func(t *testing.T) {
		m := map[string]interface{}{
			"source": "url",
			"url":    "https://github.com/ChromeDevTools/chrome-devtools-mcp.git",
			"sha":    "abc123def456",
		}
		ps := toPluginSource(m)
		if ps.Type != "url" {
			t.Errorf("expected type 'url', got '%s'", ps.Type)
		}
		if ps.URL != "https://github.com/ChromeDevTools/chrome-devtools-mcp.git" {
			t.Errorf("unexpected URL: %s", ps.URL)
		}
		if ps.SHA != "abc123def456" {
			t.Errorf("expected sha 'abc123def456', got '%s'", ps.SHA)
		}
	})

	t.Run("github source with repo", func(t *testing.T) {
		m := map[string]interface{}{
			"source": "github",
			"repo":   "owner/repo",
		}
		ps := toPluginSource(m)
		if ps.Type != "github" {
			t.Errorf("expected type 'github', got '%s'", ps.Type)
		}
		if ps.Repo != "owner/repo" {
			t.Errorf("expected repo 'owner/repo', got '%s'", ps.Repo)
		}
		if ps.URL != "https://github.com/owner/repo.git" {
			t.Errorf("expected URL 'https://github.com/owner/repo.git', got '%s'", ps.URL)
		}
	})

	t.Run("git-subdir source", func(t *testing.T) {
		m := map[string]interface{}{
			"source": "git-subdir",
			"url":    "techwolf-ai/ai-first-toolkit",
			"path":   "plugins/ai-firstify",
			"ref":    "main",
			"sha":    "7f18e11d694b",
		}
		ps := toPluginSource(m)
		if ps.Type != "git-subdir" {
			t.Errorf("expected type 'git-subdir', got '%s'", ps.Type)
		}
		if ps.URL != "techwolf-ai/ai-first-toolkit" {
			t.Errorf("unexpected URL: %s", ps.URL)
		}
		if ps.SubPath != "plugins/ai-firstify" {
			t.Errorf("expected path 'plugins/ai-firstify', got '%s'", ps.SubPath)
		}
		if ps.Ref != "main" {
			t.Errorf("expected ref 'main', got '%s'", ps.Ref)
		}
		if ps.SHA != "7f18e11d694b" {
			t.Errorf("expected sha '7f18e11d694b', got '%s'", ps.SHA)
		}
	})
}

// --- copyRecursive ---

func TestCopyRecursive(t *testing.T) {
	t.Run("copies files recursively", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := filepath.Join(t.TempDir(), "dst")

		os.MkdirAll(filepath.Join(srcDir, "skills", "nested"), 0755)
		os.MkdirAll(filepath.Join(srcDir, "commands"), 0755)
		os.WriteFile(filepath.Join(srcDir, "skills", "a.md"), []byte("a"), 0644)
		os.WriteFile(filepath.Join(srcDir, "skills", "nested", "b.md"), []byte("b"), 0644)
		os.WriteFile(filepath.Join(srcDir, "commands", "c.md"), []byte("c"), 0644)

		err := copyRecursive(srcDir, dstDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, f := range []string{"skills/a.md", "skills/nested/b.md", "commands/c.md"} {
			p := filepath.Join(dstDir, f)
			if _, err := os.Stat(p); os.IsNotExist(err) {
				t.Errorf("file not copied: %s", f)
			}
		}
	})

	t.Run("skips .git directory", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := filepath.Join(t.TempDir(), "dst")

		os.MkdirAll(filepath.Join(srcDir, ".git", "objects"), 0755)
		os.WriteFile(filepath.Join(srcDir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644)
		os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("hello"), 0644)

		err := copyRecursive(srcDir, dstDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dstDir, ".git")); os.IsNotExist(err) {
			// This is expected - .git should be skipped
		} else {
			t.Error(".git directory should be skipped")
		}

		if _, err := os.Stat(filepath.Join(dstDir, "README.md")); os.IsNotExist(err) {
			t.Error("README.md should be copied")
		}
	})

	t.Run("handles empty directory", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := filepath.Join(t.TempDir(), "dst")

		err := copyRecursive(srcDir, dstDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entries, err := os.ReadDir(dstDir)
		if err != nil {
			t.Fatalf("failed to read dir: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("expected empty dir, got %d entries", len(entries))
		}
	})

	t.Run("creates destination directory", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := filepath.Join(t.TempDir(), "dst")

		os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("data"), 0644)

		err := copyRecursive(srcDir, dstDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dstDir, "file.txt")); os.IsNotExist(err) {
			t.Error("file.txt should exist in nested dst")
		}
	})
}

// --- resolveRemoteVersion (via Installer) ---

func TestResolveRemoteVersion(t *testing.T) {
	installer := &Installer{}

	t.Run("sha from PluginSource", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "url", SHA: "abcdef1234567890"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "abcdef123456" {
			t.Errorf("expected 'abcdef123456', got '%s'", ver)
		}
	})

	t.Run("sha shorter than 12", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "url", SHA: "abc"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "abc" {
			t.Errorf("expected 'abc', got '%s'", ver)
		}
	})

	t.Run("sha from raw map", func(t *testing.T) {
		p := marketplace.Plugin{Source: map[string]interface{}{"source": "url", "sha": "abcdef1234567890"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "abcdef123456" {
			t.Errorf("expected 'abcdef123456', got '%s'", ver)
		}
	})

	t.Run("no sha, no requested version returns latest", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "url"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "latest" {
			t.Errorf("expected 'latest', got '%s'", ver)
		}
	})

	t.Run("no sha, with requested version", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "url"}}
		ver, err := installer.resolveRemoteVersion(&p, "1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "1.0.0" {
			t.Errorf("expected '1.0.0', got '%s'", ver)
		}
	})

	t.Run("no sha, requested latest returns latest", func(t *testing.T) {
		p := marketplace.Plugin{Source: marketplace.PluginSource{Type: "url"}}
		ver, err := installer.resolveRemoteVersion(&p, "latest")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "latest" {
			t.Errorf("expected 'latest', got '%s'", ver)
		}
	})
}
