package market

import (
	"fmt"
	"os"

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
		fmt.Printf("  Type: %s\n", source.Type)
		if source.Repo != "" {
			fmt.Printf("  Repo: %s\n", source.Repo)
		}
		if source.URL != "" {
			fmt.Printf("  URL: %s\n", source.URL)
		}
		if source.Path != "" {
			fmt.Printf("  Path: %s\n", source.Path)
		}

		// Determine marketplace name
		name := cmd.Flag("name").Value.String()
		if name == "" {
			if source.Repo != "" {
				name = source.Repo
			} else if source.URL != "" {
				// Extract name from URL
				name = extractNameFromURL(source.URL)
			} else if source.Path != "" {
				name = "local-marketplace"
			} else {
				fmt.Fprintf(os.Stderr, "Error: Cannot determine marketplace name. Please specify with --name flag.\n")
				os.Exit(1)
			}
		}

		mp, err := mgr.Add(name, url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to add marketplace: %v\n", err)
			os.Exit(1)
		}

		// Get the install location
		installLocation := paths.MarketsDir + "/" + name
		if source.Type == string(marketplace.SourceTypeLocal) {
			installLocation = source.Path
		}

		// Save to config
		marketSrc := map[string]interface{}{
			"source":          source.Type,
			"repo":            source.Repo,
			"url":             source.URL,
			"path":            source.Path,
			"installLocation": installLocation,
		}

		if err := configMgr.AddKnownMarket(name, marketSrc); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to save marketplace config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✓ Successfully added marketplace: %s\n", mp.Name)
		fmt.Printf("  %d plugins available\n", len(mp.Plugins))
		fmt.Printf("  Location: %s\n", installLocation)
	},
}

func extractNameFromURL(url string) string {
	// Extract name from GitHub URL
	// e.g., "https://github.com/owner/repo.git" -> "owner/repo"
	// e.g., "git@github.com:owner/repo.git" -> "owner/repo"

	// Remove .git suffix
	name := url
	if len(name) > 4 && name[len(name)-4:] == ".git" {
		name = name[:len(name)-4]
	}

	// Find last / or :
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

func init() {
	addCmd.Flags().StringP("name", "n", "", "Marketplace name (auto-detected from URL if not specified)")
	Cmd.AddCommand(addCmd)
}
