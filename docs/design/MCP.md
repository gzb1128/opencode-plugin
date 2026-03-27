# MCP Module Design

## Overview

The MCP module manages Model Context Protocol server configurations for plugins.
It reads MCP server definitions from plugins, merges them into OpenCode's global
`.mcp.json` with prefixed names, and handles variable substitution.

## File Structure

```
internal/mcp/
├── manager.go       # MCP config management
└── manager_test.go  # Unit and integration tests
```

## Domain Types (manager.go)

```go
type MCPServer struct {
    Type    string            `json:"type,omitempty"`    // "stdio", "sse", "http", "websocket"
    Command string            `json:"command,omitempty"` // For stdio type
    Args    []string          `json:"args,omitempty"`    // For stdio type
    URL     string            `json:"url,omitempty"`     // For http/sse/websocket type
    Env     map[string]string `json:"env,omitempty"`     // Environment variables
    Headers map[string]string `json:"headers,omitempty"` // HTTP headers (for http/sse)
}

type MCPConfig struct {
    Servers map[string]MCPServer `json:"mcpServers,omitempty"`
}

type PluginJSON struct {
    Name       string               `json:"name"`
    Version    string               `json:"version,omitempty"`
    MCPServers map[string]MCPServer `json:"mcpServers,omitempty"`
}
```

## Manager

```go
type Manager struct {
    configDir string  // ~/.config/opencode
}
```

## Configuration Formats

The `.mcp.json` file supports two formats:

### Direct Format (Claude Code Standard)

```json
{
  "server-name": {
    "type": "http",
    "url": "https://api.example.com/mcp"
  }
}
```

### Wrapped Format (Legacy)

```json
{
  "mcpServers": {
    "server-name": {
      "type": "http",
      "url": "https://api.example.com/mcp"
    }
  }
}
```

Both formats produce identical results. The parser tries wrapped first,
then falls back to direct.

## MCP Server Sources

MCP servers can be defined in two places within a plugin:

1. **`.mcp.json`** (at plugin root) — recommended, separate file
2. **`plugin.json`** (inline `mcpServers` field) — for simple configs

Both are merged when installing. If both define the same server name,
the `.mcp.json` entry takes precedence.

## ReadMCPConfig

```
ReadMCPConfig(pluginPath) (*MCPConfig, error)
├── Read .mcp.json from pluginPath
├── Try wrapped format → if servers found, return
├── Try direct format → if servers found, return
├── Return empty config if file doesn't exist
└── Return error for invalid JSON
```

## ReadPluginJSON

```
ReadPluginJSON(pluginPath) (*PluginJSON, error)
├── Read .claude-plugin/plugin.json from pluginPath
├── Return nil if file doesn't exist (MCP-only plugins)
└── Return error for invalid JSON or read errors
```

## Variable Substitution

Three variables are supported, applied at install time:

| Variable | Replacement |
|----------|-------------|
| `${CLAUDE_PLUGIN_ROOT}` | Absolute path to plugin cache directory |
| `${PLUGIN_NAME}` | Plugin name |
| `${PLUGIN_VERSION}` | Plugin version from plugin.json |

Substitution is applied to: `command`, `args`, `url`, `env` values.

### Example

Before:
```json
{
  "command": "bun",
  "args": ["run", "--cwd", "${CLAUDE_PLUGIN_ROOT}"],
  "env": {
    "PLUGIN_NAME": "${PLUGIN_NAME}",
    "PLUGIN_VERSION": "${PLUGIN_VERSION}"
  }
}
```

After (plugin: discord@0.0.4):
```json
{
  "command": "bun",
  "args": ["run", "--cwd", "/Users/.../cache/discord/0.0.4"],
  "env": {
    "PLUGIN_NAME": "discord",
    "PLUGIN_VERSION": "0.0.4"
  }
}
```

## Installation

```
InstallMCPConfig(pluginPath, pluginName) error
│
├── 1. GetMCPServers(pluginPath) → aggregate from .mcp.json + plugin.json
├── 2. ReadPluginJSON(pluginPath) → get version for substitution
├── 3. Read existing ~/.config/opencode/.mcp.json
├── 4. For each server:
│       ├── Prefix name: "pluginName.serverName"
│       ├── Substitute variables
│       └── Add to config
├── 5. Ensure ~/.config/opencode/ directory exists
└── 6. Write updated .mcp.json
```

## Uninstallation

```
UninstallMCPConfig(pluginName) error
│
├── 1. Read ~/.config/opencode/.mcp.json
├── 2. Remove all entries with prefix "pluginName."
├── 3. Write updated .mcp.json
└── 4. Return nil if file doesn't exist
```

## Naming Convention

MCP servers are prefixed with the plugin name to avoid conflicts:

```
plugin: discord, server: discord  →  discord.discord
plugin: github,  server: github   →  github.github
plugin: playwright, server: playwright → playwright.playwright
```

## MCP Server Types

| Type | Required Fields | Example |
|------|-----------------|---------|
| `stdio` | `command`, optional `args`, `env` | `"command": "npx", "args": ["@playwright/mcp@latest"]` |
| `http` | `url`, optional `headers`, `env` | `"url": "https://api.githubcopilot.com/mcp/", "headers": {"Authorization": "Bearer ..."}}` |
| `sse` | `url` | `"url": "https://sse.example.com/events"` |
| `websocket` | `url` | `"url": "wss://ws.example.com/mcp"` |

If no `type` is specified, defaults to `stdio`.
