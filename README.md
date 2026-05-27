# opencode-plugin

`opencode-plugin` is a CLI for installing marketplace plugins into an
OpenCode-style agent setup.

It keeps plugin files in a local cache, links plugin skills into
`~/.agents/skills`, and registers plugin MCP servers in
`~/.config/opencode/opencode.json`.

## What It Does

- Adds plugin marketplaces from GitHub shorthand, Git URLs, local marketplace
  directories, or local `marketplace.json` files.
- Searches, installs, updates, and removes plugins from added marketplaces.
- Caches installed plugin files under `~/.opencode-plugin-cli/cache`.
- Links files from a plugin's `skills/` directory into `~/.agents/skills`.
- Installs MCP server config from `.mcp.json` or `plugin.json` `mcpServers`.
- Prefixes MCP server names with the plugin name.

## Install

Build and install with `make`:

```bash
make build
make install
```

`make install` copies the binary to `/usr/local/bin/`. If that directory is not
writable, build the binary and copy it to any directory on your `PATH`:

```bash
make build
cp bin/opencode-plugin /path/on/your/PATH/
```

You can also build from source directly:

```bash
git clone https://github.com/gzb1128/opencode-plugin.git
cd opencode-plugin
go build -o bin/opencode-plugin .
```

## Quick Start

```bash
opencode-plugin market add anthropics/claude-plugins-official
opencode-plugin plugin search
opencode-plugin plugin install code-simplifier
opencode-plugin plugin list
```

## Marketplaces

Add a marketplace:

```bash
opencode-plugin market add owner/repo
opencode-plugin market add git@github.com:owner/repo.git
opencode-plugin market add https://github.com/owner/repo.git
opencode-plugin market add ./path/to/marketplace
opencode-plugin market add ./path/to/marketplace.json
```

Manage configured marketplaces:

```bash
opencode-plugin market list
opencode-plugin market update
opencode-plugin market update my-market
opencode-plugin market remove my-market
```

## Plugins

```bash
opencode-plugin plugin search git
opencode-plugin plugin search --market my-market

opencode-plugin plugin install my-plugin
opencode-plugin plugin install my-plugin@my-market
opencode-plugin plugin install my-plugin --version 1.0.0

opencode-plugin plugin info my-local-plugin
opencode-plugin plugin list

opencode-plugin plugin update
opencode-plugin plugin update my-plugin

opencode-plugin plugin remove my-plugin
```

## MCP Servers

Plugins can define MCP servers in either `.mcp.json` or the `mcpServers` field
in `.claude-plugin/plugin.json`. During install, `opencode-plugin` merges those
servers into the `mcp` section of `~/.config/opencode/opencode.json`.

Supported server types:

- `stdio`
- `http`
- `sse`
- `websocket`

Supported substitutions:

- `${CLAUDE_PLUGIN_ROOT}`
- `${PLUGIN_NAME}`
- `${PLUGIN_VERSION}`

Substitution is applied to `command`, `args`, `url`, and `env` values.

Useful commands:

```bash
opencode-plugin mcp list
opencode-plugin mcp show plugin-name.server-name
```

MCP entries installed by a plugin are removed when that plugin is removed.

See [docs/MCP.md](docs/MCP.md) for MCP configuration examples.

## Runtime Files

```text
~/.opencode-plugin-cli/
├── known_marketplaces.json
├── installed_plugins.json
├── markets/
└── cache/
    └── <market-name>/
        └── <plugin-name>/
            └── <version>/
                ├── .claude-plugin/
                ├── .mcp.json
                ├── skills/
                └── ...

~/.agents/
└── skills/
    └── <skill-name> -> ~/.opencode-plugin-cli/cache/.../skills/<skill-name>

~/.config/opencode/
└── opencode.json
```

Only `skills/` are linked today. Other plugin files stay in the plugin cache so
MCP servers and supporting files can still resolve paths relative to the plugin
root.

## Version Resolution

For plugins bundled in a marketplace, the selected version is resolved in this
order:

1. `--version`
2. `.claude-plugin/plugin.json` `version`
3. Git commit SHA, shortened to 12 characters
4. `latest`

For remote plugin sources, an explicit source SHA is used first. If no SHA is
defined, `--version` is used when provided; otherwise the version is `latest`.

## Development

- Usage guide: [docs/USAGE.md](docs/USAGE.md)
- MCP details: [docs/MCP.md](docs/MCP.md)
- Architecture notes: [docs/design/ARCHITECTURE.md](docs/design/ARCHITECTURE.md)
- Development notes: [docs/develop/develop.md](docs/develop/develop.md)

Run tests with:

```bash
make test
```

`make test` runs `go test ./...`, including e2e tests that clone the official
marketplace from GitHub.

## License

MIT
