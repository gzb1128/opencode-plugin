# opencode-plugin

> Claude Code plugin marketplace compatibility layer for OpenCode — install and manage plugins from Claude Code-compatible marketplaces with zero friction.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![GitHub Release](https://img.shields.io/github/v/release/gzb1128/opencode-plugin?include_prereleases)](https://github.com/gzb1128/opencode-plugin/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/gzb1128/opencode-plugin)](https://goreportcard.com/report/github.com/gzb1128/opencode-plugin)

`opencode-plugin` is a CLI for installing marketplace plugins into an
OpenCode-style agent setup.

It keeps plugin files in a local cache and links plugin skills into
`~/.agents/skills`.

> **Note:** This project is not built by, endorsed by, or affiliated with the
> OpenCode team. It is an independent, community-built tool that works with the
> OpenCode ecosystem.

## What It Does

- Adds plugin marketplaces from GitHub shorthand, Git URLs, local marketplace
  directories, or local `marketplace.json` files.
- Searches, installs, updates, and removes plugins from added marketplaces.
- Caches installed plugin files under `~/.opencode-plugin-cli/cache`.
- Links files from a plugin's `skills/` directory into `~/.agents/skills`.
- Preserves the plugin cache so other plugin assets remain locally available.
- Does not install or modify OpenCode MCP configuration.

## Install

### Download a Release Binary

Download the archive for your OS and CPU architecture from:

```text
https://github.com/gzb1128/opencode-plugin/releases
```

Extract the archive, move `opencode-plugin` into a directory on your `PATH`, and
verify the install:

```bash
opencode-plugin --version
```

#### macOS Gatekeeper

On macOS, downloaded binaries are tagged with a **quarantine attribute** by the
browser. When you first run `opencode-plugin`, you may see:

> **"opencode-plugin" cannot be opened because Apple cannot check it for
> malicious software.**

This happens because release binaries are built on CI and are not signed with an
Apple Developer ID. Run the following command in Terminal to remove the
quarantine attribute and re-sign the binary locally:

```bash
xattr -cr /usr/local/bin/opencode-plugin && codesign --force --sign - /usr/local/bin/opencode-plugin
```

If the binary is in a different location, replace `/usr/local/bin/opencode-plugin`
with the actual path.

Alternatively, right-click the binary in Finder and choose **Open**, then click
**Open** in the security dialog. You only need to do this once.

### Build from Source

Build and install with `make`:

```bash
make build
make install
```

`make install` copies the binary to `/usr/local/bin/`. If that directory is not
writable, use `local-install` to install to `~/.local/bin` instead (no `sudo`
needed):

```bash
make build
make local-install
```

Make sure `~/.local/bin` is on your `PATH`. You can add it with:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Building from source avoids the macOS Gatekeeper quarantine issue described
above, since the binary is built locally and not downloaded through a browser.

To remove a locally-installed binary:

```bash
make uninstall-local
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
                ├── skills/
                └── ...

~/.agents/
└── skills/
    └── <skill-name> -> ~/.opencode-plugin-cli/cache/.../skills/<skill-name>

```

Only `skills/` are linked today. Other plugin files stay in the plugin cache.

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
