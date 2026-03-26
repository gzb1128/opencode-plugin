package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadMCPConfig(t *testing.T) {
	t.Run("reads valid .mcp.json file with wrapped format", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)

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
		mgr := NewManager(tmpDir)

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
		mgr := NewManager(tmpDir)

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
		mgr := NewManager(tmpDir)

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
		mgr := NewManager(tmpDir)

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
			t.Fatalf("Failed to write test file: %v", err)
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
		mgr := NewManager(tmpDir)

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
		mgr := NewManager(tmpDir)

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
		mgr := NewManager(tmpDir)

		// Only create .mcp.json, no plugin.json
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
	t.Run("installs MCP servers with plugin prefix", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)

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
			"mcpServers": {
				"my-server": {
					"command": "${CLAUDE_PLUGIN_ROOT}/bin/server",
					"args": ["--name", "${PLUGIN_NAME}", "--version", "${PLUGIN_VERSION}"],
					"env": {
						"PLUGIN_ROOT": "${CLAUDE_PLUGIN_ROOT}",
						"PLUGIN_NAME": "${PLUGIN_NAME}"
					}
				}
			}
		}`
		if err := os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(mcpContent), 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
		}

		if err := mgr.InstallMCPConfig(pluginDir, "my-plugin"); err != nil {
			t.Fatalf("InstallMCPConfig failed: %v", err)
		}

		mcpPath := filepath.Join(tmpDir, ".mcp.json")
		data, err := os.ReadFile(mcpPath)
		if err != nil {
			t.Fatalf("Failed to read .mcp.json: %v", err)
		}

		var config MCPConfig
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("Failed to parse .mcp.json: %v", err)
		}

		if len(config.Servers) != 1 {
			t.Errorf("Expected 1 server, got %d", len(config.Servers))
		}

		server, ok := config.Servers["my-plugin.my-server"]
		if !ok {
			t.Fatal("Expected my-plugin.my-server to exist")
		}

		expectedCmd := filepath.Join(pluginDir, "bin/server")
		if server.Command != expectedCmd {
			t.Errorf("Expected command '%s', got '%s'", expectedCmd, server.Command)
		}

		if len(server.Args) != 4 {
			t.Fatalf("Expected 4 args, got %d", len(server.Args))
		}

		if server.Args[1] != "my-plugin" {
			t.Errorf("Expected arg[1] 'my-plugin', got '%s'", server.Args[1])
		}

		if server.Args[3] != "2.0.0" {
			t.Errorf("Expected arg[3] '2.0.0', got '%s'", server.Args[3])
		}

		if server.Env["PLUGIN_ROOT"] != pluginDir {
			t.Errorf("Expected env PLUGIN_ROOT '%s', got '%s'", pluginDir, server.Env["PLUGIN_ROOT"])
		}

		if server.Env["PLUGIN_NAME"] != "my-plugin" {
			t.Errorf("Expected env PLUGIN_NAME 'my-plugin', got '%s'", server.Env["PLUGIN_NAME"])
		}
	})

	t.Run("merges with existing MCP config", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)

		existingContent := `{
			"mcpServers": {
				"existing-server": {
					"type": "http",
					"url": "https://existing.example.com"
				}
			}
		}`
		if err := os.WriteFile(filepath.Join(tmpDir, ".mcp.json"), []byte(existingContent), 0644); err != nil {
			t.Fatalf("Failed to write existing .mcp.json: %v", err)
		}

		pluginDir := filepath.Join(tmpDir, "new-plugin")
		pluginJSONDir := filepath.Join(pluginDir, ".claude-plugin")
		if err := os.MkdirAll(pluginJSONDir, 0755); err != nil {
			t.Fatalf("Failed to create plugin dir: %v", err)
		}

		pluginContent := `{"name": "new-plugin"}`
		if err := os.WriteFile(filepath.Join(pluginJSONDir, "plugin.json"), []byte(pluginContent), 0644); err != nil {
			t.Fatalf("Failed to write plugin.json: %v", err)
		}

		mcpContent := `{
			"mcpServers": {
				"new-server": {
					"command": "node"
				}
			}
		}`
		if err := os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(mcpContent), 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
		}

		if err := mgr.InstallMCPConfig(pluginDir, "new-plugin"); err != nil {
			t.Fatalf("InstallMCPConfig failed: %v", err)
		}

		mcpPath := filepath.Join(tmpDir, ".mcp.json")
		data, err := os.ReadFile(mcpPath)
		if err != nil {
			t.Fatalf("Failed to read .mcp.json: %v", err)
		}

		var config MCPConfig
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("Failed to parse .mcp.json: %v", err)
		}

		if len(config.Servers) != 2 {
			t.Errorf("Expected 2 servers, got %d", len(config.Servers))
		}

		if _, ok := config.Servers["existing-server"]; !ok {
			t.Error("Expected existing-server to remain")
		}

		if _, ok := config.Servers["new-plugin.new-server"]; !ok {
			t.Error("Expected new-plugin.new-server to exist")
		}
	})
}

func TestUninstallMCPConfig(t *testing.T) {
	t.Run("removes servers with plugin prefix", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)

		existingContent := `{
			"mcpServers": {
				"existing-server": {
					"type": "http",
					"url": "https://existing.example.com"
				},
				"my-plugin.server1": {
					"command": "node"
				},
				"my-plugin.server2": {
					"command": "python"
				},
				"other-plugin.server": {
					"command": "ruby"
				}
			}
		}`
		if err := os.WriteFile(filepath.Join(tmpDir, ".mcp.json"), []byte(existingContent), 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
		}

		if err := mgr.UninstallMCPConfig("my-plugin"); err != nil {
			t.Fatalf("UninstallMCPConfig failed: %v", err)
		}

		mcpPath := filepath.Join(tmpDir, ".mcp.json")
		data, err := os.ReadFile(mcpPath)
		if err != nil {
			t.Fatalf("Failed to read .mcp.json: %v", err)
		}

		var config MCPConfig
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("Failed to parse .mcp.json: %v", err)
		}

		if len(config.Servers) != 2 {
			t.Errorf("Expected 2 servers remaining, got %d", len(config.Servers))
		}

		if _, ok := config.Servers["existing-server"]; !ok {
			t.Error("Expected existing-server to remain")
		}

		if _, ok := config.Servers["other-plugin.server"]; !ok {
			t.Error("Expected other-plugin.server to remain")
		}

		if _, ok := config.Servers["my-plugin.server1"]; ok {
			t.Error("Expected my-plugin.server1 to be removed")
		}

		if _, ok := config.Servers["my-plugin.server2"]; ok {
			t.Error("Expected my-plugin.server2 to be removed")
		}
	})

	t.Run("handles missing config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)

		err := mgr.UninstallMCPConfig("my-plugin")
		if err != nil {
			t.Errorf("Expected no error for missing config, got: %v", err)
		}
	})
}

func TestSubstitutePluginRoot(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	pluginPath := "/path/to/plugin"
	pluginName := "my-plugin"
	pluginVersion := "1.0.0"

	t.Run("substitutes CLAUDE_PLUGIN_ROOT variable", func(t *testing.T) {
		server := MCPServer{
			Command: "${CLAUDE_PLUGIN_ROOT}/bin/server",
		}

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion)

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

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion)

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

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion)

		if result.Env["VERSION"] != "1.0.0" {
			t.Errorf("Expected env '1.0.0', got '%s'", result.Env["VERSION"])
		}
	})

	t.Run("substitutes multiple variables in one string", func(t *testing.T) {
		server := MCPServer{
			Command: "${CLAUDE_PLUGIN_ROOT}/bin/${PLUGIN_NAME}-v${PLUGIN_VERSION}",
		}

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion)

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

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion)

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

		result := mgr.substituteVariables(server, pluginPath, pluginName, pluginVersion)

		if result.Command != "node" {
			t.Errorf("Expected command 'node', got '%s'", result.Command)
		}

		if result.Args[0] != "server.js" {
			t.Errorf("Expected arg 'server.js', got '%s'", result.Args[0])
		}
	})
}

func TestListMCPServers(t *testing.T) {
	t.Run("lists all installed servers", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)

		content := `{
			"mcpServers": {
				"plugin-a.server1": {
					"command": "node"
				},
				"plugin-b.server2": {
					"type": "http",
					"url": "https://api.example.com"
				}
			}
		}`
		if err := os.WriteFile(filepath.Join(tmpDir, ".mcp.json"), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
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
	})

	t.Run("returns empty map when no config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)

		servers, err := mgr.ListMCPServers()
		if err != nil {
			t.Fatalf("ListMCPServers failed: %v", err)
		}

		if len(servers) != 0 {
			t.Errorf("Expected 0 servers, got %d", len(servers))
		}
	})
}
