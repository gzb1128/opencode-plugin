package market

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode/plugin-cli/internal/config"
)

func setupListTest(t *testing.T) string {
	t.Helper()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	return homeDir
}

func captureStdout(t *testing.T, fn func()) string {
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

func TestListCmdJSONEmpty(t *testing.T) {
	setupListTest(t)

	listCmd.Flags().Set("json", "true")
	defer listCmd.Flags().Set("json", "false")

	output := captureStdout(t, func() {
		listCmd.Run(listCmd, nil)
	})

	var result []marketJSONEntry
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput was: %q", err, output)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty array, got %d items", len(result))
	}
}

func TestListCmdJSONWithData(t *testing.T) {
	homeDir := setupListTest(t)

	configMgr, err := config.NewManager()
	if err != nil {
		t.Fatal(err)
	}
	markets := config.KnownMarkets{
		"beta-market": {
			"source":          "github",
			"repo":            "org/beta",
			"url":             "https://github.com/org/beta.git",
			"installLocation": filepath.Join(homeDir, ".opencode-plugin-cli", "markets", "beta-market"),
			"lastUpdated":     "2026-06-01T10:00:00Z",
		},
		"alpha-market": {
			"source":          "github",
			"repo":            "org/alpha",
			"url":             "https://github.com/org/alpha.git",
			"installLocation": filepath.Join(homeDir, ".opencode-plugin-cli", "markets", "alpha-market"),
			"lastUpdated":     "2026-06-02T12:00:00Z",
		},
	}
	if err := configMgr.SaveKnownMarkets(markets); err != nil {
		t.Fatal(err)
	}

	listCmd.Flags().Set("json", "true")
	defer listCmd.Flags().Set("json", "false")

	output := captureStdout(t, func() {
		listCmd.Run(listCmd, nil)
	})

	var result []marketJSONEntry
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput was: %q", err, output)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}

	if result[0].Name != "alpha-market" {
		t.Errorf("expected first entry name 'alpha-market', got %q", result[0].Name)
	}
	if result[1].Name != "beta-market" {
		t.Errorf("expected second entry name 'beta-market', got %q", result[1].Name)
	}

	if result[0].SourceType != "github" {
		t.Errorf("expected sourceType 'github', got %q", result[0].SourceType)
	}
	if result[0].Repo != "org/alpha" {
		t.Errorf("expected repo 'org/alpha', got %q", result[0].Repo)
	}
	if result[0].LastUpdated != "2026-06-02T12:00:00Z" {
		t.Errorf("expected lastUpdated '2026-06-02T12:00:00Z', got %q", result[0].LastUpdated)
	}
}

func TestListCmdHumanEmpty(t *testing.T) {
	setupListTest(t)

	listCmd.Flags().Set("json", "false")

	output := captureStdout(t, func() {
		listCmd.Run(listCmd, nil)
	})

	if !bytes.Contains([]byte(output), []byte("No marketplaces added yet")) {
		t.Errorf("expected human-readable empty message, got: %q", output)
	}
}
