package config

import (
	"fmt"
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
	Disabled     bool      `json:"disabled"`
	DisabledAt   time.Time `json:"disabledAt,omitempty,omitzero"`
}

type Paths struct {
	BaseDir        string
	MarketsDir     string
	CacheDir       string
	KnownMarkets   string
	InstalledFile  string
	OpenCodeConfig string
	AgentsDir      string
	PluginDataDir  string
}

// KnownMarkets stores marketplace information
// Structure: map[marketName]marketplaceInfo
// Each marketplace info contains: source, repo, url, path, installLocation, lastUpdated
type KnownMarkets map[string]map[string]interface{}

// DefaultPaths 返回基于用户 HOME 目录的默认路径。
// 如果 HOME 无法解析（systemd unit / 容器未设置 $HOME），返回错误而不是
// 静默退化到 /.opencode-plugin-cli，避免后续 MkdirAll 报出误导性的 permission denied。
func DefaultPaths() (*Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve user home directory (is $HOME set?): %w", err)
	}
	if homeDir == "" {
		return nil, fmt.Errorf("user home directory is empty (is $HOME set?)")
	}
	baseDir := filepath.Join(homeDir, ".opencode-plugin-cli")

	return &Paths{
		BaseDir:        baseDir,
		MarketsDir:     filepath.Join(baseDir, "markets"),
		CacheDir:       filepath.Join(baseDir, "cache"),
		KnownMarkets:   filepath.Join(baseDir, "known_marketplaces.json"),
		InstalledFile:  filepath.Join(baseDir, "installed_plugins.json"),
		OpenCodeConfig: filepath.Join(homeDir, ".config", "opencode"),
		AgentsDir:      filepath.Join(homeDir, ".agents"),
		PluginDataDir:  filepath.Join(baseDir, "data"),
	}, nil
}
