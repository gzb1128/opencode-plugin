package market

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add a plugin marketplace",
	Long: `Add a plugin marketplace from various sources:

Supported formats:
  - GitHub shorthand: owner/repo
  - GitHub SSH: git@github.com:owner/repo.git
  - Git HTTPS: https://github.com/owner/repo.git
  - marketplace.json URL: https://example.com/marketplace.json
  - Local path: ./path/to/marketplace

Examples:
  opencode-plugin market add opencode/plugins-official
  opencode-plugin market add git@github.com:mycompany/plugins.git
  opencode-plugin market add https://example.com/marketplace.json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]

		source, err := marketplace.ParseMarketplaceSource(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to parse marketplace source: %v\n", err)
			os.Exit(1)
		}

		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		paths := configMgr.GetPaths()
		mgr := marketplace.NewManager(paths.MarketsDir)

		fmt.Printf("Adding marketplace from: %s\n", url)
		fmt.Printf("  Type: %s\n", source.SourceType())

		repo := marketplace.GetMarketSourceRepo(source)
		if repo != "" {
			fmt.Printf("  Repo: %s\n", repo)
		}
		displayURL := marketplace.GetMarketSourceURL(source)
		if displayURL != "" {
			fmt.Printf("  URL: %s\n", displayURL)
		}
		displayPath := marketplace.GetMarketSourcePath(source)
		if displayPath != "" {
			fmt.Printf("  Path: %s\n", displayPath)
		}

		name := cmd.Flag("name").Value.String()
		if name == "" {
			if repo != "" {
				name = extractNameFromRepo(repo)
			} else if displayURL != "" {
				name = extractNameFromURL(displayURL)
			} else if displayPath != "" {
				name = "local-marketplace"
			} else {
				fmt.Fprintf(os.Stderr, "Error: Cannot determine marketplace name. Please specify with --name flag.\n")
				os.Exit(1)
			}
		}

		if err := marketplace.ValidateMarketplaceName(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid marketplace name: %v\n", err)
			os.Exit(1)
		}

		mp, resultSource, err := mgr.Add(name, url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to add marketplace: %v\n", err)
			os.Exit(1)
		}

		name = resolveAddedMarketplaceName(name, mp.Name, cmd.Flag("name").Value.String() != "")

		installLocation := resultSource.InstallLocation()

		marketSrc := marketplace.MarketSourceToConfig(resultSource)
		marketSrc["installLocation"] = installLocation

		if err := configMgr.AddKnownMarket(name, marketSrc); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to save marketplace config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✓ Successfully added marketplace: %s\n", name)
		fmt.Printf("  %d plugins available\n", len(mp.Plugins))
		fmt.Printf("  Location: %s\n", installLocation)
	},
}

func extractNameFromRepo(repo string) string {
	if idx := strings.LastIndex(repo, "/"); idx >= 0 {
		return repo[idx+1:]
	}
	return repo
}

func extractNameFromURL(url string) string {
	name := url
	if len(name) > 4 && name[len(name)-4:] == ".git" {
		name = name[:len(name)-4]
	}

	lastSep := -1
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' || name[i] == ':' {
			lastSep = i
			break
		}
	}

	if lastSep > 0 {
		return name[lastSep+1:]
	}

	return name
}

func resolveAddedMarketplaceName(derivedName, manifestName string, userSpecifiedName bool) string {
	if userSpecifiedName || manifestName == "" {
		return derivedName
	}
	return manifestName
}

func init() {
	addCmd.Flags().StringP("name", "n", "", "Marketplace name (auto-detected from URL if not specified)")
	Cmd.AddCommand(addCmd)
}
