# Development Guide

## Development Progress

### Phase 1: MVP (Completed) ✅

- [x] Basic CLI framework (using cobra)
- [x] `market add` supports multiple URL formats
- [x] `market list` lists added marketplaces
- [x] `plugin list` lists installed plugins
- [x] Smart URL format recognition
- [x] marketplace.json parsing
- [x] Configuration file management
- [x] Unit test coverage

### Phase 2: Core Features (Completed) ✅

- [x] Git operations module
- [x] Marketplace manager module
- [x] Actual marketplace cloning (Git repository)
- [x] Actual plugin installation logic
- [x] Version resolution (plugin.json + git SHA)
- [x] Symlink creation to OpenCode
- [x] Actual plugin removal logic
- [x] Error handling and user prompts

### Phase 3: Advanced Features (Completed) ✅

- [x] Market update and remove commands
- [x] Plugin update and version management
- [x] Plugin info command
- [x] Available versions listing
- [ ] Project scope support
- [ ] Multi-version coexistence
- [ ] Offline mode
- [ ] Plugin dependency management
- [ ] Configuration file support
- [ ] `doctor` diagnostic command

## Testing

```bash
# Run all unit tests
make test

# Run e2e tests (downloads real plugins)
make test-e2e

# Run tests with coverage
make test-coverage

# Run specific e2e test
go test ./test/e2e -v -run TestE2ECodeSimplifier
```

### E2E Testing

The e2e test downloads and installs the `code-simplifier` plugin from the official Claude plugin marketplace. This test:

- Uses a temporary directory (no side effects on production config)
- Tests the complete workflow: add marketplace → install plugin → verify → remove
- Takes ~15 seconds to run

See [CONFIGURATION.md](CONFIGURATION.md) for details on the test environment system.

## Project Structure

```
opencode-plugin-cli/
├── cmd/                           # CLI commands
│   ├── root.go
│   ├── market.go
│   ├── market_add.go
│   ├── market_update.go
│   ├── plugin.go
│   ├── plugin_install.go
│   ├── plugin_update.go
│   └── plugin_info.go
├── internal/
│   ├── marketplace/               # Marketplace management
│   │   ├── source.go              # URL format recognition
│   │   ├── parser.go              # marketplace.json parsing
│   │   ├── manager.go             # Marketplace CRUD operations
│   │   ├── git.go                 # Git operations
│   │   └── types.go
│   ├── plugin/                    # Plugin management
│   │   ├── installer.go           # Install/remove plugins
│   │   └── version.go             # Version resolution
│   ├── config/                    # Configuration management
│   │   ├── manager.go
│   │   └── types.go
│   └── opencode/                  # OpenCode integration
│       └── linker.go              # Symlink management
├── test/
│   └── fixtures/                  # Test data
├── .docs/plans/                   # Design documents
├── go.mod
├── Makefile
└── README.md
```
