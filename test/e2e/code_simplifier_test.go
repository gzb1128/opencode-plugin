package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/opencode/plugin-cli/internal/plugin"
)

// TestE2ECodeSimplifier tests the complete flow of installing code-simplifier from claude-plugins-official
func TestE2ECodeSimplifier(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Create temp directory for test
	tempDir := t.TempDir()

	// Create config manager
	// For testing, we'll use the default manager but with a temp home
	// This is a simplified approach
	os.Setenv("HOME", tempDir)
	defer os.Unsetenv("HOME")

	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Create marketplace manager
	paths := configMgr.GetPaths()
	marketMgr := marketplace.NewManager(paths.MarketsDir)

	t.Run("AddClaudePluginsOfficial", func(t *testing.T) {
		// Add the official Claude plugins marketplace
		mp, err := marketMgr.Add("claude-plugins-official", "anthropics/claude-plugins-official")
		if err != nil {
			t.Fatalf("Failed to add marketplace: %v", err)
		}

		if mp.Name != "claude-plugins-official" {
			t.Errorf("Expected marketplace name 'claude-plugins-official', got '%s'", mp.Name)
		}

		// Check if code-simplifier plugin exists
		found := false
		for _, p := range mp.Plugins {
			if p.Name == "code-simplifier" {
				found = true
				t.Logf("Found code-simplifier plugin: %s", p.Description)
				break
			}
		}

		if !found {
			t.Error("code-simplifier plugin not found in marketplace")
		}

		// Save to config
		installLocation := filepath.Join(paths.MarketsDir, "claude-plugins-official")
		marketSrc := map[string]interface{}{
			"source":          "github",
			"repo":            "anthropics/claude-plugins-official",
			"installLocation": installLocation,
		}

		if err := configMgr.AddKnownMarket("claude-plugins-official", marketSrc); err != nil {
			t.Fatalf("Failed to save marketplace config: %v", err)
		}
	})

	t.Run("InstallCodeSimplifier", func(t *testing.T) {
		// Create plugin installer
		installer := plugin.NewInstaller(configMgr)

		// Install code-simplifier
		opts := plugin.InstallOptions{
			MarketName: "claude-plugins-official",
			Version:    "",
			Scope:      "user",
		}

		if err := installer.Install("code-simplifier", opts); err != nil {
			t.Fatalf("Failed to install code-simplifier: %v", err)
		}

		// Verify installation
		installed, err := installer.List()
		if err != nil {
			t.Fatalf("Failed to list installed plugins: %v", err)
		}

		key := "code-simplifier@claude-plugins-official"
		records, ok := installed[key]
		if !ok || len(records) == 0 {
			t.Fatal("code-simplifier not found in installed plugins")
		}

		record := records[0]
		t.Logf("Installed code-simplifier version: %s", record.Version)
		t.Logf("Installed at: %s", record.InstallPath)

		// Check cache directory exists
		if _, err := os.Stat(record.InstallPath); os.IsNotExist(err) {
			t.Errorf("Cache directory does not exist: %s", record.InstallPath)
		}
	})

	t.Run("VerifySymlinks", func(t *testing.T) {
		// Check if OpenCode config directory exists
		paths := configMgr.GetPaths()
		configDir := paths.OpenCodeConfig
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Fatalf("OpenCode config directory does not exist: %s", configDir)
		}

		// Check for skills directory
		skillsDir := filepath.Join(configDir, "skills")
		if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
			t.Logf("Skills directory does not exist (expected for code-simplifier)")
		} else {
			// List symlinks
			files, err := os.ReadDir(skillsDir)
			if err != nil {
				t.Fatalf("Failed to read skills directory: %v", err)
			}
			t.Logf("Found %d items in skills directory", len(files))
		}
	})

	t.Run("RemoveCodeSimplifier", func(t *testing.T) {
		// Create plugin installer
		installer := plugin.NewInstaller(configMgr)

		// Remove code-simplifier
		if err := installer.Remove("code-simplifier", "claude-plugins-official"); err != nil {
			t.Fatalf("Failed to remove code-simplifier: %v", err)
		}

		// Verify removal
		installed, err := installer.List()
		if err != nil {
			t.Fatalf("Failed to list installed plugins: %v", err)
		}

		key := "code-simplifier@claude-plugins-official"
		if _, ok := installed[key]; ok {
			t.Error("code-simplifier still in installed plugins after removal")
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		// Remove marketplace
		if err := configMgr.RemoveKnownMarket("claude-plugins-official"); err != nil {
			t.Logf("Warning: Failed to remove marketplace config: %v", err)
		}

		// Verify cleanup
		markets, err := configMgr.LoadKnownMarkets()
		if err != nil {
			t.Fatalf("Failed to load markets: %v", err)
		}

		if _, ok := markets["claude-plugins-official"]; ok {
			t.Error("Marketplace still exists in config after removal")
		}
	})
}
