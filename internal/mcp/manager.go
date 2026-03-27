package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MCPServer struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type OpenCodeMCPServer struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command,omitempty"`
	URL         string            `json:"url,omitempty"`
	Enabled     bool              `json:"enabled"`
	Environment map[string]string `json:"environment,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type MCPConfig struct {
	Servers map[string]MCPServer `json:"mcpServers,omitempty"`
}

type OpenCodeConfig struct {
	MCP map[string]OpenCodeMCPServer `json:"mcp,omitempty"`
}

type PluginJSON struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Version     string               `json:"version,omitempty"`
	Author      *Author              `json:"author,omitempty"`
	Homepage    string               `json:"homepage,omitempty"`
	Keywords    []string             `json:"keywords,omitempty"`
	MCPServers  map[string]MCPServer `json:"mcpServers,omitempty"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Manager struct {
	configDir string
}

func NewManager(configDir string) *Manager {
	return &Manager{
		configDir: configDir,
	}
}

func (m *Manager) ReadMCPConfig(pluginPath string) (*MCPConfig, error) {
	mcpPath := filepath.Join(pluginPath, ".mcp.json")

	data, err := os.ReadFile(mcpPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read .mcp.json: %w", err)
	}

	var wrappedConfig MCPConfig
	if err := json.Unmarshal(data, &wrappedConfig); err == nil && len(wrappedConfig.Servers) > 0 {
		return &wrappedConfig, nil
	}

	var directConfig map[string]MCPServer
	if err := json.Unmarshal(data, &directConfig); err != nil {
		return nil, fmt.Errorf("failed to parse .mcp.json: %w", err)
	}

	if len(directConfig) > 0 {
		return &MCPConfig{Servers: directConfig}, nil
	}

	return &MCPConfig{Servers: make(map[string]MCPServer)}, nil
}

func (m *Manager) ReadPluginJSON(pluginPath string) (*PluginJSON, error) {
	pluginPath = filepath.Join(pluginPath, ".claude-plugin", "plugin.json")

	data, err := os.ReadFile(pluginPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read plugin.json: %w", err)
	}

	var plugin PluginJSON
	if err := json.Unmarshal(data, &plugin); err != nil {
		return nil, fmt.Errorf("failed to parse plugin.json: %w", err)
	}

	return &plugin, nil
}

func (m *Manager) GetMCPServers(pluginPath string) (map[string]MCPServer, error) {
	servers := make(map[string]MCPServer)

	mcpConfig, err := m.ReadMCPConfig(pluginPath)
	if err != nil {
		return nil, err
	}

	if mcpConfig != nil && mcpConfig.Servers != nil {
		for name, server := range mcpConfig.Servers {
			servers[name] = server
		}
	}

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

func (m *Manager) InstallMCPConfig(pluginPath, pluginName string) error {
	servers, err := m.GetMCPServers(pluginPath)
	if err != nil {
		return err
	}

	if len(servers) == 0 {
		return nil
	}

	pluginInfo, _ := m.ReadPluginJSON(pluginPath)
	pluginVersion := ""
	if pluginInfo != nil {
		pluginVersion = pluginInfo.Version
	}

	opencodeConfig, err := m.readOpenCodeConfig()
	if err != nil {
		return err
	}

	if opencodeConfig.MCP == nil {
		opencodeConfig.MCP = make(map[string]OpenCodeMCPServer)
	}

	for serverName, server := range servers {
		fullName := fmt.Sprintf("%s.%s", pluginName, serverName)

		server = m.substituteVariables(server, pluginPath, pluginName, pluginVersion)
		opencodeConfig.MCP[fullName] = m.toOpenCodeServer(server)
	}

	return m.writeOpenCodeConfig(opencodeConfig)
}

func (m *Manager) UninstallMCPConfig(pluginName string) error {
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
	for name := range opencodeConfig.MCP {
		if strings.HasPrefix(name, prefix) {
			delete(opencodeConfig.MCP, name)
		}
	}

	return m.writeOpenCodeConfig(opencodeConfig)
}

func (m *Manager) ListMCPServers() (map[string]MCPServer, error) {
	opencodeConfig, err := m.readOpenCodeConfig()
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]MCPServer), nil
		}
		return nil, err
	}

	servers := make(map[string]MCPServer)
	for name, ocServer := range opencodeConfig.MCP {
		servers[name] = m.fromOpenCodeServer(ocServer)
	}

	return servers, nil
}

func (m *Manager) toOpenCodeServer(server MCPServer) OpenCodeMCPServer {
	oc := OpenCodeMCPServer{Enabled: true}

	switch server.Type {
	case "sse", "http", "websocket":
		oc.Type = "remote"
		oc.URL = server.URL
		if server.Headers != nil {
			oc.Headers = server.Headers
		}
	default:
		oc.Type = "local"
		if server.Command != "" {
			cmd := []string{server.Command}
			if server.Args != nil {
				cmd = append(cmd, server.Args...)
			}
			oc.Command = cmd
		}
		if server.Env != nil {
			oc.Environment = server.Env
		}
	}

	return oc
}

func (m *Manager) fromOpenCodeServer(oc OpenCodeMCPServer) MCPServer {
	server := MCPServer{}

	switch oc.Type {
	case "remote":
		server.Type = "http"
		server.URL = oc.URL
		if oc.Headers != nil {
			server.Headers = oc.Headers
		}
	default:
		server.Type = "stdio"
		if len(oc.Command) > 0 {
			server.Command = oc.Command[0]
			if len(oc.Command) > 1 {
				server.Args = oc.Command[1:]
			}
		}
		if oc.Environment != nil {
			server.Env = oc.Environment
		}
	}

	return server
}

func (m *Manager) readOpenCodeConfig() (*OpenCodeConfig, error) {
	configPath := m.opencodeConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &OpenCodeConfig{}, nil
		}
		return nil, err
	}

	var fullConfig map[string]json.RawMessage
	if err := json.Unmarshal(data, &fullConfig); err != nil {
		return nil, fmt.Errorf("failed to parse opencode.json: %w", err)
	}

	oc := &OpenCodeConfig{}
	if raw, ok := fullConfig["mcp"]; ok {
		json.Unmarshal(raw, &oc.MCP)
	}

	return oc, nil
}

func (m *Manager) writeOpenCodeConfig(oc *OpenCodeConfig) error {
	configPath := m.opencodeConfigPath()

	var fullConfig map[string]json.RawMessage
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &fullConfig)
	}
	if fullConfig == nil {
		fullConfig = make(map[string]json.RawMessage)
	}

	if len(oc.MCP) > 0 {
		mcpData, err := json.Marshal(oc.MCP)
		if err != nil {
			return fmt.Errorf("failed to marshal mcp config: %w", err)
		}
		fullConfig["mcp"] = mcpData
	} else {
		delete(fullConfig, "mcp")
	}

	output, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal opencode.json: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(configPath, append(output, '\n'), 0644)
}

func (m *Manager) opencodeConfigPath() string {
	return filepath.Join(m.configDir, "opencode.json")
}

func (m *Manager) substituteVariables(server MCPServer, pluginPath, pluginName, pluginVersion string) MCPServer {
	result := server

	if server.Command != "" {
		result.Command = m.substituteString(server.Command, pluginPath, pluginName, pluginVersion)
	}

	if server.Args != nil {
		result.Args = make([]string, len(server.Args))
		for i, arg := range server.Args {
			result.Args[i] = m.substituteString(arg, pluginPath, pluginName, pluginVersion)
		}
	}

	if server.URL != "" {
		result.URL = m.substituteString(server.URL, pluginPath, pluginName, pluginVersion)
	}

	if server.Env != nil {
		result.Env = make(map[string]string)
		for key, value := range server.Env {
			result.Env[key] = m.substituteString(value, pluginPath, pluginName, pluginVersion)
		}
	}

	return result
}

func (m *Manager) substituteString(str, pluginPath, pluginName, pluginVersion string) string {
	result := str
	result = strings.ReplaceAll(result, "${CLAUDE_PLUGIN_ROOT}", pluginPath)
	result = strings.ReplaceAll(result, "${PLUGIN_NAME}", pluginName)
	result = strings.ReplaceAll(result, "${PLUGIN_VERSION}", pluginVersion)
	return result
}

func (m *Manager) substitutePluginRoot(server MCPServer, pluginPath string) MCPServer {
	return m.substituteVariables(server, pluginPath, "", "")
}

func (m *Manager) substitutePath(str, pluginPath string) string {
	return m.substituteString(str, pluginPath, "", "")
}
