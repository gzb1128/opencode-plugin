package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/opencode/plugin-cli/internal/pathutil"
)

type PluginResolutionContext struct {
	MarketPath string
	PluginRoot string
}

type NPMRunner interface {
	Install(packageSpec, prefix, registry string) error
}

type VersionResolver struct {
	gitClient *GitClient
	npmRunner NPMRunner
}

type GitClient struct{}

type productionNPMRunner struct{}

func (r *productionNPMRunner) Install(packageSpec, prefix, registry string) error {
	args := []string{"install", packageSpec, "--prefix", prefix}
	if registry != "" {
		args = append(args, "--registry", registry)
	}
	cmd := exec.Command("npm", args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func NewVersionResolver() *VersionResolver {
	return &VersionResolver{
		gitClient: &GitClient{},
		npmRunner: &productionNPMRunner{},
	}
}

func (v *VersionResolver) SetNPMRunner(runner NPMRunner) {
	v.npmRunner = runner
}

func (v *VersionResolver) Resolve(pluginPath string, requested string) (string, error) {
	if requested != "" && requested != "latest" {
		return requested, nil
	}

	pluginJSONPath := filepath.Join(pluginPath, ".claude-plugin", "plugin.json")
	version, err := v.readPluginJSONVersion(pluginJSONPath)
	if err == nil && version != "" && version != "latest" {
		return version, nil
	}

	sha, err := v.gitClient.GetCommitSHA(pluginPath)
	if err == nil && sha != "" {
		if len(sha) > 12 {
			return sha[:12], nil
		}
		return sha, nil
	}

	return "latest", nil
}

func (v *VersionResolver) readPluginJSONVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read plugin.json: %w", err)
	}

	var pluginData struct {
		Version string `json:"version"`
	}

	if err := json.Unmarshal(data, &pluginData); err != nil {
		return "", fmt.Errorf("failed to parse plugin.json: %w", err)
	}

	return pluginData.Version, nil
}

func (g *GitClient) GetCommitSHA(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", err
	}

	ref, err := repo.Head()
	if err != nil {
		return "", err
	}

	return ref.Hash().String(), nil
}

func (v *VersionResolver) GetPluginSourcePathWithCtx(plugin *marketplace.Plugin, ctx PluginResolutionContext) (string, error) {
	src, ok := plugin.Source.(marketplace.PluginSource)
	if !ok {
		return "", fmt.Errorf("invalid plugin source format")
	}

	switch s := src.(type) {
	case *marketplace.LocalSource:
		base := ctx.MarketPath
		if ctx.PluginRoot != "" {
			base = filepath.Join(base, ctx.PluginRoot)
		}
		resolved := filepath.Join(base, s.Path)
		if ctx.MarketPath != "" {
			validated, err := pathutil.ResolvePathWithinBase(ctx.MarketPath, filepath.Join(func() string {
				if ctx.PluginRoot != "" {
					return ctx.PluginRoot + string(filepath.Separator) + s.Path
				}
				return s.Path
			}()))
			if err != nil {
				return "", fmt.Errorf("plugin source path escapes marketplace root: %w", err)
			}
			return validated, nil
		}
		return resolved, nil
	default:
		return "", fmt.Errorf("unsupported plugin source type: %s (plugin may need to be cloned first)", src.SourceType())
	}
}

func (v *VersionResolver) IsRemoteSource(plugin *marketplace.Plugin) bool {
	src, ok := plugin.Source.(marketplace.PluginSource)
	if !ok {
		return false
	}

	switch src.(type) {
	case *marketplace.GitHubSource, *marketplace.URLSource, *marketplace.GitSource, *marketplace.GitSubdirSource, *marketplace.NpmSource, *marketplace.PipSource:
		return true
	default:
		return false
	}
}

func (v *VersionResolver) CloneRemotePlugin(plugin *marketplace.Plugin, cachePath string) error {
	src, ok := plugin.Source.(marketplace.PluginSource)
	if !ok {
		return fmt.Errorf("invalid plugin source format")
	}
	return v.clonePluginSource(src, cachePath)
}

func (v *VersionResolver) clonePluginSource(src marketplace.PluginSource, cachePath string) error {
	switch s := src.(type) {
	case *marketplace.GitHubSource:
		gitURL := "https://github.com/" + s.Repo + ".git"
		return v.cloneGitSource(gitURL, s.Ref, s.SHA, cachePath)
	case *marketplace.URLSource:
		return v.cloneGitSource(s.URL, s.Ref, s.SHA, cachePath)
	case *marketplace.GitSource:
		return v.cloneGitSource(s.URL, s.Ref, s.SHA, cachePath)
	case *marketplace.GitSubdirSource:
		return v.cloneGitSubdirSource(s, cachePath)
	case *marketplace.NpmSource:
		return v.installNpmSource(s, cachePath)
	case *marketplace.PipSource:
		return fmt.Errorf("pip source installation not yet implemented for package: %s", s.Package)
	default:
		return fmt.Errorf("source type '%s' is not a remote source", src.SourceType())
	}
}

func (v *VersionResolver) installNpmSource(src *marketplace.NpmSource, cachePath string) error {
	if _, err := os.Stat(cachePath); err == nil {
		os.RemoveAll(cachePath)
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	isLocal := src.Package != "" && isLocalPath(src.Package)

	var packageSpec string
	var resolvedName string

	if isLocal {
		if src.Version != "" {
			return fmt.Errorf("npm source with local path must not specify version: %s", src.Version)
		}
		packageSpec = src.Package
		var err error
		resolvedName, err = readPackageNameFromJSON(src.Package)
		if err != nil {
			return fmt.Errorf("failed to read package name from local package: %w", err)
		}
	} else {
		packageSpec = src.Package
		if src.Version != "" {
			packageSpec = src.Package + "@" + src.Version
		}
		resolvedName = extractPackageNameFromSpec(src.Package)
	}

	npmCacheDir, err := os.MkdirTemp("", "opencode-npm-cache-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir for npm install: %w", err)
	}
	defer os.RemoveAll(npmCacheDir)

	if err := v.npmRunner.Install(packageSpec, npmCacheDir, src.Registry); err != nil {
		return fmt.Errorf("npm install failed: %w", err)
	}

	installedPath := filepath.Join(npmCacheDir, "node_modules", resolvedName)
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		return fmt.Errorf("installed package not found at %s", installedPath)
	}

	if err := copyRecursive(installedPath, cachePath); err != nil {
		return fmt.Errorf("failed to copy installed package to cache: %w", err)
	}

	return nil
}

func isLocalPath(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.HasPrefix(path, "~/") {
		return true
	}
	return false
}

func readPackageNameFromJSON(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", fmt.Errorf("failed to read package.json: %w", err)
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("failed to parse package.json: %w", err)
	}
	if pkg.Name == "" {
		return "", fmt.Errorf("package.json missing name field")
	}
	return pkg.Name, nil
}

func extractPackageNameFromSpec(spec string) string {
	if strings.HasPrefix(spec, "@") {
		parts := strings.SplitN(spec, "/", 2)
		if len(parts) == 2 {
			scopeAndName := parts[0] + "/" + strings.SplitN(parts[1], "@", 2)[0]
			return scopeAndName
		}
		return spec
	}
	return strings.SplitN(spec, "@", 2)[0]
}

func (v *VersionResolver) cloneGitSource(gitURL, ref, sha, cachePath string) error {
	// 如果 cache 已经是健康的 git repo，用 fetch + hard reset 更新工作树，
	// 而不是 wipe + 重新 clone。这对 superpowers 这种大仓库（10-30MB）
	// 能省掉完整的网络下载和磁盘写入——只在 fetch 增量对象 + reset 工作树。
	// 失败时回退到原本的 fresh clone 路径，保证行为兼容。
	if _, err := os.Stat(cachePath); err == nil {
		if isRepoHealthy(cachePath) {
			if syncErr := v.syncGitSource(gitURL, ref, sha, cachePath); syncErr == nil {
				return nil
			} else {
				// sync 失败信息走 stderr，让只捕获 stderr 的 CI 也能看到回退原因。
				fmt.Fprintf(os.Stderr, "  cache sync failed (%v), falling back to fresh clone\n", syncErr)
			}
		}
		// 不健康或 sync 失败：清掉重 clone
		os.RemoveAll(cachePath)
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// 不递归 submodule：很多 plugin 仓库的 .gitmodules 用 SSH URL（如
	// superpowers 的 evals 子模块），go-git 内置 SSH 客户端拿不到 ssh-agent
	// 密钥时会握手失败，导致整个 plugin clone 失败。Plugin 运行时几乎不需要
	// submodule 内容（通常是 evals / tests），所以默认不递归。
	_, err := git.PlainClone(cachePath, false, &git.CloneOptions{
		URL: gitURL,
	})
	if err != nil {
		os.RemoveAll(cachePath)
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	repo, err := git.PlainOpen(cachePath)
	if err != nil {
		os.RemoveAll(cachePath)
		return fmt.Errorf("failed to open cloned repository: %w", err)
	}
	if _, err := repo.Head(); err != nil {
		os.RemoveAll(cachePath)
		return fmt.Errorf("cloned repository has no HEAD (empty or corrupted): %w", err)
	}

	if sha != "" {
		if err := checkoutSHA(cachePath, sha); err != nil {
			return err
		}
	} else if ref != "" {
		if err := checkoutRef(cachePath, ref); err != nil {
			return err
		}
	}

	return nil
}

// syncGitSource 在已有的 cache 目录上做 fetch + hard reset，而不是 fresh clone。
// 前提：cachePath 已存在且 isRepoHealthy(cachePath) == true。
func (v *VersionResolver) syncGitSource(gitURL, ref, sha, cachePath string) error {
	repo, err := git.PlainOpen(cachePath)
	if err != nil {
		return fmt.Errorf("failed to open existing cache: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// 先 hard reset 清掉任何本地改动（cache 应是只读源，不应有改动，但兜底）。
	if err := wt.Reset(&git.ResetOptions{Mode: git.HardReset}); err != nil {
		return fmt.Errorf("failed to reset worktree: %w", err)
	}

	// fetch 所有 tags 和分支的增量对象。
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to find origin remote: %w", err)
	}

	// 校验缓存的 remote URL 还是我们想要的 URL。
	// 如果 marketplace 的 source.github.repo 变了（plugin 搬家 / 镜像切换），
	// 缓存里的 origin 仍指向旧 URL——fetch 会"成功"但拉到的是旧内容。
	// 这里返回 error 让外层回退到 fresh clone。
	if urls := remote.Config().URLs; len(urls) == 0 {
		return fmt.Errorf("cached repo has no origin URL configured")
	} else if !matchingGitURL(urls[0], gitURL) {
		return fmt.Errorf("cached origin URL %s does not match requested %s (marketplace source changed?)", urls[0], gitURL)
	}

	if err := remote.Fetch(&git.FetchOptions{Tags: git.AllTags}); err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	// checkout 到指定 sha/ref 或 pull 当前分支最新。
	if sha != "" {
		return checkoutSHA(cachePath, sha)
	}
	if ref != "" {
		return checkoutRef(cachePath, ref)
	}
	if err := wt.Pull(&git.PullOptions{}); err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull: %w", err)
	}
	return nil
}

func (v *VersionResolver) cloneGitSubdirSource(src *marketplace.GitSubdirSource, cachePath string) error {
	gitURL := src.URL

	if _, err := os.Stat(cachePath); err == nil {
		os.RemoveAll(cachePath)
	}

	tempDir, err := os.MkdirTemp("", "opencode-plugin-clone-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 同 cloneGitSource，不递归 submodule（见上注释）
	_, err = git.PlainClone(tempDir, false, &git.CloneOptions{
		URL: gitURL,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if src.SHA != "" {
		if err := checkoutSHA(tempDir, src.SHA); err != nil {
			return err
		}
	} else if src.Ref != "" {
		if err := checkoutRef(tempDir, src.Ref); err != nil {
			return err
		}
	}

	srcDir := filepath.Join(tempDir, src.SubPath)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("subdirectory '%s' not found in repository", src.SubPath)
	}

	if err := copyRecursive(srcDir, cachePath); err != nil {
		return fmt.Errorf("failed to copy plugin files: %w", err)
	}

	return nil
}

func checkoutSHA(repoPath, sha string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(sha))
	if err != nil {
		return fmt.Errorf("failed to resolve SHA %s: %w", sha, err)
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
		return fmt.Errorf("failed to checkout SHA %s: %w", sha, err)
	}
	return nil
}

func checkoutRef(repoPath, ref string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	branchErr := worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(ref)})
	if branchErr == nil {
		return nil
	}

	tagErr := worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewTagReferenceName(ref)})
	if tagErr == nil {
		return nil
	}

	return fmt.Errorf("failed to checkout ref %s: not found as branch (%v) or tag (%v)", ref, branchErr, tagErr)
}

// copyRecursive 是 copyDirTree(src, dst, skipGit) 的薄包装。
// 之前版本里有一个独立的 ~50 行递归拷贝实现，功能和 installer.go 的
// copyDirTree 几乎完全重复（都跳过 .git、跳过 symlink、保留 mode）。
// 统一到一个实现减少维护成本，并复用 copyFilePreserveMode 对 Close 错误的检查。
func copyRecursive(src, dst string) error {
	return copyDirTree(src, dst, map[string]bool{".git": true})
}

// matchingGitURL 判断两个 git URL 是否等价。
// 规范化差异：trailing .git（github.com/foo/bar.git vs github.com/foo/bar）。
// 严格的字符串比较就足够——不会出现 https vs ssh 混用，因为 cloneGitSource
// 始终构造 https URL。
func matchingGitURL(a, b string) bool {
	normalize := func(s string) string {
		s = strings.TrimSuffix(s, ".git")
		return s
	}
	return normalize(a) == normalize(b)
}

func (v *VersionResolver) GetAvailableVersions(pluginPath string) ([]string, error) {
	versions := []string{}

	pluginJSONPath := filepath.Join(pluginPath, ".claude-plugin", "plugin.json")
	version, err := v.readPluginJSONVersion(pluginJSONPath)
	if err == nil && version != "" {
		versions = append(versions, version)
	}

	repo, err := git.PlainOpen(pluginPath)
	if err == nil {
		tags, err := repo.Tags()
		if err == nil {
			tags.ForEach(func(ref *plumbing.Reference) error {
				tagName := ref.Name().Short()
				if strings.HasPrefix(tagName, "v") || strings.HasPrefix(tagName, "0") || strings.HasPrefix(tagName, "1") || strings.HasPrefix(tagName, "2") {
					versions = append(versions, tagName)
				}
				return nil
			})
		}
	}

	versions = append(versions, "latest")

	return versions, nil
}
