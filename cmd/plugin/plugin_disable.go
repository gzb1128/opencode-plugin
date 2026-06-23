package plugin

import (
	"fmt"
	"os"

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
		force, _ := cmd.Flags().GetBool("force")
		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		installer := plugin.NewInstaller(configMgr)

		pluginName, marketName, resolved := resolveMarketName(installer, args[0], actionDisable)
		if !resolved {
			fmt.Fprintf(os.Stderr, "Error: Plugin %s not found in installed list\n", pluginName)
			os.Exit(1)
		}

		if err := installer.Disable(pluginName, marketName, force); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}
