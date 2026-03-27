# Architecture

## Overview

```
opencode-plugin is a standalone CLI tool for managing OpenCode's plugin ecosystem.
It replicates Claude Code's plugin marketplace capabilities, allowing users to
add marketplaces, install plugins, and integrate them with OpenCode via symlinks
and MCP server configuration.
```

## Module Dependency Graph

```
cmd/root.go
├── cmd/market/
│   ├── add.go        → internal/config, internal/marketplace
│   ├── market.go     → internal/config, internal/marketplace
│   └── update.go     → internal/config, internal/marketplace
├── cmd/plugin/
│   ├── plugin.go
│   ├── plugin_install.go → internal/config, internal/plugin
│   ├── plugin_info.go    → internal/config, internal/marketplace, internal/plugin
│   ├── plugin_search.go  → internal/config, internal/marketplace
│   └── plugin_update.go  → internal/config, internal/plugin
└── cmd/mcp/
    └── mcp.go         → internal/config, internal/mcp

internal/config/          (no internal deps)
internal/marketplace/     (no internal deps)
internal/plugin/          → internal/config, internal/marketplace, internal/opencode, internal/mcp
internal/opencode/        (no internal deps)
internal/mcp/             (no internal deps)
```

## Data Flow

### Plugin Installation

```
User runs: opencode-plugin plugin install <name>

1. cmd/plugin/plugin_install.go
   └── Parse "name[@market]" → pluginName, marketName

2. internal/plugin/installer.go::Install()
   ├── 2a. config.Manager::LoadKnownMarkets()
   │       → Read ~/.opencode-plugin-cli/known_marketplaces.json
   │
   ├── 2b. marketplace.Manager::FindPlugin()
   │       ├── Iterate all marketplaces
   │       ├── Parse marketplace.json for each
   │       └── Return matching Plugin + MarketSource + marketName
   │
   ├── 2c. plugin.VersionResolver::Resolve()
   │       ├── Read .claude-plugin/plugin.json version
   │       └── Fallback: git SHA → "latest"
   │
   ├── 2d. Copy plugin files to cache (skip .git)
   │       └── ~/.opencode-plugin-cli/cache/<market>/<name>/<version>/
   │
   ├── 2e. opencode.Linker::CreateSymlinks()
   │       ├── ~/.config/opencode/skills/* → cache/*/skills/*
   │       ├── ~/.config/opencode/commands/* → cache/*/commands/*
   │       └── ~/.config/opencode/agents/* → cache/*/agents/*
   │
   ├── 2f. mcp.Manager::InstallMCPConfig()
   │       ├── Read .mcp.json + plugin.json mcpServers
   │       ├── Substitute ${CLAUDE_PLUGIN_ROOT}, ${PLUGIN_NAME}, ${PLUGIN_VERSION}
   │       └── Merge into ~/.config/opencode/.mcp.json with "plugin.server" prefix
   │
   └── 2g. config.Manager::AddInstallRecord()
           → Append to ~/.opencode-plugin-cli/installed_plugins.json
```

### Plugin Removal

```
User runs: opencode-plugin plugin remove <name>

1. Resolve plugin name (handle ambiguous installs)
2. config.Manager::GetInstallRecord(key)
3. opencode.Linker::RemoveSymlinks(installPath)
4. mcp.Manager::UninstallMCPConfig(pluginName)
5. os.RemoveAll(cachePath)
6. config.Manager::RemoveInstallRecord(key)
```

## Directory Layout (Runtime)

```
~/.opencode-plugin-cli/
├── known_marketplaces.json
├── installed_plugins.json
├── markets/
│   └── <market-name>/
│       ├── .claude-plugin/
│       │   └── marketplace.json
│       ├── plugins/              (bundled plugins)
│       │   └── <plugin-name>/
│       └── external_plugins/     (external plugin references)
│           └── <plugin-name>/
└── cache/
    └── <market-name>/
        └── <plugin-name>/
            └── <version>/
                ├── .claude-plugin/
                ├── .mcp.json
                ├── skills/
                ├── commands/
                ├── agents/
                ├── server.ts       (MCP server source)
                ├── package.json    (MCP dependencies)
                └── ...

~/.config/opencode/
├── .mcp.json              (global MCP server registry)
├── skills/
│   └── <name> → ~/.opencode-plugin-cli/cache/.../skills/<name>
├── commands/
│   └── <name> → ~/.opencode-plugin-cli/cache/.../commands/<name>
└── agents/
    └── <name> → ~/.opencode-plugin-cli/cache/.../agents/<name>
```

## Layer Responsibilities

| Layer | Package | Role |
|-------|---------|------|
| CLI | `cmd/` | Cobra commands, argument parsing, user I/O |
| Config | `internal/config/` | Path resolution, JSON persistence, environment abstraction |
| Marketplace | `internal/marketplace/` | Source parsing, git operations, plugin discovery |
| Plugin | `internal/plugin/` | Install/remove orchestration, version resolution, file caching |
| MCP | `internal/mcp/` | MCP server config read/write, variable substitution |
| OpenCode | `internal/opencode/` | Symlink creation/removal for OpenCode integration |

## Key Design Decisions

1. **Symlinks over copies**: Plugins are symlinked into OpenCode's config dir so OpenCode discovers them without any code changes.

2. **Plugin name prefix for MCP**: MCP servers are registered as `pluginName.serverName` to avoid conflicts between plugins.

3. **Copy-all caching**: All plugin files (including MCP source code and dependencies) are copied to cache, not just skills/commands/agents.

4. **Environment abstraction**: Config module supports production and test environments via `Environment` struct, ensuring tests don't affect real config.

5. **Marketplace name tracking**: Every install records the marketplace name (`plugin@market`) to support remove and update operations.

6. **Idempotent operations**: `git pull` handles already-up-to-date repos, install skips existing cache, symlinks skip conflicts.
