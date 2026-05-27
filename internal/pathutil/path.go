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
			if !isWithinBase(target, evalBase) && !isAncestorOf(target, evalBase) {
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

func isWithinBase(path, base string) bool {
	return strings.HasPrefix(path, base+string(filepath.Separator)) || path == base
}

func isAncestorOf(candidate, descendant string) bool {
	return strings.HasPrefix(descendant, candidate+string(filepath.Separator))
}

func sanitizeAlias(alias string) string {
	return safeCharRegex.ReplaceAllString(alias, "-")
}

func SafeMarketplaceCachePath(marketsDir, alias, suffix string) (string, error) {
	sanitized := sanitizeAlias(alias)
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
