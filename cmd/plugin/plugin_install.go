package plugin

import (
	"fmt"
	"os"
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

		// Parse plugin spec (name@market)
		var pluginName, marketName string
		if idx := strings.Index(pluginSpec, "@"); idx > 0 {
			pluginName = pluginSpec[:idx]
			marketName = pluginSpec[idx+1:]
		} else {
			pluginName = pluginSpec
			// Market name will be auto-detected
		}

		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		installer := plugin.NewInstaller(configMgr)

		opts := plugin.InstallOptions{
			Version:    version,
			MarketName: marketName,
			Scope:      "user",
		}

		if err := installer.Install(pluginName, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <plugin-name>[@<marketplace>]",
	Short: "Remove an installed plugin",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pluginSpec := args[0]

		// Parse plugin spec
		var pluginName, marketName string
		if idx := strings.Index(pluginSpec, "@"); idx >= 0 {
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

		// If market name not specified (no @ in spec), try to find the plugin
		if !strings.Contains(pluginSpec, "@") {
			installed, err := installer.List()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to list installed plugins: %v\n", err)
				os.Exit(1)
			}

			// Find all matching plugins
			var matches []string
			for key := range installed {
				if strings.HasPrefix(key, pluginName+"@") {
					matches = append(matches, key)
				}
			}

			// If multiple matches, show them to user
			if len(matches) > 1 {
				fmt.Fprintf(os.Stderr, "Error: Multiple installations of %s found:\n", pluginName)
				for _, match := range matches {
					fmt.Fprintf(os.Stderr, "  - %s\n", match)
				}
				fmt.Fprintf(os.Stderr, "\nPlease specify which one to remove:\n")
				fmt.Fprintf(os.Stderr, "  opencode-plugin plugin remove %s\n", matches[0])
				os.Exit(1)
			}

			// If exactly one match, use it
			if len(matches) == 1 {
				key := matches[0]
				marketName = strings.TrimPrefix(key, pluginName+"@")
			}
		}

		if err := installer.Remove(pluginName, marketName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

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
			fmt.Printf("  %s\n", key)
			fmt.Printf("    Version: %s\n", record.Version)
			fmt.Printf("    Scope: %s\n", record.Scope)
			fmt.Printf("    Path: %s\n", record.InstallPath)
			fmt.Printf("    Installed: %s\n", record.InstalledAt.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}
	},
}

func init() {
	installCmd.Flags().StringP("version", "v", "", "Plugin version to install")

	Cmd.AddCommand(installCmd)
	Cmd.AddCommand(removeCmd)
	Cmd.AddCommand(listCmd)
}
