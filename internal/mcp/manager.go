package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MCPServer represents an MCP server configuration
type MCPServer struct {
	Type    string            `json:"type,omitempty"`    // "stdio", "sse", "http", "websocket"
	Command string            `json:"command,omitempty"` // For stdio type
	Args    []string          `json:"args,omitempty"`    // For stdio type
	URL     string            `json:"url,omitempty"`     // For http/sse/websocket type
	Env     map[string]string `json:"env,omitempty"`     // Environment variables
	Headers map[string]string `json:"headers,omitempty"` // HTTP headers (for http/sse types)
}

// MCPConfig represents the .mcp.json file structure
// Supports two formats:
// 1. Direct format (Claude Code standard): {"server-name": {...}}
// 2. Wrapped format (backward compatible): {"mcpServers": {"server-name": {...}}}
type MCPConfig struct {
	Servers map[string]MCPServer `json:"mcpServers,omitempty"`
}

// PluginJSON represents the plugin.json structure with MCP support
type PluginJSON struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Version     string               `json:"version,omitempty"`
	Author      *Author              `json:"author,omitempty"`
	Homepage    string               `json:"homepage,omitempty"`
	Keywords    []string             `json:"keywords,omitempty"`
	MCPServers  map[string]MCPServer `json:"mcpServers,omitempty"`
}

// Author represents the plugin author
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// Manager manages MCP configurations
type Manager struct {
	configDir string
}

// NewManager creates a new MCP manager
func NewManager(configDir string) *Manager {
	return &Manager{
		configDir: configDir,
	}
}

// ReadMCPConfig reads the .mcp.json file from a plugin
// Supports two formats:
// 1. Direct format: {"server-name": {...}} (Claude Code standard)
// 2. Wrapped format: {"mcpServers": {"server-name": {...}}}
func (m *Manager) ReadMCPConfig(pluginPath string) (*MCPConfig, error) {
	mcpPath := filepath.Join(pluginPath, ".mcp.json")

	data, err := os.ReadFile(mcpPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No MCP config, that's okay
		}
		return nil, fmt.Errorf("failed to read .mcp.json: %w", err)
	}

	// Try wrapped format first
	var wrappedConfig MCPConfig
	if err := json.Unmarshal(data, &wrappedConfig); err == nil && len(wrappedConfig.Servers) > 0 {
		return &wrappedConfig, nil
	}

	// Try direct format (Claude Code standard)
	var directConfig map[string]MCPServer
	if err := json.Unmarshal(data, &directConfig); err != nil {
		return nil, fmt.Errorf("failed to parse .mcp.json: %w", err)
	}

	if len(directConfig) > 0 {
		return &MCPConfig{Servers: directConfig}, nil
	}

	return &MCPConfig{Servers: make(map[string]MCPServer)}, nil
}

// ReadPluginJSON reads the plugin.json file
// Returns nil if the file doesn't exist (not all plugins have plugin.json)
func (m *Manager) ReadPluginJSON(pluginPath string) (*PluginJSON, error) {
	pluginPath = filepath.Join(pluginPath, ".claude-plugin", "plugin.json")

	data, err := os.ReadFile(pluginPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No plugin.json, that's okay for MCP-only plugins
		}
		return nil, fmt.Errorf("failed to read plugin.json: %w", err)
	}

	var plugin PluginJSON
	if err := json.Unmarshal(data, &plugin); err != nil {
		return nil, fmt.Errorf("failed to parse plugin.json: %w", err)
	}

	return &plugin, nil
}

// GetMCPServers gets all MCP servers from a plugin
// It checks both .mcp.json and plugin.json for MCP servers
func (m *Manager) GetMCPServers(pluginPath string) (map[string]MCPServer, error) {
	servers := make(map[string]MCPServer)

	// Check .mcp.json first
	mcpConfig, err := m.ReadMCPConfig(pluginPath)
	if err != nil {
		return nil, err
	}

	if mcpConfig != nil && mcpConfig.Servers != nil {
		for name, server := range mcpConfig.Servers {
			servers[name] = server
		}
	}

	// Check plugin.json for inline mcpServers (optional)
	plugin, err := m.ReadPluginJSON(pluginPath)
	if err != nil {
		return nil, err
	}

	if plugin != nil && plugin.MCPServers != nil {
		for name, server := range plugin.MCPServers {
			servers[name] = server
		}
	}

	return servers, nil
}

// InstallMCPConfig installs MCP configuration to Claude Code config directory
func (m *Manager) InstallMCPConfig(pluginPath, pluginName string) error {
	servers, err := m.GetMCPServers(pluginPath)
	if err != nil {
		return err
	}

	if len(servers) == 0 {
		return nil // No MCP servers to install
	}

	pluginInfo, _ := m.ReadPluginJSON(pluginPath)
	pluginVersion := ""
	if pluginInfo != nil {
		pluginVersion = pluginInfo.Version
	}

	// Read existing MCP config
	mcpPath := filepath.Join(m.configDir, ".mcp.json")
	existingConfig := &MCPConfig{Servers: make(map[string]MCPServer)}

	if data, err := os.ReadFile(mcpPath); err == nil {
		json.Unmarshal(data, existingConfig)
	}

	// Add plugin servers with plugin name prefix
	for serverName, server := range servers {
		// Use plugin_name.server_name format to avoid conflicts
		fullName := fmt.Sprintf("%s.%s", pluginName, serverName)

		// Substitute variables
		server = m.substituteVariables(server, pluginPath, pluginName, pluginVersion)

		existingConfig.Servers[fullName] = server
	}

	// Write updated config
	data, err := json.MarshalIndent(existingConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write MCP config: %w", err)
	}

	return nil
}

// UninstallMCPConfig removes MCP configuration for a plugin
func (m *Manager) UninstallMCPConfig(pluginName string) error {
	mcpPath := filepath.Join(m.configDir, ".mcp.json")

	data, err := os.ReadFile(mcpPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No config to remove
		}
		return fmt.Errorf("failed to read MCP config: %w", err)
	}

	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse MCP config: %w", err)
	}

	// Remove servers with plugin name prefix
	prefix := fmt.Sprintf("%s.", pluginName)
	for name := range config.Servers {
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			delete(config.Servers, name)
		}
	}

	// Write updated config
	data, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write MCP config: %w", err)
	}

	return nil
}

// substituteVariables substitutes all supported variables in MCP server configuration
func (m *Manager) substituteVariables(server MCPServer, pluginPath, pluginName, pluginVersion string) MCPServer {
	result := server

	// Substitute in command
	if server.Command != "" {
		result.Command = m.substituteString(server.Command, pluginPath, pluginName, pluginVersion)
	}

	// Substitute in args
	if server.Args != nil {
		result.Args = make([]string, len(server.Args))
		for i, arg := range server.Args {
			result.Args[i] = m.substituteString(arg, pluginPath, pluginName, pluginVersion)
		}
	}

	// Substitute in URL
	if server.URL != "" {
		result.URL = m.substituteString(server.URL, pluginPath, pluginName, pluginVersion)
	}

	// Substitute in env values
	if server.Env != nil {
		result.Env = make(map[string]string)
		for key, value := range server.Env {
			result.Env[key] = m.substituteString(value, pluginPath, pluginName, pluginVersion)
		}
	}

	return result
}

// substituteString substitutes variables in a string
// Supported variables:
//   - ${CLAUDE_PLUGIN_ROOT} - Path to the plugin directory
//   - ${PLUGIN_NAME} - Name of the plugin
//   - ${PLUGIN_VERSION} - Version of the plugin
func (m *Manager) substituteString(str, pluginPath, pluginName, pluginVersion string) string {
	result := str

	// Replace ${CLAUDE_PLUGIN_ROOT}
	result = strings.ReplaceAll(result, "${CLAUDE_PLUGIN_ROOT}", pluginPath)

	// Replace ${PLUGIN_NAME}
	result = strings.ReplaceAll(result, "${PLUGIN_NAME}", pluginName)

	// Replace ${PLUGIN_VERSION}
	result = strings.ReplaceAll(result, "${PLUGIN_VERSION}", pluginVersion)

	return result
}

// substitutePluginRoot is kept for backward compatibility
// Deprecated: Use substituteVariables instead
func (m *Manager) substitutePluginRoot(server MCPServer, pluginPath string) MCPServer {
	return m.substituteVariables(server, pluginPath, "", "")
}

// substitutePath is kept for backward compatibility
// Deprecated: Use substituteString instead
func (m *Manager) substitutePath(str, pluginPath string) string {
	return m.substituteString(str, pluginPath, "", "")
}

// ListMCPServers lists all installed MCP servers
func (m *Manager) ListMCPServers() (map[string]MCPServer, error) {
	mcpPath := filepath.Join(m.configDir, ".mcp.json")

	data, err := os.ReadFile(mcpPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]MCPServer), nil
		}
		return nil, fmt.Errorf("failed to read MCP config: %w", err)
	}

	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse MCP config: %w", err)
	}

	return config.Servers, nil
}
