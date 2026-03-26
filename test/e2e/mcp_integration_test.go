package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/mcp"
)

func TestMCPIntegration(t *testing.T) {
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Unsetenv("HOME")

	os.Setenv("OPENCODE_PLUGIN_CLI_HOME", "")
	defer os.Unsetenv("OPENCODE_PLUGIN_CLI_HOME")

	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	paths := configMgr.GetPaths()
	mcpMgr := mcp.NewManager(paths.OpenCodeConfig)

	t.Run("initial state - no MCP servers", func(t *testing.T) {
		servers, err := mcpMgr.ListMCPServers()
		if err != nil {
			t.Fatalf("ListMCPServers failed: %v", err)
		}

		if len(servers) != 0 {
			t.Errorf("Expected 0 servers initially, got %d", len(servers))
		}
	})

	t.Run("install plugin with MCP servers", func(t *testing.T) {
		pluginDir := filepath.Join(tmpHome, "test-plugin")
		pluginJSONDir := filepath.Join(pluginDir, ".claude-plugin")

		if err := os.MkdirAll(pluginJSONDir, 0755); err != nil {
			t.Fatalf("Failed to create plugin dir: %v", err)
		}

		pluginJSON := map[string]interface{}{
			"name":        "test-mcp-plugin",
			"version":     "1.0.0",
			"description": "Test plugin with MCP servers",
			"mcpServers": map[string]interface{}{
				"inline-server": map[string]interface{}{
					"type":    "http",
					"url":     "https://api.example.com/mcp",
					"enabled": true,
				},
			},
		}

		pluginData, _ := json.MarshalIndent(pluginJSON, "", "  ")
		if err := os.WriteFile(filepath.Join(pluginJSONDir, "plugin.json"), pluginData, 0644); err != nil {
			t.Fatalf("Failed to write plugin.json: %v", err)
		}

		mcpJSON := map[string]interface{}{
			"mcpServers": map[string]interface{}{
				"file-server": map[string]interface{}{
					"command": "${CLAUDE_PLUGIN_ROOT}/bin/server",
					"args":    []string{"--port", "8080"},
					"env": map[string]string{
						"PLUGIN_NAME":    "${PLUGIN_NAME}",
						"PLUGIN_VERSION": "${PLUGIN_VERSION}",
					},
				},
			},
		}

		mcpData, _ := json.MarshalIndent(mcpJSON, "", "  ")
		if err := os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), mcpData, 0644); err != nil {
			t.Fatalf("Failed to write .mcp.json: %v", err)
		}

		if err := mcpMgr.InstallMCPConfig(pluginDir, "test-mcp-plugin"); err != nil {
			t.Fatalf("InstallMCPConfig failed: %v", err)
		}

		servers, err := mcpMgr.ListMCPServers()
		if err != nil {
			t.Fatalf("ListMCPServers failed: %v", err)
		}

		if len(servers) != 2 {
			t.Errorf("Expected 2 servers after installation, got %d", len(servers))
		}

		fileServer, ok := servers["test-mcp-plugin.file-server"]
		if !ok {
			t.Fatal("Expected test-mcp-plugin.file-server to exist")
		}

		expectedCmd := filepath.Join(pluginDir, "bin/server")
		if fileServer.Command != expectedCmd {
			t.Errorf("Expected command '%s', got '%s'", expectedCmd, fileServer.Command)
		}

		if fileServer.Env["PLUGIN_NAME"] != "test-mcp-plugin" {
			t.Errorf("Expected PLUGIN_NAME 'test-mcp-plugin', got '%s'", fileServer.Env["PLUGIN_NAME"])
		}

		if fileServer.Env["PLUGIN_VERSION"] != "1.0.0" {
			t.Errorf("Expected PLUGIN_VERSION '1.0.0', got '%s'", fileServer.Env["PLUGIN_VERSION"])
		}

		inlineServer, ok := servers["test-mcp-plugin.inline-server"]
		if !ok {
			t.Fatal("Expected test-mcp-plugin.inline-server to exist")
		}

		if inlineServer.Type != "http" {
			t.Errorf("Expected type 'http', got '%s'", inlineServer.Type)
		}

		if inlineServer.URL != "https://api.example.com/mcp" {
			t.Errorf("Expected URL 'https://api.example.com/mcp', got '%s'", inlineServer.URL)
		}
	})

	t.Run("uninstall plugin removes MCP servers", func(t *testing.T) {
		if err := mcpMgr.UninstallMCPConfig("test-mcp-plugin"); err != nil {
			t.Fatalf("UninstallMCPConfig failed: %v", err)
		}

		servers, err := mcpMgr.ListMCPServers()
		if err != nil {
			t.Fatalf("ListMCPServers failed: %v", err)
		}

		if len(servers) != 0 {
			t.Errorf("Expected 0 servers after uninstall, got %d", len(servers))
		}
	})

	t.Run("multiple plugins with same server names", func(t *testing.T) {
		pluginADir := filepath.Join(tmpHome, "plugin-a")
		pluginBDir := filepath.Join(tmpHome, "plugin-b")

		for i, pluginDir := range []string{pluginADir, pluginBDir} {
			pluginName := []string{"plugin-a", "plugin-b"}[i]

			pluginJSONDir := filepath.Join(pluginDir, ".claude-plugin")
			if err := os.MkdirAll(pluginJSONDir, 0755); err != nil {
				t.Fatalf("Failed to create plugin dir: %v", err)
			}

			pluginJSON := map[string]interface{}{
				"name":    pluginName,
				"version": "1.0.0",
			}

			pluginData, _ := json.MarshalIndent(pluginJSON, "", "  ")
			if err := os.WriteFile(filepath.Join(pluginJSONDir, "plugin.json"), pluginData, 0644); err != nil {
				t.Fatalf("Failed to write plugin.json: %v", err)
			}

			mcpJSON := map[string]interface{}{
				"mcpServers": map[string]interface{}{
					"database": map[string]interface{}{
						"command": "postgres-server",
					},
				},
			}

			mcpData, _ := json.MarshalIndent(mcpJSON, "", "  ")
			if err := os.WriteFile(filepath.Join(pluginDir, ".mcp.json"), mcpData, 0644); err != nil {
				t.Fatalf("Failed to write .mcp.json: %v", err)
			}

			if err := mcpMgr.InstallMCPConfig(pluginDir, pluginName); err != nil {
				t.Fatalf("InstallMCPConfig failed for %s: %v", pluginName, err)
			}
		}

		servers, err := mcpMgr.ListMCPServers()
		if err != nil {
			t.Fatalf("ListMCPServers failed: %v", err)
		}

		if len(servers) != 2 {
			t.Errorf("Expected 2 servers, got %d", len(servers))
		}

		if _, ok := servers["plugin-a.database"]; !ok {
			t.Error("Expected plugin-a.database to exist")
		}

		if _, ok := servers["plugin-b.database"]; !ok {
			t.Error("Expected plugin-b.database to exist")
		}

		if err := mcpMgr.UninstallMCPConfig("plugin-a"); err != nil {
			t.Fatalf("UninstallMCPConfig failed: %v", err)
		}

		servers, err = mcpMgr.ListMCPServers()
		if err != nil {
			t.Fatalf("ListMCPServers failed: %v", err)
		}

		if len(servers) != 1 {
			t.Errorf("Expected 1 server after uninstall, got %d", len(servers))
		}

		if _, ok := servers["plugin-b.database"]; !ok {
			t.Error("Expected plugin-b.database to still exist")
		}
	})
}

func TestRealMCPPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	marketPath := "/Users/gaozebin3/.opencode-plugin-cli/markets/anthropics/claude-plugins-official"
	githubPluginPath := filepath.Join(marketPath, "external_plugins/github")

	if _, err := os.Stat(githubPluginPath); os.IsNotExist(err) {
		t.Skip("GitHub plugin not found in marketplace")
	}

	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Unsetenv("HOME")

	os.Setenv("OPENCODE_PLUGIN_CLI_HOME", "")
	defer os.Unsetenv("OPENCODE_PLUGIN_CLI_HOME")

	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	paths := configMgr.GetPaths()
	mcpMgr := mcp.NewManager(paths.OpenCodeConfig)

	t.Run("install GitHub plugin MCP config", func(t *testing.T) {
		if err := mcpMgr.InstallMCPConfig(githubPluginPath, "github"); err != nil {
			t.Fatalf("InstallMCPConfig failed: %v", err)
		}

		servers, err := mcpMgr.ListMCPServers()
		if err != nil {
			t.Fatalf("ListMCPServers failed: %v", err)
		}

		if len(servers) != 1 {
			t.Errorf("Expected 1 server, got %d", len(servers))
		}

		githubServer, ok := servers["github.github"]
		if !ok {
			t.Fatal("Expected github.github server to exist")
		}

		if githubServer.Type != "http" {
			t.Errorf("Expected type 'http', got '%s'", githubServer.Type)
		}

		if githubServer.URL != "https://api.githubcopilot.com/mcp/" {
			t.Errorf("Expected URL 'https://api.githubcopilot.com/mcp/', got '%s'", githubServer.URL)
		}
	})

	t.Run("uninstall GitHub plugin", func(t *testing.T) {
		if err := mcpMgr.UninstallMCPConfig("github"); err != nil {
			t.Fatalf("UninstallMCPConfig failed: %v", err)
		}

		servers, err := mcpMgr.ListMCPServers()
		if err != nil {
			t.Fatalf("ListMCPServers failed: %v", err)
		}

		if len(servers) != 0 {
			t.Errorf("Expected 0 servers after uninstall, got %d", len(servers))
		}
	})
}
