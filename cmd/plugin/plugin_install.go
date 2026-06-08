package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/plugin"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install <plugin-name>[@<marketplace>] [--version <version>]",
	Short: "Install a plugin",
	Long: `Install a plugin from a marketplace.

Examples:
  opencode-plugin install test-plugin
  opencode-plugin install test-plugin@test
  opencode-plugin install test-plugin --version 1.0.0`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pluginSpec := args[0]
		version, _ := cmd.Flags().GetString("version")
		force, _ := cmd.Flags().GetBool("force")

		var pluginName, marketName string
		if idx := strings.LastIndex(pluginSpec, "@"); idx > 0 {
			pluginName = pluginSpec[:idx]
			marketName = pluginSpec[idx+1:]
		} else {
			pluginName = pluginSpec
		}

		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		installer := plugin.NewInstaller(configMgr)

		installed, err := installer.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to list installed plugins: %v\n", err)
			os.Exit(1)
		}

		disabledKeys := findDisabledInstallMatches(installed, pluginName, marketName, version)

		if len(disabledKeys) > 1 {
			fmt.Fprintf(os.Stderr, "Error: Multiple disabled installations of %s found:\n", pluginName)
			for _, k := range disabledKeys {
				fmt.Fprintf(os.Stderr, "  - %s\n", k)
			}
			fmt.Fprintf(os.Stderr, "\nPlease specify which one to enable:\n")
			fmt.Fprintf(os.Stderr, "  opencode-plugin plugin install %s\n", disabledKeys[0])
			os.Exit(1)
		}

		if len(disabledKeys) == 1 {
			existingKey := disabledKeys[0]
			if idx := strings.LastIndex(existingKey, "@"); idx > 0 {
				marketName = existingKey[idx+1:]
			}
			fmt.Printf("Plugin %s is installed but disabled. Enabling...\n", existingKey)
			if err := installer.Enable(pluginName, marketName, force); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		opts := plugin.InstallOptions{
			Version:    version,
			MarketName: marketName,
			Scope:      "user",
			Force:      force,
		}

		if err := installer.Install(pluginName, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func findDisabledInstallMatches(installed map[string][]config.InstallRecord, pluginName, marketName, version string) []string {
	var matches []string
	for key, records := range installed {
		if len(records) == 0 || !records[0].Disabled {
			continue
		}
		idx := strings.LastIndex(key, "@")
		if idx <= 0 || key[:idx] != pluginName {
			continue
		}
		if marketName != "" && key[idx+1:] != marketName {
			continue
		}
		if version != "" && records[0].Version != version {
			continue
		}
		matches = append(matches, key)
	}
	sort.Strings(matches)
	return matches
}

var removeCmd = &cobra.Command{
	Use:   "remove <plugin-name>[@<marketplace>]",
	Short: "Remove an installed plugin",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		installer := plugin.NewInstaller(configMgr)

		pluginName, marketName, resolved := resolveMarketName(installer, args[0], actionRemove)
		if !resolved {
			fmt.Fprintf(os.Stderr, "Error: Plugin %s not found in installed list\n", pluginName)
			os.Exit(1)
		}

		if err := installer.Remove(pluginName, marketName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

type pluginJSONEntry struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Marketplace string `json:"marketplace"`
	Version     string `json:"version"`
	InstallPath string `json:"installPath"`
	InstalledAt string `json:"installedAt"`
	Disabled    bool   `json:"disabled"`
}

var listJSONFlag bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	Run: func(cmd *cobra.Command, args []string) {
		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		installer := plugin.NewInstaller(configMgr)

		installed, err := installer.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to list installed plugins: %v\n", err)
			os.Exit(1)
		}

		if listJSONFlag {
			printPluginsJSON(installed)
			return
		}

		if len(installed) == 0 {
			fmt.Println("No plugins installed yet.")
			fmt.Println("\nUse 'opencode-plugin install <name>' to install a plugin.")
			return
		}

		fmt.Println("Installed Plugins:")
		fmt.Println()
		for key, records := range installed {
			if len(records) == 0 {
				continue
			}
			record := records[0]
			status := "enabled"
			if record.Disabled {
				status = "disabled"
			}
			fmt.Printf("  %s [%s]\n", key, status)
			fmt.Printf("    Version: %s\n", record.Version)
			fmt.Printf("    Scope: %s\n", record.Scope)
			fmt.Printf("    Path: %s\n", record.InstallPath)
			fmt.Printf("    Installed: %s\n", record.InstalledAt.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}
	},
}

func printPluginsJSON(installed map[string][]config.InstallRecord) {
	entries := make([]pluginJSONEntry, 0, len(installed))
	for key, records := range installed {
		if len(records) == 0 {
			continue
		}
		record := records[0]
		name := key
		market := ""
		if idx := strings.LastIndex(key, "@"); idx > 0 {
			name = key[:idx]
			market = key[idx+1:]
		}
		entry := pluginJSONEntry{
			Key:         key,
			Name:        name,
			Marketplace: market,
			Version:     record.Version,
			InstallPath: record.InstallPath,
			InstalledAt: record.InstalledAt.Format("2006-01-02T15:04:05Z07:00"),
			Disabled:    record.Disabled,
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(entries)
}
