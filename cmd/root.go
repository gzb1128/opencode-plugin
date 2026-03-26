package cmd

import (
	"fmt"
	"os"

	"github.com/opencode/plugin-cli/cmd/market"
	"github.com/opencode/plugin-cli/cmd/mcp"
	"github.com/opencode/plugin-cli/cmd/plugin"
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
)

var rootCmd = &cobra.Command{
	Use:   "opencode-plugin",
	Short: "OpenCode Plugin CLI - Manage OpenCode plugins and marketplaces",
	Long: `OpenCode Plugin CLI is a command-line tool for managing OpenCode plugins.

It allows you to:
- Add and manage plugin marketplaces
- Install, update, and remove plugins
- Search for available plugins
- Manage MCP (Model Context Protocol) servers
- Diagnose plugin issues`,
	Version: version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(market.Cmd)
	rootCmd.AddCommand(plugin.Cmd)
	rootCmd.AddCommand(mcp.Cmd)
}
