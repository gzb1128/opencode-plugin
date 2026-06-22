package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencode/plugin-cli/internal/pathutil"
)

type Linker struct {
	agentsDir string
}

func NewLinker(agentsDir string) *Linker {
	return &Linker{
		agentsDir: agentsDir,
	}
}

type ComponentCounts struct {
	Skills   int
	Commands int
	Agents   int
}

type ComponentPath struct {
	Name string
	Path string
}

func (l *Linker) CreateSymlinksFromManifest(pluginPath string, manifestData map[string]interface{}, force bool) (*ComponentCounts, error) {
	counts := &ComponentCounts{}

	skillPaths, skillFromManifest := l.extractComponentPaths(pluginPath, manifestData, "skills")
	cmdPaths, cmdFromManifest := l.extractCommandPaths(pluginPath, manifestData)
	agentPaths, agentFromManifest := l.extractComponentPaths(pluginPath, manifestData, "agents")

	if skillFromManifest {
		n, _, err := l.linkComponentPaths(pluginPath, "skills", skillPaths, force)
		if err != nil {
			return nil, err
		}
		counts.Skills = n
	} else {
		n, _, err := l.linkComponentDir(pluginPath, "skills", force)
		if err != nil {
			return nil, err
		}
		counts.Skills = n
	}

	if cmdFromManifest {
		n, _, err := l.linkComponentPaths(pluginPath, "commands", cmdPaths, force)
		if err != nil {
			return nil, err
		}
		counts.Commands = n
	} else {
		n, _, err := l.linkComponentDir(pluginPath, "commands", force)
		if err != nil {
			return nil, err
		}
		counts.Commands = n
	}

	if agentFromManifest {
		n, _, err := l.linkComponentPaths(pluginPath, "agents", agentPaths, force)
		if err != nil {
			return nil, err
		}
		counts.Agents = n
	} else {
		n, _, err := l.linkComponentDir(pluginPath, "agents", force)
		if err != nil {
			return nil, err
		}
		counts.Agents = n
	}

	return counts, nil
}

func (l *Linker) linkComponentDir(pluginPath, component string, force bool) (int, []string, error) {
	targetDir := filepath.Join(l.agentsDir, component)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return 0, nil, fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	srcDir := filepath.Join(pluginPath, component)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return 0, nil, nil
	}

	files, err := os.ReadDir(srcDir)
	if err != nil {
		// 之前直接 return (0, nil, nil)，把权限错误/IO 错误当成"插件没有这个组件"，
		// 用户看到 "✓ Successfully installed" 但其实 0 个 skill 被链接。
		return 0, nil, fmt.Errorf("failed to read plugin %s dir %s: %w", component, srcDir, err)
	}

	var conflicts []string
	var replaced []string
	count := 0

	for _, file := range files {
		srcPath, err := pathutil.ResolvePathWithinBase(pluginPath, filepath.Join(component, file.Name()))
		if err != nil {
			return 0, conflicts, fmt.Errorf("component source path escapes plugin root: %w", err)
		}
		linkPath := filepath.Join(targetDir, file.Name())

		if _, err := os.Lstat(linkPath); err == nil {
			if isSymlink(linkPath) {
				existingTarget, _ := os.Readlink(linkPath)
				absExisting, _ := filepath.Abs(existingTarget)
				absSrc, _ := filepath.Abs(srcPath)
				if absExisting == absSrc {
					count++
					continue
				}
			}
			if force {
				removeFn := os.Remove
				if info, err := os.Lstat(linkPath); err == nil && info.IsDir() {
					removeFn = os.RemoveAll
				}
				if err := removeFn(linkPath); err != nil {
					conflicts = append(conflicts, linkPath)
					continue
				}
				replaced = append(replaced, linkPath)
			} else {
				conflicts = append(conflicts, linkPath)
				continue
			}
		}

		if err := os.Symlink(srcPath, linkPath); err != nil {
			return 0, conflicts, fmt.Errorf("failed to create symlink %s: %w", linkPath, err)
		}
		count++
	}

	if len(replaced) > 0 {
		fmt.Printf("✓ Force-replaced existing %s:\n", component)
		for _, r := range replaced {
			fmt.Printf("  - %s\n", r)
		}
	}

	if len(conflicts) > 0 {
		fmt.Printf("⚠️  Some %s already exist and were skipped:\n", component)
		for _, conflict := range conflicts {
			fmt.Printf("  - %s\n", conflict)
		}
	}

	return count, conflicts, nil
}

func (l *Linker) linkComponentPaths(pluginPath, component string, paths []ComponentPath, force bool) (int, []string, error) {
	targetDir := filepath.Join(l.agentsDir, component)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return 0, nil, fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	var conflicts []string
	var replaced []string
	count := 0

	for _, cp := range paths {
		resolvedSrc := cp.Path
		if filepath.IsAbs(resolvedSrc) {
			return 0, conflicts, fmt.Errorf("component source path must not be absolute: %s", resolvedSrc)
		}
		var err error
		resolvedSrc, err = pathutil.ResolvePathWithinBase(pluginPath, resolvedSrc)
		if err != nil {
			return 0, conflicts, fmt.Errorf("component source path escapes plugin root: %w", err)
		}

		if _, err := os.Stat(resolvedSrc); os.IsNotExist(err) {
			continue
		}

		linkName := cp.Name
		if err := validateLinkName(linkName); err != nil {
			return 0, conflicts, fmt.Errorf("invalid link name %q: %w", linkName, err)
		}
		linkPath := filepath.Join(targetDir, linkName)

		if _, err := os.Lstat(linkPath); err == nil {
			if isSymlink(linkPath) {
				existingTarget, _ := os.Readlink(linkPath)
				absExisting, _ := filepath.Abs(existingTarget)
				absSrc, _ := filepath.Abs(resolvedSrc)
				if absExisting == absSrc {
					count++
					continue
				}
			}
			if force {
				removeFn := os.Remove
				if info, err := os.Lstat(linkPath); err == nil && info.IsDir() {
					removeFn = os.RemoveAll
				}
				if err := removeFn(linkPath); err != nil {
					conflicts = append(conflicts, linkPath)
					continue
				}
				replaced = append(replaced, linkPath)
			} else {
				conflicts = append(conflicts, linkPath)
				continue
			}
		}

		if err := os.Symlink(resolvedSrc, linkPath); err != nil {
			return 0, conflicts, fmt.Errorf("failed to create symlink %s: %w", linkPath, err)
		}
		count++
	}

	if len(replaced) > 0 {
		fmt.Printf("✓ Force-replaced existing %s:\n", component)
		for _, r := range replaced {
			fmt.Printf("  - %s\n", r)
		}
	}

	if len(conflicts) > 0 {
		fmt.Printf("⚠️  Some %s already exist and were skipped:\n", component)
		for _, conflict := range conflicts {
			fmt.Printf("  - %s\n", conflict)
		}
	}

	return count, conflicts, nil
}

func (l *Linker) extractComponentPaths(pluginPath string, manifest map[string]interface{}, component string) ([]ComponentPath, bool) {
	if manifest == nil {
		return nil, false
	}

	raw, ok := manifest[component]
	if !ok || raw == nil {
		return nil, false
	}

	return parseComponentField(raw, component), true
}

func (l *Linker) extractCommandPaths(pluginPath string, manifest map[string]interface{}) ([]ComponentPath, bool) {
	if manifest == nil {
		return nil, false
	}

	raw, ok := manifest["commands"]
	if !ok || raw == nil {
		return nil, false
	}

	return parseCommandField(raw), true
}

func parseComponentField(raw interface{}, component string) []ComponentPath {
	switch v := raw.(type) {
	case string:
		name := filepath.Base(v)
		return []ComponentPath{{Name: name, Path: v}}
	case []interface{}:
		var paths []ComponentPath
		for _, item := range v {
			switch s := item.(type) {
			case string:
				name := filepath.Base(s)
				paths = append(paths, ComponentPath{Name: name, Path: s})
			}
		}
		return paths
	default:
		return nil
	}
}

func parseCommandField(raw interface{}) []ComponentPath {
	switch v := raw.(type) {
	case string:
		name := strings.TrimSuffix(filepath.Base(v), filepath.Ext(v))
		return []ComponentPath{{Name: name, Path: v}}
	case []interface{}:
		var paths []ComponentPath
		for _, item := range v {
			switch s := item.(type) {
			case string:
				name := strings.TrimSuffix(filepath.Base(s), filepath.Ext(s))
				paths = append(paths, ComponentPath{Name: name, Path: s})
			}
		}
		return paths
	case map[string]interface{}:
		var paths []ComponentPath
		for name, val := range v {
			switch entry := val.(type) {
			case string:
				paths = append(paths, ComponentPath{Name: name, Path: entry})
			case map[string]interface{}:
				if source, ok := entry["source"].(string); ok {
					paths = append(paths, ComponentPath{Name: name, Path: source})
				} else if _, hasContent := entry["content"]; hasContent {
					fmt.Printf("⚠️  Warning: command %q has inline content, skipping\n", name)
				}
			}
		}
		return paths
	default:
		return nil
	}
}

// RemoveSymlinks removes symlinks under ~/.agents/{skills,commands,agents} that
// belong to the given plugin. Returns (removedCount, orphanWarnings, error).
//
// pluginPath is the canonical plugin cache dir used to decide whether a symlink
// target is "inside" the plugin. nameClashPath, when non-empty, overrides where
// to look up the plugin's own component entries for the orphan name-clash test
// (used by Update Stage 2: oldCachePath has been renamed to *.update-backup by
// Stage 1, so we look up names in the backup while still removing symlinks whose
// target was the original cache path).
//
// orphanWarnings holds symlinks whose name matches a plugin entry but whose
// target resolves OUTSIDE pluginPath (typical of dev-time 调试态软链留下的孤儿).
// 这些链接严格说不归本 plugin 管，但名字撞车会让用户困惑，所以单独上报：
//   - force=false：保留并放进 orphanWarnings，让调用方决定如何提示用户；
//   - force=true ：直接删除（按名字匹配对齐创建侧 linkComponentDir 的语义）。
func (l *Linker) RemoveSymlinks(pluginPath string, force bool) (int, []string, error) {
	return l.RemoveSymlinksWithNameClashPath(pluginPath, "", force)
}

// RemoveSymlinksWithNameClashPath is the advanced form; see RemoveSymlinks doc.
// When nameClashPath == "", it falls back to pluginPath (the common case for
// disable/remove where the cache dir is still in place).
func (l *Linker) RemoveSymlinksWithNameClashPath(pluginPath, nameClashPath string, force bool) (int, []string, error) {
	total := 0
	var allWarnings []string
	var errs []error

	components := []string{"skills", "commands", "agents"}
	for _, component := range components {
		n, warnings, err := l.unlinkComponentDir(pluginPath, nameClashPath, component, force)
		// 即使该 component 返回 err，已收集到的 warnings 也要上报——
		// 用户在 err 提示下最需要知道哪些 orphan 可以用 -f 清理。
		allWarnings = append(allWarnings, warnings...)
		if err != nil {
			// 累积所有 component 的错误，返回聚合后的 error。
			// 之前的实现把错误 Printf 出去然后吞掉，调用方永远看不到失败，
			// 导致 plugin remove 后 ~/.agents/skills/<x> 仍可能是野指针。
			errs = append(errs, fmt.Errorf("%s: %w", component, err))
			continue
		}
		total += n
	}

	if len(errs) > 0 {
		return total, allWarnings, fmt.Errorf("failed to remove some symlinks: %v", errs)
	}
	return total, allWarnings, nil
}

func (l *Linker) unlinkComponentDir(pluginPath, nameClashPath, component string, force bool) (int, []string, error) {
	componentDir := filepath.Join(l.agentsDir, component)
	if _, err := os.Stat(componentDir); os.IsNotExist(err) {
		return 0, nil, nil
	}

	files, err := os.ReadDir(componentDir)
	if err != nil {
		// 之前直接 return (0, nil) 把 ReadDir 错误当成"空目录"，
		// 这样 plugin remove 看起来成功了，但所有 symlink 都还在。
		return 0, nil, fmt.Errorf("failed to read %s: %w", componentDir, err)
	}

	count := 0
	absPluginPath, _ := filepath.Abs(pluginPath)
	if evaluatedPluginPath, err := filepath.EvalSymlinks(absPluginPath); err == nil {
		absPluginPath = evaluatedPluginPath
	}
	// orphan 名字撞车检查用的 plugin 目录：默认是 pluginPath，
	// 但 Update Stage 2 在调用前已经把 pluginPath rename 成 .update-backup，
	// 此时用 backupPath 才能查到旧 plugin 的 component entry 列表。
	absNameClashPath := absPluginPath
	if nameClashPath != "" {
		absNameClashPath, _ = filepath.Abs(nameClashPath)
		if ev, err := filepath.EvalSymlinks(absNameClashPath); err == nil {
			absNameClashPath = ev
		}
	}

	var skipped []string
	var warnings []string
	for _, file := range files {
		linkPath := filepath.Join(componentDir, file.Name())

		if !isSymlink(linkPath) {
			continue
		}

		linkTarget, err := os.Readlink(linkPath)
		if err != nil {
			// 单个 Readlink 失败只跳过这一项，但记录下来上报。
			skipped = append(skipped, fmt.Sprintf("readlink %s: %v", linkPath, err))
			continue
		}

		absLinkTarget, _ := filepath.Abs(linkTarget)
		if evaluatedLinkTarget, err := filepath.EvalSymlinks(absLinkTarget); err == nil {
			absLinkTarget = evaluatedLinkTarget
		}
		rel, err := filepath.Rel(absPluginPath, absLinkTarget)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("rel %s: %v", linkPath, err))
			continue
		}

		if strings.HasPrefix(rel, "..") {
			// target 落在 pluginPath 之外。这有两种情况，必须区分：
			//   (a) 这个 symlink 本就不归本 plugin 管（target 指向其它 plugin 的 cache）——
			//       正常状态，沉默跳过。
			//   (b) symlink 名字撞了本 plugin 的某项（pluginPath/<component>/<name>
			//       存在），但 target 指向别处——这才是 orphan：通常是早期调试态
			//       直接 ln -s 源码仓库残留下的、本该属于本 plugin 的旧链接。
			// 创建侧 linkComponentDir 只按 file.Name() 建链接，删除侧按 target
			// 判断归属本就不对称；只有 (b) 这种"名字撞车"才需要上报 / force 删除。
			srcEntry := filepath.Join(absNameClashPath, component, file.Name())
			if _, statErr := os.Stat(srcEntry); statErr != nil {
				if os.IsNotExist(statErr) {
					// (a) 不归本 plugin 管 —— 沉默跳过。
					continue
				}
				// 其它错误（EACCES / ELOOP / I/O）不能当成"不归本 plugin 管"：
				// 否则 -f 也会沉默失效。上报到 skipped，让调用方知道。
				skipped = append(skipped, fmt.Sprintf("stat plugin entry %s: %v", srcEntry, statErr))
				continue
			}
			// (b) orphan：名字撞车但 target 在外。
			if force {
				if err := os.Remove(linkPath); err != nil {
					skipped = append(skipped, fmt.Sprintf("remove orphan %s: %v", linkPath, err))
				} else {
					count++
				}
			} else {
				warnings = append(warnings, fmt.Sprintf("%s -> %s (outside plugin cache)", linkPath, absLinkTarget))
			}
			continue
		}

		if err := os.Remove(linkPath); err != nil {
			// Remove 失败也要上报，否则会留下野指针。
			skipped = append(skipped, fmt.Sprintf("remove %s: %v", linkPath, err))
		} else {
			count++
		}
	}

	if len(skipped) > 0 {
		return count, warnings, fmt.Errorf("some symlinks could not be removed: %v", skipped)
	}
	return count, warnings, nil
}

func ReadManifest(manifestPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

func validateLinkName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("link name must not be empty, '.', or '..'")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("link name must not contain path separators")
	}
	return nil
}
