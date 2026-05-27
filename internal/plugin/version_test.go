package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode/plugin-cli/internal/marketplace"
)

func TestIsRemoteSource(t *testing.T) {
	resolver := NewVersionResolver()

	t.Run("url type", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.URLSource{URL: "https://example.com/foo.git"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for url type")
		}
	})

	t.Run("github type", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.GitHubSource{Repo: "owner/repo"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for github type")
		}
	})

	t.Run("git-subdir type", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.GitSubdirSource{URL: "https://example.com/repo.git"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for git-subdir type")
		}
	})

	t.Run("git type", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.GitSource{URL: "https://gitlab.com/repo.git"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for git type")
		}
	})

	t.Run("npm type", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "@org/plugin"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for npm type")
		}
	})

	t.Run("pip type", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.PipSource{Package: "my-plugin"}}
		if !resolver.IsRemoteSource(&p) {
			t.Fatal("expected remote for pip type")
		}
	})

	t.Run("local type", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.LocalSource{Path: "./plugins/foo"}}
		if resolver.IsRemoteSource(&p) {
			t.Fatal("local type should not be remote")
		}
	})

	t.Run("string source is not remote", func(t *testing.T) {
		p := marketplace.Plugin{Source: "./plugins/foo"}
		if resolver.IsRemoteSource(&p) {
			t.Fatal("raw string source should not be remote")
		}
	})
}

func TestGetPluginSourcePath(t *testing.T) {
	resolver := NewVersionResolver()
	marketPath := filepath.Join(t.TempDir(), "markets", "test")

	t.Run("local source", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.LocalSource{Path: "./plugins/foo"}}
		path, err := resolver.GetPluginSourcePath(&p, marketPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(marketPath, "plugins", "foo")
		if path != expected {
			t.Errorf("expected %s, got %s", expected, path)
		}
	})

	t.Run("remote source returns error", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.GitHubSource{Repo: "owner/repo"}}
		_, err := resolver.GetPluginSourcePath(&p, marketPath)
		if err == nil {
			t.Fatal("expected error for remote source")
		}
	})

	t.Run("raw string source returns error", func(t *testing.T) {
		p := marketplace.Plugin{Source: "./plugins/foo"}
		_, err := resolver.GetPluginSourcePath(&p, marketPath)
		if err == nil {
			t.Fatal("expected error for raw string source")
		}
	})
}

func TestCloneRemotePlugin_UnsupportedTypes(t *testing.T) {
	resolver := NewVersionResolver()
	cachePath := filepath.Join(t.TempDir(), "cache")

	t.Run("npm returns not implemented", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "@org/plugin"}}
		err := resolver.CloneRemotePlugin(&p, cachePath)
		if err == nil {
			t.Fatal("expected error for npm source")
		}
	})

	t.Run("pip returns not implemented", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.PipSource{Package: "my-plugin"}}
		err := resolver.CloneRemotePlugin(&p, cachePath)
		if err == nil {
			t.Fatal("expected error for pip source")
		}
	})
}

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

func TestResolveRemoteVersion(t *testing.T) {
	installer := &Installer{}

	t.Run("sha from GitHubSource", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.GitHubSource{SHA: "abcdef1234567890abcdef1234567890abcdef12"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "abcdef123456" {
			t.Errorf("expected 'abcdef123456', got '%s'", ver)
		}
	})

	t.Run("sha from URLSource", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.URLSource{SHA: "abcdef1234567890abcdef1234567890abcdef12"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "abcdef123456" {
			t.Errorf("expected 'abcdef123456', got '%s'", ver)
		}
	})

	t.Run("no sha, with requested version", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.GitHubSource{Repo: "owner/repo"}}
		ver, err := installer.resolveRemoteVersion(&p, "1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "1.0.0" {
			t.Errorf("expected '1.0.0', got '%s'", ver)
		}
	})

	t.Run("no sha, no requested returns latest", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.GitHubSource{Repo: "owner/repo"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "latest" {
			t.Errorf("expected 'latest', got '%s'", ver)
		}
	})

	t.Run("npm source no sha returns latest", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "@org/plugin"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "latest" {
			t.Errorf("expected 'latest', got '%s'", ver)
		}
	})
}
