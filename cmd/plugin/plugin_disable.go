package plugin

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/plugin"
	"github.com/spf13/cobra"
)

var disableCmd = &cobra.Command{
	Use:   "disable <plugin-name>[@<marketplace>]",
	Short: "Disable an installed plugin",
	Long: `Disable an installed plugin without removing it.

The plugin's symlinks and MCP servers will be deactivated, but the
installation record and cached files are preserved. Use 'plugin enable'
to reactivate the plugin.

Examples:
  opencode-plugin plugin disable superpowers
  opencode-plugin plugin disable superpowers@official`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pluginSpec := args[0]

		var pluginName, marketName string
		if idx := strings.Index(pluginSpec, "@"); idx > 0 {
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

		if marketName == "" {
			installed, err := installer.List()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to list installed plugins: %v\n", err)
				os.Exit(1)
			}

			var matches []string
			for key := range installed {
				if strings.HasPrefix(key, pluginName+"@") {
					matches = append(matches, key)
				}
			}

			if len(matches) > 1 {
				fmt.Fprintf(os.Stderr, "Error: Multiple installations of %s found:\n", pluginName)
				for _, match := range matches {
					fmt.Fprintf(os.Stderr, "  - %s\n", match)
				}
				fmt.Fprintf(os.Stderr, "\nPlease specify which one to disable:\n")
				fmt.Fprintf(os.Stderr, "  opencode-plugin plugin disable %s\n", matches[0])
				os.Exit(1)
			}

			if len(matches) == 1 {
				key := matches[0]
				marketName = strings.TrimPrefix(key, pluginName+"@")
			}
		}

		if marketName == "" {
			fmt.Fprintf(os.Stderr, "Error: Plugin %s not found in installed list\n", pluginName)
			os.Exit(1)
		}

		if err := installer.Disable(pluginName, marketName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}
