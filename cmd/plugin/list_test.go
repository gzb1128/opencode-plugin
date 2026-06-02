package plugin

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencode/plugin-cli/internal/config"
)

func setupPluginListTest(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func capturePluginStdout(t *testing.T, fn func()) string {
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

func TestPluginListCmdJSONEmpty(t *testing.T) {
	setupPluginListTest(t)

	listCmd.Flags().Set("json", "true")
	defer listCmd.Flags().Set("json", "false")

	output := capturePluginStdout(t, func() {
		listCmd.Run(listCmd, nil)
	})

	var result []pluginJSONEntry
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput was: %q", err, output)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty array, got %d items", len(result))
	}
}

func TestPluginListCmdJSONWithData(t *testing.T) {
	setupPluginListTest(t)

	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatal(err)
	}

	record1 := &config.InstallRecord{
		Scope:       "user",
		InstallPath: filepath.Join("cache", "plugin-a@my-market", "1.0.0"),
		Version:     "1.0.0",
		InstalledAt: time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
	}
	if err := configMgr.AddInstallRecord("plugin-a@my-market", record1); err != nil {
		t.Fatal(err)
	}

	record2 := &config.InstallRecord{
		Scope:       "user",
		InstallPath: filepath.Join("cache", "plugin-b@other-market", "2.0.0"),
		Version:     "2.0.0",
		InstalledAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
	}
	if err := configMgr.AddInstallRecord("plugin-b@other-market", record2); err != nil {
		t.Fatal(err)
	}

	listCmd.Flags().Set("json", "true")
	defer listCmd.Flags().Set("json", "false")

	output := capturePluginStdout(t, func() {
		listCmd.Run(listCmd, nil)
	})

	var result []pluginJSONEntry
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput was: %q", err, output)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}

	if result[0].Key != "plugin-a@my-market" {
		t.Errorf("expected first key 'plugin-a@my-market', got %q", result[0].Key)
	}
	if result[0].Name != "plugin-a" {
		t.Errorf("expected first name 'plugin-a', got %q", result[0].Name)
	}
	if result[0].Marketplace != "my-market" {
		t.Errorf("expected first marketplace 'my-market', got %q", result[0].Marketplace)
	}
	if result[0].Version != "1.0.0" {
		t.Errorf("expected first version '1.0.0', got %q", result[0].Version)
	}
	if result[0].InstalledAt == "" {
		t.Error("expected installedAt to be non-empty")
	}

	if result[1].Key != "plugin-b@other-market" {
		t.Errorf("expected second key 'plugin-b@other-market', got %q", result[1].Key)
	}
}

func TestPluginListCmdJSONScopedPluginName(t *testing.T) {
	setupPluginListTest(t)

	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatal(err)
	}

	record := &config.InstallRecord{
		Scope:       "user",
		InstallPath: filepath.Join("cache", "scope-tool", "1.0.0"),
		Version:     "1.0.0",
		InstalledAt: time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
	}
	if err := configMgr.AddInstallRecord("@scope/tool@my-market", record); err != nil {
		t.Fatal(err)
	}

	listCmd.Flags().Set("json", "true")
	defer listCmd.Flags().Set("json", "false")

	output := capturePluginStdout(t, func() {
		listCmd.Run(listCmd, nil)
	})

	var result []pluginJSONEntry
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput was: %q", err, output)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Name != "@scope/tool" {
		t.Errorf("expected scoped plugin name, got %q", result[0].Name)
	}
	if result[0].Marketplace != "my-market" {
		t.Errorf("expected marketplace my-market, got %q", result[0].Marketplace)
	}
}

func TestPluginListCmdHumanEmpty(t *testing.T) {
	setupPluginListTest(t)

	listCmd.Flags().Set("json", "false")

	output := capturePluginStdout(t, func() {
		listCmd.Run(listCmd, nil)
	})

	if !bytes.Contains([]byte(output), []byte("No plugins installed yet")) {
		t.Errorf("expected human-readable empty message, got: %q", output)
	}
}
