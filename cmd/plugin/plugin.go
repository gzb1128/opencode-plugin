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
	// Commands will be added in their respective files
}
