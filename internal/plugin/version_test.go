package plugin

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/opencode/plugin-cli/internal/gitutil"
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
		path, err := resolver.GetPluginSourcePathWithCtx(&p, PluginResolutionContext{MarketPath: marketPath})
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
		_, err := resolver.GetPluginSourcePathWithCtx(&p, PluginResolutionContext{MarketPath: marketPath})
		if err == nil {
			t.Fatal("expected error for remote source")
		}
	})

	t.Run("raw string source returns error", func(t *testing.T) {
		p := marketplace.Plugin{Source: "./plugins/foo"}
		_, err := resolver.GetPluginSourcePathWithCtx(&p, PluginResolutionContext{MarketPath: marketPath})
		if err == nil {
			t.Fatal("expected error for raw string source")
		}
	})
}

func TestGetPluginSourcePathWithCtx_PluginRoot(t *testing.T) {
	resolver := NewVersionResolver()
	marketPath := filepath.Join(t.TempDir(), "markets", "test")
	os.MkdirAll(filepath.Join(marketPath, "packages", "tool"), 0755)

	p := marketplace.Plugin{Source: &marketplace.LocalSource{Path: "./tool"}}
	ctx := PluginResolutionContext{MarketPath: marketPath, PluginRoot: "packages"}
	path, err := resolver.GetPluginSourcePathWithCtx(&p, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(marketPath, "packages", "tool")
	evalExpected, _ := filepath.EvalSymlinks(expected)
	if evalExpected != "" {
		expected = evalExpected
	}
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestGetPluginSourcePathWithCtx_TraversalCheck(t *testing.T) {
	resolver := NewVersionResolver()
	marketPath := t.TempDir()

	p := marketplace.Plugin{Source: &marketplace.LocalSource{Path: "../../etc/passwd"}}
	ctx := PluginResolutionContext{MarketPath: marketPath, PluginRoot: ""}
	_, err := resolver.GetPluginSourcePathWithCtx(&p, ctx)
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestCloneRemotePlugin_UnsupportedTypes(t *testing.T) {
	resolver := NewVersionResolver()
	cachePath := filepath.Join(t.TempDir(), "cache")

	t.Run("pip returns not implemented", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.PipSource{Package: "my-plugin"}}
		err := resolver.CloneRemotePlugin(&p, cachePath)
		if err == nil {
			t.Fatal("expected error for pip source")
		}
	})
}

func TestCloneRemotePlugin_UsesGitHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("not a git server"))
	}))
	t.Cleanup(server.Close)

	resolver := NewVersionResolver()
	resolver.gitClient = &GitClient{git: gitutil.NewClient(gitutil.Options{Timeout: 20 * time.Millisecond, Attempts: 1})}

	cachePath := filepath.Join(t.TempDir(), "cache")
	p := marketplace.Plugin{Source: &marketplace.URLSource{URL: server.URL + "/plugin.git"}}

	started := time.Now()
	err := resolver.CloneRemotePlugin(&p, cachePath)
	if err == nil {
		t.Fatal("expected clone timeout")
	}
	if elapsed := time.Since(started); elapsed >= 150*time.Millisecond {
		t.Fatalf("CloneRemotePlugin took %s, want timeout before server responds", elapsed)
	}
}

func TestCloneRemotePlugin_SubdirSourceUsesGitHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	t.Cleanup(server.Close)

	resolver := NewVersionResolver()
	resolver.gitClient = &GitClient{git: gitutil.NewClient(gitutil.Options{Timeout: 20 * time.Millisecond, Attempts: 1})}

	cachePath := filepath.Join(t.TempDir(), "cache")
	p := marketplace.Plugin{Source: &marketplace.GitSubdirSource{URL: server.URL + "/plugin.git", SubPath: "skills"}}

	started := time.Now()
	err := resolver.CloneRemotePlugin(&p, cachePath)
	if err == nil {
		t.Fatal("expected clone timeout")
	}
	if elapsed := time.Since(started); elapsed >= 150*time.Millisecond {
		t.Fatalf("CloneRemotePlugin subdir took %s, want timeout before server responds", elapsed)
	}
}

func TestSyncGitSource_UsesGitHTTPTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: go-git file transport depends on system git version compatibility")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	t.Cleanup(server.Close)

	// 在 cachePath 创建一个健康的 git repo，origin 指向慢服务器。
	// cloneGitSource 检测到健康 cache → syncGitSource → fetch 慢服务器 → 超时。
	cacheBase := t.TempDir()
	cachePath := filepath.Join(cacheBase, "cache")
	repo, err := git.PlainInit(cachePath, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	if err := os.WriteFile(filepath.Join(cachePath, "f.txt"), []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("f.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("init", &git.CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{server.URL + "/repo.git"},
	}); err != nil {
		t.Fatal(err)
	}

	resolver := &VersionResolver{
		gitClient: &GitClient{git: gitutil.NewClient(gitutil.Options{Timeout: 20 * time.Millisecond, Attempts: 1})},
	}

	started := time.Now()
	err = resolver.cloneGitSource(server.URL+"/repo.git", "", "", cachePath)
	if err == nil {
		t.Fatal("expected fetch timeout")
	}
	if elapsed := time.Since(started); elapsed >= 150*time.Millisecond {
		t.Fatalf("syncGitSource took %s, want timeout before server responds", elapsed)
	}
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

	t.Run("npm source with version returns version", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "left-pad", Version: "1.3.0"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "1.3.0" {
			t.Errorf("expected '1.3.0', got '%s'", ver)
		}
	})

	t.Run("npm source with explicit requested overrides version", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "left-pad", Version: "1.3.0"}}
		ver, err := installer.resolveRemoteVersion(&p, "2.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "2.0.0" {
			t.Errorf("expected '2.0.0', got '%s'", ver)
		}
	})

	t.Run("npm source local path reads package.json version", func(t *testing.T) {
		pkgDir := t.TempDir()
		pkgJSON := `{"name": "test-pkg", "version": "3.5.0"}`
		os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(pkgJSON), 0644)

		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: pkgDir}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "3.5.0" {
			t.Errorf("expected '3.5.0', got '%s'", ver)
		}
	})
}

type fakeNPMRunner struct {
	calls       []fakeNPMBall
	installFunc func(packageSpec, prefix, registry string) error
}

type fakeNPMBall struct {
	PackageSpec string
	Prefix      string
	Registry    string
}

func (f *fakeNPMRunner) Install(packageSpec, prefix, registry string) error {
	f.calls = append(f.calls, fakeNPMBall{
		PackageSpec: packageSpec,
		Prefix:      prefix,
		Registry:    registry,
	})

	if f.installFunc != nil {
		return f.installFunc(packageSpec, prefix, registry)
	}

	return nil
}

func createFakeNPMPackage(prefix, packageName string) {
	pkgDir := filepath.Join(prefix, "node_modules", packageName)
	os.MkdirAll(pkgDir, 0755)

	manifestDir := filepath.Join(pkgDir, ".claude-plugin")
	os.MkdirAll(manifestDir, 0755)
	os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"test-plugin"}`), 0644)

	skillDir := filepath.Join(pkgDir, "skills")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0644)
}

func createLocalNPMPackageDir(t *testing.T, packageName, version string) string {
	t.Helper()
	pkgDir := t.TempDir()
	pkgJSON := fmt.Sprintf(`{"name": "%s", "version": "%s"}`, packageName, version)
	os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(pkgJSON), 0644)

	manifestDir := filepath.Join(pkgDir, ".claude-plugin")
	os.MkdirAll(manifestDir, 0755)
	os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"test-plugin"}`), 0644)

	skillDir := filepath.Join(pkgDir, "skills")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0644)

	return pkgDir
}

func TestNpmInstall_LocalPackage(t *testing.T) {
	resolver := NewVersionResolver()

	pkgDir := createLocalNPMPackageDir(t, "test-opencode-plugin", "1.0.0")

	fakeRunner := &fakeNPMRunner{
		installFunc: func(packageSpec, prefix, registry string) error {
			createFakeNPMPackage(prefix, "test-opencode-plugin")
			return nil
		},
	}
	resolver.SetNPMRunner(fakeRunner)

	cachePath := filepath.Join(t.TempDir(), "cache", "test-opencode-plugin")
	p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: pkgDir}}
	err := resolver.CloneRemotePlugin(&p, cachePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("expected cache directory to exist")
	}

	if _, err := os.Stat(filepath.Join(cachePath, ".claude-plugin", "plugin.json")); os.IsNotExist(err) {
		t.Fatal("expected plugin.json to exist in cache")
	}

	if len(fakeRunner.calls) != 1 {
		t.Fatalf("expected 1 npm call, got %d", len(fakeRunner.calls))
	}
	if fakeRunner.calls[0].PackageSpec != pkgDir {
		t.Errorf("expected packageSpec %s, got %s", pkgDir, fakeRunner.calls[0].PackageSpec)
	}
}

func TestNpmInstall_ScopedLocalPackage(t *testing.T) {
	resolver := NewVersionResolver()

	pkgDir := createLocalNPMPackageDir(t, "@scope/test-opencode-plugin", "2.0.0")

	fakeRunner := &fakeNPMRunner{
		installFunc: func(packageSpec, prefix, registry string) error {
			pkgPath := filepath.Join(prefix, "node_modules", "@scope", "test-opencode-plugin")
			os.MkdirAll(pkgPath, 0755)
			manifestDir := filepath.Join(pkgPath, ".claude-plugin")
			os.MkdirAll(manifestDir, 0755)
			os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"scoped-plugin"}`), 0644)
			return nil
		},
	}
	resolver.SetNPMRunner(fakeRunner)

	cachePath := filepath.Join(t.TempDir(), "cache", "scoped-plugin")
	p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: pkgDir}}
	err := resolver.CloneRemotePlugin(&p, cachePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("expected cache directory to exist")
	}

	if _, err := os.Stat(filepath.Join(cachePath, ".claude-plugin", "plugin.json")); os.IsNotExist(err) {
		t.Fatal("expected plugin.json in cached scoped package")
	}

	if len(fakeRunner.calls) != 1 {
		t.Fatalf("expected 1 npm call, got %d", len(fakeRunner.calls))
	}

	if !strings.Contains(fakeRunner.calls[0].PackageSpec, pkgDir) {
		t.Errorf("expected packageSpec to contain %s, got %s", pkgDir, fakeRunner.calls[0].PackageSpec)
	}
}

func TestNpmInstall_RegistryPackageWithVersion(t *testing.T) {
	resolver := NewVersionResolver()

	fakeRunner := &fakeNPMRunner{
		installFunc: func(packageSpec, prefix, registry string) error {
			createFakeNPMPackage(prefix, "left-pad")
			return nil
		},
	}
	resolver.SetNPMRunner(fakeRunner)

	cachePath := filepath.Join(t.TempDir(), "cache", "left-pad")
	p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "left-pad", Version: "1.3.0"}}
	err := resolver.CloneRemotePlugin(&p, cachePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fakeRunner.calls) != 1 {
		t.Fatalf("expected 1 npm call, got %d", len(fakeRunner.calls))
	}
	if fakeRunner.calls[0].PackageSpec != "left-pad@1.3.0" {
		t.Errorf("expected packageSpec 'left-pad@1.3.0', got '%s'", fakeRunner.calls[0].PackageSpec)
	}
}

func TestNpmInstall_RegistryPackageScoped(t *testing.T) {
	resolver := NewVersionResolver()

	fakeRunner := &fakeNPMRunner{
		installFunc: func(packageSpec, prefix, registry string) error {
			pkgPath := filepath.Join(prefix, "node_modules", "@scope", "my-plugin")
			os.MkdirAll(pkgPath, 0755)
			os.WriteFile(filepath.Join(pkgPath, "skill.md"), []byte("# Skill"), 0644)
			return nil
		},
	}
	resolver.SetNPMRunner(fakeRunner)

	cachePath := filepath.Join(t.TempDir(), "cache", "scoped-reg")
	p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "@scope/my-plugin", Version: "2.0.0"}}
	err := resolver.CloneRemotePlugin(&p, cachePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeRunner.calls[0].PackageSpec != "@scope/my-plugin@2.0.0" {
		t.Errorf("expected packageSpec '@scope/my-plugin@2.0.0', got '%s'", fakeRunner.calls[0].PackageSpec)
	}
}

func TestNpmInstall_RegistryWithCustomRegistry(t *testing.T) {
	resolver := NewVersionResolver()

	fakeRunner := &fakeNPMRunner{
		installFunc: func(packageSpec, prefix, registry string) error {
			createFakeNPMPackage(prefix, "my-pkg")
			return nil
		},
	}
	resolver.SetNPMRunner(fakeRunner)

	cachePath := filepath.Join(t.TempDir(), "cache", "custom-reg")
	p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "my-pkg", Version: "1.0.0", Registry: "https://my.registry.com"}}
	err := resolver.CloneRemotePlugin(&p, cachePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeRunner.calls[0].Registry != "https://my.registry.com" {
		t.Errorf("expected registry 'https://my.registry.com', got '%s'", fakeRunner.calls[0].Registry)
	}
}

func TestNpmInstall_LocalPackageRejectsVersion(t *testing.T) {
	resolver := NewVersionResolver()

	pkgDir := createLocalNPMPackageDir(t, "test-pkg", "1.0.0")
	resolver.SetNPMRunner(&fakeNPMRunner{})

	cachePath := filepath.Join(t.TempDir(), "cache", "test")
	p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: pkgDir, Version: "1.0.0"}}
	err := resolver.CloneRemotePlugin(&p, cachePath)
	if err == nil {
		t.Fatal("expected error for local package with version")
	}
}

func TestNpmInstall_VersionResolution(t *testing.T) {
	installer := &Installer{}

	t.Run("registry package version from source", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "left-pad", Version: "1.3.0"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "1.3.0" {
			t.Errorf("expected '1.3.0', got '%s'", ver)
		}
	})

	t.Run("local path reads package.json version", func(t *testing.T) {
		pkgDir := t.TempDir()
		os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"name":"local-pkg","version":"1.0.0"}`), 0644)

		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: pkgDir}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "1.0.0" {
			t.Errorf("expected '1.0.0', got '%s'", ver)
		}
	})

	t.Run("explicit CLI version wins", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "left-pad", Version: "1.3.0"}}
		ver, err := installer.resolveRemoteVersion(&p, "2.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "2.0.0" {
			t.Errorf("expected '2.0.0', got '%s'", ver)
		}
	})

	t.Run("no version returns latest", func(t *testing.T) {
		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "left-pad"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "latest" {
			t.Errorf("expected 'latest', got '%s'", ver)
		}
	})

	t.Run("registry package name matching cwd directory is not local", func(t *testing.T) {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd() error = %v", err)
		}
		tmp := t.TempDir()
		if err := os.Mkdir(filepath.Join(tmp, "left-pad"), 0755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.Chdir(tmp); err != nil {
			t.Fatalf("Chdir() error = %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(cwd)
		})

		p := marketplace.Plugin{Source: &marketplace.NpmSource{Package: "left-pad", Version: "1.3.0"}}
		ver, err := installer.resolveRemoteVersion(&p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "1.3.0" {
			t.Errorf("expected '1.3.0', got '%s'", ver)
		}
	})
}

func TestExtractPackageNameFromSpec(t *testing.T) {
	tests := []struct {
		spec     string
		expected string
	}{
		{"left-pad", "left-pad"},
		{"@scope/name", "@scope/name"},
		{"simple-pkg", "simple-pkg"},
	}

	for _, tt := range tests {
		got := extractPackageNameFromSpec(tt.spec)
		if got != tt.expected {
			t.Errorf("extractPackageNameFromSpec(%q) = %q, want %q", tt.spec, got, tt.expected)
		}
	}
}
