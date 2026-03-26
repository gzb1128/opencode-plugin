package mcp

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/mcp"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP (Model Context Protocol) servers",
	Long: `Manage MCP servers for Claude Code plugins.

MCP servers allow plugins to integrate with external services like databases,
APIs, file systems, and more. They provide structured tool access within Claude Code.`,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed MCP servers",
	Long:  "List all MCP servers installed by plugins",
	Run: func(cmd *cobra.Command, args []string) {
		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		paths := configMgr.GetPaths()
		mgr := mcp.NewManager(paths.OpenCodeConfig)

		servers, err := mgr.ListMCPServers()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to list MCP servers: %v\n", err)
			os.Exit(1)
		}

		if len(servers) == 0 {
			fmt.Println("No MCP servers installed.")
			fmt.Println("\nMCP servers are installed automatically when you install a plugin that includes them.")
			fmt.Println("Use 'opencode-plugin plugin install <name>' to install a plugin.")
			return
		}

		fmt.Println("Installed MCP Servers:")
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tCOMMAND/URL")

		for name, server := range servers {
			var info string
			if server.Command != "" {
				info = server.Command
			} else if server.URL != "" {
				info = server.URL
			} else {
				info = "N/A"
			}

			serverType := server.Type
			if serverType == "" {
				serverType = "stdio"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\n", name, serverType, info)
		}

		w.Flush()

		fmt.Println()
		fmt.Printf("Total: %d MCP server(s)\n", len(servers))
	},
}

var showCmd = &cobra.Command{
	Use:   "show <server-name>",
	Short: "Show details of an MCP server",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		serverName := args[0]

		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		paths := configMgr.GetPaths()
		mgr := mcp.NewManager(paths.OpenCodeConfig)

		servers, err := mgr.ListMCPServers()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to list MCP servers: %v\n", err)
			os.Exit(1)
		}

		server, ok := servers[serverName]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: MCP server '%s' not found\n", serverName)
			os.Exit(1)
		}

		fmt.Printf("MCP Server: %s\n", serverName)
		fmt.Println()

		serverType := server.Type
		if serverType == "" {
			serverType = "stdio"
		}
		fmt.Printf("Type: %s\n", serverType)

		if server.Command != "" {
			fmt.Printf("Command: %s\n", server.Command)
		}

		if server.URL != "" {
			fmt.Printf("URL: %s\n", server.URL)
		}

		if len(server.Args) > 0 {
			fmt.Printf("Arguments:\n")
			for _, arg := range server.Args {
				fmt.Printf("  - %s\n", arg)
			}
		}

		if len(server.Env) > 0 {
			fmt.Printf("Environment Variables:\n")
			for key, value := range server.Env {
				fmt.Printf("  %s=%s\n", key, value)
			}
		}
	},
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(showCmd)
}
