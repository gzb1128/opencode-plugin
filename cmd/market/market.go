package market

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "market",
	Aliases: []string{"marketplace"},
	Short:   "Manage plugin marketplaces",
	Long:    "Add, list, update, and remove plugin marketplaces",
}

type marketJSONEntry struct {
	Name            string `json:"name"`
	SourceType      string `json:"sourceType"`
	Repo            string `json:"repo,omitempty"`
	URL             string `json:"url,omitempty"`
	Path            string `json:"path,omitempty"`
	InstallLocation string `json:"installLocation"`
	Cloned          bool   `json:"cloned"`
	LastUpdated     string `json:"lastUpdated,omitempty"`
}

var listJSON bool

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

		if listJSON {
			printMarketsJSON(markets)
			return
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

				source := marketplace.NewMarketSourceFromConfig(market)
				indexPath, err := marketplace.MarketSourceIndexPath(source)
				if err != nil {
					fmt.Printf("    Status: Error (%v)\n", err)
					notClonedCount++
				} else if _, err := os.Stat(indexPath); os.IsNotExist(err) {
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

func printMarketsJSON(markets config.KnownMarkets) {
	entries := make([]marketJSONEntry, 0, len(markets))
	for name, market := range markets {
		entry := marketJSONEntry{
			Name: name,
		}
		if sourceType, ok := market["source"].(string); ok {
			entry.SourceType = sourceType
		}
		if repo, ok := market["repo"].(string); ok {
			entry.Repo = repo
		}
		if url, ok := market["url"].(string); ok {
			entry.URL = url
		}
		if path, ok := market["path"].(string); ok {
			entry.Path = path
		}
		if installLoc, ok := market["installLocation"].(string); ok {
			entry.InstallLocation = installLoc
		}
		entry.Cloned = isMarketCloned(market)
		if lastUpdated, ok := market["lastUpdated"].(string); ok {
			entry.LastUpdated = lastUpdated
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(entries)
}

func isMarketCloned(market map[string]interface{}) bool {
	installLoc, ok := market["installLocation"].(string)
	if !ok || installLoc == "" {
		return false
	}
	source := marketplace.NewMarketSourceFromConfig(market)
	indexPath, err := marketplace.MarketSourceIndexPath(source)
	if err != nil {
		return false
	}
	if _, err := os.Stat(indexPath); err != nil {
		return false
	}
	return true
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	Cmd.AddCommand(listCmd)
}
