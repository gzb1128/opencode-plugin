# Architecture

## Overview

```
opencode-plugin is a standalone CLI tool for managing OpenCode's plugin ecosystem.
It replicates Claude Code's plugin marketplace capabilities, allowing users to
add marketplaces, install plugins, and integrate them with OpenCode via symlinks.
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
internal/config/          (no internal deps)
internal/marketplace/     (no internal deps)
internal/plugin/          → internal/config, internal/marketplace, internal/opencode
internal/opencode/        (no internal deps)
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
    │       └── ~/.agents/skills/* → cache/*/skills/*
   │
   └── 2f. config.Manager::AddInstallRecord()
           → Append to ~/.opencode-plugin-cli/installed_plugins.json
```

### Plugin Removal

```
User runs: opencode-plugin plugin remove <name>

1. Resolve plugin name (handle ambiguous installs)
2. config.Manager::GetInstallRecord(key)
3. opencode.Linker::RemoveSymlinks(installPath)
4. os.RemoveAll(cachePath)
5. config.Manager::RemoveInstallRecord(key)
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
                ├── skills/
                └── ...

~/.agents/
└── skills/
    └── <name> → ~/.opencode-plugin-cli/cache/.../skills/<name>
```

## Layer Responsibilities

| Layer | Package | Role |
|-------|---------|------|
| CLI | `cmd/` | Cobra commands, argument parsing, user I/O |
| Config | `internal/config/` | Path resolution, JSON persistence, environment abstraction |
| Marketplace | `internal/marketplace/` | Source parsing, git operations, plugin discovery |
| Plugin | `internal/plugin/` | Install/remove orchestration, version resolution, file caching |
| OpenCode | `internal/opencode/` | Symlink creation/removal for OpenCode integration |

## Key Design Decisions

1. **Symlinks over copies**: Plugin skills are symlinked into the agents dir so they are discovered without any code changes.

2. **Copy-all caching**: All plugin files are copied to cache, not just skills.

3. **Environment abstraction**: Config module supports production and test environments via `Environment` struct, ensuring tests don't affect real config.

4. **Marketplace name tracking**: Every install records the marketplace name (`plugin@market`) to support remove and update operations.

5. **Idempotent operations**: `git pull` handles already-up-to-date repos, install skips existing cache, symlinks skip conflicts.
