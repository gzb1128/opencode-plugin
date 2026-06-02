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

func TestSearchCmd_OutputIncludesDisplayName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	marketPath := filepath.Join(tmpDir, "market")
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

	searchCmd.Flags().Set("market", "test-market")
	defer searchCmd.Flags().Set("market", "")

	output := captureSearchStdout(t, func() {
		searchCmd.Run(searchCmd, nil)
	})

	if !strings.Contains(output, "Display Name: Tool Pro") {
		t.Errorf("expected 'Display Name: Tool Pro' in output, got:\n%s", output)
	}
}

func TestSearchCmd_OutputOmitsDisplayNameWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	marketPath := filepath.Join(tmpDir, "market")
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

	searchCmd.Flags().Set("market", "test-market")
	defer searchCmd.Flags().Set("market", "")

	output := captureSearchStdout(t, func() {
		searchCmd.Run(searchCmd, nil)
	})

	if strings.Contains(output, "Display Name:") {
		t.Errorf("expected no 'Display Name' when empty, got:\n%s", output)
	}
}

func TestSearchCmd_MatchesByDisplayName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	marketPath := filepath.Join(tmpDir, "market")
	marketJSON := `{
  "name": "test-market",
  "plugins": [
    {
      "name": "tool",
      "displayName": "Tool Pro",
      "description": "A tool plugin",
      "source": "./plugins/tool"
    },
    {
      "name": "other",
      "description": "Something else",
      "source": "./plugins/other"
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

	searchCmd.Flags().Set("market", "test-market")
	defer searchCmd.Flags().Set("market", "")

	output := captureSearchStdout(t, func() {
		searchCmd.Run(searchCmd, []string{"pro"})
	})

	if !strings.Contains(output, "tool") {
		t.Errorf("expected 'tool' in search results for 'pro', got:\n%s", output)
	}
	if strings.Contains(output, "other") {
		t.Errorf("did not expect 'other' in search results for 'pro', got:\n%s", output)
	}
}

func captureSearchStdout(t *testing.T, fn func()) string {
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

func init() {
	_ = json.Marshal
}
