# Plugin Disable/Enable Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `plugin disable` and `plugin enable` commands that temporarily deactivate a plugin's symlinks and MCP servers while preserving its installation record and cache.

**Architecture:** Extend `InstallRecord` with `Disabled`/`DisabledAt` fields. Add `Disable`/`Enable` methods to the installer that manipulate symlinks and MCP `enabled` state. New CLI commands `plugin disable` and `plugin enable` call these methods. Existing `install`, `update`, `remove`, and `list` commands are adjusted to respect disabled state.

**Tech Stack:** Go, Cobra CLI, existing linker + MCP manager

---

### Task 1: Extend InstallRecord with Disabled fields

**Files:**
- Modify: `internal/config/types.go:14-22`
- Test: `internal/config/manager_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/manager_test.go`, after `TestManager_InstalledPlugins`:

```go
func TestInstallRecord_DisabledBackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &Paths{
		BaseDir:       tmpDir,
		MarketsDir:    filepath.Join(tmpDir, "markets"),
		CacheDir:      filepath.Join(tmpDir, "cache"),
		KnownMarkets:  filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	manager := &Manager{paths: paths}

	oldFormat := `{
		"version": 2,
		"plugins": {
			"old-plugin@test-market": [
				{
					"scope": "user",
					"installPath": "/tmp/cache/old-plugin/1.0.0",
					"version": "1.0.0",
					"installedAt": "2026-01-01T00:00:00Z"
				}
			]
		}
	}`

	if err := os.WriteFile(paths.InstalledFile, []byte(oldFormat), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	record, err := manager.GetInstallRecord("old-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord() error = %v", err)
	}

	if record.Disabled {
		t.Error("Expected Disabled to default to false for old-format records")
	}

	if !record.DisabledAt.IsZero() {
		t.Error("Expected DisabledAt to be zero for old-format records")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestInstallRecord_DisabledBackwardCompatibility -v`
Expected: FAIL — `InstallRecord` has no `Disabled` field yet, but JSON unmarshaling won't error; the field simply stays zero-value `false`. This test should actually **pass** once we add the fields because Go's zero-value for `bool` is `false` and `omitempty` skips zero `time.Time`. We write the test first to lock in the contract.

- [ ] **Step 3: Add Disabled and DisabledAt fields to InstallRecord**

In `internal/config/types.go`, change the `InstallRecord` struct:

```go
type InstallRecord struct {
	Scope        string    `json:"scope"`
	ProjectPath  string    `json:"projectPath,omitempty"`
	InstallPath  string    `json:"installPath"`
	Version      string    `json:"version"`
	InstalledAt  time.Time `json:"installedAt"`
	LastUpdated  time.Time `json:"lastUpdated"`
	GitCommitSHA string    `json:"gitCommitSha,omitempty"`
	Disabled     bool      `json:"disabled,omitempty"`
	DisabledAt   time.Time `json:"disabledAt,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestInstallRecord_DisabledBackwardCompatibility -v`
Expected: PASS

- [ ] **Step 5: Run full config test suite**

Run: `go test ./internal/config/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/types.go internal/config/manager_test.go
git commit -m "feat: add Disabled and DisabledAt fields to InstallRecord"
```

---

### Task 2: Add UpdateInstallRecord method to config manager

**Files:**
- Modify: `internal/config/manager.go`
- Test: `internal/config/manager_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/manager_test.go`:

```go
func TestManager_UpdateInstallRecord(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &Paths{
		BaseDir:       tmpDir,
		MarketsDir:    filepath.Join(tmpDir, "markets"),
		CacheDir:      filepath.Join(tmpDir, "cache"),
		KnownMarkets:  filepath.Join(tmpDir, "known_marketplaces.json"),
		InstalledFile: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	manager := &Manager{paths: paths}

	record := &InstallRecord{
		Scope:       "user",
		InstallPath: "/tmp/cache/test-plugin/1.0.0",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
	}

	if err := manager.AddInstallRecord("test-plugin@test-market", record); err != nil {
		t.Fatalf("AddInstallRecord() error = %v", err)
	}

	now := time.Now()
	record.Disabled = true
	record.DisabledAt = now

	if err := manager.UpdateInstallRecord("test-plugin@test-market", record); err != nil {
		t.Fatalf("UpdateInstallRecord() error = %v", err)
	}

	loaded, err := manager.GetInstallRecord("test-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord() error = %v", err)
	}

	if !loaded.Disabled {
		t.Error("Expected Disabled to be true after update")
	}

	if loaded.DisabledAt.IsZero() {
		t.Error("Expected DisabledAt to be set after update")
	}

	record.Disabled = false
	record.DisabledAt = time.Time{}

	if err := manager.UpdateInstallRecord("test-plugin@test-market", record); err != nil {
		t.Fatalf("UpdateInstallRecord() error = %v", err)
	}

	loaded, err = manager.GetInstallRecord("test-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord() error = %v", err)
	}

	if loaded.Disabled {
		t.Error("Expected Disabled to be false after re-enable")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestManager_UpdateInstallRecord -v`
Expected: FAIL — `Manager.UpdateInstallRecord` does not exist

- [ ] **Step 3: Add UpdateInstallRecord method**

In `internal/config/manager.go`, add after `RemoveInstallRecord`:

```go
func (m *Manager) UpdateInstallRecord(key string, record *InstallRecord) error {
	installed, err := m.LoadInstalledPlugins()
	if err != nil {
		return err
	}

	records, ok := installed.Plugins[key]
	if !ok || len(records) == 0 {
		return fmt.Errorf("plugin %s not found", key)
	}

	records[0] = *record
	installed.Plugins[key] = records

	return m.SaveInstalledPlugins(installed)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestManager_UpdateInstallRecord -v`
Expected: PASS

- [ ] **Step 5: Run full config test suite**

Run: `go test ./internal/config/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/manager.go internal/config/manager_test.go
git commit -m "feat: add UpdateInstallRecord method to config manager"
```

---

### Task 3: Add MCP DisableMCPConfig and EnableMCPConfig methods

**Files:**
- Modify: `internal/mcp/manager.go`
- Test: `internal/mcp/manager_test.go`

- [ ] **Step 1: Write the failing test for DisableMCPConfig**

Add to `internal/mcp/manager_test.go`:

```go
func TestDisableMCPConfig(t *testing.T) {
	t.Run("sets enabled=false for plugin-prefixed servers", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		existingContent := `{
			"mcp": {
				"my-plugin.server1": {
					"command": ["node", "s1.js"],
					"enabled": true,
					"type": "local"
				},
				"my-plugin.server2": {
					"command": ["python", "s2.py"],
					"enabled": true,
					"type": "local"
				},
				"other-plugin.server": {
					"command": ["ruby", "s.rb"],
					"enabled": true,
					"type": "local"
				}
			}
		}`
		if err := os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte(existingContent), 0644); err != nil {
			t.Fatalf("Failed to write opencode.json: %v", err)
		}

		if err := mgr.DisableMCPConfig("my-plugin"); err != nil {
			t.Fatalf("DisableMCPConfig failed: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))

		var config map[string]json.RawMessage
		json.Unmarshal(data, &config)

		var mcp map[string]OpenCodeMCPServer
		json.Unmarshal(config["mcp"], &mcp)

		if mcp["my-plugin.server1"].Enabled {
			t.Error("Expected my-plugin.server1 to be disabled")
		}
		if mcp["my-plugin.server2"].Enabled {
			t.Error("Expected my-plugin.server2 to be disabled")
		}
		if !mcp["other-plugin.server"].Enabled {
			t.Error("Expected other-plugin.server to remain enabled")
		}
	})

	t.Run("no-op when no matching servers", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		existingContent := `{
			"mcp": {
				"other.server": {
					"command": ["node", "s.js"],
					"enabled": true,
					"type": "local"
				}
			}
		}`
		os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte(existingContent), 0644)

		if err := mgr.DisableMCPConfig("my-plugin"); err != nil {
			t.Fatalf("DisableMCPConfig failed: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))
		var config map[string]json.RawMessage
		json.Unmarshal(data, &config)
		var mcp map[string]OpenCodeMCPServer
		json.Unmarshal(config["mcp"], &mcp)

		if !mcp["other.server"].Enabled {
			t.Error("Expected other.server to remain enabled")
		}
	})

	t.Run("no-op when config file missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		if err := mgr.DisableMCPConfig("my-plugin"); err != nil {
			t.Errorf("Expected no error for missing config, got: %v", err)
		}
	})
}

func TestEnableMCPConfig(t *testing.T) {
	t.Run("sets enabled=true for plugin-prefixed servers", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		existingContent := `{
			"mcp": {
				"my-plugin.server1": {
					"command": ["node", "s1.js"],
					"enabled": false,
					"type": "local"
				},
				"my-plugin.server2": {
					"command": ["python", "s2.py"],
					"enabled": false,
					"type": "local"
				},
				"other-plugin.server": {
					"command": ["ruby", "s.rb"],
					"enabled": true,
					"type": "local"
				}
			}
		}`
		os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte(existingContent), 0644)

		if err := mgr.EnableMCPConfig("my-plugin"); err != nil {
			t.Fatalf("EnableMCPConfig failed: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))
		var config map[string]json.RawMessage
		json.Unmarshal(data, &config)
		var mcp map[string]OpenCodeMCPServer
		json.Unmarshal(config["mcp"], &mcp)

		if !mcp["my-plugin.server1"].Enabled {
			t.Error("Expected my-plugin.server1 to be enabled")
		}
		if !mcp["my-plugin.server2"].Enabled {
			t.Error("Expected my-plugin.server2 to be enabled")
		}
		if !mcp["other-plugin.server"].Enabled {
			t.Error("Expected other-plugin.server to remain enabled")
		}
	})

	t.Run("preserves user-modified server config when enabling", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		existingContent := `{
			"mcp": {
				"my-plugin.server": {
					"command": ["node", "custom-server.js"],
					"enabled": false,
					"type": "local",
					"environment": {"DEBUG": "true"}
				}
			}
		}`
		os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte(existingContent), 0644)

		mgr.EnableMCPConfig("my-plugin")

		data, _ := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))
		var config map[string]json.RawMessage
		json.Unmarshal(data, &config)
		var mcp map[string]OpenCodeMCPServer
		json.Unmarshal(config["mcp"], &mcp)

		server := mcp["my-plugin.server"]
		if len(server.Command) != 2 || server.Command[0] != "node" || server.Command[1] != "custom-server.js" {
			t.Errorf("Expected command preserved, got %v", server.Command)
		}
		if server.Environment["DEBUG"] != "true" {
			t.Error("Expected environment to be preserved")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run "TestDisableMCPConfig|TestEnableMCPConfig" -v`
Expected: FAIL — `Manager.DisableMCPConfig` and `Manager.EnableMCPConfig` do not exist

- [ ] **Step 3: Add DisableMCPConfig and EnableMCPConfig methods**

In `internal/mcp/manager.go`, add after `UninstallMCPConfig`:

```go
func (m *Manager) DisableMCPConfig(pluginName string) error {
	opencodeConfig, err := m.readOpenCodeConfig()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if opencodeConfig.MCP == nil {
		return nil
	}

	prefix := fmt.Sprintf("%s.", pluginName)
	changed := false
	for name, server := range opencodeConfig.MCP {
		if strings.HasPrefix(name, prefix) && server.Enabled {
			server.Enabled = false
			opencodeConfig.MCP[name] = server
			changed = true
		}
	}

	if !changed {
		return nil
	}

	return m.writeOpenCodeConfig(opencodeConfig)
}

func (m *Manager) EnableMCPConfig(pluginName string) error {
	opencodeConfig, err := m.readOpenCodeConfig()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if opencodeConfig.MCP == nil {
		return nil
	}

	prefix := fmt.Sprintf("%s.", pluginName)
	changed := false
	for name, server := range opencodeConfig.MCP {
		if strings.HasPrefix(name, prefix) && !server.Enabled {
			server.Enabled = true
			opencodeConfig.MCP[name] = server
			changed = true
		}
	}

	if !changed {
		return nil
	}

	return m.writeOpenCodeConfig(opencodeConfig)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/ -run "TestDisableMCPConfig|TestEnableMCPConfig" -v`
Expected: PASS

- [ ] **Step 5: Run full MCP test suite**

Run: `go test ./internal/mcp/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/manager.go internal/mcp/manager_test.go
git commit -m "feat: add DisableMCPConfig and EnableMCPConfig to MCP manager"
```

---

### Task 4: Add Disable and Enable methods to Installer

**Files:**
- Modify: `internal/plugin/installer.go`
- Test: `internal/plugin/installer_test.go`

- [ ] **Step 1: Write the failing test for Disable**

Add to `internal/plugin/installer_test.go`:

```go
func setupInstalledPlugin(t *testing.T, installer *Installer, pluginName, marketName string) string {
	t.Helper()
	paths := installer.configMgr.GetPaths()

	cachePath := filepath.Join(paths.CacheDir, marketName, pluginName, "1.0.0")
	skillsDir := filepath.Join(cachePath, "skills")
	os.MkdirAll(skillsDir, 0755)
	os.WriteFile(filepath.Join(skillsDir, "test-skill.md"), []byte("# Test Skill"), 0644)

	manifestDir := filepath.Join(cachePath, ".claude-plugin")
	os.MkdirAll(manifestDir, 0755)
	os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"`+pluginName+`","version":"1.0.0","skills":["./skills"]}`), 0644)

	key := fmt.Sprintf("%s@%s", pluginName, marketName)
	record := &config.InstallRecord{
		Scope:       "user",
		InstallPath: cachePath,
		Version:     "1.0.0",
		InstalledAt: time.Now(),
	}
	installer.configMgr.AddInstallRecord(key, record)

	manifest, _ := opencode.ReadManifest(filepath.Join(cachePath, ".claude-plugin", "plugin.json"))
	installer.linker.CreateSymlinksFromManifest(cachePath, manifest, false)

	return cachePath
}

func TestInstaller_Disable(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	cachePath := setupInstalledPlugin(t, installer, "test-plugin", "test-market")

	symlinkPath := filepath.Join(installer.configMgr.GetPaths().AgentsDir, "skills", "test-skill.md")
	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		t.Fatalf("symlink should exist before disable")
	}

	if err := installer.Disable("test-plugin", "test-market"); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("symlink should be removed after disable")
	}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("cache directory should still exist after disable")
	}

	record, err := installer.configMgr.GetInstallRecord("test-plugin@test-market")
	if err != nil {
		t.Fatalf("install record should still exist: %v", err)
	}
	if !record.Disabled {
		t.Error("record should be marked as disabled")
	}

	err = installer.Disable("test-plugin", "test-market")
	if err != nil {
		t.Fatalf("double disable should not error: %v", err)
	}
}

func TestInstaller_Enable(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	setupInstalledPlugin(t, installer, "test-plugin", "test-market")

	if err := installer.Disable("test-plugin", "test-market"); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	symlinkPath := filepath.Join(installer.configMgr.GetPaths().AgentsDir, "skills", "test-skill.md")
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("symlink should not exist after disable")
	}

	if err := installer.Enable("test-plugin", "test-market", false); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}

	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		t.Error("symlink should exist after enable")
	}

	record, err := installer.configMgr.GetInstallRecord("test-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord() error = %v", err)
	}
	if record.Disabled {
		t.Error("record should not be disabled after enable")
	}

	err = installer.Enable("test-plugin", "test-market", false)
	if err != nil {
		t.Fatalf("double enable should not error: %v", err)
	}
}

func TestInstaller_Disable_NotInstalled(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	err := installer.Disable("nonexistent", "market")
	if err == nil {
		t.Error("expected error when disabling non-installed plugin")
	}
}

func TestInstaller_Enable_NotInstalled(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	err := installer.Enable("nonexistent", "market", false)
	if err == nil {
		t.Error("expected error when enabling non-installed plugin")
	}
}

func TestInstaller_Enable_CacheMissing(t *testing.T) {
	installer, _ := setupInstallerTest(t)

	key := "missing@test-market"
	record := &config.InstallRecord{
		Scope:       "user",
		InstallPath: "/nonexistent/path",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		Disabled:    true,
	}
	installer.configMgr.AddInstallRecord(key, record)

	err := installer.Enable("missing", "test-market", false)
	if err == nil {
		t.Error("expected error when enabling plugin with missing cache")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugin/ -run "TestInstaller_Disable|TestInstaller_Enable" -v`
Expected: FAIL — `Installer.Disable` and `Installer.Enable` do not exist

- [ ] **Step 3: Add Disable and Enable methods to Installer**

In `internal/plugin/installer.go`, add after the `Remove` method:

```go
func (i *Installer) Disable(pluginName, marketName string) error {
	key := fmt.Sprintf("%s@%s", pluginName, marketName)
	record, err := i.configMgr.GetInstallRecord(key)
	if err != nil {
		return fmt.Errorf("plugin %s not found", key)
	}

	if record.Disabled {
		fmt.Printf("Plugin %s is already disabled\n", key)
		return nil
	}

	installPath := record.InstallPath
	cacheDir := i.configMgr.GetPaths().CacheDir
	if !isWithinDir(installPath, cacheDir) {
		return fmt.Errorf("refusing to disable path %q outside cache directory %q", installPath, cacheDir)
	}

	count, err := i.linker.RemoveSymlinks(installPath)
	if err != nil {
		fmt.Printf("⚠️  Error removing symlinks: %v\n", err)
	}

	if err := i.mcpManager.DisableMCPConfig(pluginName); err != nil {
		fmt.Printf("⚠️  Warning: Failed to disable MCP servers: %v\n", err)
	}

	record.Disabled = true
	record.DisabledAt = time.Now()

	if err := i.configMgr.UpdateInstallRecord(key, record); err != nil {
		return fmt.Errorf("failed to update installation record: %w", err)
	}

	fmt.Printf("✓ Disabled plugin: %s (%d symlinks removed)\n", key, count)

	return nil
}

func (i *Installer) Enable(pluginName, marketName string, force bool) error {
	key := fmt.Sprintf("%s@%s", pluginName, marketName)
	record, err := i.configMgr.GetInstallRecord(key)
	if err != nil {
		return fmt.Errorf("plugin %s not found", key)
	}

	if !record.Disabled {
		fmt.Printf("Plugin %s is already enabled\n", key)
		return nil
	}

	installPath := record.InstallPath
	cacheDir := i.configMgr.GetPaths().CacheDir
	if !isWithinDir(installPath, cacheDir) {
		return fmt.Errorf("refusing to enable path %q outside cache directory %q", installPath, cacheDir)
	}

	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin cache not found at %s, run 'opencode-plugin plugin update %s' or reinstall", installPath, key)
	}

	manifestPath := filepath.Join(installPath, ".claude-plugin", "plugin.json")
	var manifest map[string]interface{}
	if _, err := os.Stat(manifestPath); err == nil {
		manifest, _ = opencode.ReadManifest(manifestPath)
	}

	counts, err := i.linker.CreateSymlinksFromManifest(installPath, manifest, force)
	if err != nil {
		return fmt.Errorf("failed to create symlinks: %w", err)
	}

	if err := i.mcpManager.EnableMCPConfig(pluginName); err != nil {
		fmt.Printf("⚠️  Warning: Failed to enable MCP servers: %v\n", err)
	}

	record.Disabled = false
	record.DisabledAt = time.Time{}

	if err := i.configMgr.UpdateInstallRecord(key, record); err != nil {
		return fmt.Errorf("failed to update installation record: %w", err)
	}

	fmt.Printf("✓ Enabled plugin: %s\n", key)
	if counts.Skills > 0 {
		fmt.Printf("  Skills: %d\n", counts.Skills)
	}
	if counts.Commands > 0 {
		fmt.Printf("  Commands: %d\n", counts.Commands)
	}
	if counts.Agents > 0 {
		fmt.Printf("  Agents: %d\n", counts.Agents)
	}

	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugin/ -run "TestInstaller_Disable|TestInstaller_Enable" -v`
Expected: PASS

- [ ] **Step 5: Run full plugin test suite**

Run: `go test ./internal/plugin/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/plugin/installer.go internal/plugin/installer_test.go
git commit -m "feat: add Disable and Enable methods to Installer"
```

---

### Task 5: Add plugin disable and enable CLI commands

**Files:**
- Create: `cmd/plugin/plugin_disable.go`
- Create: `cmd/plugin/plugin_enable.go`
- Modify: `cmd/plugin/plugin.go`

- [ ] **Step 1: Create plugin_disable.go**

Create `cmd/plugin/plugin_disable.go`:

```go
package plugin

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/plugin"
	"github.com/spf13/cobra"
)

var disableCmd = &cobra.Command{
	Use:   "disable <plugin-name>[@<marketplace>]",
	Short: "Disable an installed plugin",
	Long: `Disable an installed plugin without removing it.

The plugin's symlinks and MCP servers will be deactivated, but the
installation record and cached files are preserved. Use 'plugin enable'
to reactivate the plugin.

Examples:
  opencode-plugin plugin disable superpowers
  opencode-plugin plugin disable superpowers@official`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pluginSpec := args[0]

		var pluginName, marketName string
		if idx := strings.Index(pluginSpec, "@"); idx > 0 {
			pluginName = pluginSpec[:idx]
			marketName = pluginSpec[idx+1:]
		} else {
			pluginName = pluginSpec
		}

		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		installer := plugin.NewInstaller(configMgr)

		if marketName == "" {
			installed, err := installer.List()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to list installed plugins: %v\n", err)
				os.Exit(1)
			}

			var matches []string
			for key := range installed {
				if strings.HasPrefix(key, pluginName+"@") {
					matches = append(matches, key)
				}
			}

			if len(matches) > 1 {
				fmt.Fprintf(os.Stderr, "Error: Multiple installations of %s found:\n", pluginName)
				for _, match := range matches {
					fmt.Fprintf(os.Stderr, "  - %s\n", match)
				}
				fmt.Fprintf(os.Stderr, "\nPlease specify which one to disable:\n")
				fmt.Fprintf(os.Stderr, "  opencode-plugin plugin disable %s\n", matches[0])
				os.Exit(1)
			}

			if len(matches) == 1 {
				key := matches[0]
				marketName = strings.TrimPrefix(key, pluginName+"@")
			}
		}

		if marketName == "" {
			fmt.Fprintf(os.Stderr, "Error: Plugin %s not found in installed list\n", pluginName)
			os.Exit(1)
		}

		if err := installer.Disable(pluginName, marketName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}
```

- [ ] **Step 2: Create plugin_enable.go**

Create `cmd/plugin/plugin_enable.go`:

```go
package plugin

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/plugin"
	"github.com/spf13/cobra"
)

var enableForce bool

var enableCmd = &cobra.Command{
	Use:   "enable <plugin-name>[@<marketplace>]",
	Short: "Enable a disabled plugin",
	Long: `Re-enable a previously disabled plugin.

Restores the plugin's symlinks and MCP servers from the cached files.

Examples:
  opencode-plugin plugin enable superpowers
  opencode-plugin plugin enable superpowers@official
  opencode-plugin plugin enable superpowers --force`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pluginSpec := args[0]
		force, _ := cmd.Flags().GetBool("force")

		var pluginName, marketName string
		if idx := strings.Index(pluginSpec, "@"); idx > 0 {
			pluginName = pluginSpec[:idx]
			marketName = pluginSpec[idx+1:]
		} else {
			pluginName = pluginSpec
		}

		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		installer := plugin.NewInstaller(configMgr)

		if marketName == "" {
			installed, err := installer.List()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to list installed plugins: %v\n", err)
				os.Exit(1)
			}

			var matches []string
			for key := range installed {
				if strings.HasPrefix(key, pluginName+"@") {
					matches = append(matches, key)
				}
			}

			if len(matches) > 1 {
				fmt.Fprintf(os.Stderr, "Error: Multiple installations of %s found:\n", pluginName)
				for _, match := range matches {
					fmt.Fprintf(os.Stderr, "  - %s\n", match)
				}
				fmt.Fprintf(os.Stderr, "\nPlease specify which one to enable:\n")
				fmt.Fprintf(os.Stderr, "  opencode-plugin plugin enable %s\n", matches[0])
				os.Exit(1)
			}

			if len(matches) == 1 {
				key := matches[0]
				marketName = strings.TrimPrefix(key, pluginName+"@")
			}
		}

		if marketName == "" {
			fmt.Fprintf(os.Stderr, "Error: Plugin %s not found in installed list\n", pluginName)
			os.Exit(1)
		}

		if err := installer.Enable(pluginName, marketName, force); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}
```

- [ ] **Step 3: Register new commands in plugin.go init()**

In `cmd/plugin/plugin.go`, update init:

```go
func init() {
	installCmd.Flags().StringP("version", "v", "", "Plugin version to install")
	installCmd.Flags().BoolP("force", "f", false, "Force overwrite existing skills, commands, and agents")
	listCmd.Flags().BoolVar(&listJSONFlag, "json", false, "Output as JSON")
	enableCmd.Flags().BoolP("force", "f", false, "Force overwrite existing skills, commands, and agents")

	Cmd.AddCommand(installCmd)
	Cmd.AddCommand(removeCmd)
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(disableCmd)
	Cmd.AddCommand(enableCmd)
}
```

Note: `updateCmd` is already registered in `plugin_update.go` init(). Remove the duplicate registration if present. Check that `Cmd.AddCommand(updateCmd)` is not in both files.

- [ ] **Step 4: Build and verify commands appear**

Run: `go build -o /dev/null ./...`
Expected: Build succeeds

Run: `go run . plugin --help`
Expected: Output lists `disable` and `enable` subcommands

- [ ] **Step 5: Commit**

```bash
git add cmd/plugin/plugin_disable.go cmd/plugin/plugin_enable.go cmd/plugin/plugin.go
git commit -m "feat: add plugin disable and enable CLI commands"
```

---

### Task 6: Update plugin list to show disabled status

**Files:**
- Modify: `cmd/plugin/plugin_install.go:125-221`
- Test: `cmd/plugin/list_test.go`

- [ ] **Step 1: Read current list test**

Read `cmd/plugin/list_test.go` to understand existing test patterns.

- [ ] **Step 2: Add Status and Disabled fields to pluginJSONEntry**

In `cmd/plugin/plugin_install.go`, update `pluginJSONEntry`:

```go
type pluginJSONEntry struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Marketplace string `json:"marketplace"`
	Version     string `json:"version"`
	InstallPath string `json:"installPath"`
	InstalledAt string `json:"installedAt"`
	Status      string `json:"status"`
	Disabled    bool   `json:"disabled"`
}
```

- [ ] **Step 3: Update text list output to show status**

In the `listCmd` Run function, update the output loop:

```go
for key, records := range installed {
	if len(records) == 0 {
		continue
	}
	record := records[0]
	status := "enabled"
	if record.Disabled {
		status = "disabled"
	}
	fmt.Printf("  %s\n", key)
	fmt.Printf("    Version: %s\n", record.Version)
	fmt.Printf("    Status: %s\n", status)
	fmt.Printf("    Scope: %s\n", record.Scope)
	fmt.Printf("    Path: %s\n", record.InstallPath)
	fmt.Printf("    Installed: %s\n", record.InstalledAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
}
```

- [ ] **Step 4: Update JSON output to include status fields**

In `printPluginsJSON`, update the entry construction:

```go
entry := pluginJSONEntry{
	Key:         key,
	Name:        name,
	Marketplace: market,
	Version:     record.Version,
	InstallPath: record.InstallPath,
	InstalledAt: record.InstalledAt.Format("2006-01-02T15:04:05Z07:00"),
	Status:      "enabled",
	Disabled:    record.Disabled,
}
if record.Disabled {
	entry.Status = "disabled"
}
```

- [ ] **Step 5: Run build and tests**

Run: `go build -o /dev/null ./... && go test ./cmd/plugin/ -v`
Expected: Build and tests pass

- [ ] **Step 6: Commit**

```bash
git add cmd/plugin/plugin_install.go
git commit -m "feat: show enabled/disabled status in plugin list"
```

---

### Task 7: Handle install on disabled plugin (enable instead)

**Files:**
- Modify: `cmd/plugin/plugin_install.go:15-58`

- [ ] **Step 1: Add disabled-plugin detection before install**

In the `installCmd` Run function, after creating the installer, add a check before calling `installer.Install`:

```go
installer := plugin.NewInstaller(configMgr)

installed, err := installer.List()
if err != nil {
	fmt.Fprintf(os.Stderr, "Error: Failed to list installed plugins: %v\n", err)
	os.Exit(1)
}

existingKey := ""
for key, records := range installed {
	if strings.HasPrefix(key, pluginName+"@") && len(records) > 0 && records[0].Disabled {
		existingKey = key
		break
	}
}

if existingKey != "" {
	parts := strings.SplitN(existingKey, "@", 2)
	if len(parts) == 2 {
		marketName = parts[1]
	}
	fmt.Printf("Plugin %s is installed but disabled. Enabling...\n", existingKey)
	if err := installer.Enable(pluginName, marketName, force); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return
}
```

Place this block after the `pluginName`/`marketName` parsing and before `opts := plugin.InstallOptions{...}`.

- [ ] **Step 2: Build and verify**

Run: `go build -o /dev/null ./...`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add cmd/plugin/plugin_install.go
git commit -m "feat: enable disabled plugin on install instead of re-downloading"
```

---

### Task 8: Preserve disabled state during plugin update

**Files:**
- Modify: `cmd/plugin/plugin_update.go:112-140`

- [ ] **Step 1: Save and restore disabled state in updatePlugin**

In `cmd/plugin/plugin_update.go`, update `updatePlugin` to capture disabled state before remove and restore after install:

```go
func updatePlugin(installer *plugin.Installer, configMgr *config.Manager, pluginName, marketName string) error {
	key := fmt.Sprintf("%s@%s", pluginName, marketName)

	wasDisabled := false
	if record, err := configMgr.GetInstallRecord(key); err == nil {
		wasDisabled = record.Disabled
	}

	if err := installer.Remove(pluginName, marketName); err != nil {
		return fmt.Errorf("failed to remove old version: %w", err)
	}

	opts := plugin.InstallOptions{
		MarketName: marketName,
		Version:    "",
		Scope:      "user",
		Force:      forceUpdate,
	}

	if err := installer.Install(pluginName, opts); err != nil {
		return fmt.Errorf("failed to install new version: %w", err)
	}

	if wasDisabled {
		if err := installer.Disable(pluginName, marketName); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to re-disable plugin after update: %v\n", err)
		}
	}

	record, err := configMgr.GetInstallRecord(key)
	if err != nil {
		return nil
	}

	if err := installer.CleanupOldVersions(record.InstallPath); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Warning: cache cleanup failed: %v\n", err)
	}

	return nil
}
```

- [ ] **Step 2: Build and verify**

Run: `go build -o /dev/null ./...`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add cmd/plugin/plugin_update.go
git commit -m "feat: preserve disabled state across plugin update"
```

---

### Task 9: Run full test suite and verify

**Files:** None

- [ ] **Step 1: Run all tests**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 2: Build the binary**

Run: `make build`
Expected: Binary built to `bin/opencode-plugin`

- [ ] **Step 3: Verify help output**

Run: `./bin/opencode-plugin plugin --help`
Expected: Lists install, remove, list, update, disable, enable, search, info

- [ ] **Step 4: Final commit (if any fixups needed)**

Only if any adjustments were made during verification.
