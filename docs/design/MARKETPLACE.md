# Marketplace Module Design

## Overview

The marketplace module handles plugin marketplace lifecycle: source parsing,
repository management, plugin discovery, and marketplace indexing.

## File Structure

```
internal/marketplace/
├── types.go       # Domain types
├── source.go      # URL/format parsing
├── parser.go      # marketplace.json parsing
├── manager.go     # Marketplace CRUD operations
└── git.go         # Git operations (clone, pull, fetch)
```

## Domain Types (types.go)

```go
type Marketplace struct {
    Name        string
    Description string
    Owner       *Owner
    Plugins     []Plugin
}

type Plugin struct {
    Name        string
    Description string
    Version     string
    Category    string
    Author      *Author
    Source      interface{}    // string or map[string]interface{}
    Homepage    string
    Keywords    []string
}

type PluginSource struct {
    Type    string  // "local", "github", "git", "url", "git-subdir"
    Path    string
    Repo    string
    URL     string
    SubPath string
    Ref     string
    SHA     string
}

type MarketSource struct {
    Type            string
    Repo            string
    URL             string
    Path            string
    InstallLocation string
    LastUpdated     time.Time
}
```

## Source Parsing (source.go)

`ParseMarketplaceSource(url string) (*MarketSource, error)` determines the source type:

| Input | Type | Parsed Fields |
|-------|------|---------------|
| `owner/repo` | `github` | Repo = `owner/repo`, URL = `https://github.com/owner/repo.git` |
| `git@github.com:org/repo.git` | `git` | URL = input |
| `https://github.com/org/repo.git` | `git` | URL = input |
| `https://example.com/marketplace.json` | `json-url` | URL = input |
| `./path/to/dir` | `local` | Path = input |

## Marketplace Index Parsing (parser.go)

`ParseMarketplaceIndex(path string) (*Marketplace, error)` reads `marketplace.json`
and normalizes plugin sources from raw JSON into `PluginSource` structs.

Source field can be:
- String (`"./plugins/my-plugin"`) → Type: `local`
- Object with `source` key → Type: `github`, `git`, `url`, or `git-subdir`

## Manager (manager.go)

```go
type Manager struct {
    marketsDir string
    gitClient  *GitClient
}
```

### Key Methods

| Method | Description |
|--------|-------------|
| `Add(name, url)` | Clone/pull repo or read local path, parse marketplace.json |
| `Get(marketDir)` | Parse marketplace.json from existing directory |
| `List(marketDirs)` | Parse all marketplaces from a name→dir map |
| `Update(marketDir)` | `git pull` + validate marketplace.json |
| `FindPlugin(markets, name, marketName)` | Search across marketplaces; returns Plugin, MarketSource, marketName |
| `Remove(name)` | Remove marketplace directory from disk |

### FindPlugin Behavior

- If `marketName` specified: search only that marketplace
- If not specified: iterate all marketplaces, return first match
- Returns `(Plugin, MarketSource, marketName, error)` — the `marketName` return
  is critical for recording which marketplace a plugin came from

## Git Operations (git.go)

```go
type GitClient struct {
    timeout time.Duration  // default: 60s
}
```

Uses [go-git](https://github.com/go-git/go-git) library.

| Method | Description |
|--------|-------------|
| `Clone(url, path)` | Clone with submodule recursion |
| `Pull(path)` | Pull latest (ignores AlreadyUpToDate) |
| `CloneOrPull(url, path)` | Idempotent: pull if repo, clone if not |
| `IsGitRepo(path)` | Check if directory is a git repo |
| `GetLatestCommitSHA(path)` | Get HEAD commit SHA |
| `Checkout(path, ref)` | Checkout branch/tag/SHA |
| `FetchSubDir(url, subPath, ref, target)` | Clone repo, checkout ref, copy subdirectory |
