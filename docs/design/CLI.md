# CLI Layer Design

## Overview

The CLI layer uses [Cobra](https://github.com/spf13/cobra) and is organized into three
command groups: `market`, `plugin`, and `mcp`.

## Command Tree

```
opencode-plugin
├── market
│   ├── add <url>
│   ├── list
│   ├── update [name]
│   └── remove <name>
├── plugin
│   ├── install <name>[@<market>] [--version <ver>]
│   ├── list
│   ├── info <name>
│   ├── search [keyword]
│   ├── update [name]
│   └── remove <name>[@<market>]
└── mcp
    ├── list
    └── show <server-name>
```

## File Structure

```
cmd/
├── root.go                  # Root command, version, init()
├── market/
│   ├── market.go            # market Cmd (parent), market list
│   ├── add.go              # market add
│   └── update.go           # market update, market remove
├── plugin/
│   ├── plugin.go           # plugin Cmd (parent)
│   ├── plugin_install.go   # plugin install, plugin remove, plugin list
│   ├── plugin_info.go      # plugin info
│   ├── plugin_search.go    # plugin search
│   └── plugin_update.go    # plugin update
└── mcp/
    └── mcp.go              # mcp Cmd (parent), mcp list, mcp show
```

## cmd/root.go

```go
var version string = "0.1.0"

func Execute()          // Entry point, runs rootCmd
```

Registers three subcommands: `market.Cmd`, `plugin.Cmd`, `mcp.Cmd`.

## cmd/market/

### market add

Parses user input URL through `ParseMarketplaceSource()`, then:
1. `config.NewManager()` → load config
2. `marketplace.NewManager()` → create marketplace manager
3. `gitClient.Clone()` or read local path
4. `ParseMarketplaceIndex()` → validate marketplace.json
5. `configMgr.SaveKnownMarkets()` → persist

Supported URL formats:
- GitHub shorthand: `anthropics/claude-plugins-official`
- SSH: `git@github.com:org/repo.git`
- HTTPS: `https://github.com/org/repo.git`
- Local: `./path/to/marketplace`

### market list

Reads `known_marketplaces.json`, displays table with:
- Name, Type, Repo/URL, Location, Clone Status, Last Updated

### market update [name]

- No argument: update all marketplaces (clone or pull each)
- With argument: update specific marketplace
- Uses `GitClient.CloneOrPull()` for idempotent behavior

### market remove <name>

- Prompts for confirmation
- Removes marketplace directory and config entry
- Does NOT remove installed plugins from that marketplace

## cmd/plugin/

### plugin install <name>[@<market>] [--version <ver>]

```go
// Parse "name[@market]" format
idx := strings.Index(pluginSpec, "@")
```

Flow: `config.NewManager()` → `plugin.NewInstaller()` → `installer.Install()`

Output includes: version, marketplace, cache path, skill/command/agent counts, MCP server count.

### plugin remove <name>[@<market>]

Handles three cases:
1. `plugin remove name@market` — exact match
2. `plugin remove name@` — empty marketplace name (legacy installs)
3. `plugin remove name` — auto-detect; if multiple matches, show error with suggestions

### plugin list

Reads `installed_plugins.json`, displays table with:
- Name@Marketplace, Version, Scope, Install Path, Installed date

### plugin info <name>

Shows detailed plugin information:
- Description, version, category, author
- Homepage, keywords
- Available versions (plugin.json + git tags)
- Installation status

### plugin search [keyword]

- No keyword: list all plugins across all marketplaces
- With keyword: filter by name/description
- With `--market` flag: search specific marketplace only

### plugin update [name]

- No argument: update all installed plugins
- With argument: update specific plugin
- Implementation: remove old version, install latest

## cmd/mcp/

### mcp list

Reads `~/.config/opencode/.mcp.json`, displays table with:
- Name, Type (stdio/http/sse/websocket), Command or URL

### mcp show <server-name>

Shows detailed MCP server info:
- Type, Command, URL, Arguments, Environment Variables
