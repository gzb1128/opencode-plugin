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
	t.Run("installs stdio server to opencode.json with correct format", func(t *testing.T) {
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
			"my-server": {
				"command": "npx",
				"args": ["chrome-devtools-mcp@latest"]
			}
		}`
		if err := os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), []byte(mcpContent), 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
		}

		if err := mgr.InstallMCPConfig(pluginDir, "my-plugin"); err != nil {
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
		mgr := NewManager(tmpDir)

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

		if err := mgr.InstallMCPConfig(pluginDir, "gh-plugin"); err != nil {
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
		mgr := NewManager(tmpDir)

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

		if err := mgr.InstallMCPConfig(pluginDir, "my-plugin"); err != nil {
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
		mgr := NewManager(tmpDir)

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

		if err := mgr.InstallMCPConfig(pluginDir, "new-plugin"); err != nil {
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
		mgr := NewManager(tmpDir)

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
		mgr := NewManager(tmpDir)

		err := mgr.UninstallMCPConfig("my-plugin")
		if err != nil {
			t.Errorf("Expected no error for missing config, got: %v", err)
		}
	})
}

func TestSubstituteVariables(t *testing.T) {
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

func TestToOpenCodeServer(t *testing.T) {
	t.Run("stdio server converts to local", func(t *testing.T) {
		mgr := NewManager(t.TempDir())
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
		mgr := NewManager(t.TempDir())
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
		mgr := NewManager(t.TempDir())
		server := MCPServer{Type: "sse", URL: "https://example.com/sse"}

		oc := mgr.toOpenCodeServer(server)

		if oc.Type != "remote" {
			t.Errorf("Expected type 'remote', got '%s'", oc.Type)
		}
	})

	t.Run("stdio with no args", func(t *testing.T) {
		mgr := NewManager(t.TempDir())
		server := MCPServer{Command: "node"}

		oc := mgr.toOpenCodeServer(server)

		if len(oc.Command) != 1 || oc.Command[0] != "node" {
			t.Errorf("Expected command ['node'], got %v", oc.Command)
		}
	})
}

func TestFromOpenCodeServer(t *testing.T) {
	t.Run("local converts to stdio", func(t *testing.T) {
		mgr := NewManager(t.TempDir())
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
		mgr := NewManager(t.TempDir())
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
		mgr := NewManager(tmpDir)

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
