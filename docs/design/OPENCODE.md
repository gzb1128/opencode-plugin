# OpenCode Integration Design

## Overview

The OpenCode integration module creates and manages symbolic links in the agents
directory (`~/.agents/`) so that installed plugin skills are discovered.

## File Structure

```
internal/opencode/
└── linker.go    # Symlink creation and removal
```

## Linker

```go
type Linker struct {
    agentsDir string  // ~/.agents
}

type ComponentCounts struct {
    Skills int
}
```

## How It Works

Skills are synced to the agents directory:

```
~/.agents/skills/
├── code-simplifier → ~/.opencode-plugin-cli/cache/.../code-simplifier/skills/code-simplifier
└── frontend-design → ~/.opencode-plugin-cli/cache/.../frontend-design/skills/frontend-design
```

## CreateSymlinks

```
CreateSymlinks(pluginPath string) (*ComponentCounts, error)
│
├── For the skills component directory:
│   ├── Check if source exists in pluginPath
│   ├── Read entries from source directory
│   └── For each entry:
│       ├── Compute target path: agentsDir/skills/<entry_name>
│       ├── Skip if target already exists as symlink to same location
│       ├── Warn and skip if target exists as symlink to different location (conflict)
│       ├── Warn and skip if target exists as regular file
│       └── Create symlink: target → source
│
└── Return counts
```

### Conflict Handling

| Target State | Action |
|-------------|--------|
| Doesn't exist | Create symlink |
| Symlink to same source | Skip (no-op) |
| Symlink to different source | Warn, skip (another plugin owns it) |
| Regular file (not symlink) | Warn, skip (don't overwrite user files) |

## RemoveSymlinks

```
RemoveSymlinks(pluginPath string) (int, error)
│
├── For the skills component directory:
│   ├── Read entries from agentsDir/skills/
│   └── For each entry:
│       ├── Resolve symlink target
│       ├── If target is inside pluginPath → remove symlink
│       └── If not → skip (belongs to another plugin)
│
└── Return count of removed symlinks
```

## Directory Scanning

The linker scans plugin directories for the `skills/` subdirectory:
- `skills/` — scanned recursively, top-level entries become symlinks

Each entry (file or directory) in these directories is symlinked individually.
For example, a plugin with:

```
skills/
├── my-skill/
│   ├── SKILL.md
│   └── references/
└── another-skill.md
```

Creates two symlinks:
- `~/.agents/skills/my-skill → .../skills/my-skill/`
- `~/.agents/skills/another-skill.md → .../skills/another-skill.md`
