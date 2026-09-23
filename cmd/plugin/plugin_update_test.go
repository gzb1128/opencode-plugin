package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	plugininstaller "github.com/opencode/plugin-cli/internal/plugin"
)

func TestUpdateCommandSkipsDisabledPluginsDuringUpdateAll(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	marketPath := filepath.Join(homeDir, "market")
	writeUpdateTestPlugin(t, marketPath, "enabled-plugin", "1.0.0")
	writeUpdateTestPlugin(t, marketPath, "disabled-plugin", "1.0.0")
	writeUpdateTestMarketplace(t, marketPath)

	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatalf("config.NewManager() error = %v", err)
	}
	source := &marketplace.LocalMarketSource{Path: marketPath}
	source.SetInstallLocation(marketPath)
	if err := configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(source)); err != nil {
		t.Fatalf("AddKnownMarket() error = %v", err)
	}

	installer := plugininstaller.NewInstaller(configMgr)
	for _, pluginName := range []string{"enabled-plugin", "disabled-plugin"} {
		err := installer.Install(pluginName, plugininstaller.InstallOptions{
			MarketName: "test-market",
			Scope:      "user",
		})
		if err != nil {
			t.Fatalf("Install(%q) error = %v", pluginName, err)
		}
	}
	if err := installer.Disable("disabled-plugin", "test-market", false); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	writeUpdateTestPlugin(t, marketPath, "enabled-plugin", "2.0.0")
	writeUpdateTestPlugin(t, marketPath, "disabled-plugin", "2.0.0")

	var runErr error
	output := capturePluginStdout(t, func() {
		Cmd.SetArgs([]string{"update"})
		runErr = Cmd.Execute()
	})

	if runErr != nil {
		t.Fatalf("plugin Cmd.Execute() error = %v\noutput:\n%s", runErr, output)
	}
	if !strings.Contains(output, "Skipping disabled-plugin@test-market (disabled)") {
		t.Fatalf("command output did not skip the disabled plugin:\n%s", output)
	}
	if strings.Contains(output, "Updating disabled-plugin@test-market...") {
		t.Fatalf("command attempted to update the disabled plugin:\n%s", output)
	}
	if !strings.Contains(output, "✓ Updated 1 plugins, 1 skipped, 0 failed") {
		t.Fatalf("command summary did not report one update and one skip:\n%s", output)
	}

	enabledRecord, err := configMgr.GetInstallRecord("enabled-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord(enabled) error = %v", err)
	}
	if enabledRecord.Version != "2.0.0" {
		t.Errorf("enabled plugin version = %q, want 2.0.0", enabledRecord.Version)
	}

	disabledRecord, err := configMgr.GetInstallRecord("disabled-plugin@test-market")
	if err != nil {
		t.Fatalf("GetInstallRecord(disabled) error = %v", err)
	}
	if disabledRecord.Version != "1.0.0" {
		t.Errorf("disabled plugin version = %q, want unchanged 1.0.0", disabledRecord.Version)
	}
	disabledSkill := filepath.Join(disabledRecord.InstallPath, "skills", "disabled-plugin.md")
	got, err := os.ReadFile(disabledSkill)
	if err != nil {
		t.Fatalf("ReadFile(disabled skill) error = %v", err)
	}
	if string(got) != "# disabled-plugin v1.0.0 skill" {
		t.Errorf("disabled plugin cache was updated; skill content = %q", string(got))
	}
}

func writeUpdateTestPlugin(t *testing.T, marketPath, pluginName, version string) {
	t.Helper()

	pluginPath := filepath.Join(marketPath, "plugins", pluginName)
	skillsDir := filepath.Join(pluginPath, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", skillsDir, err)
	}
	skillPath := filepath.Join(skillsDir, pluginName+".md")
	content := fmt.Sprintf("# %s v%s skill", pluginName, version)
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", skillPath, err)
	}

	manifestDir := filepath.Join(pluginPath, ".claude-plugin")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", manifestDir, err)
	}
	manifest := fmt.Sprintf(`{"name":%q,"version":%q}`, pluginName, version)
	manifestPath := filepath.Join(manifestDir, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", manifestPath, err)
	}
}

func writeUpdateTestMarketplace(t *testing.T, marketPath string) {
	t.Helper()

	manifest := `{
  "name": "test-market",
  "plugins": [
    {"name": "enabled-plugin", "source": "./plugins/enabled-plugin"},
    {"name": "disabled-plugin", "source": "./plugins/disabled-plugin"}
  ]
}`
	manifestPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(manifestPath), err)
	}
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", manifestPath, err)
	}
}
