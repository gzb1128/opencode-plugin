package plugin

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/plugin"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update [<plugin-name>[@<marketplace>]]",
	Short: "Update installed plugins",
	Long: `Update one or all installed plugins to their latest versions.

If no plugin is specified, updates all installed plugins.

Examples:
  opencode-plugin plugin update
  opencode-plugin plugin update my-plugin
  opencode-plugin plugin update my-plugin@my-market`,
	Args: cobra.MaximumNArgs(1),
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
			fmt.Println("No plugins installed.")
			return
		}

		if len(args) == 0 {
			// Update all plugins
			fmt.Printf("Updating all installed plugins (%d)...\n\n", len(installed))
			updated := 0
			failed := 0

			for key, records := range installed {
				if len(records) == 0 {
					continue
				}

				// Parse plugin name and market
				parts := strings.Split(key, "@")
				if len(parts) != 2 {
					continue
				}
				pluginName := parts[0]
				marketName := parts[1]

				fmt.Printf("Updating %s...\n", key)
				if err := updatePlugin(installer, configMgr, pluginName, marketName); err != nil {
					fmt.Fprintf(os.Stderr, "  Error: %v\n\n", err)
					failed++
				} else {
					updated++
				}
			}

			fmt.Printf("\n✓ Updated %d plugins, %d failed\n", updated, failed)
		} else {
			// Update specific plugin
			pluginSpec := args[0]

			var pluginName, marketName string
			if idx := strings.Index(pluginSpec, "@"); idx > 0 {
				pluginName = pluginSpec[:idx]
				marketName = pluginSpec[idx+1:]
			} else {
				pluginName = pluginSpec
				// Find the plugin in installed list
				for key := range installed {
					if strings.HasPrefix(key, pluginName+"@") {
						parts := strings.Split(key, "@")
						marketName = parts[1]
						break
					}
				}
			}

			if marketName == "" {
				fmt.Fprintf(os.Stderr, "Error: Plugin %s not found in installed list\n", pluginName)
				os.Exit(1)
			}

			if err := updatePlugin(installer, configMgr, pluginName, marketName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("\n✓ Successfully updated plugin: %s@%s\n", pluginName, marketName)
		}
	},
}

func updatePlugin(installer *plugin.Installer, configMgr *config.Manager, pluginName, marketName string) error {
	// Remove old version
	if err := installer.Remove(pluginName, marketName); err != nil {
		return fmt.Errorf("failed to remove old version: %w", err)
	}

	// Install latest version
	opts := plugin.InstallOptions{
		MarketName: marketName,
		Version:    "", // Empty means latest
		Scope:      "user",
	}

	if err := installer.Install(pluginName, opts); err != nil {
		return fmt.Errorf("failed to install new version: %w", err)
	}

	return nil
}

func init() {
	Cmd.AddCommand(updateCmd)
}
