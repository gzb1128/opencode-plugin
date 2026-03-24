package config

import (
	"os"
	"path/filepath"
)

// Environment represents the runtime environment
type Environment struct {
	BaseDir        string
	OpenCodeConfig string
}

// DefaultEnvironment returns the default environment using user's home directory
func DefaultEnvironment() *Environment {
	homeDir, _ := os.UserHomeDir()
	baseDir := filepath.Join(homeDir, ".opencode-plugin-cli")

	return &Environment{
		BaseDir:        baseDir,
		OpenCodeConfig: filepath.Join(homeDir, ".config", "opencode"),
	}
}

// TestEnvironment creates a test environment in a temporary directory
func TestEnvironment(tempDir string) *Environment {
	return &Environment{
		BaseDir:        filepath.Join(tempDir, ".opencode-plugin-cli"),
		OpenCodeConfig: filepath.Join(tempDir, ".config", "opencode"),
	}
}

// Paths returns the paths for the environment
func (e *Environment) Paths() *Paths {
	return &Paths{
		BaseDir:        e.BaseDir,
		MarketsDir:     filepath.Join(e.BaseDir, "markets"),
		CacheDir:       filepath.Join(e.BaseDir, "cache"),
		KnownMarkets:   filepath.Join(e.BaseDir, "known_marketplaces.json"),
		InstalledFile:  filepath.Join(e.BaseDir, "installed_plugins.json"),
		OpenCodeConfig: e.OpenCodeConfig,
	}
}
