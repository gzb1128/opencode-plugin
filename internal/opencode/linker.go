package opencode

import (
	"fmt"
	"os"
	"path/filepath"
)

type Linker struct {
	opencodeConfig string
}

func NewLinker(opencodeConfig string) *Linker {
	return &Linker{
		opencodeConfig: opencodeConfig,
	}
}

type ComponentCounts struct {
	Skills   int
	Commands int
	Agents   int
}

func (l *Linker) CreateSymlinks(pluginPath string) (*ComponentCounts, error) {
	counts := &ComponentCounts{}

	// Ensure OpenCode config directories exist
	dirs := []string{
		filepath.Join(l.opencodeConfig, "skills"),
		filepath.Join(l.opencodeConfig, "commands"),
		filepath.Join(l.opencodeConfig, "agents"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Define component types
	components := []struct {
		dir    string
		target string
		count  *int
	}{
		{"skills", "skills", &counts.Skills},
		{"commands", "commands", &counts.Commands},
		{"agents", "agents", &counts.Agents},
	}

	var conflicts []string

	// Create symlinks for each component type
	for _, comp := range components {
		srcDir := filepath.Join(pluginPath, comp.dir)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			continue
		}

		files, err := os.ReadDir(srcDir)
		if err != nil {
			continue
		}

		for _, file := range files {
			srcPath := filepath.Join(srcDir, file.Name())
			targetPath := filepath.Join(l.opencodeConfig, comp.target, file.Name())

			// Check if already exists
			if _, err := os.Lstat(targetPath); err == nil {
				// If it's a symlink, check if it points to our plugin
				if isSymlink(targetPath) {
					existingTarget, _ := os.Readlink(targetPath)
					if filepath.Dir(existingTarget) == srcDir {
						// Already points to our plugin, skip
						(*comp.count)++
						continue
					}
				}
				// Record conflict
				conflicts = append(conflicts, targetPath)
				continue
			}

			// Create symlink
			if err := os.Symlink(srcPath, targetPath); err != nil {
				return nil, fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
			}
			(*comp.count)++
		}
	}

	// Report conflicts
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

	// Define component types
	components := []string{"skills", "commands", "agents"}

	for _, comp := range components {
		targetDir := filepath.Join(l.opencodeConfig, comp)
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			continue
		}

		files, err := os.ReadDir(targetDir)
		if err != nil {
			continue
		}

		for _, file := range files {
			targetPath := filepath.Join(targetDir, file.Name())

			// Check if it's a symlink
			if !isSymlink(targetPath) {
				continue
			}

			// Check if it points to our plugin
			linkTarget, err := os.Readlink(targetPath)
			if err != nil {
				continue
			}

			// Check if the link target is inside our plugin path
			absPluginPath, _ := filepath.Abs(pluginPath)
			absLinkTarget, _ := filepath.Abs(linkTarget)

			if filepath.Dir(absLinkTarget) == filepath.Join(absPluginPath, comp) {
				if err := os.Remove(targetPath); err != nil {
					fmt.Printf("⚠️  Failed to remove symlink: %s (%v)\n", targetPath, err)
				} else {
					count++
				}
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

func (l *Linker) DetectOpenCodeConfig() (string, error) {
	if _, err := os.Stat(l.opencodeConfig); err == nil {
		return l.opencodeConfig, nil
	}

	// Try to create it
	if err := os.MkdirAll(l.opencodeConfig, 0755); err != nil {
		return "", fmt.Errorf("failed to create OpenCode config directory: %w", err)
	}

	return l.opencodeConfig, nil
}
