package config

import (
	"path/filepath"
)

// Environment represents the runtime environment.
// 生产代码请使用 config.DefaultPaths()；Environment 仅保留给测试构造临时路径。
type Environment struct {
	BaseDir        string
	OpenCodeConfig string
	AgentsDir      string
}

// TestEnvironment creates a test environment in a temporary directory.
func TestEnvironment(tempDir string) *Environment {
	return &Environment{
		BaseDir:        filepath.Join(tempDir, ".opencode-plugin-cli"),
		OpenCodeConfig: filepath.Join(tempDir, ".config", "opencode"),
		AgentsDir:      filepath.Join(tempDir, ".agents"),
	}
}

// Paths returns the paths for the environment.
func (e *Environment) Paths() *Paths {
	return &Paths{
		BaseDir:        e.BaseDir,
		MarketsDir:     filepath.Join(e.BaseDir, "markets"),
		CacheDir:       filepath.Join(e.BaseDir, "cache"),
		KnownMarkets:   filepath.Join(e.BaseDir, "known_marketplaces.json"),
		InstalledFile:  filepath.Join(e.BaseDir, "installed_plugins.json"),
		OpenCodeConfig: e.OpenCodeConfig,
		AgentsDir:      e.AgentsDir,
		PluginDataDir:  filepath.Join(e.BaseDir, "data"),
	}
}
