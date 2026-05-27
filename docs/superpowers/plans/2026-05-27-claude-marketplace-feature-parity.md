# Claude Marketplace Feature Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the OpenCode plugin marketplace runtime closer to Claude Code's marketplace behavior while preserving the current CLI, cache layout, and verified `coding@local-marketplace` install flow.

**Architecture:** Keep the existing Go module boundaries: `internal/marketplace` owns marketplace source parsing, fetching, caching, and index paths; `internal/plugin` owns plugin materialization, dependency closure, versioning, and install records; `internal/opencode` owns the OpenCode-facing linking/config surface. The implementation should land in small compatibility slices so each slice can be built, tested, and manually verified without requiring all Claude Code features at once.

**Tech Stack:** Go, Cobra CLI, existing `go-git` usage for normal git flows, standard library HTTP/JSON/file APIs, existing config JSON files under `~/.opencode-plugin-cli`, OpenCode config under `~/.config/opencode`, skills links under `~/.agents`.

---

## Current State

The parser refactor already added typed marketplace and plugin source models for `local`, `github`, `git`, `git-subdir`, `url`, `npm`, `pip`, `file`, and `directory`. It also parses `forceRemoveDeletedPlugins`, `metadata.pluginRoot`, `allowCrossMarketplaceDependenciesOn`, `tags`, and `strict`.

The runtime still has gaps:

- `url` marketplace sources parse but cannot be fetched or cached.
- Marketplace `ref`, `path`, `sparsePaths`, and `headers` are persisted but only partially used.
- Plugin `npm` and `pip` sources parse but intentionally fail at install time.
- Plugin dependencies are not parsed into a dedicated field and are not installed as a closure.
- Only `skills/` are linked into `~/.agents/skills`; commands, agents, hooks, output styles, settings, LSP, and user config are not handled.
- `strict`, `metadata.pluginRoot`, and `forceRemoveDeletedPlugins` are metadata only.
- There is no policy layer for reserved marketplace names, strict allowlists, blocklists, or seed-managed marketplaces.

## Design Principles

- Preserve current CLI compatibility. Existing commands and config files must keep working.
- Prefer additive config fields over rewrites. Existing flat `known_marketplaces.json` entries remain valid.
- Make every feature testable without the network where possible by using local HTTP servers, local git repositories, and temporary config directories.
- Treat local filesystem paths as untrusted input. Resolve and validate before copying or removing.
- Do not implement Claude Code-only UI behavior unless it maps to a useful CLI behavior.
- Keep not-yet-supported source types explicit. Parsing support is acceptable only when install/update emits a clear error.

## Feature Priorities

### P0: Marketplace Source Runtime Parity

P0 closes the largest mismatch between parsed config and runtime behavior.

1. Parse Claude Code marketplace input forms: arbitrary SSH usernames, `#ref` fragments, GitHub shorthand `owner/repo@ref`, `~` paths, and Azure DevOps `/_git/` URLs.
2. Fetch and cache `url` marketplace sources.
3. Honor marketplace `ref` and `path` for `github` and `git`.
4. Preserve and use `headers` for URL sources during update.
5. Make `market list`, `market update`, `plugin search`, `plugin info`, and plugin install use one shared index-path resolver.

### P1: Plugin Materialization Semantics

P1 makes installed plugin behavior match marketplace metadata more closely.

1. Apply `metadata.pluginRoot` when resolving local plugin paths.
2. Merge marketplace entry metadata with plugin manifests, including marketplace-entry fallback when plugin manifests are absent.
3. Add dependency closure installation with cross-marketplace blocking by default.
4. Implement plugin-level `npm` source installation.
5. Make `forceRemoveDeletedPlugins` drive cleanup during marketplace update.

### P2: OpenCode Component Integration

P2 expands beyond skills only when OpenCode has an equivalent runtime surface.

1. Link `commands/` and `agents/` if OpenCode reads them from `~/.agents`.
2. Add hook support only after mapping Claude Code hook settings to OpenCode's supported hook format.
3. Defer output styles, LSP, channels, and userConfig until OpenCode has a stable equivalent.

### P3: Policy and Managed Distribution

P3 is useful for enterprise environments but is not needed for personal marketplace compatibility.

1. Reserved official marketplace name protection.
2. Strict marketplace allowlist and blocklist.
3. Seed-managed marketplaces.
4. Automatic marketplace reconciliation from settings.

## Data Model Changes

### `internal/marketplace/types.go`

Add fields that are currently missing from `Plugin` and add an `encoding/json` import for `json.RawMessage`:

```go
type ComponentEntry struct {
	Source      string `json:"source,omitempty"`
	Content     string `json:"content,omitempty"`
	Description string `json:"description,omitempty"`
}

type Plugin struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Version      string      `json:"version,omitempty"`
	Category     string      `json:"category,omitempty"`
	Author       *Author     `json:"author,omitempty"`
	Source       interface{} `json:"source"`
	Homepage     string      `json:"homepage,omitempty"`
	Repository   string      `json:"repository,omitempty"`
	License      string      `json:"license,omitempty"`
	Keywords     []string    `json:"keywords,omitempty"`
	Tags         []string    `json:"tags,omitempty"`
	DependenciesRaw json.RawMessage `json:"dependencies,omitempty"`
	Dependencies    []string        `json:"-"`
	Skills       interface{} `json:"skills,omitempty"`
	Commands     interface{} `json:"commands,omitempty"`
	Agents       interface{} `json:"agents,omitempty"`
	MCPServersRaw json.RawMessage `json:"mcpServers,omitempty"`
	MCPServers    interface{}     `json:"-"`
	HooksRaw        json.RawMessage `json:"hooks,omitempty"`
	OutputStylesRaw json.RawMessage `json:"outputStyles,omitempty"`
	ChannelsRaw     json.RawMessage `json:"channels,omitempty"`
	LSPServersRaw    json.RawMessage `json:"lspServers,omitempty"`
	UserConfigRaw    json.RawMessage `json:"userConfig,omitempty"`
	Strict       *bool       `json:"strict,omitempty"`
}
```

Use `interface{}` for `Skills`, `Commands`, and `Agents` in the first pass because Claude Code accepts multiple component forms. Normalize those fields later in the installer/linker layer into concrete component paths. Supported component shapes in this sequence:

- `skills`: string or string array.
- `commands`: string, string array, or object map whose values may contain `source`.
- `agents`: string or string array.

Use `MCPServersRaw` for JSON input because Claude Code accepts `mcpServers` as a JSON path string, MCPB path string, inline object record, or array of those forms. `ParseMarketplaceIndex()` should preserve the raw value, and `internal/mcp` should normalize the subset this project supports into its existing `MCPServer` map when generating fallback manifests or installing MCP config.

Use `HooksRaw`, `OutputStylesRaw`, `ChannelsRaw`, `LSPServersRaw`, and `UserConfigRaw` only to detect conflicts and emit warnings. Do not consume those fields in the first implementation sequence unless a task also adds an OpenCode runtime mapping for them.

Use `DependenciesRaw` for JSON input so object-form dependency entries can be accepted by `json.Unmarshal`. `ParseMarketplaceIndex()` must normalize `DependenciesRaw` into `Dependencies` and generated fallback manifests should write the normalized `Dependencies` value, not the raw input blob.

Keep dependency references simple in the first implementation:

- Accept `"plugin"`.
- Accept `"plugin@marketplace"`.
- Accept `"plugin@marketplace@^1.2.0"` and strip the trailing version constraint.
- Accept object form `{ "name": "plugin", "marketplace": "market" }` and normalize it to `plugin@market`.

### `internal/marketplace/types.go`

Keep `MarketSourceToConfig` flat for backward compatibility. Continue storing:

```json
{
  "source": "github",
  "repo": "owner/repo",
  "url": "git@github.com:owner/repo.git",
  "ref": "main",
  "path": "subdir/.claude-plugin/marketplace.json",
  "sparsePaths": [".claude-plugin", "plugins"],
  "installLocation": "/abs/path"
}
```

Do not switch to Claude Code's nested `source` object in this project. The existing config manager and CLI already expect flat maps.

## Task 1: Marketplace Input Parsing Parity

**Files:**

- Modify: `internal/marketplace/source.go`
- Modify: `cmd/market/add.go`
- Modify: `cmd/market/update.go`
- Test: `internal/marketplace/source_test.go`
- Test: `cmd/market/update_test.go`

- [ ] **Step 1: Write failing source parser tests**

Add tests for these inputs:

| Input | Expected Source |
|---|---|
| `deploy@gitlab.com:group/project.git` | `GitMarketSource{URL: "deploy@gitlab.com:group/project.git"}` |
| `org-123456@github.com:owner/repo.git#release` | `GitMarketSource{URL: "org-123456@github.com:owner/repo.git", Ref: "release"}` |
| `owner/repo#main` | `GitHubMarketSource{Repo: "owner/repo", Ref: "main"}` |
| `owner/repo@v1.0.0` | `GitHubMarketSource{Repo: "owner/repo", Ref: "v1.0.0"}` |
| `https://github.com/owner/repo#main` | `GitMarketSource{URL: "https://github.com/owner/repo.git", Ref: "main"}` |
| `https://dev.azure.com/org/proj/_git/repo#main` | `GitMarketSource{URL: "https://dev.azure.com/org/proj/_git/repo", Ref: "main"}` |
| `~/marketplaces/company` | local directory source with an absolute expanded path |

The GitHub HTTPS case intentionally changes the current Go behavior from `GitHubMarketSource` to `GitMarketSource` to match Claude Code. Update existing `internal/marketplace/source_test.go` expectations that currently classify `https://github.com/owner/repo.git` as `github`.

- [ ] **Step 2: Run failing source parser tests**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/marketplace -run TestParseMarketplaceSource_InputParity -count=1 -v
```

Expected before implementation: one or more cases either fail parsing or lose `ref`.

- [ ] **Step 3: Implement input splitting**

Add a helper in `source.go`:

```go
func splitSourceRef(input string) (base string, ref string)
```

Rules:

- Split `#ref` for all source types.
- Split `@ref` only for GitHub shorthand. Do not split SSH URLs at `@`.
- Trim whitespace before parsing.
- Preserve the original URL without fragment in the stored source.

- [ ] **Step 4: Expand home-relative paths**

Before `os.Stat`, expand leading `~/` using `os.UserHomeDir()`. Keep non-existing paths as errors for explicit local-looking inputs instead of falling through to GitHub shorthand.

- [ ] **Step 5: Update update source reconstruction tests**

Update `getMarketURL()` tests to be source-aware:

- `source=github` returns `repo`, even if a generated `url` is present.
- `source=git` returns `url`.
- `source=url` returns `url`.
- `source=file`, `source=directory`, and `source=local` return `path`.

This avoids converting a configured GitHub marketplace into a generic git marketplace during fallback reconstruction. The normal update path should still use `NewMarketSourceFromConfig()` and `Manager.AddSource()` so no fields are lost.

- [ ] **Step 6: Verify**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/marketplace ./cmd/market -count=1
GOCACHE=/private/tmp/codex-gocache go test ./... -count=1
```

Expected: all tests pass.

## Task 2: URL Marketplace Fetching

**Files:**

- Create: `internal/pathutil/path.go`
- Modify: `internal/marketplace/manager.go`
- Modify: `internal/marketplace/types.go`
- Modify: `cmd/market/market.go`
- Modify: `cmd/market/update.go`
- Modify: `cmd/plugin/plugin_search.go`
- Test: `internal/marketplace/manager_test.go`
- Test: `internal/pathutil/path_test.go`
- Test: `cmd/market/market_test.go`
- Test: `cmd/market/update_test.go`

- [ ] **Step 1: Write failing tests for URL marketplace add**

Add a test in `internal/marketplace/manager_test.go` that starts an `httptest.Server`, serves this body, calls `mgr.Add("remote-json", server.URL+"/marketplace.json")`, and asserts:

```json
{
  "name": "remote-json",
  "plugins": [
    {
      "name": "remote-plugin",
      "description": "Remote plugin",
      "source": "./plugins/remote-plugin"
    }
  ]
}
```

Expected assertions:

- `mp.Name == "remote-json"`
- `source.SourceType() == "url"`
- `source.InstallLocation()` points to a local cached `marketplace.json` file.
- `MarketSourceIndexPath(source)` returns that cached file path with no error.

- [ ] **Step 2: Run the failing test**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/marketplace -run TestManager_AddURLMarketplace -count=1 -v
```

Expected before implementation: failure with `JSON URL marketplace not yet implemented`.

- [ ] **Step 3: Implement URL cache path and fetch**

Implement `Manager.cacheMarketplaceFromURL(name string, source *URLMarketSource) (string, error)`:

- Create `m.marketsDir` if missing.
- Fetch with `http.Client{Timeout: 10 * time.Second}`.
- Apply `source.Headers` to the request.
- Require HTTP 2xx.
- Decode into `Marketplace` using the existing parser by writing bytes to a temporary file first.
- Write to `pathutil.SafeMarketplaceCachePath(m.marketsDir, name, ".json")`. Do not rename the config key or cache file to `marketplace.Name` after parsing; marketplace manifests can disagree with the user's chosen alias, and update/remove commands are keyed by the configured name.
- Set `source.InstallLocation()` to the cached file path.

Use a temporary file in the same directory and `os.Rename` into place so failed downloads do not leave a partial cache.

Add shared path helpers in `internal/pathutil/path.go` in this task:

```go
func ResolvePathWithinBase(basePath, relativePath string) (string, error)

func SafeMarketplaceCachePath(marketsDir, alias, suffix string) (string, error)
```

Behavior:

- `ResolvePathWithinBase()` rejects absolute relative paths, `..` traversal, and symlink escapes. Use `filepath.Abs`, `filepath.Clean`, a path-separator-aware prefix check, and a second `filepath.EvalSymlinks` check when the resolved path exists.
- Preserve the user-visible config alias exactly as the config key. Do not rewrite `anthropics/claude-plugins-official` to a new config name.
- Sanitize only the disk path segment by replacing every byte outside `[A-Za-z0-9@._-]` with `-`.
- Reject sanitized aliases that are empty, `.`, or `..`.
- Join the sanitized alias plus `suffix` under `marketsDir` and verify the result with `ResolvePathWithinBase`.
- Assert the final path is a strict child of `marketsDir`, not `marketsDir` itself.
- For remote sources with an existing `installLocation`, allow reusing it only if it resolves inside `marketsDir`; reject existing remote install locations that escape `marketsDir`.

Add tests for:

- legacy alias `anthropics/claude-plugins-official` keeps the config key and maps to a safe cache path under `marketsDir`;
- malicious alias `../../outside` maps to a safe filename under `marketsDir` and never escapes;
- alias `.` and `..` are rejected;
- an existing remote `installLocation` outside `marketsDir` is rejected.

- [ ] **Step 4: Make URL source index paths file-based**

Update `MarketSourceIndexPath(source)`:

```go
func MarketSourceIndexPath(source MarketSource) (string, error) {
	switch s := source.(type) {
	case *FileMarketSource:
		return s.Path, nil
	case *URLMarketSource:
		return s.InstallLocation(), nil
	default:
		return pathutil.ResolvePathWithinBase(source.InstallLocation(), ".claude-plugin/marketplace.json")
	}
}
```

This is required because cached URL marketplaces are single JSON files, not directories with `.claude-plugin/marketplace.json`.

Update all existing callers to handle the new `(string, error)` return:

```go
indexPath, err := marketplace.MarketSourceIndexPath(source)
if err != nil {
	return err
}
```

Required call-site updates include `internal/marketplace/manager.go`, `cmd/market/market.go`, and `cmd/plugin/plugin_search.go`.

- [ ] **Step 5: Preserve and use URL headers during update**

Do not call `mgr.Add(name, getMarketURL(market))` for URL sources that have headers because that reconstructs a source without headers before the download happens.

Add a method:

```go
func (m *Manager) AddSource(name string, source MarketSource) (*Marketplace, MarketSource, error)
```

Then make `Add(name, url)` parse the input and delegate to `AddSource`. In `updateMarket()`, call `NewMarketSourceFromConfig(market)` and pass that source to `AddSource` so URL headers, git refs, custom paths, and sparse paths are available during the fetch/clone operation. Keep `preserveConfigFields()` as a defensive guard for fields that a source parser cannot reconstruct.

`AddSource()` must compute remote cache locations through `pathutil.SafeMarketplaceCachePath()` unless the source already has an `InstallLocation()` that resolves inside `m.marketsDir`. This keeps legacy aliases such as `anthropics/claude-plugins-official` working without allowing new path traversal through config keys.

Add a test that a URL marketplace update sends the configured `Authorization` header to the `httptest.Server` and persists the header after update.

- [ ] **Step 6: Make marketplace removal source-aware**

`Manager.Remove(name)` currently removes `<marketsDir>/<name>`, which is incomplete after cache paths move through `pathutil.SafeMarketplaceCachePath()` and URL marketplaces become cached JSON files.

Add a source-aware removal helper:

```go
func (m *Manager) RemoveSource(name string, source MarketSource) error
```

Behavior:

- `GitHubMarketSource` and `GitMarketSource`: remove the validated remote install location, defaulting to `pathutil.SafeMarketplaceCachePath(m.marketsDir, name, "")`.
- `URLMarketSource`: remove `source.InstallLocation()` only if it is inside `m.marketsDir` and is a file.
- `FileMarketSource`, `DirectoryMarketSource`, and `LocalMarketSource`: do not delete user-owned paths.

Update the CLI remove command to load the existing config entry, call `NewMarketSourceFromConfig()`, then call `RemoveSource()` before removing the config entry. Keep `Remove(name)` as a compatibility wrapper for git-style directory removal if existing callers still use it.

Add tests:

- removing a URL marketplace deletes the sanitized URL cache file;
- removing a local/file/directory marketplace removes only the config entry and leaves the user path on disk;
- removing a git marketplace with legacy alias `anthropics/claude-plugins-official` deletes only its validated cache directory.

- [ ] **Step 7: Verify**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/marketplace ./cmd/market -count=1
GOCACHE=/private/tmp/codex-gocache go test ./... -count=1
```

Expected: all tests pass.

## Task 3: Marketplace Git `ref`, `path`, and Sparse Checkout

**Files:**

- Modify: `internal/marketplace/git.go`
- Modify: `internal/marketplace/manager.go`
- Modify: `internal/pathutil/path.go`
- Modify: `internal/marketplace/types.go`
- Test: `internal/marketplace/manager_test.go`
- Test: `internal/marketplace/source_test.go`

- [ ] **Step 1: Write failing tests for custom marketplace path**

Create a local git repo in a temp dir with marketplace JSON at `catalog/.claude-plugin/marketplace.json`. Configure a `GitMarketSource` with:

```go
source := &GitMarketSource{
	URL:  repoPath,
	Path: "catalog/.claude-plugin/marketplace.json",
}
```

Assert `MarketSourceIndexPath(source)` returns `<installLocation>/catalog/.claude-plugin/marketplace.json` with no error.

Add negative cases:

- `Path: "../outside/marketplace.json"` returns an error;
- `Path: "/tmp/marketplace.json"` returns an error;
- `Path` pointing through a symlink that escapes `InstallLocation()` returns an error after `filepath.EvalSymlinks`.

- [ ] **Step 2: Write failing tests for `ref`**

Create a temp git repo with two branches:

- `main` has marketplace name `main-market`.
- `feature` has marketplace name `feature-market`.

Call add/update through a config that stores `ref: "feature"` and assert the parsed marketplace name is `feature-market`.

- [ ] **Step 3: Run focused tests**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/marketplace -run 'TestManager_AddGitMarketplace(CustomPath|Ref)' -count=1 -v
```

Expected before implementation: either default path is used or default branch content is parsed.

- [ ] **Step 4: Implement ref-aware clone and pull**

Add:

```go
type CloneOptions struct {
	Ref string
	SparsePaths []string
}

func (g *GitClient) CloneOrPullWithOptions(url, path string, opts CloneOptions) error
```

Behavior:

- For new clones, pass `ReferenceName` or checkout the resolved `Ref` immediately after clone.
- For existing repos, fetch before checkout when `Ref` is non-empty, then pull the selected ref when possible.
- Preserve the current `CloneOrPull(url, path)` wrapper by delegating with zero-value options.
- Keep branch, tag, and SHA checkout support through `GitClient.Checkout`.

- [ ] **Step 5: Implement safe custom marketplace index path**

Ensure `MarketSourceIndexPath(source)` uses `Path` for `GitHubMarketSource` and `GitMarketSource` when set:

```go
func ResolvePathWithinBase(basePath, relativePath string) (string, error) {
	// Implement in internal/pathutil/path.go.
}

func GetMarketSourceManifestPath(source MarketSource) string {
	switch s := source.(type) {
	case *GitHubMarketSource:
		return s.Path
	case *GitMarketSource:
		return s.Path
	default:
		return ""
	}
}

if path := GetMarketSourceManifestPath(source); path != "" {
	return pathutil.ResolvePathWithinBase(source.InstallLocation(), path)
}
return pathutil.ResolvePathWithinBase(source.InstallLocation(), ".claude-plugin/marketplace.json")
```

`pathutil.ResolvePathWithinBase()` must reject absolute manifest paths, `..` traversal, and symlink escapes. Use `filepath.Abs`, `filepath.Clean`, a path-separator-aware prefix check, and a second `filepath.EvalSymlinks` check when the resolved path exists.

- [ ] **Step 6: Defer sparse checkout behind a capability boundary**

Do not fake sparse checkout with `go-git`. Add a `GitClient.CloneOrPullWithOptions(url, path string, opts CloneOptions)` wrapper and initially use full clone when `SparsePaths` is set. Emit a clear warning in CLI output:

```text
Sparse checkout requested but not implemented; using full clone.
```

Add a test asserting `sparsePaths` is preserved in config and does not break add/update. Implement true sparse checkout in a later system-git slice if repository size makes it necessary.

- [ ] **Step 7: Verify**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/marketplace ./cmd/market -count=1
GOCACHE=/private/tmp/codex-gocache go test ./... -count=1
```

Expected: all tests pass.

## Task 4: Manifest Semantics (`pluginRoot`, Entry Fallback, and Dependencies Field)

**Files:**

- Modify: `internal/marketplace/types.go`
- Modify: `internal/marketplace/parser.go`
- Modify: `internal/mcp/manager.go`
- Modify: `cmd/plugin/plugin_info.go`
- Modify: `internal/plugin/version.go`
- Modify: `internal/plugin/installer.go`
- Modify: `internal/pathutil/path.go`
- Test: `internal/marketplace/parser_test.go`
- Test: `internal/mcp/manager_test.go`
- Test: `internal/pathutil/path_test.go`
- Test: `internal/plugin/version_test.go`
- Test: `internal/plugin/installer_test.go`

- [ ] **Step 1: Write parser tests for dependencies**

Add a marketplace fixture with:

```json
{
  "name": "deps-market",
  "plugins": [
    {
      "name": "root",
      "description": "Root plugin",
      "source": "./plugins/root",
      "dependencies": ["dep", "other@shared", "range@shared@^1.2.0"]
    }
  ]
}
```

Assert parsed dependencies are:

```go
[]string{"dep", "other@shared", "range@shared"}
```

- [ ] **Step 2: Write object-form dependency parser tests**

Add a marketplace fixture with:

```json
{
  "name": "object-deps-market",
  "plugins": [
    {
      "name": "root",
      "description": "Root plugin",
      "source": "./plugins/root",
      "dependencies": [
        { "name": "dep" },
        { "name": "shared-dep", "marketplace": "shared" }
      ]
    }
  ]
}
```

Assert parsed dependencies are:

```go
[]string{"dep", "shared-dep@shared"}
```

- [ ] **Step 3: Write pluginRoot path tests**

Add a marketplace with:

```json
{
  "name": "rooted-market",
  "metadata": { "pluginRoot": "packages" },
  "plugins": [
    {
      "name": "tool",
      "description": "Tool",
      "source": "./tool"
    }
  ]
}
```

Assert local source path resolves to `<marketInstallLocation>/packages/tool`.

- [ ] **Step 4: Run focused tests**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/marketplace ./internal/plugin -run 'TestParseMarketplaceIndex.*Dependencies|TestGetPluginSourcePath.*PluginRoot|TestInstaller.*EntryFallback' -count=1 -v
```

Expected before implementation: dependencies are empty and pluginRoot is ignored.

- [ ] **Step 5: Parse dependencies**

Add a helper in `parser.go`:

```go
func parseDependencyRef(raw interface{}) (string, error)
```

Decode `Plugin.DependenciesRaw` into `[]interface{}` and call `parseDependencyRef()` for each entry. For `plugin@market@^1.2.0`, strip the trailing `@^...` segment. Reject empty strings and strings with spaces. Set `Plugin.Dependencies` to the normalized string slice after parsing; leave `DependenciesRaw` unchanged for debugging but do not use it after normalization.

Use explicit validation rules:

- plugin and marketplace segments must match `^[A-Za-z0-9._-]+$`;
- bare dependency format is `plugin`;
- marketplace-qualified format is `plugin@marketplace`;
- version-constrained format is `plugin@marketplace@<constraint>` and normalizes to `plugin@marketplace`;
- reject `/`, `\`, `..`, empty plugin names, empty marketplace names, and more than three `@`-separated segments.

Add negative tests for `bad/name`, `dep@`, `@market`, `dep@@market`, `dep@market@range@extra`, `../dep`, and `dep@../market`.

- [ ] **Step 6: Add metadata to plugin lookup results**

Current `FindPlugin()` returns only `Plugin`, `MarketSource`, and market name. That is not enough to apply `metadata.pluginRoot` during path resolution.

Add:

```go
type ResolvedPlugin struct {
	Plugin *Plugin
	Market MarketSource
	MarketName string
	Marketplace *Marketplace
}
```

Then add:

```go
func (m *Manager) ResolvePlugin(markets map[string]MarketSource, pluginName, marketName string) (*ResolvedPlugin, error)
```

Keep `FindPlugin()` as a compatibility wrapper around `ResolvePlugin()` so existing call sites can migrate gradually. Migrate `cmd/plugin/plugin_info.go` in this task: `plugin info` must call `ResolvePlugin()` and pass the resulting `Marketplace.Metadata.PluginRoot` into source-path resolution. Add a focused `pluginRoot` info test or CLI smoke assertion so `plugin info tool@rooted-market` reads `<marketInstallLocation>/packages/tool`, not `<marketInstallLocation>/tool`.

- [ ] **Step 7: Apply pluginRoot**

Thread marketplace metadata into source path resolution. Prefer changing the install orchestration to pass a `PluginResolutionContext`:

```go
type PluginResolutionContext struct {
	MarketPath string
	PluginRoot string
}
```

Keep the old `GetPluginSourcePath(plugin, marketPath)` as a wrapper with empty `PluginRoot` to avoid broad call-site churn in the first slice.

- [ ] **Step 8: Add plugin materialization stage**

Introduce a staging/materialization step before versioned cache copy, linking, MCP install, or install-record writes:

```go
type MaterializedPlugin struct {
	Path string
	Version string
	ManifestPath string
}

func (i *Installer) materializePlugin(resolved *marketplace.ResolvedPlugin, opts InstallOptions) (*MaterializedPlugin, error)
```

Behavior:

- Resolve the source path using `PluginResolutionContext`.
- If `.claude-plugin/plugin.json` exists, parse it and compute version from explicit CLI version, plugin manifest version, marketplace entry version, git SHA, then `latest`.
- If `.claude-plugin/plugin.json` is missing, create a staging copy/cache first, generate `.claude-plugin/plugin.json` from the marketplace entry in that staging path, then compute version using explicit CLI version first and marketplace entry `version` second.
- Build the versioned cache path with `pathutil.SafePluginCachePath(cacheDir, pluginID, version)`, not `filepath.Join(cacheDir, marketName, pluginName, version)`.
- Install/link/MCP setup must operate on the materialized path so generated fallback manifest fields such as `version` and `mcpServers` are visible to existing managers.
- Install records must use `MaterializedPlugin.Version`.

Add a test: entry-only plugin with marketplace entry `version: "2.0.0"` and no plugin.json installs into cache path ending in `/2.0.0`, writes install record version `2.0.0`, and can install MCP config from fallback `mcpServers`.

Add:

```go
func SafePluginCachePath(cacheDir, pluginID, version string) (string, error)
```

Behavior:

- Parse `pluginID` as `plugin@marketplace`.
- Sanitize marketplace, plugin name, and version path segments by replacing every byte outside `[A-Za-z0-9@._-]` with `-`.
- Reject sanitized segments that are empty, `.`, or `..`.
- Join sanitized segments under `cacheDir` and verify with `ResolvePathWithinBase`.
- Assert the final path is exactly three segments below `cacheDir`: `<marketplace>/<plugin>/<version>`.
- `SafePluginCachePath()` should split `pluginID` at the last `@` only. This preserves scoped npm-style plugin names such as `@scope/name@local-marketplace` as plugin name `@scope/name` and marketplace `local-marketplace`. This is cache-key parsing only; CLI plugin spec parsing remains a separate compatibility task unless explicitly changed.
- Do not treat npm scoped names such as `@scope/name` as path separators; the `/` must become `-`.

Add tests for plugin IDs and versions containing `/`, `.`, `..`, and npm scoped names such as `@scope/name@local-marketplace`.

Add `GitSubdirSource` version tests:

- two git-subdir plugins from the same commit SHA but different subpaths must get different version strings;
- version format is `<shortSHA>-<sha256(normalizedSubPath)[:8]>`;
- subpath normalization replaces backslashes with `/`, strips one leading `./`, and strips trailing `/`.

- [ ] **Step 9: Add local source path traversal checks**

Before joining any marketplace-relative plugin source path, validate the resolved path stays inside the marketplace root plus optional `pluginRoot`.

Reuse the shared helper introduced in Task 2:

```go
func pathutil.ResolvePathWithinBase(basePath, relativePath string) (string, error)
```

Use it to reject `./../outside` traversal. During copy/link operations, validate the final discovered component file or directory with the same helper before using it.

- [ ] **Step 10: Define marketplace-entry fallback behavior**

Do not reject a plugin solely because `.claude-plugin/plugin.json` is missing. Claude Code uses marketplace entry data as the manifest fallback when the plugin manifest is absent.

Implement this behavior:

- If `.claude-plugin/plugin.json` is missing, generate `.claude-plugin/plugin.json` from the marketplace entry fields.
- If `strict` is `false`, allow the generated manifest to be the full source of truth even when the plugin directory does not carry metadata.
- If `strict` is `nil` or `true`, still allow marketplace-entry fallback, but fail if neither plugin manifest nor marketplace entry has enough fields to create a valid manifest.
- If `.claude-plugin/plugin.json` exists and `strict` is `nil` or `true`, parse it and merge marketplace entry component fields as supplemental metadata.
- If `.claude-plugin/plugin.json` exists, `strict` is `false`, and the marketplace entry declares `commands`, `agents`, `skills`, `hooks`, or `outputStyles`, fail with a conflict error. This matches Claude Code's behavior: non-strict marketplace-entry components are the manifest source of truth only when no plugin manifest exists.

The generated manifest should include `name`, `version`, `description`, `author`, `homepage`, `repository`, `license`, `keywords`, `dependencies`, `skills`, `commands`, `agents`, and `mcpServers`. Preserve `hooks`, `outputStyles`, `channels`, `lspServers`, and `userConfig` as deferred fields by ignoring them with a warning rather than silently claiming they were installed. For component entries that use inline `content` instead of `source`, emit a warning and skip the inline content in this sequence; do not synthesize files until OpenCode inline command/agent semantics are designed.

Normalize `mcpServers` through `internal/mcp`:

```go
func NormalizeMCPServers(pluginPath string, raw json.RawMessage) (map[string]mcp.MCPServer, []string, error)
```

Supported first-pass forms:

- inline object record, matching the existing `map[string]MCPServer` shape;
- string path to a JSON file relative to the plugin root, parsed with the existing `.mcp.json` reader rules;
- array containing inline object records and JSON path strings.

Every relative JSON path must be resolved with `pathutil.ResolvePathWithinBase(pluginPath, rawPath)` before reading the file. Add tests for `./../../outside.json` and a symlink that points outside the plugin root.

MCPB path strings and URL strings should return a warning and be skipped in this sequence because this project does not have MCPB installation support.

- [ ] **Step 11: Add fallback component tests**

Add installer tests for a plugin directory without `.claude-plugin/plugin.json` where the marketplace entry contains:

```json
{
  "name": "entry-only",
  "description": "Entry-only plugin",
  "source": "./plugins/entry-only",
  "version": "2.0.0",
  "skills": ["./skills/entry-skill"],
  "commands": { "review": { "source": "./commands/review.md" } },
  "agents": ["./agents/reviewer.md"],
  "mcpServers": {
    "entry-server": {
      "type": "stdio",
      "command": "node",
      "args": ["server.js"]
    }
  }
}
```

Expected assertions:

- generated fallback manifest contains `skills`, `commands`, `agents`, and `mcpServers`;
- generated fallback manifest contains `version: "2.0.0"` and install record/cache version is `2.0.0`;
- existing skill-directory linking still works when the source contains a `skills/` directory;
- install writes the `entry-server` MCP config using the existing `internal/mcp` manager;
- unsupported deferred fields emit warnings when present and are not written into OpenCode config.
- `mcpServers` as a relative JSON path is normalized and installed when the referenced JSON file exists.

Do not assert command or agent symlink creation in this task. Task 6 owns component linking and should use this generated fallback manifest as one of its fixtures.

Add a conflict test:

- create a plugin with `.claude-plugin/plugin.json`;
- add marketplace entry `strict: false` and `skills: ["./skills/entry-skill"]`;
- assert loading/installing that entry fails with a conflict error and does not partially install the plugin.

- [ ] **Step 12: Verify**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/marketplace ./internal/plugin -count=1
GOCACHE=/private/tmp/codex-gocache go test ./... -count=1
```

Expected: all tests pass.

## Task 5: Dependency Closure Installation

**Files:**

- Create: `internal/plugin/dependencies.go`
- Modify: `internal/plugin/installer.go`
- Test: `internal/plugin/dependencies_test.go`
- Test: `internal/plugin/installer_test.go`

- [ ] **Step 1: Write pure resolver tests**

Create `internal/plugin/dependencies_test.go` with cases:

- Bare dependency `dep` from `root@main` qualifies to `dep@main`.
- Already-installed dependency is skipped.
- Cycle `a@main -> b@main -> a@main` returns a cycle error.
- Cross-marketplace dependency `dep@other` is blocked unless `other` appears in root marketplace `AllowCrossMarketplaceDeps`.

- [ ] **Step 2: Run failing resolver tests**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/plugin -run TestResolveDependencyClosure -count=1 -v
```

Expected before implementation: compile fails because resolver does not exist.

- [ ] **Step 3: Implement resolver**

Add:

```go
type DependencyResolutionResult struct {
	Closure []string
}

func ResolveDependencyClosure(root string, lookup func(string) (*marketplace.ResolvedPlugin, error), alreadyInstalled map[string]bool, allowedCrossMarketplaces map[string]bool) (*DependencyResolutionResult, error)
```

Closure order must install dependencies before the root plugin and must include the root id as the last element. Already-installed dependencies are skipped, but the root is never skipped because an explicit install should repair missing cache/config state. The resolver should read dependencies from `resolved.Plugin.Dependencies`, but it must return only ids in `Closure`; the installer keeps a side map `map[string]*marketplace.ResolvedPlugin` populated by the lookup so installation can pass full context to `installOneResolvedPlugin()`.

- [ ] **Step 4: Integrate into `Installer.Install`**

Before installing the requested plugin:

- Build the root id as `<pluginName>@<marketName>`.
- Read root marketplace `AllowCrossMarketplaceDeps`.
- Resolve dependencies.
- Install each dependency with the same scope, skipping installed records.
- Then install root.

Avoid recursion through public `Install()` to keep errors and progress understandable. Extract a private `installOneResolvedPlugin()` that takes an already-found `*marketplace.ResolvedPlugin`. Do not reduce the value back to `Plugin`, `MarketSource`, and market name, because dependencies need the same `Marketplace.Metadata.PluginRoot`, `AllowCrossMarketplaceDeps`, and fallback-manifest context as the root plugin.

- [ ] **Step 5: Verify real local marketplace still works**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go build -o opencode-plugin .
./opencode-plugin plugin remove coding@local-marketplace || true
./opencode-plugin plugin install coding@local-marketplace
./opencode-plugin plugin info coding@local-marketplace
```

Expected: install succeeds and reports `coding@1.2.0`.

## Task 6: Component Linking Beyond Skills

**Files:**

- Modify: `internal/opencode/linker.go`
- Modify: `internal/plugin/installer.go`
- Test: `internal/opencode/linker_test.go`

- [ ] **Step 1: Confirm OpenCode target directories and stop if unavailable**

Before implementation, confirm whether OpenCode reads commands and agents from:

```text
~/.agents/commands
~/.agents/agents
```

If OpenCode uses different paths, update this task with the confirmed paths before editing code. If OpenCode does not currently load commands or agents from filesystem directories, stop this task and report that component linking is not implementable without OpenCode-side support.

- [ ] **Step 2: Write linker tests**

Create test plugin content for the default directory convention:

```text
skills/coding/SKILL.md
commands/review.md
agents/reviewer.md
```

Assert symlinks are created under the configured agents dir:

```text
<agentsDir>/skills/coding
<agentsDir>/commands/review.md
<agentsDir>/agents/reviewer.md
```

Add a second fixture that uses the fallback manifest generated in Task 4:

```json
{
  "name": "entry-only",
  "skills": ["./skills/entry-skill"],
  "commands": { "review": { "source": "./commands/review.md" } },
  "agents": ["./agents/reviewer.md"]
}
```

Assert the same links are created from manifest-declared component paths. Manifest-declared paths must be resolved with `pathutil.ResolvePathWithinBase()` before linking.

Add command path shape tests for:

```json
{ "commands": "./commands/review.md" }
```

and:

```json
{ "commands": ["./commands/review.md", "./commands/fix.md"] }
```

These manifest-declared component path tests must use Claude-valid `./` relative paths. If the implementation chooses to also accept `commands/review.md` without `./`, test that as an explicitly documented compatibility extension, not as the Claude parity baseline.

- [ ] **Step 3: Implement generic component linking**

Replace skill-only logic with:

```go
type ComponentCounts struct {
	Skills   int
	Commands int
	Agents   int
}
```

Implement a helper:

```go
func (l *Linker) linkComponentDir(pluginPath, component string) (int, []string, error)
```

Use it for `skills`, `commands`, and `agents` when a plugin uses default directories. Add a second helper for manifest-declared component paths:

```go
type ComponentPath struct {
	Name string
	Path string
}

func (l *Linker) linkComponentPaths(pluginPath, component string, paths []ComponentPath) (int, []string, error)
```

The installer should build `[]ComponentPath` from fallback or parsed manifests. Supported first-pass forms:

- `skills`: string or string array of relative directories or files.
- `commands`: string, string array, or object map where each value has `source`.
- `agents`: string or string array of relative markdown files.

If a manifest declares a component field, use those paths. If it does not, fall back to the default component directory. If a command or agent entry uses inline `content` without `source`, emit a warning and skip that entry in this sequence; OpenCode inline command/agent semantics need a separate design before synthesizing files.

- [ ] **Step 4: Implement generic component unlinking**

Update `RemoveSymlinks(pluginPath string)` to scan every supported component directory, not only `skills`.

Add:

```go
func (l *Linker) unlinkComponentDir(pluginPath, component string) (int, error)
```

Expected behavior:

- Remove links under `<agentsDir>/<component>/`.
- Only remove links whose target is inside the installed plugin path after resolving symlinks.
- Leave user-owned files and links to other plugins untouched.

Add tests that installing then removing a plugin removes `skills`, `commands`, and `agents` links.

- [ ] **Step 5: Update installer output**

Print non-zero component counts:

```text
Skills: 1
Commands: 2
Agents: 1
```

- [ ] **Step 6: Verify**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/opencode ./internal/plugin -count=1
GOCACHE=/private/tmp/codex-gocache go test ./... -count=1
```

Expected: all tests pass.

## Task 7: Deleted Plugin Cleanup

**Files:**

- Modify: `cmd/market/update.go`
- Modify: `internal/plugin/installer.go`
- Test: `cmd/market/update_test.go`

- [ ] **Step 1: Write cleanup test**

Create a temp installed records file with `old-plugin@cleanup-market`. Create a marketplace update result where `cleanup-market` has `ForceRemoveDeletedPlugins=true` and no `old-plugin`.

Expected:

- install record is removed.
- cache directory is removed or marked for removal according to current project behavior.
- symlinks and MCP config are removed by calling existing installer remove logic.
- deleted plugin detection compares the pre-update old index against the newly fetched index, not a post-update overwritten file.

- [ ] **Step 2: Implement deleted-plugin detection**

In `updateMarket()`, before calling `AddSource()` or any clone/pull/fetch that can mutate the marketplace cache:

- Load old marketplace index if present.
- If the old index cannot be loaded because the marketplace is missing or corrupt, log a warning and skip deletion cleanup for that update rather than guessing.
- Fetch/pull the marketplace and load new marketplace index from `mp`.
- Compute deleted plugin names.
- If `ForceRemoveDeletedPlugins` is true, call the existing installer removal flow for each installed key that matches a deleted plugin in this market.
- Only remove install records after symlink, MCP, and cache cleanup have used the record's install path.

Do not delete installed records first. `Installer.Remove()` currently depends on the record to find the cache path and clean plugin-owned resources.

For URL marketplaces, use the same pre-update snapshot rule: read the old cached file at `pathutil.SafeMarketplaceCachePath(marketsDir, name, ".json")` before replacing it with the downloaded temp file. The URL cache write should remain atomic: compare old/new after the temp file parses successfully, then rename the temp file into place.

- [ ] **Step 3: Verify**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./cmd/market ./internal/plugin -count=1
GOCACHE=/private/tmp/codex-gocache go test ./... -count=1
```

Expected: all tests pass.

## Task 8: Plugin-Level NPM Source Install

**Files:**

- Modify: `internal/plugin/version.go`
- Modify: `internal/plugin/installer.go`
- Test: `internal/plugin/version_test.go`
- Test: `internal/plugin/installer_test.go`

- [ ] **Step 1: Write npm install tests with a local package**

All npm install tests in this task must inject a fake npm runner. Do not shell out to the real `npm` binary in Go tests and do not require network access. The fake runner should record the package spec, prefix, and registry arguments, then create the expected `node_modules/<packageName>` tree inside the requested prefix.

Create a temp npm package directory containing:

```json
{
  "name": "test-opencode-plugin",
  "version": "1.0.0"
}
```

Include `.claude-plugin/plugin.json` and `skills/npm-skill/SKILL.md` in that package directory. Use an npm source that points at the local package path:

```go
&marketplace.NpmSource{
	Package: packageDir,
	Version: "",
}
```

Assert the package is installed into the plugin cache and the skill is linkable.

The fake runner for this test must:

- receive package spec equal to `packageDir`;
- receive a prefix under the test temp directory;
- read `<packageDir>/package.json` to determine package name `test-opencode-plugin`;
- create `<prefix>/node_modules/test-opencode-plugin/.claude-plugin/plugin.json`;
- create `<prefix>/node_modules/test-opencode-plugin/skills/npm-skill/SKILL.md`.

Add a second test for a scoped local package:

```json
{
  "name": "@scope/test-opencode-plugin",
  "version": "1.0.0"
}
```

Assert the installed package is copied from `<npmCacheDir>/node_modules/@scope/test-opencode-plugin`, not from a path derived by stripping the scope.

The fake runner for the scoped package test must create `<prefix>/node_modules/@scope/test-opencode-plugin/...` and assert the copied cache path preserves the nested scoped package directory when resolving from `node_modules`.

Add a third test for a registry package with a version:

```go
&marketplace.NpmSource{
	Package: "left-pad",
	Version: "1.3.0",
}
```

Use the same fake npm runner to assert the installer builds package spec `left-pad@1.3.0` without requiring network access. The fake runner must create `<prefix>/node_modules/left-pad/.claude-plugin/plugin.json` and `<prefix>/node_modules/left-pad/skills/npm-skill/SKILL.md`.

Add version resolution tests:

- registry package with `Version: "1.3.0"` installs into cache/record version `1.3.0` when the user did not request an explicit version;
- explicit CLI version still wins over `NpmSource.Version`;
- local path package with empty `Version` reads `package.json.version` and uses `1.0.0` for the cache/record version.

Each test should construct a resolver with the fake runner:

```go
resolver := plugin.NewVersionResolver()
resolver.SetNPMRunner(fakeRunner)
```

If tests need to assert installer-level cache/record behavior, pass that resolver into `Installer` through a small constructor or test-only option rather than mutating package globals.

- [ ] **Step 2: Run failing npm tests**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/plugin -run TestCloneRemotePlugin_NpmSource -count=1 -v
```

Expected before implementation: `npm source installation not yet implemented`.

- [ ] **Step 3: Implement npm install into a package cache**

Add:

```go
type NPMRunner interface {
	Install(packageSpec, prefix, registry string) error
}

func (v *VersionResolver) SetNPMRunner(runner NPMRunner)

func (v *VersionResolver) installNpmSource(src *marketplace.NpmSource, cachePath string) error
```

The production runner should execute:

```bash
npm install <packageSpec> --prefix <npmCacheDir>
```

and append `--registry <registry>` when `registry` is non-empty. Tests must inject `fakeNPMRunner`; no unit test in this task should invoke production npm.

Behavior:

- Build `packageSpec` before invoking npm:
  - if `src.Package` is a local path, use the path as `packageSpec` and reject `src.Version` with a clear error because npm local path plus version is ambiguous;
  - if `src.Package` is a registry package and `src.Version` is non-empty, use `src.Package + "@" + src.Version`;
  - preserve scoped package names, so `@scope/name` plus version becomes `@scope/name@1.2.3`.
- Call `v.npmRunner.Install(packageSpec, npmCacheDir, src.Registry)`.
- The production runner translates that call to `npm install <packageSpec> --prefix <npmCacheDir>` and includes `--registry <registry>` when `src.Registry` is set.
- Resolve the installed package name before copying:
  - if `src.Package` is an existing local directory, read `<src.Package>/package.json` and use its `name`;
  - otherwise, derive the package directory from the npm package spec, preserving scoped names like `@scope/name`;
  - if derivation is ambiguous, read `<npmCacheDir>/package-lock.json` and find the package whose resolved spec matches the install request.
- Copy `<npmCacheDir>/node_modules/<resolvedPackageName>` into `cachePath`.
- Support local package paths because tests should not require network.

- [ ] **Step 4: Update npm version resolution**

Update `resolveRemoteVersion()` so `NpmSource` participates in cache and install-record versioning:

- if the user requested an explicit version, keep returning that requested version;
- if `NpmSource.Version` is non-empty, return it;
- if `NpmSource.Package` is an existing local directory, read its `package.json.version` and return that value when present;
- otherwise return `latest`.

This prevents a source such as `{ "source": "npm", "package": "left-pad", "version": "1.3.0" }` from installing into a `latest` cache directory.

- [ ] **Step 5: Keep pip explicitly unsupported**

Leave `PipSource` as a clear unsupported error:

```text
pip source installation not yet implemented for package: coding-tools
```

- [ ] **Step 6: Verify**

Run:

```bash
GOCACHE=/private/tmp/codex-gocache go test ./internal/plugin -count=1
GOCACHE=/private/tmp/codex-gocache go test ./... -count=1
```

Expected: all tests pass.

## Deferred Items

These are intentionally outside the first implementation sequence:

- `npm` marketplace sources. Claude Code still marks marketplace-level npm as not implemented.
- `pip` plugin install. Claude Code parses pip but reports it unsupported.
- `userConfig`, channels, output-styles, and LSP. They need a direct OpenCode runtime mapping before implementation.
- Enterprise policy features. They should be designed after runtime parity lands because policy enforcement changes user-visible behavior.
- Seed-managed marketplaces and settings-sourced inline marketplaces. These are useful for managed deployments but not required for local CLI compatibility.
- Background marketplace `autoUpdate`. Keep manual `opencode-plugin market update` as the only update trigger in this sequence.
- Zip cache. It optimizes storage and distribution, not compatibility.

## Verification Matrix

Use this matrix after each task:

```bash
GOCACHE=/private/tmp/codex-gocache go build ./...
GOCACHE=/private/tmp/codex-gocache go vet ./...
GOCACHE=/private/tmp/codex-gocache go test ./... -count=1
```

Run the real local install smoke test before declaring runtime-affecting tasks complete:

```bash
GOCACHE=/private/tmp/codex-gocache go build -o opencode-plugin .
./opencode-plugin plugin remove coding@local-marketplace || true
./opencode-plugin plugin install coding@local-marketplace
./opencode-plugin plugin info coding@local-marketplace
```

Expected smoke result:

- `coding@local-marketplace` installs successfully.
- version is `1.2.0` unless the local marketplace changes.
- cache path is under `~/.opencode-plugin-cli/cache/local-marketplace/coding/`.
- `~/.agents/skills/coding` points at the installed cache.

## Rollout Order

1. Task 1: Marketplace input parsing parity.
2. Task 2: URL marketplace fetching.
3. Task 3: Marketplace `ref` and custom `path`.
4. Task 4: `pluginRoot`, `strict`, and dependency field parsing.
5. Task 5: Dependency closure installation.
6. Task 6: Component linking after confirming OpenCode target directories.
7. Task 7: Deleted plugin cleanup.
8. Task 8: Plugin-level npm source install.

This order keeps parser/runtime consistency first, then install semantics, then broader OpenCode integration.
