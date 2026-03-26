# OpenCode Plugin CLI

A standalone CLI tool for managing OpenCode's plugin ecosystem, replicating Claude Code's plugin marketplace capabilities.

## Features

- ✅ Multi-source marketplace support (GitHub, Git, local paths)
- ✅ Smart URL format recognition (owner/repo, SSH, HTTPS, local)
- ✅ Plugin installation and management
- ✅ Symlink integration (no OpenCode source modification needed)
- ✅ Version management (plugin.json version + git SHA fallback)
- ✅ Automatic component discovery (skills, commands, agents)
- ✅ Marketplace update and remove
- ✅ Plugin update and version management
- ✅ Plugin info command
- ✅ Plugin search across all marketplaces
- ✅ **Full MCP (Model Context Protocol) support**
  - MCP server auto-installation and configuration
  - Support for stdio, HTTP, SSE, WebSocket MCP servers
  - Automatic variable substitution (${CLAUDE_PLUGIN_ROOT}, ${PLUGIN_NAME}, ${PLUGIN_VERSION})
  - Plugin prefix to avoid MCP server name conflicts
  - Auto-cleanup on plugin removal

## How It Works

When you install a plugin, the CLI:

1. **Downloads the plugin** to a cache directory
2. **Creates symbolic links** to OpenCode's config directory
3. **OpenCode automatically discovers** the plugin components (skills, commands, agents)

```
~/.config/opencode/
├── skills/
│   └── skill-name.md -> ~/.opencode-plugin-cli/cache/.../skills/skill-name.md
├── commands/
│   └── command-name.md -> ~/.opencode-plugin-cli/cache/.../commands/command-name.md
└── agents/
    └── agent-name.md -> ~/.opencode-plugin-cli/cache/.../agents/agent-name.md
```

See [docs/USAGE.md](docs/USAGE.md) for detailed usage instructions.

## Installation

### Pre-built Binary

```bash
make build
make install
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/gzb1128/opencode-plugin.git
cd opencode-plugin

# Build
go build -o bin/opencode-plugin .

# Install to PATH
go install .

# Or use make
make build
make install
```

## Quick Start

### Add Claude Official Marketplace

The fastest way to get started is to add the official Claude plugins marketplace:

```bash
# Add the official Claude plugins marketplace
opencode-plugin market add anthropics/claude-plugins-official

# Search for available plugins (118 plugins available)
opencode-plugin plugin search

# Install a plugin
opencode-plugin plugin install code-simplifier

# List installed plugins
opencode-plugin plugin list
```

### Add Other Marketplaces

Supports multiple formats:

```bash
# GitHub shorthand
opencode-plugin market add owner/repo

# GitHub SSH
opencode-plugin market add git@github.com:owner/repo.git

# Git HTTPS
opencode-plugin market add https://github.com/owner/repo.git

# Local path
opencode-plugin market add ./path/to/marketplace
```

### Manage Marketplaces

```bash
# List all marketplaces
opencode-plugin market list

# Update all marketplaces
opencode-plugin market update

# Update specific marketplace
opencode-plugin market update my-market

# Remove a marketplace
opencode-plugin market remove my-market
```

### Manage Plugins

```bash
# Install plugin
opencode-plugin plugin install my-plugin

# Install from specific marketplace
opencode-plugin plugin install my-plugin@test-market

# Install specific version
opencode-plugin plugin install my-plugin --version 1.0.0

# Show plugin info
opencode-plugin plugin info my-plugin

# List installed plugins
opencode-plugin plugin list

# Search for plugins
opencode-plugin plugin search git                    # Search all marketplaces
opencode-plugin plugin search --market my-market    # List all plugins in a market

# Update plugins
opencode-plugin plugin update                       # Update all
opencode-plugin plugin update my-plugin             # Update specific

# Remove plugins
opencode-plugin plugin remove my-plugin
```

### Manage MCP Servers

MCP servers are automatically installed and configured when you install a plugin that includes them.

```bash
# List all installed MCP servers
opencode-plugin mcp list

# Show MCP server details
opencode-plugin mcp show plugin-name.server-name

# MCP servers are automatically removed when plugin is removed
opencode-plugin plugin remove my-plugin
```

See [docs/MCP.md](docs/MCP.md) for detailed MCP documentation.

## Project Structure

See [docs/develop.md](docs/develop.md) for detailed project structure.

## How It Works

### Directory Structure

```
~/.opencode-plugin-cli/
├── known_marketplaces.json    # Added marketplaces
├── installed_plugins.json     # Installed plugins
├── markets/                   # Cloned marketplace repositories
│   └── official/
│       └── plugins/
└── cache/                     # Installed plugin cache
    └── official/
        └── plugin-name/
            └── 1.0.0/
                ├── .claude-plugin/
                │   └── plugin.json
                ├── .mcp.json           # MCP server configuration (if any)
                ├── skills/
                ├── commands/
                ├── agents/
                ├── server.ts           # MCP server source code (if any)
                ├── package.json        # MCP server dependencies (if any)
                └── ...                 # Other MCP server files

~/.config/opencode/            # OpenCode configuration
├── .mcp.json                  # MCP server registry
├── skills/
│   └── skill-name -> ~/.opencode-plugin-cli/cache/.../skills/skill-name.md
├── commands/
│   └── command-name -> ~/.opencode-plugin-cli/cache/.../commands/command-name.md
└── agents/
    └── agent-name -> ~/.opencode-plugin-cli/cache/.../agents/agent-name.md
```

### Symlink Strategy

The CLI creates symlinks from OpenCode's configuration directory to the plugin cache:

- **No source modification**: Works with existing OpenCode installation
- **Automatic discovery**: OpenCode automatically detects skills, commands, and agents
- **Easy cleanup**: Removing a plugin removes all symlinks
- **Version isolation**: Each version has its own cache directory

### Version Resolution

1. **User-specified version** (--version flag)
2. **plugin.json version field**
3. **Git commit SHA** (first 12 characters)
4. **Fallback to "latest"**

## License

MIT License
