package plugin

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
)

func TestInfoCmd_OutputIncludesDisplayName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	marketPath := filepath.Join(tmpDir, "market")
	pluginSrcPath := filepath.Join(marketPath, "plugins", "tool")
	os.MkdirAll(pluginSrcPath, 0755)
	os.WriteFile(filepath.Join(pluginSrcPath, "skill.md"), []byte("# Skill"), 0644)

	manifestDir := filepath.Join(pluginSrcPath, ".claude-plugin")
	os.MkdirAll(manifestDir, 0755)
	os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"tool","version":"1.0.0"}`), 0644)

	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "tool",
      "displayName": "Tool Pro",
      "description": "A tool plugin",
      "version": "1.0.0",
      "source": "./plugins/tool"
    }
  ]
}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0644)

	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatal(err)
	}

	src := &marketplace.LocalMarketSource{Path: marketPath}
	src.SetInstallLocation(marketPath)
	if err := configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(src)); err != nil {
		t.Fatal(err)
	}

	output := captureInfoStdout(t, func() {
		infoCmd.Run(infoCmd, []string{"tool@test-market"})
	})

	if !strings.Contains(output, "Display Name: Tool Pro") {
		t.Errorf("expected 'Display Name: Tool Pro' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Plugin: tool") {
		t.Errorf("expected 'Plugin: tool' in output, got:\n%s", output)
	}
}

func TestInfoCmd_OutputOmitsDisplayNameWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	marketPath := filepath.Join(tmpDir, "market")
	pluginSrcPath := filepath.Join(marketPath, "plugins", "plain")
	os.MkdirAll(pluginSrcPath, 0755)
	os.WriteFile(filepath.Join(pluginSrcPath, "skill.md"), []byte("# Skill"), 0644)

	manifestDir := filepath.Join(pluginSrcPath, ".claude-plugin")
	os.MkdirAll(manifestDir, 0755)
	os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"plain","version":"1.0.0"}`), 0644)

	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "plain",
      "description": "A plain plugin",
      "source": "./plugins/plain"
    }
  ]
}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0644)

	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatal(err)
	}

	src := &marketplace.LocalMarketSource{Path: marketPath}
	src.SetInstallLocation(marketPath)
	if err := configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(src)); err != nil {
		t.Fatal(err)
	}

	output := captureInfoStdout(t, func() {
		infoCmd.Run(infoCmd, []string{"plain@test-market"})
	})

	if strings.Contains(output, "Display Name:") {
		t.Errorf("expected no 'Display Name' when empty, got:\n%s", output)
	}
}

func captureInfoStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	w.Close()
	return <-done
}

func TestInfoCmd_DisplayNameDoesNotChangePluginID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	marketPath := filepath.Join(tmpDir, "market")
	pluginSrcPath := filepath.Join(marketPath, "plugins", "tool")
	os.MkdirAll(pluginSrcPath, 0755)
	os.WriteFile(filepath.Join(pluginSrcPath, "skill.md"), []byte("# Skill"), 0644)

	manifestDir := filepath.Join(pluginSrcPath, ".claude-plugin")
	os.MkdirAll(manifestDir, 0755)
	os.WriteFile(filepath.Join(manifestDir, "plugin.json"), []byte(`{"name":"tool","version":"1.0.0"}`), 0644)

	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "tool",
      "displayName": "Tool Pro",
      "description": "A tool plugin",
      "source": "./plugins/tool"
    }
  ]
}`

	indexPath := filepath.Join(marketPath, ".claude-plugin", "marketplace.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte(marketJSON), 0644)

	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatal(err)
	}

	src := &marketplace.LocalMarketSource{Path: marketPath}
	src.SetInstallLocation(marketPath)
	if err := configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(src)); err != nil {
		t.Fatal(err)
	}

	output := captureInfoStdout(t, func() {
		infoCmd.Run(infoCmd, []string{"tool@test-market"})
	})

	if !strings.Contains(output, "Plugin: tool\n") {
		t.Errorf("plugin ID should still be 'tool', got:\n%s", output)
	}
}

func init() {
	_ = json.Marshal
}
