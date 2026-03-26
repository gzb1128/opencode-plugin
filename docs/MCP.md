# MCP (Model Context Protocol) Support

OpenCode Plugin CLI provides full support for MCP servers, allowing plugins to integrate with external services.

## What is MCP?

MCP (Model Context Protocol) is a protocol that enables Claude Code plugins to:
- Connect to external services (databases, APIs, file systems)
- Provide structured tool access within Claude Code
- Bundle MCP servers with plugins for automatic setup

## MCP Server Types

### 1. stdio (Local Process)
Execute local MCP servers as child processes.

**Configuration (.mcp.json):**
```json
{
  "my-server": {
    "command": "node",
    "args": ["${CLAUDE_PLUGIN_ROOT}/server.js"],
    "env": {
      "LOG_LEVEL": "debug"
    }
  }
}
```

**Use cases:**
- File system access
- Local database connections
- Custom MCP servers
- NPM-packaged MCP servers

### 2. HTTP/HTTPS
Connect to hosted MCP servers via HTTP.

**Configuration:**
```json
{
  "my-api": {
    "type": "http",
    "url": "https://api.example.com/mcp"
  }
}
```

**Use cases:**
- Cloud services
- REST APIs
- Remote databases

### 3. SSE (Server-Sent Events)
Connect to MCP servers using Server-Sent Events.

**Configuration:**
```json
{
  "my-sse-server": {
    "type": "sse",
    "url": "https://sse.example.com/events"
  }
}
```

### 4. WebSocket
Connect to MCP servers via WebSocket.

**Configuration:**
```json
{
  "my-ws-server": {
    "type": "websocket",
    "url": "wss://ws.example.com/mcp"
  }
}
```

## Configuration Methods

### Method 1: Dedicated .mcp.json (Recommended)

Create `.mcp.json` at plugin root. **Two formats are supported:**

#### Claude Code Standard Format (Direct)

```json
{
  "my-database": {
    "command": "${CLAUDE_PLUGIN_ROOT}/servers/db-server",
    "args": ["--config", "${CLAUDE_PLUGIN_ROOT}/config.json"]
  },
  "my-api": {
    "type": "http",
    "url": "https://api.example.com/mcp"
  }
}
```

#### Wrapped Format (Legacy)

```json
{
  "mcpServers": {
    "my-database": {
      "command": "${CLAUDE_PLUGIN_ROOT}/servers/db-server",
      "args": ["--config", "${CLAUDE_PLUGIN_ROOT}/config.json"]
    }
  }
}
```

**Both formats work identically.** The direct format is the Claude Code standard.

### Method 2: Inline in plugin.json

Add `mcpServers` field to plugin.json:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "mcpServers": {
    "my-server": {
      "command": "node",
      "args": ["${CLAUDE_PLUGIN_ROOT}/server.js"]
    }
  }
}
```

**Note:** Both `.mcp.json` and inline `mcpServers` are merged when installing a plugin.

## Variable Substitution

OpenCode Plugin CLI automatically substitutes variables in MCP configurations:

- `${CLAUDE_PLUGIN_ROOT}` - Path to the plugin directory
- `${PLUGIN_NAME}` - Name of the plugin
- `${PLUGIN_VERSION}` - Version of the plugin

**Example:**
```json
{
  "my-server": {
    "command": "${CLAUDE_PLUGIN_ROOT}/bin/server",
    "args": ["--port", "8080"],
    "env": {
      "PLUGIN_ROOT": "${CLAUDE_PLUGIN_ROOT}",
      "PLUGIN_NAME": "${PLUGIN_NAME}",
      "PLUGIN_VERSION": "${PLUGIN_VERSION}"
    }
  }
}
```

Variables are substituted in:
- `command` field
- `args` array
- `url` field
- `env` values

## Managing MCP Servers

### List Installed MCP Servers

```bash
opencode-plugin mcp list
```

Output:
```
Installed MCP Servers:

NAME                        TYPE     COMMAND/URL
github.github               http     https://api.githubcopilot.com/mcp/
plugin-a.database           stdio    /path/to/plugin/servers/db-server

Total: 2 MCP server(s)
```

### Show MCP Server Details

```bash
opencode-plugin mcp show github.github
```

Output:
```
MCP Server: github.github

Type: http
URL: https://api.githubcopilot.com/mcp/
Environment Variables:
  Authorization=Bearer ${GITHUB_PERSONAL_ACCESS_TOKEN}
```

## Installation

MCP servers are installed automatically when you install a plugin:

```bash
# Install plugin with MCP servers
opencode-plugin plugin install github

# Output:
✓ Successfully installed plugin: github
  From marketplace: anthropics/claude-plugins-official
  Cache: ~/.opencode-plugin-cli/cache/anthropics/claude-plugins-official/github/latest
  Skills: 0, Commands: 0, Agents: 0
  MCP Servers: 1  # ← MCP servers installed
```

The MCP configuration is merged into `~/.config/opencode/.mcp.json`.

## Uninstallation

When you remove a plugin, its MCP servers are also removed:

```bash
opencode-plugin plugin remove github

# Output:
✓ Removed MCP servers: github.github
✓ Successfully removed plugin: github (0 symlinks removed)
```

## MCP Server Conflicts

If multiple plugins define MCP servers with the same name, OpenCode Plugin CLI prefixes them with the plugin name:

```
plugin-a.database
plugin-b.database
```

This ensures no conflicts between plugins.

## Real-World Examples

### GitHub Plugin

**.mcp.json:**
```json
{
  "github": {
    "type": "http",
    "url": "https://api.githubcopilot.com/mcp/",
    "headers": {
      "Authorization": "Bearer ${GITHUB_PERSONAL_ACCESS_TOKEN}"
    }
  }
}
```

### Playwright Plugin

**.mcp.json:**
```json
{
  "playwright": {
    "command": "npx",
    "args": ["@playwright/mcp@latest"]
  }
}
```

### Custom Server Plugin

**.mcp.json:**
```json
{
  "custom-tools": {
    "command": "${CLAUDE_PLUGIN_ROOT}/bin/server",
    "args": ["--verbose"],
    "env": {
      "LOG_LEVEL": "info"
    }
  }
}
```

## OAuth and Authentication

For MCP servers that require OAuth or complex authentication:

1. **Environment Variables**: Use `${VAR_NAME}` syntax
2. **Configuration Files**: Bundle config files in plugin
3. **Interactive Setup**: Plugin can prompt for credentials on first use

**Example:**
```json
{
  "github": {
    "command": "${CLAUDE_PLUGIN_ROOT}/servers/github-mcp",
    "env": {
      "GITHUB_TOKEN": "${GITHUB_TOKEN}",
      "GITHUB_ORG": "${GITHUB_ORG:-myorg}"
    }
  }
}
```

## Best Practices

### 1. Use Dedicated .mcp.json
Prefer separate `.mcp.json` for clarity and maintainability.

### 2. Prefix Server Names
Use descriptive names that include the service:
```json
{
  "postgres-main": {...},
  "github-api": {...}
}
```

### 3. Document Required Variables
Document required environment variables in plugin README.

### 4. Test Locally
Test MCP servers locally before packaging with plugin.

### 5. Version MCP Servers
If MCP servers have versions, track them in plugin.json.

## Troubleshooting

### MCP servers not showing up

1. Check `.mcp.json` or `plugin.json` syntax
2. Verify `${CLAUDE_PLUGIN_ROOT}` paths are correct
3. Run `opencode-plugin mcp list` to see installed servers

### MCP server not working

1. Check command path exists
2. Verify environment variables are set
3. Check MCP server logs

### Conflicts between plugins

MCP servers are automatically prefixed with plugin name to avoid conflicts.

## See Also

- [Plugin Development Guide](docs/PLUGIN_DEVELOPMENT.md)
- [MCP Integration Skill](https://github.com/anthropics/claude-plugins-official/tree/main/plugins/plugin-dev/skills/mcp-integration)
- [OpenCode Plugin CLI README](README.md)
