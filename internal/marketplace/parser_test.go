package marketplace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMarketplaceIndex(t *testing.T) {
	marketplacePath := filepath.Join("..", "..", "test", "fixtures", "sample-marketplace", ".claude-plugin", "marketplace.json")

	marketplace, err := ParseMarketplaceIndex(marketplacePath)
	if err != nil {
		t.Fatalf("ParseMarketplaceIndex() error = %v", err)
	}

	if marketplace.Name != "test-marketplace" {
		t.Errorf("Name = %v, want test-marketplace", marketplace.Name)
	}

	if marketplace.Description != "Test marketplace for unit tests" {
		t.Errorf("Description = %v, want 'Test marketplace for unit tests'", marketplace.Description)
	}

	if marketplace.Owner == nil {
		t.Fatal("Owner is nil")
	}

	if marketplace.Owner.Name != "Test Org" {
		t.Errorf("Owner.Name = %v, want Test Org", marketplace.Owner.Name)
	}

	if len(marketplace.Plugins) != 2 {
		t.Fatalf("Plugins count = %v, want 2", len(marketplace.Plugins))
	}

	plugin1 := marketplace.Plugins[0]
	if plugin1.Name != "test-plugin" {
		t.Errorf("Plugin[0].Name = %v, want test-plugin", plugin1.Name)
	}

	pluginSource1, ok := plugin1.Source.(PluginSource)
	if !ok {
		t.Fatal("Plugin[0].Source is not PluginSource")
	}

	if pluginSource1.Type != string(SourceTypeLocal) {
		t.Errorf("Plugin[0].Source.Type = %v, want local", pluginSource1.Type)
	}

	if pluginSource1.Path != "./plugins/test-plugin" {
		t.Errorf("Plugin[0].Source.Path = %v, want ./plugins/test-plugin", pluginSource1.Path)
	}

	plugin2 := marketplace.Plugins[1]
	if plugin2.Name != "external-plugin" {
		t.Errorf("Plugin[1].Name = %v, want external-plugin", plugin2.Name)
	}

	pluginSource2, ok := plugin2.Source.(PluginSource)
	if !ok {
		t.Fatal("Plugin[1].Source is not PluginSource")
	}

	if pluginSource2.Type != string(SourceTypeGitHub) {
		t.Errorf("Plugin[1].Source.Type = %v, want github", pluginSource2.Type)
	}

	if pluginSource2.Repo != "test/external-plugin" {
		t.Errorf("Plugin[1].Source.Repo = %v, want test/external-plugin", pluginSource2.Repo)
	}
}

func TestParseMarketplaceIndex_InvalidJSON(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(tmpFile, []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseMarketplaceIndex(tmpFile)
	if err == nil {
		t.Error("ParseMarketplaceIndex() expected error for invalid JSON, got nil")
	}
}

func TestParseMarketplaceIndex_MissingName(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "missing-name.json")
	content := `{"description": "test"}`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseMarketplaceIndex(tmpFile)
	if err == nil {
		t.Error("ParseMarketplaceIndex() expected error for missing name, got nil")
	}
}
