package plugin

import (
	"fmt"
	"os"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/plugin"
	"github.com/spf13/cobra"
)

var enableCmd = &cobra.Command{
	Use:   "enable <plugin-name>[@<marketplace>]",
	Short: "Enable a disabled plugin",
	Long: `Re-enable a previously disabled plugin.

Restores the plugin's symlinks and MCP servers from the cached files.

Examples:
  opencode-plugin plugin enable superpowers
  opencode-plugin plugin enable superpowers@official
  opencode-plugin plugin enable superpowers --force`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")

		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		installer := plugin.NewInstaller(configMgr)

		pluginName, marketName, resolved := resolveMarketName(installer, args[0], actionEnable)
		if !resolved {
			fmt.Fprintf(os.Stderr, "Error: Plugin %s not found in installed list\n", pluginName)
			os.Exit(1)
		}

		if err := installer.Enable(pluginName, marketName, force); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}
