package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencode/plugin-cli/internal/pathutil"
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
	configDir     string
	pluginDataDir string
}

func NewManager(configDir string, pluginDataDir string) *Manager {
	return &Manager{
		configDir:     configDir,
		pluginDataDir: pluginDataDir,
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

func (m *Manager) InstallMCPConfig(pluginPath, pluginName, marketName string) error {
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

	var pluginDataDir string
	if m.pluginDataDir != "" && pluginName != "" && marketName != "" {
		pluginDataDir = filepath.Join(m.pluginDataDir, pathutil.SanitizeAlias(marketName), pathutil.SanitizeAlias(pluginName))
		if usesPluginData(servers) {
			if err := os.MkdirAll(pluginDataDir, 0755); err != nil {
				return fmt.Errorf("failed to create plugin data directory: %w", err)
			}
		}
	}

	// 通过 json.RawMessage round-trip 写入，保留 opencode.json 里所有 server
	// 的未知字段（用户手写的 disabledReason / tools / 自定义 transport 字段等）。
	// 旧的 typed struct 写法会把不在 OpenCodeMCPServer 里的字段全部删掉。
	return m.mutateMCPRaw(func(mcp map[string]json.RawMessage) (bool, error) {
		for serverName, server := range servers {
			fullName := fmt.Sprintf("%s.%s", pluginName, serverName)
			server = m.substituteVariables(server, pluginPath, pluginName, pluginVersion, pluginDataDir)
			data, err := json.Marshal(m.toOpenCodeServer(server))
			if err != nil {
				return false, fmt.Errorf("failed to marshal mcp server %s: %w", fullName, err)
			}
			mcp[fullName] = data
		}
		return true, nil
	})
}

func (m *Manager) InstallMissingMCPConfig(pluginPath, pluginName, marketName string, servers map[string]MCPServer) error {
	if len(servers) == 0 {
		return nil
	}

	pluginInfo, _ := m.ReadPluginJSON(pluginPath)
	pluginVersion := ""
	if pluginInfo != nil {
		pluginVersion = pluginInfo.Version
	}

	var pluginDataDir string
	if m.pluginDataDir != "" && pluginName != "" && marketName != "" {
		pluginDataDir = filepath.Join(m.pluginDataDir, pathutil.SanitizeAlias(marketName), pathutil.SanitizeAlias(pluginName))
		if usesPluginData(servers) {
			if err := os.MkdirAll(pluginDataDir, 0755); err != nil {
				return fmt.Errorf("failed to create plugin data directory: %w", err)
			}
		}
	}

	return m.mutateMCPRaw(func(mcp map[string]json.RawMessage) (bool, error) {
		changed := false
		for serverName, server := range servers {
			fullName := fmt.Sprintf("%s.%s", pluginName, serverName)
			if _, ok := mcp[fullName]; ok {
				continue
			}

			server = m.substituteVariables(server, pluginPath, pluginName, pluginVersion, pluginDataDir)
			data, err := json.Marshal(m.toOpenCodeServer(server))
			if err != nil {
				return false, fmt.Errorf("failed to marshal mcp server %s: %w", fullName, err)
			}
			mcp[fullName] = data
			changed = true
		}
		return changed, nil
	})
}

func usesPluginData(servers map[string]MCPServer) bool {
	for _, server := range servers {
		if strings.Contains(server.Command, "${CLAUDE_PLUGIN_DATA}") || strings.Contains(server.URL, "${CLAUDE_PLUGIN_DATA}") {
			return true
		}
		for _, arg := range server.Args {
			if strings.Contains(arg, "${CLAUDE_PLUGIN_DATA}") {
				return true
			}
		}
		for _, value := range server.Env {
			if strings.Contains(value, "${CLAUDE_PLUGIN_DATA}") {
				return true
			}
		}
	}
	return false
}

func (m *Manager) UninstallMCPConfig(pluginName string) error {
	prefix := fmt.Sprintf("%s.", pluginName)
	return m.mutateMCPRaw(func(mcp map[string]json.RawMessage) (bool, error) {
		changed := false
		for name := range mcp {
			if strings.HasPrefix(name, prefix) {
				delete(mcp, name)
				changed = true
			}
		}
		return changed, nil
	})
}

func (m *Manager) DisableMCPConfig(pluginName string) error {
	return m.setMCPEnabled(pluginName, false)
}

func (m *Manager) EnableMCPConfig(pluginName string) error {
	return m.setMCPEnabled(pluginName, true)
}

func (m *Manager) setMCPEnabled(pluginName string, enabled bool) error {
	prefix := fmt.Sprintf("%s.", pluginName)
	return m.mutateMCPRaw(func(mcp map[string]json.RawMessage) (bool, error) {
		changed := false
		for name, rawServer := range mcp {
			if !strings.HasPrefix(name, prefix) {
				continue
			}

			var server map[string]json.RawMessage
			if err := json.Unmarshal(rawServer, &server); err != nil {
				return false, fmt.Errorf("failed to parse mcp server %s: %w", name, err)
			}
			if server == nil {
				return false, fmt.Errorf("failed to parse mcp server %s: server must be an object", name)
			}

			if rawEnabled, ok := server["enabled"]; ok {
				var current bool
				if err := json.Unmarshal(rawEnabled, &current); err == nil && current == enabled {
					continue
				}
			}

			server["enabled"] = json.RawMessage(fmt.Sprintf("%t", enabled))
			updated, err := json.Marshal(server)
			if err != nil {
				return false, fmt.Errorf("failed to marshal mcp server %s: %w", name, err)
			}
			mcp[name] = updated
			changed = true
		}
		return changed, nil
	})
}

func (m *Manager) ListMCPServers() (map[string]MCPServer, error) {
	mcp, err := m.readMCPRaw()
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]MCPServer), nil
		}
		return nil, err
	}

	servers := make(map[string]MCPServer)
	// 单个 entry 解析失败时跳过该 entry 但不让整个列表失败：
	// ListMCPServers 是只读展示 API，坏掉一行不应阻断其他正常 server 的展示。
	// 但解析失败必须给用户可见的信号（之前是静默 continue，typo 永远查不到）。
	for name, raw := range mcp {
		var oc OpenCodeMCPServer
		if err := json.Unmarshal(raw, &oc); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: skipping MCP server %q: failed to parse entry: %v\n", name, err)
			continue
		}
		servers[name] = m.fromOpenCodeServer(oc)
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

// readOpenCodeFull 读取 opencode.json 为 map[string]json.RawMessage，保留所有顶层 key。
// 文件不存在时返回 (nil, nil)。文件损坏时返回错误而不是覆盖用户配置。
func (m *Manager) readOpenCodeFull() (map[string]json.RawMessage, error) {
	configPath := m.opencodeConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var full map[string]json.RawMessage
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("failed to parse opencode.json: %w", err)
	}
	return full, nil
}

// writeOpenCodeFull 把完整 raw map 原子地写回 opencode.json。
// 写到同目录临时文件再 rename，避免 SIGKILL / 磁盘满时留下截断的文件。
func (m *Manager) writeOpenCodeFull(full map[string]json.RawMessage) error {
	if full == nil {
		full = make(map[string]json.RawMessage)
	}
	output, err := json.MarshalIndent(full, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal opencode.json: %w", err)
	}
	return pathutil.WriteFileAtomic(m.opencodeConfigPath(), append(output, '\n'), 0644)
}

// mutateMCPRaw 加载 opencode.json 一次，把 mcp 子映射交给 fn 修改，
// 再写回一次。fn 返回 (changed, error)：error != nil 时跳过写回并把 error 透传，
// changed == false 时跳过写回（no-op 优化）。
//
// 之前每个 Install/Uninstall/Enable/Disable 都先读出 mcp 子映射，写回前又
// 重新读取完整 opencode.json——每次命令把整个 opencode.json 读了两遍。
// 本 helper 一次读、一次写，并且保留所有顶层 key 和
// 每个 server 的未知字段。
func (m *Manager) mutateMCPRaw(fn func(mcp map[string]json.RawMessage) (changed bool, err error)) error {
	full, err := m.readOpenCodeFull()
	if err != nil {
		return err
	}
	if full == nil {
		full = make(map[string]json.RawMessage)
	}

	var mcp map[string]json.RawMessage
	if rawMCP, ok := full["mcp"]; ok && len(rawMCP) > 0 {
		if err := json.Unmarshal(rawMCP, &mcp); err != nil {
			return fmt.Errorf("failed to parse mcp block of opencode.json: %w", err)
		}
	}
	if mcp == nil {
		mcp = make(map[string]json.RawMessage)
	}

	changed, fnErr := fn(mcp)
	if fnErr != nil {
		return fnErr
	}
	if !changed {
		return nil
	}

	if len(mcp) > 0 {
		data, err := json.Marshal(mcp)
		if err != nil {
			return fmt.Errorf("failed to marshal mcp config: %w", err)
		}
		full["mcp"] = data
	} else {
		delete(full, "mcp")
	}
	return m.writeOpenCodeFull(full)
}

// readMCPRaw 仅给 ListMCPServers 这种只读 API 用。读改写流程请用 mutateMCPRaw。
// 保留每个 server 的所有字段，包括 OpenCodeMCPServer 之外的未知字段。
// 文件不存在或没有 mcp 块时返回空（非 nil）map。
func (m *Manager) readMCPRaw() (map[string]json.RawMessage, error) {
	full, err := m.readOpenCodeFull()
	if err != nil {
		return nil, err
	}
	if full == nil {
		return make(map[string]json.RawMessage), nil
	}
	rawMCP, ok := full["mcp"]
	if !ok || len(rawMCP) == 0 {
		return make(map[string]json.RawMessage), nil
	}
	var mcp map[string]json.RawMessage
	if err := json.Unmarshal(rawMCP, &mcp); err != nil {
		return nil, fmt.Errorf("failed to parse mcp block of opencode.json: %w", err)
	}
	if mcp == nil {
		mcp = make(map[string]json.RawMessage)
	}
	return mcp, nil
}

func (m *Manager) opencodeConfigPath() string {
	return filepath.Join(m.configDir, "opencode.json")
}

func (m *Manager) substituteVariables(server MCPServer, pluginPath, pluginName, pluginVersion, pluginDataDir string) MCPServer {
	result := server

	if server.Command != "" {
		result.Command = m.substituteString(server.Command, pluginPath, pluginName, pluginVersion, pluginDataDir)
	}

	if server.Args != nil {
		result.Args = make([]string, len(server.Args))
		for i, arg := range server.Args {
			result.Args[i] = m.substituteString(arg, pluginPath, pluginName, pluginVersion, pluginDataDir)
		}
	}

	if server.URL != "" {
		result.URL = m.substituteString(server.URL, pluginPath, pluginName, pluginVersion, pluginDataDir)
	}

	if server.Env != nil {
		result.Env = make(map[string]string)
		for key, value := range server.Env {
			result.Env[key] = m.substituteString(value, pluginPath, pluginName, pluginVersion, pluginDataDir)
		}
	}

	// 注意：Headers 不做变量替换。Headers 会随 HTTP 请求发到远端，
	// 把本地路径（如 ${CLAUDE_PLUGIN_DATA}）注入到远端可见的 header 是不安全的。
	// 参见 TestSubstitutePluginData/does_not_substitute_headers。

	return result
}

func (m *Manager) substituteString(str, pluginPath, pluginName, pluginVersion, pluginDataDir string) string {
	result := str
	result = strings.ReplaceAll(result, "${CLAUDE_PLUGIN_ROOT}", pluginPath)
	result = strings.ReplaceAll(result, "${PLUGIN_NAME}", pluginName)
	result = strings.ReplaceAll(result, "${PLUGIN_VERSION}", pluginVersion)
	if pluginDataDir != "" {
		result = strings.ReplaceAll(result, "${CLAUDE_PLUGIN_DATA}", pluginDataDir)
	}
	return result
}
