package market

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode/plugin-cli/internal/config"
)

func TestAddCommandPersistsManifestNameAsKnownMarketKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	marketDir := writeTestMarketplace(t, "agent-docs-template", "agent-docs-plugins")

	runAddCommandForTest(t, marketDir, "")

	markets := loadKnownMarketsForTest(t)
	if _, ok := markets["agent-docs-plugins"]; !ok {
		t.Fatalf("known marketplaces missing manifest key %q: %#v", "agent-docs-plugins", markets)
	}
	if _, ok := markets["local-marketplace"]; ok {
		t.Fatalf("known marketplaces should not use derived local key: %#v", markets)
	}
}

func TestAddCommandPreservesExplicitNameAsKnownMarketKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	marketDir := writeTestMarketplace(t, "agent-docs-template", "agent-docs-plugins")

	runAddCommandForTest(t, marketDir, "custom-market")

	markets := loadKnownMarketsForTest(t)
	if _, ok := markets["custom-market"]; !ok {
		t.Fatalf("known marketplaces missing explicit key %q: %#v", "custom-market", markets)
	}
	if _, ok := markets["agent-docs-plugins"]; ok {
		t.Fatalf("known marketplaces should not use manifest key when --name is provided: %#v", markets)
	}
}

func TestResolveAddedMarketplaceNameUsesManifestName(t *testing.T) {
	got := resolveAddedMarketplaceName("agent-docs-template", "agent-docs-plugins", false)
	want := "agent-docs-plugins"
	if got != want {
		t.Fatalf("resolveAddedMarketplaceName() = %q, want %q", got, want)
	}
}

func TestResolveAddedMarketplaceNamePreservesExplicitName(t *testing.T) {
	got := resolveAddedMarketplaceName("custom-market", "agent-docs-plugins", true)
	want := "custom-market"
	if got != want {
		t.Fatalf("resolveAddedMarketplaceName() = %q, want %q", got, want)
	}
}

func TestResolveAddedMarketplaceNameFallsBackToDerivedName(t *testing.T) {
	got := resolveAddedMarketplaceName("agent-docs-template", "", false)
	want := "agent-docs-template"
	if got != want {
		t.Fatalf("resolveAddedMarketplaceName() = %q, want %q", got, want)
	}
}

func runAddCommandForTest(t *testing.T, source, name string) {
	t.Helper()
	if err := addCmd.Flags().Set("name", name); err != nil {
		t.Fatalf("failed to set name flag: %v", err)
	}
	t.Cleanup(func() {
		if err := addCmd.Flags().Set("name", ""); err != nil {
			t.Fatalf("failed to reset name flag: %v", err)
		}
	})
	addCmd.Run(addCmd, []string{source})
}

func writeTestMarketplace(t *testing.T, dirName, marketName string) string {
	t.Helper()
	marketDir := filepath.Join(t.TempDir(), dirName)
	pluginDir := filepath.Join(marketDir, ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create marketplace directory: %v", err)
	}
	data, err := json.Marshal(map[string]interface{}{
		"name":    marketName,
		"plugins": []interface{}{},
	})
	if err != nil {
		t.Fatalf("failed to marshal marketplace index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "marketplace.json"), data, 0644); err != nil {
		t.Fatalf("failed to write marketplace index: %v", err)
	}
	return marketDir
}

func loadKnownMarketsForTest(t *testing.T) config.KnownMarkets {
	t.Helper()
	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatalf("failed to initialize config: %v", err)
	}
	markets, err := configMgr.LoadKnownMarkets()
	if err != nil {
		t.Fatalf("failed to load known marketplaces: %v", err)
	}
	return markets
}
