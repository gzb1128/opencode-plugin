# OpenCode Plugin CLI - Configuration Management

## Overview

The CLI uses a modular configuration system that supports different environments (production vs testing).

## Configuration Structure

### Environment

The `Environment` struct defines the runtime environment:

```go
type Environment struct {
    BaseDir        string  // Base directory for CLI data
    OpenCodeConfig string  // OpenCode configuration directory
}
```

### Default Environment

Production environment uses the user's home directory:

```
~/.opencode-plugin-cli/           # Base directory
├── known_marketplaces.json       # Marketplace registry
├── installed_plugins.json        # Installed plugins
├── markets/                      # Cloned marketplaces
└── cache/                        # Plugin cache

~/.config/opencode/               # OpenCode config
├── skills/                       # Symlinks to skills
├── commands/                     # Symlinks to commands
└── agents/                       # Symlinks to agents
```

### Test Environment

For testing, a temporary directory is used:

```go
env := config.TestEnvironment(tempDir)
configMgr, err := config.NewManagerWithEnv(env)
```

## Usage Examples

### Production Usage

```go
// Default environment (uses home directory)
configMgr, err := config.NewManager()
```

### Testing Usage

```go
func TestSomething(t *testing.T) {
    tempDir := t.TempDir()
    env := config.TestEnvironment(tempDir)
    configMgr, err := config.NewManagerWithEnv(env)
    // Use configMgr for testing...
}
```

## E2E Testing

### Test Structure

E2E tests are located in `test/e2e/` directory:

```
test/e2e/
└── code_simplifier_test.go
```

### Running E2E Tests

```bash
# Run all e2e tests
make test-e2e

# Run specific test
go test ./test/e2e -v -run TestE2ECodeSimplifier

# Skip e2e tests in short mode
go test ./test/e2e -v -short
```

### E2E Test Example

```go
func TestE2ECodeSimplifier(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping e2e test in short mode")
    }

    tempDir := t.TempDir()
    env := config.TestEnvironment(tempDir)
    configMgr, _ := config.NewManagerWithEnv(env)

    // Test adding marketplace
    t.Run("AddClaudePluginsOfficial", func(t *testing.T) {
        // Add marketplace...
    })

    // Test installing plugin
    t.Run("InstallCodeSimplifier", func(t *testing.T) {
        // Install plugin...
    })
}
```

## Default Marketplace

When no configuration exists, the CLI automatically initializes with:

```json
{
  "claude-plugins-official": {
    "source": {
      "source": "github",
      "repo": "anthropics/claude-plugins-official"
    },
    "installLocation": "~/.opencode-plugin-cli/markets/claude-plugins-official",
    "lastUpdated": "2026-03-24T14:24:33+08:00"
  }
}
```

This happens in `config/manager.go`:

```go
func NewManager() (*Manager, error) {
    env := DefaultEnvironment()
    return NewManagerWithEnv(env)
}
```

## Configuration Files

### known_marketplaces.json

Stores all added marketplaces:

```json
{
  "marketplace-name": {
    "source": "github",
    "repo": "owner/repo",
    "installLocation": "/path/to/marketplace",
    "lastUpdated": "2026-03-24T14:24:33+08:00"
  }
}
```

### installed_plugins.json

Tracks installed plugins:

```json
{
  "version": 2,
  "plugins": {
    "plugin-name@marketplace": [
      {
        "scope": "user",
        "installPath": "/path/to/cache",
        "version": "1.0.0",
        "installedAt": "2026-03-24T14:24:33Z",
        "lastUpdated": "2026-03-24T14:24:33Z"
      }
    ]
  }
}
```

## Best Practices

1. **Always use TestEnvironment for tests**: Never use the production environment in tests
2. **Use t.TempDir()**: Automatically cleaned up after test
3. **Test isolation**: Each test gets its own environment
4. **No side effects**: Tests don't affect production configuration

## Implementation Details

### File Locations

```go
type Paths struct {
    BaseDir        string  // ~/.opencode-plugin-cli
    MarketsDir     string  // ~/.opencode-plugin-cli/markets
    CacheDir       string  // ~/.opencode-plugin-cli/cache
    KnownMarkets   string  // ~/.opencode-plugin-cli/known_marketplaces.json
    InstalledFile  string  // ~/.opencode-plugin-cli/installed_plugins.json
    OpenCodeConfig string  // ~/.config/opencode
}
```

### Manager Creation

```go
// Production
configMgr, _ := config.NewManager()

// Testing
env := config.TestEnvironment(tempDir)
configMgr, _ := config.NewManagerWithEnv(env)
```

## Migration Guide

If you have code using hardcoded paths, migrate to use the environment system:

**Before:**
```go
homeDir, _ := os.UserHomeDir()
baseDir := filepath.Join(homeDir, ".opencode-plugin-cli")
```

**After:**
```go
env := config.DefaultEnvironment()
baseDir := env.BaseDir
```

For tests:
```go
tempDir := t.TempDir()
env := config.TestEnvironment(tempDir)
baseDir := env.BaseDir
```
