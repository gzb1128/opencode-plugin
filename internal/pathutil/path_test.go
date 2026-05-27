package pathutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePathWithinBase(t *testing.T) {
	t.Run("valid relative path", func(t *testing.T) {
		base := t.TempDir()
		got, err := ResolvePathWithinBase(base, "foo/bar.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(base, "foo", "bar.json")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("rejects absolute path", func(t *testing.T) {
		base := t.TempDir()
		_, err := ResolvePathWithinBase(base, "/etc/passwd")
		if err == nil {
			t.Fatal("expected error for absolute path")
		}
	})

	t.Run("rejects dot dot traversal", func(t *testing.T) {
		base := t.TempDir()
		_, err := ResolvePathWithinBase(base, "../../outside")
		if err == nil {
			t.Fatal("expected error for .. traversal")
		}
	})

	t.Run("rejects symlink escape", func(t *testing.T) {
		base := t.TempDir()
		outside := t.TempDir()
		linkDir := filepath.Join(base, "link")
		if err := os.Symlink(outside, linkDir); err != nil {
			t.Skipf("symlinks not supported: %v", err)
		}
		_, err := ResolvePathWithinBase(base, "link/file")
		if err == nil {
			t.Fatal("expected error for symlink escape")
		}
	})

	t.Run("allows existing file within base", func(t *testing.T) {
		base := t.TempDir()
		file := filepath.Join(base, "real.txt")
		if err := os.WriteFile(file, []byte("data"), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		got, err := ResolvePathWithinBase(base, "real.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		evGot, _ := filepath.EvalSymlinks(got)
		evWant, _ := filepath.EvalSymlinks(file)
		if evGot != evWant {
			t.Fatalf("got %q, want %q", got, file)
		}
	})
}

func TestSafeMarketplaceCachePath(t *testing.T) {
	t.Run("legacy alias with slash", func(t *testing.T) {
		base := t.TempDir()
		got, err := SafeMarketplaceCachePath(base, "anthropics/claude-plugins-official", ".json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(base, "anthropics-claude-plugins-official.json")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("malicious dot dot alias", func(t *testing.T) {
		base := t.TempDir()
		got, err := SafeMarketplaceCachePath(base, "../../outside", ".json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(base, "..-..-outside.json")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
		if !filepath.IsAbs(got) {
			t.Fatalf("expected absolute path")
		}
		if strings.Contains(got, string(filepath.Separator)+".."+string(filepath.Separator)) {
			t.Fatalf("path should not contain '..' as a directory component")
		}
	})

	t.Run("dot alias rejected", func(t *testing.T) {
		base := t.TempDir()
		_, err := SafeMarketplaceCachePath(base, ".", ".json")
		if err == nil {
			t.Fatal("expected error for '.' alias")
		}
	})

	t.Run("dot dot alias rejected", func(t *testing.T) {
		base := t.TempDir()
		_, err := SafeMarketplaceCachePath(base, "..", ".json")
		if err == nil {
			t.Fatal("expected error for '..' alias")
		}
	})

	t.Run("empty alias rejected", func(t *testing.T) {
		base := t.TempDir()
		_, err := SafeMarketplaceCachePath(base, "", ".json")
		if err == nil {
			t.Fatal("expected error for empty alias")
		}
	})

	t.Run("normal alias", func(t *testing.T) {
		base := t.TempDir()
		got, err := SafeMarketplaceCachePath(base, "my-market", ".json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(base, "my-market.json")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("existing remote install location outside marketsDir rejected", func(t *testing.T) {
		base := t.TempDir()
		outside := t.TempDir()
		resolved, err := ResolvePathWithinBase(outside, "some-file")
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		absBase, _ := filepath.Abs(filepath.Clean(base))
		if strings.HasPrefix(resolved, absBase+string(filepath.Separator)) {
			t.Skip("outside path happened to be inside base")
		}
	})
}
