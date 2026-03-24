package config

import (
	"os"
	"path/filepath"
	"time"
)

type InstalledPlugins struct {
	Version int                        `json:"version"`
	Plugins map[string][]InstallRecord `json:"plugins"`
}

type InstallRecord struct {
	Scope        string    `json:"scope"` // "user" or "project"
	ProjectPath  string    `json:"projectPath,omitempty"`
	InstallPath  string    `json:"installPath"`
	Version      string    `json:"version"`
	InstalledAt  time.Time `json:"installedAt"`
	LastUpdated  time.Time `json:"lastUpdated"`
	GitCommitSHA string    `json:"gitCommitSha,omitempty"`
}

type Paths struct {
	BaseDir        string
	MarketsDir     string
	CacheDir       string
	KnownMarkets   string
	InstalledFile  string
	OpenCodeConfig string
}

// KnownMarkets stores marketplace information
// Structure: map[marketName]marketplaceInfo
// Each marketplace info contains: source, repo, url, path, installLocation, lastUpdated
type KnownMarkets map[string]map[string]interface{}

func DefaultPaths() *Paths {
	homeDir, _ := os.UserHomeDir()
	baseDir := filepath.Join(homeDir, ".opencode-plugin-cli")

	return &Paths{
		BaseDir:        baseDir,
		MarketsDir:     filepath.Join(baseDir, "markets"),
		CacheDir:       filepath.Join(baseDir, "cache"),
		KnownMarkets:   filepath.Join(baseDir, "known_marketplaces.json"),
		InstalledFile:  filepath.Join(baseDir, "installed_plugins.json"),
		OpenCodeConfig: filepath.Join(homeDir, ".config", "opencode"),
	}
}
