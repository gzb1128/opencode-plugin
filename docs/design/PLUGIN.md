# Plugin Module Design

## Overview

The plugin module orchestrates plugin installation, removal, listing, and version
resolution. It coordinates between config, marketplace, opencode, and mcp modules.

## File Structure

```
internal/plugin/
├── installer.go    # Install/remove/list orchestration
└── version.go      # Version resolution and plugin path resolution
```

## Installer (installer.go)

```go
type Installer struct {
    configMgr  *config.Manager
    resolver   *VersionResolver
    linker     *opencode.Linker
    marketMgr  *marketplace.Manager
    mcpManager *mcp.Manager
}

type InstallOptions struct {
    MarketName string
    Version    string
    Scope      string  // "user" or "project"
}
```

### Install Flow

```
Install(pluginName, opts) error
│
├── 1. LoadKnownMarkets() → get all marketplace sources
├── 2. FindPlugin(markets, pluginName, opts.MarketName)
│       → returns Plugin, MarketSource, actualMarketName
│       → updates opts.MarketName with resolved name
│
├── 3. GetPluginSourcePath(plugin, marketPath)
│       → determine filesystem path to plugin source
│
├── 4. Resolve(pluginPath, opts.Version)
│       → determine version string
│
├── 5. Copy plugin files to cache
│       copyPluginToCache(src, dst)
│       → copies ALL files (skip .git)
│       → includes: .claude-plugin/, .mcp.json, skills/, commands/,
│         agents/, server.ts, package.json, etc.
│
├── 6. CreateSymlinks(cachePath)
│       → skills/*, commands/*, agents/* → ~/.config/opencode/
│       → returns ComponentCounts{Skills, Commands, Agents}
│
├── 7. installMCP(cachePath, pluginName)
│       → GetMCPServers() → count
│       → InstallMCPConfig() → write to ~/.config/opencode/.mcp.json
│
└── 8. AddInstallRecord(key, record)
        → key = "pluginName@marketName"
        → record = {Scope, InstallPath, Version, InstalledAt, ...}
```

### Remove Flow

```
Remove(pluginName, marketName) error
│
├── 1. GetInstallRecord(key) → get cache path
├── 2. RemoveSymlinks(installPath) → unlink from ~/.config/opencode/
├── 3. UninstallMCPConfig(pluginName) → remove from .mcp.json
├── 4. os.RemoveAll(cachePath)
└── 5. RemoveInstallRecord(key)
```

### List

Returns `map[string][]InstallRecord` — a map from `pluginName@marketName` to
a slice of install records.

## File Copying

`copyPluginToCache` copies the entire plugin directory recursively, skipping
only `.git/`. This is essential for MCP server plugins that include source
code, dependencies, and configuration files.

```
Source plugin directory:
├── .claude-plugin/
│   └── plugin.json
├── .mcp.json
├── skills/
├── commands/
├── agents/
├── server.ts            ← MCP server source
├── package.json         ← MCP dependencies
├── bun.lock             ← Lock file
└── README.md

Cache directory: (identical structure, all files copied)
```

## Version Resolution (version.go)

### VersionResolver

```go
type VersionResolver struct {
    gitClient *GitClient
}
```

### Resolve Strategy

Priority order:
1. **User-specified version** (`--version` flag)
2. **plugin.json version** field (`.claude-plugin/plugin.json`)
3. **Git commit SHA** (first 12 characters of HEAD)
4. **Fallback**: `"latest"`

### GetPluginSourcePath

Determines filesystem path to a plugin based on its source type:

| Source Type | Path Resolution |
|-------------|----------------|
| `local` | `marketPath + "/" + source.Path` |
| `github` | `marketPath + "/plugins/" + source.Repo (last segment)` |
| `git` | Same as github (assumes same directory structure) |
| `url` | Fetched via `FetchSubDir` |

### GetAvailableVersions

Returns available versions from:
- `plugin.json` version field
- Git tags in the plugin directory
- Always includes `"latest"`
