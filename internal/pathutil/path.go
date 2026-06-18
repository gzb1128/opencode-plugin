package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var safeCharRegex = regexp.MustCompile(`[^A-Za-z0-9@._-]`)

func ResolvePathWithinBase(basePath, relativePath string) (string, error) {
	cleanBase, err := filepath.Abs(filepath.Clean(basePath))
	if err != nil {
		return "", fmt.Errorf("failed to resolve base path: %w", err)
	}

	evalBase := cleanBase
	if ev, err := filepath.EvalSymlinks(cleanBase); err == nil {
		evalBase = ev
	}

	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("relative path must not be absolute: %s", relativePath)
	}

	for _, part := range strings.Split(relativePath, string(filepath.Separator)) {
		if part == ".." {
			return "", fmt.Errorf("relative path must not contain '..': %s", relativePath)
		}
	}

	resolved := filepath.Join(cleanBase, relativePath)
	resolved = filepath.Clean(resolved)

	if !strings.HasPrefix(resolved, cleanBase+string(filepath.Separator)) && resolved != cleanBase {
		return "", fmt.Errorf("path %s escapes base %s", resolved, cleanBase)
	}

	if _, err := os.Stat(resolved); err == nil {
		evaluated, err := filepath.EvalSymlinks(resolved)
		if err != nil {
			return "", fmt.Errorf("failed to evaluate symlinks: %w", err)
		}
		if !isWithinBase(evaluated, evalBase) {
			return "", fmt.Errorf("symlink target %s escapes base %s", evaluated, evalBase)
		}
		return evaluated, nil
	}

	dir := filepath.Dir(resolved)
	for len(dir) >= len(cleanBase) {
		info, err := os.Lstat(dir)
		if err != nil {
			dir = filepath.Dir(dir)
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return "", fmt.Errorf("failed to evaluate symlink %s: %w", dir, err)
			}
			if !isWithinBase(target, evalBase) {
				return "", fmt.Errorf("symlink target %s escapes base %s", target, evalBase)
			}
		}
		dir = filepath.Dir(dir)
		if dir == filepath.Dir(dir) {
			break
		}
	}

	return resolved, nil
}

func SafePluginCachePath(cacheDir, pluginID, version string) (string, error) {
	var pluginName, marketName string
	lastAt := strings.LastIndex(pluginID, "@")
	if lastAt > 0 {
		pluginName = pluginID[:lastAt]
		marketName = pluginID[lastAt+1:]
	} else {
		pluginName = pluginID
	}

	sanitizedPlugin := SanitizeAlias(pluginName)
	sanitizedMarket := SanitizeAlias(marketName)
	sanitizedVersion := SanitizeAlias(version)

	if sanitizedPlugin == "" || sanitizedPlugin == "." || sanitizedPlugin == ".." {
		return "", fmt.Errorf("invalid plugin name after sanitization: %q", pluginName)
	}
	if sanitizedMarket == "" || sanitizedMarket == "." || sanitizedMarket == ".." {
		return "", fmt.Errorf("invalid marketplace name after sanitization: %q", marketName)
	}
	if sanitizedVersion == "" || sanitizedVersion == "." || sanitizedVersion == ".." {
		return "", fmt.Errorf("invalid version after sanitization: %q", version)
	}

	relativePath := filepath.Join(sanitizedMarket, sanitizedPlugin, sanitizedVersion)
	resolved, err := ResolvePathWithinBase(cacheDir, relativePath)
	if err != nil {
		return "", fmt.Errorf("plugin cache path escapes cache directory: %w", err)
	}

	absCacheDir, _ := filepath.Abs(filepath.Clean(cacheDir))
	if resolved == absCacheDir {
		return "", fmt.Errorf("plugin cache path must be a child of cache directory, not the directory itself")
	}

	return resolved, nil
}

func isWithinBase(path, base string) bool {
	return strings.HasPrefix(path, base+string(filepath.Separator)) || path == base
}

// IsWithinDir 判断 path 是否词法和 symlink 解析后都在 base 之内。
// 用于所有 "拒绝删除/操作 base 之外的路径" 的安全守卫。
// 之前 plugin.isWithinDir 和 marketplace.isWithinMarketsDir 各写了一份，
// 语义略有差别——这里采用 plugin 版本（更宽松，能处理 path 还不存在的情况）。
func IsWithinDir(path, base string) bool {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	absBase, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return false
	}
	sep := string(filepath.Separator)
	if !strings.HasPrefix(absPath, absBase+sep) {
		return false
	}
	evalBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		return false
	}
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return strings.HasPrefix(evalPath, evalBase+sep)
	}
	if !os.IsNotExist(err) {
		return false
	}
	// path 本身不存在（例如 cache 刚被 rename 走），向上查找最近的 symlink 祖先
	// 验证它仍解析到 base 之下，避免 ../escape 通过不存在的中间段绕过检查。
	dir := absPath
	for len(dir) > len(absBase) {
		dir = filepath.Dir(dir)
		info, err := os.Lstat(dir)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return false
			}
			if !strings.HasPrefix(target, evalBase+sep) {
				return false
			}
		}
	}
	return true
}

func SanitizeAlias(alias string) string {
	return safeCharRegex.ReplaceAllString(alias, "-")
}

func SafeMarketplaceCachePath(marketsDir, alias, suffix string) (string, error) {
	sanitized := SanitizeAlias(alias)
	if sanitized == "" || sanitized == "." || sanitized == ".." {
		return "", fmt.Errorf("invalid alias after sanitization: %q (original: %q)", sanitized, alias)
	}

	candidate := sanitized + suffix
	resolved, err := ResolvePathWithinBase(marketsDir, candidate)
	if err != nil {
		return "", fmt.Errorf("cache path escapes markets directory: %w", err)
	}

	absMarketsDir, _ := filepath.Abs(filepath.Clean(marketsDir))
	if resolved == absMarketsDir {
		return "", fmt.Errorf("cache path must be a child of markets directory, not the directory itself")
	}

	return resolved, nil
}
