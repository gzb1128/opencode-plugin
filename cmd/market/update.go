package market

import (
	"fmt"
	"os"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update a marketplace",
	Long: `Update a marketplace to get the latest plugins.

If no name is specified, updates all marketplaces.

Examples:
  opencode-plugin market update
  opencode-plugin market update my-market`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
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
			fmt.Println("No marketplaces to update.")
			return
		}

		paths := configMgr.GetPaths()
		mgr := marketplace.NewManager(paths.MarketsDir)

		if len(args) == 0 {
			// Update all marketplaces
			fmt.Printf("Updating all marketplaces (%d)...\n\n", len(markets))
			updated := 0
			for name := range markets {
				if err := updateMarket(mgr, configMgr, name, markets); err != nil {
					fmt.Fprintf(os.Stderr, "Error updating %s: %v\n\n", name, err)
				} else {
					updated++
				}
			}
			fmt.Printf("\n✓ Updated %d/%d marketplaces\n", updated, len(markets))
		} else {
			// Update specific marketplace
			name := args[0]
			if err := updateMarket(mgr, configMgr, name, markets); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("\n✓ Successfully updated marketplace: %s\n", name)
		}
	},
}

func updateMarket(mgr *marketplace.Manager, configMgr *config.Manager, name string, markets config.KnownMarkets) error {
	market, ok := markets[name]
	if !ok {
		return fmt.Errorf("marketplace %s not found", name)
	}

	marketType, _ := market["source"].(string)
	if isLocalMarketType(marketType) {
		fmt.Printf("Skipping %s (local marketplace)\n", name)
		return nil
	}

	fmt.Printf("Updating %s...\n", name)

	url := getMarketURL(market)

	mp, resultSource, err := mgr.Add(name, url)
	if err != nil {
		return err
	}

	installLoc := resultSource.InstallLocation()

	marketCfg := marketplace.MarketSourceToConfig(resultSource)
	marketCfg["installLocation"] = installLoc

	preserveConfigFields(market, marketCfg)

	if err := configMgr.AddKnownMarket(name, marketCfg); err != nil {
		return fmt.Errorf("failed to update marketplace config: %w", err)
	}

	fmt.Printf("  %d plugins available\n", len(mp.Plugins))
	return nil
}

func getMarketURL(market map[string]interface{}) string {
	source, _ := market["source"].(string)
	switch source {
	case "github":
		if repo, ok := market["repo"].(string); ok && repo != "" {
			return repo
		}
		if url, ok := market["url"].(string); ok && url != "" {
			return url
		}
	case "file", "directory", "local":
		if path, ok := market["path"].(string); ok && path != "" {
			return path
		}
	default:
		if url, ok := market["url"].(string); ok && url != "" {
			return url
		}
		if repo, ok := market["repo"].(string); ok && repo != "" {
			return repo
		}
		if path, ok := market["path"].(string); ok && path != "" {
			return path
		}
	}
	return ""
}

func preserveConfigFields(orig, cfg map[string]interface{}) {
	for _, key := range []string{"ref", "sparsePaths", "headers"} {
		if v, ok := orig[key]; ok && v != nil {
			cfg[key] = v
		}
	}
}

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a marketplace",
	Long: `Remove a marketplace from the list.

This will NOT uninstall plugins from this marketplace.
Use 'plugin remove' to uninstall plugins first.

Examples:
  opencode-plugin market remove my-market`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

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

		market, ok := markets[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: Marketplace %s not found\n", name)
			os.Exit(1)
		}

		// Check if it's a local marketplace
		marketType, _ := market["source"].(string)
		installLoc, _ := market["installLocation"].(string)

		// Ask for confirmation
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("This will remove marketplace: %s\n", name)
			fmt.Printf("Location: %s\n", installLoc)
			fmt.Printf("\nAre you sure? (y/n): ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Aborted.")
				return
			}
		}

		// Remove directory if not local
		if !isLocalMarketType(marketType) {
			paths := configMgr.GetPaths()
			mgr := marketplace.NewManager(paths.MarketsDir)
			if err := mgr.Remove(name); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to remove marketplace directory: %v\n", err)
			} else {
				fmt.Printf("✓ Removed marketplace directory: %s\n", installLoc)
			}
		}

		// Remove from config
		if err := configMgr.RemoveKnownMarket(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to remove from config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Successfully removed marketplace: %s\n", name)
		fmt.Println("\nNote: Installed plugins from this marketplace were NOT removed.")
		fmt.Println("Use 'plugin remove <name>@<market>' to remove plugins.")
	},
}

func init() {
	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(removeCmd)
}

func isLocalMarketType(marketType string) bool {
	return marketType == string(marketplace.SourceTypeLocal) ||
		marketType == string(marketplace.SourceTypeDirectory) ||
		marketType == string(marketplace.SourceTypeFile)
}
