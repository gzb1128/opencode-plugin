package plugin

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <keyword> [--market <name>]",
	Short: "Search for plugins in marketplaces",
	Long: `Search for plugins by name or description across all marketplaces or a specific one.

Examples:
  opencode-plugin plugin search code
  opencode-plugin plugin search test --market claude-plugins-official
  opencode-plugin plugin search --market my-market  # List all plugins in a market`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		marketName, _ := cmd.Flags().GetString("market")

		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		markets, err := configMgr.LoadKnownMarkets()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to load marketplaces: %v\n", err)
			os.Exit(1)
		}

		if len(markets) == 0 {
			fmt.Println("No marketplaces added yet.")
			fmt.Println("\nUse 'opencode-plugin market add <url>' to add a marketplace.")
			return
		}

		keyword := ""
		if len(args) > 0 {
			keyword = strings.ToLower(args[0])
		}

		paths := configMgr.GetPaths()
		mgr := marketplace.NewManager(paths.MarketsDir)

		totalPlugins := 0
		foundMarkets := 0
		skippedMarkets := []string{}

		for mName, market := range markets {
			// Filter by market if specified
			if marketName != "" && mName != marketName {
				continue
			}

			installLoc, ok := market["installLocation"].(string)
			if !ok {
				continue
			}

			mp, err := mgr.Get(installLoc)
			if err != nil {
				// Marketplace not cloned yet, track it
				skippedMarkets = append(skippedMarkets, mName)
				continue
			}

			var plugins []marketplace.Plugin

			// Filter by keyword
			if keyword != "" {
				for _, p := range mp.Plugins {
					if strings.Contains(strings.ToLower(p.Name), keyword) ||
						strings.Contains(strings.ToLower(p.Description), keyword) {
						plugins = append(plugins, p)
					}
				}
			} else {
				plugins = mp.Plugins
			}

			if len(plugins) == 0 {
				continue
			}

			totalPlugins += len(plugins)
			foundMarkets++

			fmt.Printf("\nMarketplace: %s (%d plugins)\n", mName, len(plugins))
			fmt.Println(strings.Repeat("-", 80))

			for _, p := range plugins {
				fmt.Printf("\n%s\n", p.Name)
				fmt.Printf("  %s\n", p.Description)
				if p.Version != "" {
					fmt.Printf("  Version: %s\n", p.Version)
				}
				if p.Category != "" {
					fmt.Printf("  Category: %s\n", p.Category)
				}
				if p.Author != nil && p.Author.Name != "" {
					fmt.Printf("  Author: %s", p.Author.Name)
					if p.Author.Email != "" {
						fmt.Printf(" <%s>", p.Author.Email)
					}
					fmt.Println()
				}
				if p.Homepage != "" {
					fmt.Printf("  Homepage: %s\n", p.Homepage)
				}
				if len(p.Keywords) > 0 {
					fmt.Printf("  Keywords: %s\n", strings.Join(p.Keywords, ", "))
				}
			}
		}

		if foundMarkets == 0 {
			if keyword != "" {
				fmt.Printf("No plugins found matching '%s'\n", keyword)
			} else if marketName != "" {
				fmt.Printf("Marketplace '%s' not found or not cloned yet\n", marketName)
			}

			// Show skipped markets
			if len(skippedMarkets) > 0 {
				fmt.Printf("\nNote: %d marketplace(s) not cloned yet:\n", len(skippedMarkets))
				for _, name := range skippedMarkets {
					fmt.Printf("  - %s\n", name)
				}
				fmt.Println("\nUse 'opencode-plugin market update' to clone them.")
			}
		} else {
			fmt.Printf("\nTotal: %d plugins in %d marketplace(s)\n", totalPlugins, foundMarkets)

			// Show skipped markets
			if len(skippedMarkets) > 0 {
				fmt.Printf("\nNote: %d marketplace(s) not cloned yet:\n", len(skippedMarkets))
				for _, name := range skippedMarkets {
					fmt.Printf("  - %s\n", name)
				}
				fmt.Println("\nUse 'opencode-plugin market update' to clone them.")
			}
		}
	},
}

func init() {
	searchCmd.Flags().StringP("market", "m", "", "Search in specific marketplace")
	Cmd.AddCommand(searchCmd)
}
