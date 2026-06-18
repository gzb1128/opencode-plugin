package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic 原子地写入文件：先写到同目录下的临时文件，
// 再 rename 到目标路径。避免在 SIGKILL / 系统崩溃 / 磁盘满时
// 留下被截断的 JSON 状态文件（installed_plugins.json、opencode.json 等）。
//
// rename 在同一文件系统下是原子的，所以临时文件必须和目标在同一目录。
// perm 为最终文件的权限模式。
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()

	// 任何失败路径都必须清理临时文件。
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to chmod temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename %s -> %s: %w", tmpPath, path, err)
	}

	success = true
	return nil
}
