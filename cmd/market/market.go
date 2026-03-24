package market

import (
	"fmt"
	"os"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "market",
	Short: "Manage plugin marketplaces",
	Long:  "Add, list, update, and remove plugin marketplaces",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all added marketplaces",
	Run: func(cmd *cobra.Command, args []string) {
		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
			os.Exit(1)
		}

		markets, err := configMgr.LoadKnownMarkets()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading markets: %v\n", err)
			os.Exit(1)
		}

		if len(markets) == 0 {
			fmt.Println("No marketplaces added yet.")
			fmt.Println("\nUse 'opencode-plugin market add <url>' to add a marketplace.")
			return
		}

		fmt.Println("Added Marketplaces:")
		fmt.Println()

		notClonedCount := 0
		for name, market := range markets {
			fmt.Printf("  %s\n", name)
			if marketType, ok := market["source"].(string); ok {
				fmt.Printf("    Type: %s\n", marketType)
			}
			if repo, ok := market["repo"].(string); ok && repo != "" {
				fmt.Printf("    Repo: %s\n", repo)
			}
			if url, ok := market["url"].(string); ok && url != "" {
				fmt.Printf("    URL: %s\n", url)
			}

			installLoc, hasInstallLoc := market["installLocation"].(string)
			if hasInstallLoc && installLoc != "" {
				fmt.Printf("    Location: %s\n", installLoc)

				// Check if marketplace is cloned
				indexPath := installLoc + "/.claude-plugin/marketplace.json"
				if _, err := os.Stat(indexPath); os.IsNotExist(err) {
					fmt.Printf("    Status: Not cloned\n")
					notClonedCount++
				} else {
					fmt.Printf("    Status: Ready\n")
				}
			} else {
				fmt.Printf("    Status: Not cloned\n")
				notClonedCount++
			}

			if lastUpdated, ok := market["lastUpdated"].(string); ok {
				fmt.Printf("    Last Updated: %s\n", lastUpdated)
			}
			fmt.Println()
		}

		if notClonedCount > 0 {
			fmt.Printf("Note: %d marketplace(s) not cloned yet.\n", notClonedCount)
			fmt.Println("Use 'opencode-plugin market update' to clone all marketplaces.")
		}
	},
}

func init() {
	Cmd.AddCommand(listCmd)
}
