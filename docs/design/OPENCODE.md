# OpenCode Integration Design

## Overview

The OpenCode integration module creates and manages symbolic links in OpenCode's
configuration directory (`~/.config/opencode/`) so that OpenCode automatically
discovers installed plugin components.

## File Structure

```
internal/opencode/
└── linker.go    # Symlink creation and removal
```

## Linker

```go
type Linker struct {
    opencodeConfig string  // ~/.config/opencode
}

type ComponentCounts struct {
    Skills   int
    Commands int
    Agents   int
}
```

## How It Works

OpenCode automatically discovers files in these directories:
- `~/.config/opencode/skills/` — skill definitions
- `~/.config/opencode/commands/` — slash command definitions
- `~/.config/opencode/agents/` — agent definitions

The linker creates symbolic links from these directories to the plugin cache:

```
~/.config/opencode/skills/
├── code-simplifier → ~/.opencode-plugin-cli/cache/.../code-simplifier/skills/code-simplifier
└── frontend-design → ~/.opencode-plugin-cli/cache/.../frontend-design/skills/frontend-design

~/.config/opencode/agents/
└── code-simplifier → ~/.opencode-plugin-cli/cache/.../code-simplifier/agents/code-simplifier
```

## CreateSymlinks

```
CreateSymlinks(pluginPath string) (*ComponentCounts, error)
│
├── For each component directory (skills, commands, agents):
│   ├── Check if source exists in pluginPath
│   ├── Read entries from source directory
│   └── For each entry:
│       ├── Compute target path: opencodeConfig/<component>/<entry_name>
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
├── For each component directory (skills, commands, agents):
│   ├── Read entries from opencodeConfig/<component>/
│   └── For each entry:
│       ├── Resolve symlink target
│       ├── If target is inside pluginPath → remove symlink
│       └── If not → skip (belongs to another plugin)
│
└── Return count of removed symlinks
```

## DetectOpenCodeConfig

Verifies that `~/.config/opencode/` exists. Creates it if missing.

```go
func (l *Linker) DetectOpenCodeConfig() (string, error)
```

## Directory Scanning

The linker scans plugin directories for these subdirectories:
- `skills/` — scanned recursively, top-level entries become symlinks
- `commands/` — same as skills
- `agents/` — same as skills

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
- `~/.config/opencode/skills/my-skill → .../skills/my-skill/`
- `~/.config/opencode/skills/another-skill.md → .../skills/another-skill.md`
