package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Manager struct {
	paths *Paths
}

func NewManager() (*Manager, error) {
	paths := DefaultPaths()

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

	if err := os.WriteFile(m.paths.KnownMarkets, data, 0644); err != nil {
		return fmt.Errorf("failed to write known_marketplaces.json: %w", err)
	}

	return nil
}

func (m *Manager) AddKnownMarket(name string, marketSrc map[string]interface{}) error {
	markets, err := m.LoadKnownMarkets()
	if err != nil {
		return err
	}

	marketSrc["lastUpdated"] = time.Now()
	markets[name] = marketSrc

	return m.SaveKnownMarkets(markets)
}

func (m *Manager) RemoveKnownMarket(name string) error {
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

	if err := os.WriteFile(m.paths.InstalledFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write installed_plugins.json: %w", err)
	}

	return nil
}

func (m *Manager) AddInstallRecord(key string, record *InstallRecord) error {
	installed, err := m.LoadInstalledPlugins()
	if err != nil {
		return err
	}

	installed.Plugins[key] = append(installed.Plugins[key], *record)

	return m.SaveInstalledPlugins(installed)
}

func (m *Manager) RemoveInstallRecord(key string) error {
	installed, err := m.LoadInstalledPlugins()
	if err != nil {
		return err
	}

	delete(installed.Plugins, key)

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
