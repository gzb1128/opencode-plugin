package plugin

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage plugins",
	Long:  "Install, list, update, and remove plugins from marketplaces",
}

func init() {
	installCmd.Flags().StringP("version", "v", "", "Plugin version to install")
	installCmd.Flags().BoolP("force", "f", false, "Force overwrite existing skills, commands, and agents")
	listCmd.Flags().BoolVar(&listJSONFlag, "json", false, "Output as JSON")
	updateCmd.Flags().BoolP("force", "f", false, "Force overwrite existing skills, commands, and agents")
	searchCmd.Flags().StringP("market", "m", "", "Search in specific marketplace")
	enableCmd.Flags().BoolP("force", "f", false, "Force overwrite existing skills, commands, and agents")

	Cmd.AddCommand(installCmd)
	Cmd.AddCommand(removeCmd)
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(infoCmd)
	Cmd.AddCommand(searchCmd)
	Cmd.AddCommand(disableCmd)
	Cmd.AddCommand(enableCmd)
}
