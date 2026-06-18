package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/opencode/plugin-cli/internal/pathutil"
)

type Manager struct {
	paths *Paths
	// mu 保护 installed_plugins.json 和 known_marketplaces.json 的读改写序列。
	// 之前每个 Add/Remove/Mutate 都独立 Load → Modify → Save，两个 goroutine 同时
	// 调用会丢更新（last writer wins）。注意：这个 mutex 只保护进程内并发；
	// 跨进程并发（两个 CLI 调用）仍然存在，需要 flock 才能彻底解决。
	mu sync.Mutex
}

func NewManager() (*Manager, error) {
	paths, err := DefaultPaths()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(paths.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	if err := os.MkdirAll(paths.MarketsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create markets directory: %w", err)
	}

	if err := os.MkdirAll(paths.CacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Manager{paths: paths}, nil
}

func NewManagerWithPath(paths *Paths) *Manager {
	return &Manager{paths: paths}
}

func (m *Manager) LoadKnownMarkets() (KnownMarkets, error) {
	data, err := os.ReadFile(m.paths.KnownMarkets)
	if err != nil {
		if os.IsNotExist(err) {
			return make(KnownMarkets), nil
		}
		return nil, fmt.Errorf("failed to read known_marketplaces.json: %w", err)
	}

	var markets KnownMarkets
	if err := json.Unmarshal(data, &markets); err != nil {
		return nil, fmt.Errorf("failed to parse known_marketplaces.json: %w", err)
	}

	return markets, nil
}

func (m *Manager) SaveKnownMarkets(markets KnownMarkets) error {
	data, err := json.MarshalIndent(markets, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal known markets: %w", err)
	}

	if err := pathutil.WriteFileAtomic(m.paths.KnownMarkets, data, 0644); err != nil {
		return fmt.Errorf("failed to write known_marketplaces.json: %w", err)
	}

	return nil
}

func (m *Manager) AddKnownMarket(name string, marketSrc map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	markets, err := m.LoadKnownMarkets()
	if err != nil {
		return err
	}

	// 拷贝入参 map，避免修改调用方的数据（调用方可能继续使用这个 map）。
	// 同时写入 lastUpdated 时间戳。
	entry := make(map[string]interface{}, len(marketSrc)+1)
	for k, v := range marketSrc {
		entry[k] = v
	}
	entry["lastUpdated"] = time.Now()
	markets[name] = entry

	return m.SaveKnownMarkets(markets)
}

func (m *Manager) RemoveKnownMarket(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	markets, err := m.LoadKnownMarkets()
	if err != nil {
		return err
	}

	delete(markets, name)

	return m.SaveKnownMarkets(markets)
}

func (m *Manager) LoadInstalledPlugins() (*InstalledPlugins, error) {
	data, err := os.ReadFile(m.paths.InstalledFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &InstalledPlugins{
				Version: 2,
				Plugins: make(map[string][]InstallRecord),
			}, nil
		}
		return nil, fmt.Errorf("failed to read installed_plugins.json: %w", err)
	}

	var installed InstalledPlugins
	if err := json.Unmarshal(data, &installed); err != nil {
		return nil, fmt.Errorf("failed to parse installed_plugins.json: %w", err)
	}

	if installed.Plugins == nil {
		installed.Plugins = make(map[string][]InstallRecord)
	}

	return &installed, nil
}

func (m *Manager) SaveInstalledPlugins(installed *InstalledPlugins) error {
	data, err := json.MarshalIndent(installed, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal installed plugins: %w", err)
	}

	if err := pathutil.WriteFileAtomic(m.paths.InstalledFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write installed_plugins.json: %w", err)
	}

	return nil
}

func (m *Manager) AddInstallRecord(key string, record *InstallRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, err := m.LoadInstalledPlugins()
	if err != nil {
		return err
	}

	installed.Plugins[key] = []InstallRecord{*record}

	return m.SaveInstalledPlugins(installed)
}

func (m *Manager) RemoveInstallRecord(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, err := m.LoadInstalledPlugins()
	if err != nil {
		return err
	}

	delete(installed.Plugins, key)

	return m.SaveInstalledPlugins(installed)
}

func (m *Manager) MutateInstallRecord(key string, fn func(*InstallRecord)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, err := m.LoadInstalledPlugins()
	if err != nil {
		return err
	}

	records, ok := installed.Plugins[key]
	if !ok || len(records) == 0 {
		return fmt.Errorf("plugin %s not found", key)
	}

	before := records[0]
	fn(&records[0])

	if records[0] == before {
		return nil
	}

	installed.Plugins[key] = records

	return m.SaveInstalledPlugins(installed)
}

func (m *Manager) GetInstallRecord(key string) (*InstallRecord, error) {
	installed, err := m.LoadInstalledPlugins()
	if err != nil {
		return nil, err
	}

	records, ok := installed.Plugins[key]
	if !ok || len(records) == 0 {
		return nil, fmt.Errorf("plugin %s not found", key)
	}

	return &records[0], nil
}

func (m *Manager) GetPaths() *Paths {
	return m.paths
}

func (m *Manager) GetAllInstallPaths() (map[string]bool, error) {
	installed, err := m.LoadInstalledPlugins()
	if err != nil {
		return nil, err
	}

	paths := make(map[string]bool)
	for _, records := range installed.Plugins {
		for _, record := range records {
			if record.InstallPath != "" {
				paths[record.InstallPath] = true
			}
		}
	}
	return paths, nil
}
