package opencode

import (
	"fmt"
	"os"
	"path/filepath"
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
	Skills int
}

func (l *Linker) CreateSymlinks(pluginPath string) (*ComponentCounts, error) {
	counts := &ComponentCounts{}

	skillsDir := filepath.Join(l.agentsDir, "skills")

	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", skillsDir, err)
	}

	srcDir := filepath.Join(pluginPath, "skills")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return counts, nil
	}

	files, err := os.ReadDir(srcDir)
	if err != nil {
		return counts, nil
	}

	var conflicts []string

	for _, file := range files {
		srcPath := filepath.Join(srcDir, file.Name())
		targetPath := filepath.Join(skillsDir, file.Name())

		if _, err := os.Lstat(targetPath); err == nil {
			if isSymlink(targetPath) {
				existingTarget, _ := os.Readlink(targetPath)
				if filepath.Dir(existingTarget) == srcDir {
					counts.Skills++
					continue
				}
			}
			conflicts = append(conflicts, targetPath)
			continue
		}

		if err := os.Symlink(srcPath, targetPath); err != nil {
			return nil, fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
		}
		counts.Skills++
	}

	if len(conflicts) > 0 {
		fmt.Println("⚠️  Some files already exist and were skipped:")
		for _, conflict := range conflicts {
			fmt.Printf("  - %s\n", conflict)
		}
	}

	return counts, nil
}

func (l *Linker) RemoveSymlinks(pluginPath string) (int, error) {
	count := 0

	skillsDir := filepath.Join(l.agentsDir, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return 0, nil
	}

	files, err := os.ReadDir(skillsDir)
	if err != nil {
		return 0, nil
	}

	for _, file := range files {
		targetPath := filepath.Join(skillsDir, file.Name())

		if !isSymlink(targetPath) {
			continue
		}

		linkTarget, err := os.Readlink(targetPath)
		if err != nil {
			continue
		}

		absPluginPath, _ := filepath.Abs(pluginPath)
		absLinkTarget, _ := filepath.Abs(linkTarget)

		if filepath.Dir(absLinkTarget) == filepath.Join(absPluginPath, "skills") {
			if err := os.Remove(targetPath); err != nil {
				fmt.Printf("⚠️  Failed to remove symlink: %s (%v)\n", targetPath, err)
			} else {
				count++
			}
		}
	}

	return count, nil
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
