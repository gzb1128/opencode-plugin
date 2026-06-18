package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadMCPConfig(t *testing.T) {
	t.Run("reads valid .mcp.json file with wrapped format", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		mcpContent := `{
			"mcpServers": {
				"test-server": {
					"command": "node",
					"args": ["server.js"],
					"env": {
						"DEBUG": "true"
					}
				}
			}
		}`

		mcpPath := filepath.Join(tmpDir, ".mcp.json")
		if err := os.WriteFile(mcpPath, []byte(mcpContent), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		config, err := mgr.ReadMCPConfig(tmpDir)
		if err != nil {
			t.Fatalf("ReadMCPConfig failed: %v", err)
		}

		if config == nil {
			t.Fatal("Expected config, got nil")
		}

		if len(config.Servers) != 1 {
			t.Errorf("Expected 1 server, got %d", len(config.Servers))
		}

		server, ok := config.Servers["test-server"]
		if !ok {
			t.Fatal("Expected test-server to exist")
		}

		if server.Command != "node" {
			t.Errorf("Expected command 'node', got '%s'", server.Command)
		}

		if len(server.Args) != 1 || server.Args[0] != "server.js" {
			t.Errorf("Expected args ['server.js'], got %v", server.Args)
		}

		if server.Env["DEBUG"] != "true" {
			t.Errorf("Expected env DEBUG='true', got '%s'", server.Env["DEBUG"])
		}
	})

	t.Run("reads valid .mcp.json file with direct format (Claude Code standard)", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		mcpContent := `{
			"github": {
				"type": "http",
				"url": "https://api.githubcopilot.com/mcp/",
				"headers": {
					"Authorization": "Bearer ${GITHUB_TOKEN}"
				}
			},
			"playwright": {
				"command": "npx",
				"args": ["@playwright/mcp@latest"]
			}
		}`

		mcpPath := filepath.Join(tmpDir, ".mcp.json")
		if err := os.WriteFile(mcpPath, []byte(mcpContent), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		config, err := mgr.ReadMCPConfig(tmpDir)
		if err != nil {
			t.Fatalf("ReadMCPConfig failed: %v", err)
		}

		if config == nil {
			t.Fatal("Expected config, got nil")
		}

		if len(config.Servers) != 2 {
			t.Errorf("Expected 2 servers, got %d", len(config.Servers))
		}

		github, ok := config.Servers["github"]
		if !ok {
			t.Fatal("Expected github server to exist")
		}

		if github.Type != "http" {
			t.Errorf("Expected type 'http', got '%s'", github.Type)
		}

		if github.URL != "https://api.githubcopilot.com/mcp/" {
			t.Errorf("Expected URL, got '%s'", github.URL)
		}

		if github.Headers == nil {
			t.Fatal("Expected headers to exist")
		}

		if github.Headers["Authorization"] != "Bearer ${GITHUB_TOKEN}" {
			t.Errorf("Expected Authorization header, got '%s'", github.Headers["Authorization"])
		}

		playwright, ok := config.Servers["playwright"]
		if !ok {
			t.Fatal("Expected playwright server to exist")
		}

		if playwright.Command != "npx" {
			t.Errorf("Expected command 'npx', got '%s'", playwright.Command)
		}
	})

	t.Run("returns nil when .mcp.json does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		config, err := mgr.ReadMCPConfig(tmpDir)
		if err != nil {
			t.Fatalf("ReadMCPConfig failed: %v", err)
		}

		if config != nil {
			t.Errorf("Expected nil config, got %+v", config)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		mcpPath := filepath.Join(tmpDir, ".mcp.json")
		if err := os.WriteFile(mcpPath, []byte("invalid json"), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		_, err := mgr.ReadMCPConfig(tmpDir)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})
}

func TestReadPluginJSON(t *testing.T) {
	t.Run("reads valid plugin.json with mcpServers", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		pluginDir := filepath.Join(tmpDir, ".claude-plugin")
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatalf("Failed to create plugin dir: %v", err)
		}

		pluginContent := `{
			"name": "test-plugin",
			"version": "1.0.0",
			"mcpServers": {
				"inline-server": {
					"type": "http",
					"url": "https://api.example.com/mcp"
				}
			}
		}`

		pluginPath := filepath.Join(pluginDir, "plugin.json")
		if err := os.WriteFile(pluginPath, []byte(pluginContent), 0644); err != nil {
			t.Fatalf("Failed to write plugin.json: %v", err)
		}

		plugin, err := mgr.ReadPluginJSON(tmpDir)
		if err != nil {
			t.Fatalf("ReadPluginJSON failed: %v", err)
		}

		if plugin.Name != "test-plugin" {
			t.Errorf("Expected name 'test-plugin', got '%s'", plugin.Name)
		}

		if len(plugin.MCPServers) != 1 {
			t.Errorf("Expected 1 MCP server, got %d", len(plugin.MCPServers))
		}

		server, ok := plugin.MCPServers["inline-server"]
		if !ok {
			t.Fatal("Expected inline-server to exist")
		}

		if server.Type != "http" {
			t.Errorf("Expected type 'http', got '%s'", server.Type)
		}
	})
}

func TestGetMCPServers(t *testing.T) {
	t.Run("merges servers from .mcp.json and plugin.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		mcpContent := `{
			"mcpServers": {
				"server-from-mcp": {
					"command": "node",
					"args": ["mcp-server.js"]
				}
			}
		}`
		if err := os.WriteFile(filepath.Join(tmpDir, ".mcp.json"), []byte(mcpContent), 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
		}

		pluginDir := filepath.Join(tmpDir, ".claude-plugin")
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatalf("Failed to create plugin dir: %v", err)
		}

		pluginContent := `{
			"name": "test-plugin",
			"mcpServers": {
				"server-from-plugin": {
					"type": "http",
					"url": "https://api.example.com"
				}
			}
		}`
		if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginContent), 0644); err != nil {
			t.Fatalf("Failed to write plugin.json: %v", err)
		}

		servers, err := mgr.GetMCPServers(tmpDir)
		if err != nil {
			t.Fatalf("GetMCPServers failed: %v", err)
		}

		if len(servers) != 2 {
			t.Errorf("Expected 2 servers, got %d", len(servers))
		}

		if _, ok := servers["server-from-mcp"]; !ok {
			t.Error("Expected server-from-mcp to exist")
		}

		if _, ok := servers["server-from-plugin"]; !ok {
			t.Error("Expected server-from-plugin to exist")
		}
	})

	t.Run("returns empty map when no MCP servers", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		pluginDir := filepath.Join(tmpDir, ".claude-plugin")
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatalf("Failed to create plugin dir: %v", err)
		}

		pluginContent := `{"name": "test-plugin"}`
		if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginContent), 0644); err != nil {
			t.Fatalf("Failed to write plugin.json: %v", err)
		}

		servers, err := mgr.GetMCPServers(tmpDir)
		if err != nil {
			t.Fatalf("GetMCPServers failed: %v", err)
		}

		if len(servers) != 0 {
			t.Errorf("Expected 0 servers, got %d", len(servers))
		}
	})

	t.Run("reads MCP servers from .mcp.json without plugin.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		mcpContent := `{
			"github": {
				"type": "http",
				"url": "https://api.github.com/mcp"
			}
		}`
		if err := os.WriteFile(filepath.Join(tmpDir, ".mcp.json"), []byte(mcpContent), 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
		}

		servers, err := mgr.GetMCPServers(tmpDir)
		if err != nil {
			t.Fatalf("GetMCPServers failed: %v", err)
		}

		if len(servers) != 1 {
			t.Errorf("Expected 1 server, got %d", len(servers))
		}

		if _, ok := servers["github"]; !ok {
			t.Error("Expected github server to exist")
		}
	})
}

func TestInstallMCPConfig(t *testing.T) {
	t.Run("preserves unknown fields on existing servers (regression for opencode.json field stripping)", func(t *testing.T) {
		// 之前 InstallMCPConfig 会把 opencode.json 的 mcp 块 round-trip 进
		// map[string]OpenCodeMCPServer，导致任何不在该 struct 里的字段
		// （如 disabledReason、tools、自定义 transport 字段）被静默删除。
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		// 预先存在的 opencode.json 带有未知字段。
		existing := `{
			"model": "gpt-x",
			"mcp": {
				"user handwritten": {
					"type": "local",
					"command": ["node"],
					"enabled": true,
					"disabledReason": "manual pause",
					"tools": ["fs", "shell"],
					"customTransport": "webrtc"
				}
			}
		}`
		configPath := filepath.Join(tmpDir, "opencode.json")
		if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
			t.Fatalf("seed opencode.json: %v", err)
		}

		// 准备 plugin：提供一个新的 MCP server。
		pluginDir := filepath.Join(tmpDir, "my-plugin")
		pluginJSONDir := filepath.Join(pluginDir, ".claude-plugin")
		if err := os.MkdirAll(pluginJSONDir, 0755); err != nil {
			t.Fatalf("create plugin dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(pluginJSONDir, "plugin.json"), []byte(`{"name":"my-plugin","version":"1.0.0"}`), 0644); err != nil {
			t.Fatalf("write plugin.json: %v", err)
		}
		mcpContent := `{"srv":{"command":"npx","args":["foo"]}}`
		if err := os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(mcpContent), 0644); err != nil {
			t.Fatalf("write .mcp.json: %v", err)
		}

		if err := mgr.InstallMCPConfig(pluginDir, "my-plugin", "test-market"); err != nil {
			t.Fatalf("InstallMCPConfig failed: %v", err)
		}

		after, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read back opencode.json: %v", err)
		}
		// 比较时压缩空白，避免和 MarshalIndent 的换行细节耦合。
		flatten := func(s string) string { return strings.Join(strings.Fields(s), " ") }
		flat := flatten(string(after))

		// 顶层 model key 必须保留。
		if !strings.Contains(flat, `"model": "gpt-x"`) {
			t.Errorf("top-level 'model' key was wiped.\n got: %s", string(after))
		}
		// 已有 server 的未知字段必须保留。
		for _, want := range []string{
			`"disabledReason": "manual pause"`,
			`"tools": [ "fs", "shell" ]`,
			`"customTransport": "webrtc"`,
			`"user handwritten"`,
			`"my-plugin.srv"`,
		} {
			if !strings.Contains(flat, want) {
				t.Errorf("expected %s to be preserved in opencode.json.\n got: %s", want, string(after))
			}
		}
	})

	t.Run("installs stdio server to opencode.json with correct format", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		pluginDir := filepath.Join(tmpDir, "my-plugin")
		pluginJSONDir := filepath.Join(pluginDir, ".claude-plugin")
		if err := os.MkdirAll(pluginJSONDir, 0755); err != nil {
			t.Fatalf("Failed to create plugin dir: %v", err)
		}

		pluginContent := `{"name": "my-plugin", "version": "2.0.0"}`
		if err := os.WriteFile(filepath.Join(pluginJSONDir, "plugin.json"), []byte(pluginContent), 0644); err != nil {
			t.Fatalf("Failed to write plugin.json: %v", err)
		}

		mcpContent := `{
			"my-server": {
				"command": "npx",
				"args": ["chrome-devtools-mcp@latest"]
			}
		}`
		if err := os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(mcpContent), 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
		}

		if err := mgr.InstallMCPConfig(pluginDir, "my-plugin", "test-market"); err != nil {
			t.Fatalf("InstallMCPConfig failed: %v", err)
		}

		configPath := filepath.Join(tmpDir, "opencode.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read opencode.json: %v", err)
		}

		var config map[string]json.RawMessage
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("Failed to parse opencode.json: %v", err)
		}

		if _, ok := config["mcp"]; !ok {
			t.Fatal("Expected 'mcp' key in opencode.json")
		}

		var mcp map[string]OpenCodeMCPServer
		if err := json.Unmarshal(config["mcp"], &mcp); err != nil {
			t.Fatalf("Failed to parse mcp section: %v", err)
		}

		server, ok := mcp["my-plugin.my-server"]
		if !ok {
			t.Fatal("Expected my-plugin.my-server to exist")
		}

		if server.Type != "local" {
			t.Errorf("Expected type 'local', got '%s'", server.Type)
		}

		if !server.Enabled {
			t.Error("Expected enabled to be true")
		}

		if len(server.Command) != 2 || server.Command[0] != "npx" || server.Command[1] != "chrome-devtools-mcp@latest" {
			t.Errorf("Expected command ['npx', 'chrome-devtools-mcp@latest'], got %v", server.Command)
		}
	})

	t.Run("installs http server as remote type", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		pluginDir := filepath.Join(tmpDir, "gh-plugin")
		os.MkdirAll(pluginDir, 0755)
		if err := os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(`{
			"github": {
				"type": "http",
				"url": "https://api.githubcopilot.com/mcp/",
				"headers": {
					"Authorization": "Bearer ${GITHUB_TOKEN}"
				}
			}
		}`), 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
		}

		if err := mgr.InstallMCPConfig(pluginDir, "gh-plugin", "test-market"); err != nil {
			t.Fatalf("InstallMCPConfig failed: %v", err)
		}

		configPath := filepath.Join(tmpDir, "opencode.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read opencode.json: %v", err)
		}

		var config map[string]json.RawMessage
		json.Unmarshal(data, &config)

		var mcp map[string]OpenCodeMCPServer
		json.Unmarshal(config["mcp"], &mcp)

		server := mcp["gh-plugin.github"]
		if server.Type != "remote" {
			t.Errorf("Expected type 'remote', got '%s'", server.Type)
		}
		if server.URL != "https://api.githubcopilot.com/mcp/" {
			t.Errorf("Expected URL, got '%s'", server.URL)
		}
		if !server.Enabled {
			t.Error("Expected enabled to be true")
		}
		if server.Headers["Authorization"] != "Bearer ${GITHUB_TOKEN}" {
			t.Errorf("Expected Authorization header, got '%s'", server.Headers["Authorization"])
		}
	})

	t.Run("substitutes variables in command and args", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		pluginDir := filepath.Join(tmpDir, "my-plugin")
		pluginJSONDir := filepath.Join(pluginDir, ".claude-plugin")
		os.MkdirAll(pluginJSONDir, 0755)

		os.WriteFile(filepath.Join(pluginJSONDir, "plugin.json"), []byte(`{"name": "my-plugin", "version": "2.0.0"}`), 0644)
		os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(`{
			"my-server": {
				"command": "${CLAUDE_PLUGIN_ROOT}/bin/server",
				"args": ["--name", "${PLUGIN_NAME}", "--version", "${PLUGIN_VERSION}"],
				"env": {
					"PLUGIN_ROOT": "${CLAUDE_PLUGIN_ROOT}",
					"PLUGIN_NAME": "${PLUGIN_NAME}"
				}
			}
		}`), 0644)

		if err := mgr.InstallMCPConfig(pluginDir, "my-plugin", "test-market"); err != nil {
			t.Fatalf("InstallMCPConfig failed: %v", err)
		}

		configPath := filepath.Join(tmpDir, "opencode.json")
		data, _ := os.ReadFile(configPath)

		var config map[string]json.RawMessage
		json.Unmarshal(data, &config)

		var mcp map[string]OpenCodeMCPServer
		json.Unmarshal(config["mcp"], &mcp)

		server := mcp["my-plugin.my-server"]
		expectedCmd := filepath.Join(pluginDir, "bin/server")
		if server.Command[0] != expectedCmd {
			t.Errorf("Expected command[0] '%s', got '%s'", expectedCmd, server.Command[0])
		}
		if server.Command[2] != "my-plugin" {
			t.Errorf("Expected command[2] 'my-plugin', got '%s'", server.Command[2])
		}
		if server.Command[4] != "2.0.0" {
			t.Errorf("Expected command[4] '2.0.0', got '%s'", server.Command[4])
		}
		if server.Environment["PLUGIN_ROOT"] != pluginDir {
			t.Errorf("Expected env PLUGIN_ROOT '%s', got '%s'", pluginDir, server.Environment["PLUGIN_ROOT"])
		}
		if server.Environment["PLUGIN_NAME"] != "my-plugin" {
			t.Errorf("Expected env PLUGIN_NAME 'my-plugin', got '%s'", server.Environment["PLUGIN_NAME"])
		}
	})

	t.Run("merges with existing opencode.json preserving other keys", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		existingContent := `{
			"$schema": "https://opencode.ai/config.json",
			"model": "test-model",
			"mcp": {
				"context7": {
					"command": ["npx", "-y", "@context7/mcp-server"],
					"enabled": true,
					"type": "local"
				}
			}
		}`
		if err := os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte(existingContent), 0644); err != nil {
			t.Fatalf("Failed to write existing opencode.json: %v", err)
		}

		pluginDir := filepath.Join(tmpDir, "new-plugin")
		os.MkdirAll(pluginDir, 0755)
		if err := os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(`{
			"new-server": {
				"command": "node",
				"args": ["server.js"]
			}
		}`), 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
		}

		if err := mgr.InstallMCPConfig(pluginDir, "new-plugin", "test-market"); err != nil {
			t.Fatalf("InstallMCPConfig failed: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))

		var config map[string]json.RawMessage
		json.Unmarshal(data, &config)

		if _, ok := config["$schema"]; !ok {
			t.Error("Expected $schema to be preserved")
		}
		if _, ok := config["model"]; !ok {
			t.Error("Expected model to be preserved")
		}

		var mcp map[string]OpenCodeMCPServer
		json.Unmarshal(config["mcp"], &mcp)

		if len(mcp) != 2 {
			t.Errorf("Expected 2 mcp servers, got %d", len(mcp))
		}
		if _, ok := mcp["context7"]; !ok {
			t.Error("Expected context7 to remain")
		}
		if _, ok := mcp["new-plugin.new-server"]; !ok {
			t.Error("Expected new-plugin.new-server to exist")
		}
	})
}

func TestUninstallMCPConfig(t *testing.T) {
	t.Run("removes servers with plugin prefix from opencode.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		existingContent := `{
			"mcp": {
				"context7": {
					"command": ["npx", "-y", "@context7/mcp-server"],
					"enabled": true,
					"type": "local"
				},
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

		if err := mgr.UninstallMCPConfig("my-plugin"); err != nil {
			t.Fatalf("UninstallMCPConfig failed: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))

		var config map[string]json.RawMessage
		json.Unmarshal(data, &config)

		var mcp map[string]OpenCodeMCPServer
		json.Unmarshal(config["mcp"], &mcp)

		if len(mcp) != 2 {
			t.Errorf("Expected 2 servers remaining, got %d", len(mcp))
		}

		if _, ok := mcp["context7"]; !ok {
			t.Error("Expected context7 to remain")
		}
		if _, ok := mcp["other-plugin.server"]; !ok {
			t.Error("Expected other-plugin.server to remain")
		}
		if _, ok := mcp["my-plugin.server1"]; ok {
			t.Error("Expected my-plugin.server1 to be removed")
		}
		if _, ok := mcp["my-plugin.server2"]; ok {
			t.Error("Expected my-plugin.server2 to be removed")
		}
	})

	t.Run("handles missing config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		err := mgr.UninstallMCPConfig("my-plugin")
		if err != nil {
			t.Errorf("Expected no error for missing config, got: %v", err)
		}
	})
}

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

	t.Run("preserves unknown fields and omitted enabled on unrelated servers", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		existingContent := `{
			"mcp": {
				"my-plugin.server": {
					"command": ["node", "s.js"],
					"enabled": true,
					"customField": "keep-me",
					"type": "local"
				},
				"other.server": {
					"command": ["node", "other.js"],
					"customField": "keep-me-too",
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
		var mcp map[string]map[string]json.RawMessage
		json.Unmarshal(config["mcp"], &mcp)

		if _, ok := mcp["my-plugin.server"]["customField"]; !ok {
			t.Error("Expected plugin server customField to be preserved")
		}
		if _, ok := mcp["other.server"]["customField"]; !ok {
			t.Error("Expected unrelated server customField to be preserved")
		}
		if _, ok := mcp["other.server"]["enabled"]; ok {
			t.Error("Expected unrelated omitted enabled field to stay omitted")
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

	t.Run("returns error for null plugin server", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		existingContent := `{
			"mcp": {
				"my-plugin.server": null
			}
		}`
		os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte(existingContent), 0644)

		if err := mgr.EnableMCPConfig("my-plugin"); err == nil {
			t.Fatal("expected error for null plugin server")
		}
	})
}

func TestInstallMissingMCPConfig(t *testing.T) {
	t.Run("adds missing servers without overwriting existing raw config", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		pluginDir := filepath.Join(tmpDir, "plugin")
		manifestDir := filepath.Join(pluginDir, ".claude-plugin")
		os.MkdirAll(manifestDir, 0755)
		os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"my-plugin","version":"1.2.3"}`), 0644)

		existingContent := `{
			"mcp": {
				"my-plugin.server": {
					"type": "local",
					"command": ["node", "custom.js"],
					"enabled": false,
					"environment": {"USER_EDIT": "yes"},
					"customField": "keep-me"
				},
				"other.server": {
					"type": "local",
					"command": ["node", "other.js"],
					"enabled": true
				}
			}
		}`
		os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte(existingContent), 0644)

		servers := map[string]MCPServer{
			"server":  {Command: "node", Args: []string{"default.js"}},
			"missing": {Command: "node", Args: []string{"${PLUGIN_VERSION}.js"}},
		}

		if err := mgr.InstallMissingMCPConfig(pluginDir, "my-plugin", "test-market", servers); err != nil {
			t.Fatalf("InstallMissingMCPConfig failed: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))
		var config map[string]json.RawMessage
		json.Unmarshal(data, &config)
		var mcpConfig map[string]map[string]json.RawMessage
		json.Unmarshal(config["mcp"], &mcpConfig)

		var existingCommand []string
		json.Unmarshal(mcpConfig["my-plugin.server"]["command"], &existingCommand)
		if len(existingCommand) != 2 || existingCommand[1] != "custom.js" {
			t.Fatalf("expected existing command to be preserved, got %v", existingCommand)
		}
		if _, ok := mcpConfig["my-plugin.server"]["customField"]; !ok {
			t.Fatal("expected custom field to be preserved")
		}

		var missingCommand []string
		json.Unmarshal(mcpConfig["my-plugin.missing"]["command"], &missingCommand)
		if len(missingCommand) != 2 || missingCommand[1] != "1.2.3.js" {
			t.Fatalf("expected missing server to be installed with substituted version, got %v", missingCommand)
		}
		if _, ok := mcpConfig["other.server"]; !ok {
			t.Fatal("expected unrelated server to be preserved")
		}
	})
}

func TestSubstituteVariables(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

	pluginPath := "/path/to/plugin"
	pluginName := "my-plugin"
	pluginVersion := "1.0.0"

	t.Run("substitutes CLAUDE_PLUGIN_ROOT variable", func(t *testing.T) {
		server := MCPServer{
			Command: "${CLAUDE_PLUGIN_ROOT}/bin/server",
		}

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion, "")

		expected := filepath.Join(pluginPath, "bin/server")
		if result.Command != expected {
			t.Errorf("Expected command '%s', got '%s'", expected, result.Command)
		}
	})

	t.Run("substitutes PLUGIN_NAME variable", func(t *testing.T) {
		server := MCPServer{
			Command: "node",
			Args:    []string{"${PLUGIN_NAME}.js"},
			Env: map[string]string{
				"PLUGIN_NAME": "${PLUGIN_NAME}",
			},
		}

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion, "")

		if result.Args[0] != "my-plugin.js" {
			t.Errorf("Expected arg 'my-plugin.js', got '%s'", result.Args[0])
		}

		if result.Env["PLUGIN_NAME"] != "my-plugin" {
			t.Errorf("Expected env 'my-plugin', got '%s'", result.Env["PLUGIN_NAME"])
		}
	})

	t.Run("substitutes PLUGIN_VERSION variable", func(t *testing.T) {
		server := MCPServer{
			Env: map[string]string{
				"VERSION": "${PLUGIN_VERSION}",
			},
		}

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion, "")

		if result.Env["VERSION"] != "1.0.0" {
			t.Errorf("Expected env '1.0.0', got '%s'", result.Env["VERSION"])
		}
	})

	t.Run("substitutes multiple variables in one string", func(t *testing.T) {
		server := MCPServer{
			Command: "${CLAUDE_PLUGIN_ROOT}/bin/${PLUGIN_NAME}-v${PLUGIN_VERSION}",
		}

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion, "")

		expected := filepath.Join(pluginPath, "bin/my-plugin-v1.0.0")
		if result.Command != expected {
			t.Errorf("Expected command '%s', got '%s'", expected, result.Command)
		}
	})

	t.Run("substitutes URL field", func(t *testing.T) {
		server := MCPServer{
			Type: "http",
			URL:  "https://api.example.com/plugins/${PLUGIN_NAME}/${PLUGIN_VERSION}",
		}

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion, "")

		expected := "https://api.example.com/plugins/my-plugin/1.0.0"
		if result.URL != expected {
			t.Errorf("Expected URL '%s', got '%s'", expected, result.URL)
		}
	})

	t.Run("handles strings without variables", func(t *testing.T) {
		server := MCPServer{
			Command: "node",
			Args:    []string{"server.js"},
		}

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion, "")

		if result.Command != "node" {
			t.Errorf("Expected command 'node', got '%s'", result.Command)
		}

		if result.Args[0] != "server.js" {
			t.Errorf("Expected arg 'server.js', got '%s'", result.Args[0])
		}
	})
}

func TestSubstitutePluginData(t *testing.T) {
	t.Run("substitutes CLAUDE_PLUGIN_DATA in command", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		mgr := NewManager(tmpDir, dataDir)

		pluginDataDir := filepath.Join(dataDir, "my-plugin-test-market")
		server := MCPServer{
			Command: "${CLAUDE_PLUGIN_DATA}/bin/server",
			Args:    []string{"--data", "${CLAUDE_PLUGIN_DATA}"},
			Env: map[string]string{
				"DATA_DIR": "${CLAUDE_PLUGIN_DATA}",
			},
		}

		result := mgr.substituteVariables(server, "/plugin/path", "my-plugin", "1.0.0", pluginDataDir)

		expected := filepath.Join(dataDir, "my-plugin-test-market", "bin/server")
		if result.Command != expected {
			t.Errorf("Expected command '%s', got '%s'", expected, result.Command)
		}
		if result.Args[1] != pluginDataDir {
			t.Errorf("Expected arg '%s', got '%s'", pluginDataDir, result.Args[1])
		}
		if result.Env["DATA_DIR"] != pluginDataDir {
			t.Errorf("Expected env DATA_DIR '%s', got '%s'", pluginDataDir, result.Env["DATA_DIR"])
		}
	})

	t.Run("substitution does not create CLAUDE_PLUGIN_DATA directory directly", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		mgr := NewManager(tmpDir, dataDir)

		pluginDataDir := filepath.Join(dataDir, "my-plugin-test-market")
		server := MCPServer{
			Command: "${CLAUDE_PLUGIN_DATA}/run",
		}

		mgr.substituteVariables(server, "/plugin/path", "my-plugin", "1.0.0", pluginDataDir)

		if _, err := os.Stat(pluginDataDir); !os.IsNotExist(err) {
			t.Errorf("Expected directory '%s' not to be created directly", pluginDataDir)
		}
	})

	t.Run("different marketplaces get different data paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		mgr := NewManager(tmpDir, dataDir)

		dataDir1 := filepath.Join(dataDir, "market-a", "my-plugin")
		dataDir2 := filepath.Join(dataDir, "market-b", "my-plugin")

		server := MCPServer{Command: "${CLAUDE_PLUGIN_DATA}/run"}

		result1 := mgr.substituteVariables(server, "/p1", "my-plugin", "1.0.0", dataDir1)
		result2 := mgr.substituteVariables(server, "/p2", "my-plugin", "1.0.0", dataDir2)

		if result1.Command == result2.Command {
			t.Errorf("Expected different data paths for different marketplaces, got same: %s", result1.Command)
		}
	})

	t.Run("does not substitute headers", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		mgr := NewManager(tmpDir, dataDir)

		pluginDataDir := filepath.Join(dataDir, "my-plugin-test-market")
		server := MCPServer{
			Type:    "http",
			URL:     "https://api.example.com/mcp",
			Headers: map[string]string{"Authorization": "Bearer ${CLAUDE_PLUGIN_DATA}"},
		}

		result := mgr.substituteVariables(server, "/plugin/path", "my-plugin", "1.0.0", pluginDataDir)

		if result.Headers["Authorization"] != "Bearer ${CLAUDE_PLUGIN_DATA}" {
			t.Errorf("Headers should not be substituted, got '%s'", result.Headers["Authorization"])
		}
	})

	t.Run("does not substitute unrelated env placeholders", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		mgr := NewManager(tmpDir, dataDir)

		pluginDataDir := filepath.Join(dataDir, "my-plugin-test-market")
		server := MCPServer{
			Command: "${CLAUDE_PLUGIN_DATA}/bin/server",
			Env: map[string]string{
				"GITHUB_TOKEN": "${GITHUB_TOKEN}",
				"API_KEY":      "${API_KEY}",
			},
		}

		result := mgr.substituteVariables(server, "/plugin/path", "my-plugin", "1.0.0", pluginDataDir)

		if result.Env["GITHUB_TOKEN"] != "${GITHUB_TOKEN}" {
			t.Errorf("Expected GITHUB_TOKEN untouched, got '%s'", result.Env["GITHUB_TOKEN"])
		}
		if result.Env["API_KEY"] != "${API_KEY}" {
			t.Errorf("Expected API_KEY untouched, got '%s'", result.Env["API_KEY"])
		}
	})

	t.Run("empty pluginDataDir does not substitute", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, "")

		server := MCPServer{
			Command: "${CLAUDE_PLUGIN_DATA}/bin/server",
		}

		result := mgr.substituteVariables(server, "/plugin/path", "my-plugin", "1.0.0", "")

		if result.Command != "${CLAUDE_PLUGIN_DATA}/bin/server" {
			t.Errorf("Expected no substitution when pluginDataDir is empty, got '%s'", result.Command)
		}
	})
}

func TestInstallMCPConfigWithPluginData(t *testing.T) {
	t.Run("resolves CLAUDE_PLUGIN_DATA via InstallMCPConfig", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		mgr := NewManager(tmpDir, dataDir)

		pluginDir := filepath.Join(tmpDir, "my-plugin")
		pluginJSONDir := filepath.Join(pluginDir, ".claude-plugin")
		os.MkdirAll(pluginJSONDir, 0755)

		os.WriteFile(filepath.Join(pluginJSONDir, "plugin.json"), []byte(`{"name": "my-plugin", "version": "2.0.0"}`), 0644)
		os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(`{
			"my-server": {
				"command": "${CLAUDE_PLUGIN_DATA}/bin/server",
				"args": ["--root", "${CLAUDE_PLUGIN_ROOT}"],
				"env": {
					"DATA": "${CLAUDE_PLUGIN_DATA}",
					"NAME": "${PLUGIN_NAME}",
					"TOKEN": "${GITHUB_TOKEN}"
				}
			}
		}`), 0644)

		if err := mgr.InstallMCPConfig(pluginDir, "my-plugin", "my-market"); err != nil {
			t.Fatalf("InstallMCPConfig failed: %v", err)
		}

		expectedDataDir := filepath.Join(dataDir, "my-market", "my-plugin")
		if _, err := os.Stat(expectedDataDir); os.IsNotExist(err) {
			t.Errorf("Expected data directory '%s' to be created", expectedDataDir)
		}

		configPath := filepath.Join(tmpDir, "opencode.json")
		data, _ := os.ReadFile(configPath)

		var config map[string]json.RawMessage
		json.Unmarshal(data, &config)

		var mcp map[string]OpenCodeMCPServer
		json.Unmarshal(config["mcp"], &mcp)

		server := mcp["my-plugin.my-server"]
		expectedCmd := filepath.Join(dataDir, "my-market", "my-plugin", "bin", "server")
		if server.Command[0] != expectedCmd {
			t.Errorf("Expected command[0] '%s', got '%s'", expectedCmd, server.Command[0])
		}
		if server.Command[2] != pluginDir {
			t.Errorf("Expected CLAUDE_PLUGIN_ROOT substituted to '%s', got '%s'", pluginDir, server.Command[2])
		}
		if server.Environment["DATA"] != expectedDataDir {
			t.Errorf("Expected env DATA '%s', got '%s'", expectedDataDir, server.Environment["DATA"])
		}
		if server.Environment["NAME"] != "my-plugin" {
			t.Errorf("Expected env NAME 'my-plugin', got '%s'", server.Environment["NAME"])
		}
		if server.Environment["TOKEN"] != "${GITHUB_TOKEN}" {
			t.Errorf("Expected env TOKEN untouched, got '%s'", server.Environment["TOKEN"])
		}
	})

	t.Run("same plugin name different markets get different data dirs", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		mgr := NewManager(tmpDir, dataDir)

		pluginDir1 := filepath.Join(tmpDir, "plugin-a")
		os.MkdirAll(filepath.Join(pluginDir1, ".claude-plugin"), 0755)
		os.WriteFile(filepath.Join(pluginDir1, ".claude-plugin", "plugin.json"), []byte(`{"name": "my-plugin", "version": "1.0.0"}`), 0644)
		os.WriteFile(filepath.Join(pluginDir1, ".mcp.json"), []byte(`{"srv": {"command": "${CLAUDE_PLUGIN_DATA}/run"}}`), 0644)

		pluginDir2 := filepath.Join(tmpDir, "plugin-b")
		os.MkdirAll(filepath.Join(pluginDir2, ".claude-plugin"), 0755)
		os.WriteFile(filepath.Join(pluginDir2, ".claude-plugin", "plugin.json"), []byte(`{"name": "my-plugin", "version": "1.0.0"}`), 0644)
		os.WriteFile(filepath.Join(pluginDir2, ".mcp.json"), []byte(`{"srv": {"command": "${CLAUDE_PLUGIN_DATA}/run"}}`), 0644)

		mgr.InstallMCPConfig(pluginDir1, "my-plugin", "market-a")
		mgr.InstallMCPConfig(pluginDir2, "my-plugin", "market-b")

		data1 := filepath.Join(dataDir, "market-a", "my-plugin")
		data2 := filepath.Join(dataDir, "market-b", "my-plugin")
		if _, err := os.Stat(data1); os.IsNotExist(err) {
			t.Errorf("Expected data dir for market-a to exist")
		}
		if _, err := os.Stat(data2); os.IsNotExist(err) {
			t.Errorf("Expected data dir for market-b to exist")
		}
		if data1 == data2 {
			t.Errorf("Expected different data paths, both got '%s'", data1)
		}
	})

	t.Run("plugin and market names do not collide after sanitization", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		mgr := NewManager(tmpDir, dataDir)

		pluginDir1 := filepath.Join(tmpDir, "plugin-one")
		os.MkdirAll(filepath.Join(pluginDir1, ".claude-plugin"), 0755)
		os.WriteFile(filepath.Join(pluginDir1, ".claude-plugin", "plugin.json"), []byte(`{"name": "a-b", "version": "1.0.0"}`), 0644)
		os.WriteFile(filepath.Join(pluginDir1, ".mcp.json"), []byte(`{"srv": {"command": "${CLAUDE_PLUGIN_DATA}/run"}}`), 0644)

		pluginDir2 := filepath.Join(tmpDir, "plugin-two")
		os.MkdirAll(filepath.Join(pluginDir2, ".claude-plugin"), 0755)
		os.WriteFile(filepath.Join(pluginDir2, ".claude-plugin", "plugin.json"), []byte(`{"name": "a", "version": "1.0.0"}`), 0644)
		os.WriteFile(filepath.Join(pluginDir2, ".mcp.json"), []byte(`{"srv": {"command": "${CLAUDE_PLUGIN_DATA}/run"}}`), 0644)

		if err := mgr.InstallMCPConfig(pluginDir1, "a-b", "c"); err != nil {
			t.Fatal(err)
		}
		if err := mgr.InstallMCPConfig(pluginDir2, "a", "b-c"); err != nil {
			t.Fatal(err)
		}

		data1 := filepath.Join(dataDir, "c", "a-b")
		data2 := filepath.Join(dataDir, "b-c", "a")
		if data1 == data2 {
			t.Fatalf("expected non-colliding paths")
		}
		if _, err := os.Stat(data1); os.IsNotExist(err) {
			t.Errorf("expected first data dir to exist")
		}
		if _, err := os.Stat(data2); os.IsNotExist(err) {
			t.Errorf("expected second data dir to exist")
		}
	})

	t.Run("fails when CLAUDE_PLUGIN_DATA directory cannot be created", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataPath := filepath.Join(tmpDir, "data-file")
		if err := os.WriteFile(dataPath, []byte("not a directory"), 0644); err != nil {
			t.Fatal(err)
		}
		mgr := NewManager(tmpDir, dataPath)

		pluginDir := filepath.Join(tmpDir, "plugin")
		os.MkdirAll(filepath.Join(pluginDir, ".claude-plugin"), 0755)
		os.WriteFile(filepath.Join(pluginDir, ".claude-plugin", "plugin.json"), []byte(`{"name": "plugin", "version": "1.0.0"}`), 0644)
		os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(`{"srv": {"command": "${CLAUDE_PLUGIN_DATA}/run"}}`), 0644)

		if err := mgr.InstallMCPConfig(pluginDir, "plugin", "market"); err == nil {
			t.Fatal("expected InstallMCPConfig to fail")
		}
	})

	t.Run("preserves existing substitutions alongside CLAUDE_PLUGIN_DATA", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		mgr := NewManager(tmpDir, dataDir)

		pluginDir := filepath.Join(tmpDir, "test-plugin")
		pluginJSONDir := filepath.Join(pluginDir, ".claude-plugin")
		os.MkdirAll(pluginJSONDir, 0755)

		os.WriteFile(filepath.Join(pluginJSONDir, "plugin.json"), []byte(`{"name": "test-plugin", "version": "3.1.0"}`), 0644)
		os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(`{
			"server": {
				"command": "${CLAUDE_PLUGIN_ROOT}/bin/${PLUGIN_NAME}-v${PLUGIN_VERSION}",
				"args": ["--data", "${CLAUDE_PLUGIN_DATA}"],
				"env": {
					"ROOT": "${CLAUDE_PLUGIN_ROOT}",
					"NAME": "${PLUGIN_NAME}",
					"VER": "${PLUGIN_VERSION}",
					"DATA": "${CLAUDE_PLUGIN_DATA}"
				}
			}
		}`), 0644)

		mgr.InstallMCPConfig(pluginDir, "test-plugin", "test-market")

		configData, _ := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))
		var config map[string]json.RawMessage
		json.Unmarshal(configData, &config)
		var mcp map[string]OpenCodeMCPServer
		json.Unmarshal(config["mcp"], &mcp)

		server := mcp["test-plugin.server"]

		expectedCmd := filepath.Join(pluginDir, "bin", "test-plugin-v3.1.0")
		if server.Command[0] != expectedCmd {
			t.Errorf("Expected command '%s', got '%s'", expectedCmd, server.Command[0])
		}

		expectedData := filepath.Join(dataDir, "test-market", "test-plugin")
		if server.Command[2] != expectedData {
			t.Errorf("Expected arg '%s', got '%s'", expectedData, server.Command[2])
		}

		if server.Environment["ROOT"] != pluginDir {
			t.Errorf("Expected ROOT '%s', got '%s'", pluginDir, server.Environment["ROOT"])
		}
		if server.Environment["NAME"] != "test-plugin" {
			t.Errorf("Expected NAME 'test-plugin', got '%s'", server.Environment["NAME"])
		}
		if server.Environment["VER"] != "3.1.0" {
			t.Errorf("Expected VER '3.1.0', got '%s'", server.Environment["VER"])
		}
		if server.Environment["DATA"] != expectedData {
			t.Errorf("Expected DATA '%s', got '%s'", expectedData, server.Environment["DATA"])
		}
	})
}

func TestToOpenCodeServer(t *testing.T) {
	t.Run("stdio server converts to local", func(t *testing.T) {
		mgr := NewManager(t.TempDir(), filepath.Join(t.TempDir(), "data"))
		server := MCPServer{
			Command: "npx",
			Args:    []string{"-y", "@playwright/mcp@latest"},
			Env:     map[string]string{"NODE_ENV": "production"},
		}

		oc := mgr.toOpenCodeServer(server)

		if oc.Type != "local" {
			t.Errorf("Expected type 'local', got '%s'", oc.Type)
		}
		if !oc.Enabled {
			t.Error("Expected enabled to be true")
		}
		if len(oc.Command) != 3 || oc.Command[0] != "npx" || oc.Command[1] != "-y" || oc.Command[2] != "@playwright/mcp@latest" {
			t.Errorf("Expected command ['npx', '-y', '@playwright/mcp@latest'], got %v", oc.Command)
		}
		if oc.Environment["NODE_ENV"] != "production" {
			t.Errorf("Expected environment NODE_ENV='production', got '%s'", oc.Environment["NODE_ENV"])
		}
	})

	t.Run("http server converts to remote", func(t *testing.T) {
		mgr := NewManager(t.TempDir(), filepath.Join(t.TempDir(), "data"))
		server := MCPServer{
			Type:    "http",
			URL:     "https://api.githubcopilot.com/mcp/",
			Headers: map[string]string{"Authorization": "Bearer token"},
		}

		oc := mgr.toOpenCodeServer(server)

		if oc.Type != "remote" {
			t.Errorf("Expected type 'remote', got '%s'", oc.Type)
		}
		if oc.URL != "https://api.githubcopilot.com/mcp/" {
			t.Errorf("Expected URL, got '%s'", oc.URL)
		}
		if oc.Headers["Authorization"] != "Bearer token" {
			t.Errorf("Expected Authorization header")
		}
	})

	t.Run("sse server converts to remote", func(t *testing.T) {
		mgr := NewManager(t.TempDir(), filepath.Join(t.TempDir(), "data"))
		server := MCPServer{Type: "sse", URL: "https://example.com/sse"}

		oc := mgr.toOpenCodeServer(server)

		if oc.Type != "remote" {
			t.Errorf("Expected type 'remote', got '%s'", oc.Type)
		}
	})

	t.Run("stdio with no args", func(t *testing.T) {
		mgr := NewManager(t.TempDir(), filepath.Join(t.TempDir(), "data"))
		server := MCPServer{Command: "node"}

		oc := mgr.toOpenCodeServer(server)

		if len(oc.Command) != 1 || oc.Command[0] != "node" {
			t.Errorf("Expected command ['node'], got %v", oc.Command)
		}
	})
}

func TestFromOpenCodeServer(t *testing.T) {
	t.Run("local converts to stdio", func(t *testing.T) {
		mgr := NewManager(t.TempDir(), filepath.Join(t.TempDir(), "data"))
		oc := OpenCodeMCPServer{
			Type:    "local",
			Command: []string{"npx", "-y", "server"},
			Enabled: true,
			Environment: map[string]string{
				"NODE_ENV": "production",
			},
		}

		server := mgr.fromOpenCodeServer(oc)

		if server.Type != "stdio" {
			t.Errorf("Expected type 'stdio', got '%s'", server.Type)
		}
		if server.Command != "npx" {
			t.Errorf("Expected command 'npx', got '%s'", server.Command)
		}
		if len(server.Args) != 2 || server.Args[1] != "server" {
			t.Errorf("Expected args ['-y', 'server'], got %v", server.Args)
		}
		if server.Env["NODE_ENV"] != "production" {
			t.Errorf("Expected env NODE_ENV='production'")
		}
	})

	t.Run("remote converts to http", func(t *testing.T) {
		mgr := NewManager(t.TempDir(), filepath.Join(t.TempDir(), "data"))
		oc := OpenCodeMCPServer{
			Type:    "remote",
			URL:     "https://api.example.com/mcp",
			Headers: map[string]string{"Authorization": "Bearer token"},
			Enabled: true,
		}

		server := mgr.fromOpenCodeServer(oc)

		if server.Type != "http" {
			t.Errorf("Expected type 'http', got '%s'", server.Type)
		}
		if server.URL != "https://api.example.com/mcp" {
			t.Errorf("Expected URL, got '%s'", server.URL)
		}
	})
}

func TestListMCPServers(t *testing.T) {
	t.Run("lists servers from opencode.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		content := `{
			"mcp": {
				"plugin-a.server1": {
					"command": ["node", "s1.js"],
					"enabled": true,
					"type": "local"
				},
				"plugin-b.server2": {
					"type": "remote",
					"url": "https://api.example.com",
					"enabled": true
				}
			}
		}`
		if err := os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write opencode.json: %v", err)
		}

		servers, err := mgr.ListMCPServers()
		if err != nil {
			t.Fatalf("ListMCPServers failed: %v", err)
		}

		if len(servers) != 2 {
			t.Errorf("Expected 2 servers, got %d", len(servers))
		}

		if _, ok := servers["plugin-a.server1"]; !ok {
			t.Error("Expected plugin-a.server1 to exist")
		}

		if _, ok := servers["plugin-b.server2"]; !ok {
			t.Error("Expected plugin-b.server2 to exist")
		}

		s1 := servers["plugin-a.server1"]
		if s1.Type != "stdio" {
			t.Errorf("Expected type 'stdio', got '%s'", s1.Type)
		}
		if s1.Command != "node" {
			t.Errorf("Expected command 'node', got '%s'", s1.Command)
		}

		s2 := servers["plugin-b.server2"]
		if s2.Type != "http" {
			t.Errorf("Expected type 'http', got '%s'", s2.Type)
		}
	})

	t.Run("returns empty map when no config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

		servers, err := mgr.ListMCPServers()
		if err != nil {
			t.Fatalf("ListMCPServers failed: %v", err)
		}

		if len(servers) != 0 {
			t.Errorf("Expected 0 servers, got %d", len(servers))
		}
	})
}

// RED for #1: writeOpenCodeConfig 不得用一个残缺/损坏的 opencode.json
// 覆盖掉用户原有的 model/provider/permissions 配置。
// 现状：读取后 json.Unmarshal 的错误被忽略，fullConfig 停留在 nil，
// 随后被重置为空 map 并只写入 mcp 键 —— 等于擦除用户配置。
func TestWriteOpenCodeConfig_CorruptFileIsPreservedNotWiped(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

	corrupt := `{"$schema":"https://opencode.ai/config.json","model":"gpt-x", broken`
	if err := os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte(corrupt), 0644); err != nil {
		t.Fatalf("seed corrupt opencode.json: %v", err)
	}

	err := mgr.mutateMCPRaw(func(mcp map[string]json.RawMessage) (bool, error) {
		mcp["p.s"] = json.RawMessage(`{"type":"local","command":["node"],"enabled":true}`)
		return true, nil
	})
	if err == nil {
		t.Fatal("expected error when opencode.json is corrupt, got nil (data-loss risk)")
	}

	after, readErr := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))
	if readErr != nil {
		t.Fatalf("read back opencode.json: %v", readErr)
	}
	if string(after) != corrupt {
		t.Fatalf("opencode.json was modified — user config destroyed.\nwant: %q\ngot:  %q", corrupt, string(after))
	}
}

// RED for #6: readMCPRaw 不得在 "mcp" 块是错误类型时
// 静默吞掉错误并把 MCP 当作空。该错误对调用方必须可见。
func TestReadOpenCodeConfig_CorruptMCPBlock_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir, filepath.Join(tmpDir, "data"))

	// 外层 JSON 合法（能被解析为 map[string]json.RawMessage），
	// 但 "mcp" 值是字符串而非对象 —— 单独 unmarshal 进 map 会失败。
	content := `{"model":"x","mcp":"not-an-object"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "opencode.json"), []byte(content), 0644); err != nil {
		t.Fatalf("seed opencode.json: %v", err)
	}

	if _, err := mgr.readMCPRaw(); err == nil {
		t.Fatal("expected error when mcp block is corrupt, got nil (silent corruption)")
	}
}
