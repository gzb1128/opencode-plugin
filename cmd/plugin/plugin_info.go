package plugin

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
	"github.com/opencode/plugin-cli/internal/plugin"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <plugin-name>[@<marketplace>]",
	Short: "Show detailed information about a plugin",
	Long: `Show detailed information about a plugin, including available versions.

Examples:
  opencode-plugin plugin info my-plugin
  opencode-plugin plugin info my-plugin@my-market`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pluginSpec := args[0]

		// Parse plugin spec
		var pluginName, marketName string
		if idx := strings.Index(pluginSpec, "@"); idx > 0 {
			pluginName = pluginSpec[:idx]
			marketName = pluginSpec[idx+1:]
		} else {
			pluginName = pluginSpec
		}

		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		// Load markets
		markets, err := configMgr.LoadKnownMarkets()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to load marketplaces: %v\n", err)
			os.Exit(1)
		}

		// Convert to MarketSource format
		marketSources := make(map[string]marketplace.MarketSource)
		for name, src := range markets {
			marketSources[name] = marketplace.NewMarketSourceFromConfig(src)
		}

		// Find plugin
		paths := configMgr.GetPaths()
		mgr := marketplace.NewManager(paths.MarketsDir)
		resolved, err := mgr.ResolvePlugin(marketSources, pluginName, marketName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		p := resolved.Plugin
		market := resolved.Market
		if marketName == "" {
			marketName = resolved.MarketName
		}

		// Get plugin path
		resolver := plugin.NewVersionResolver()
		pluginRoot := ""
		if resolved.Marketplace != nil && resolved.Marketplace.Metadata != nil {
			pluginRoot = resolved.Marketplace.Metadata.PluginRoot
		}
		pluginPath, err := resolver.GetPluginSourcePathWithCtx(p, plugin.PluginResolutionContext{
			MarketPath: market.InstallLocation(),
			PluginRoot: pluginRoot,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Print plugin info
		fmt.Printf("Plugin: %s\n", p.Name)
		if p.DisplayName != "" {
			fmt.Printf("Display Name: %s\n", p.DisplayName)
		}
		fmt.Printf("Description: %s\n", p.Description)
		if p.Version != "" {
			fmt.Printf("Version: %s\n", p.Version)
		}
		if p.Category != "" {
			fmt.Printf("Category: %s\n", p.Category)
		}
		if p.Author != nil {
			fmt.Printf("Author: %s", p.Author.Name)
			if p.Author.Email != "" {
				fmt.Printf(" <%s>", p.Author.Email)
			}
			fmt.Println()
		}
		fmt.Printf("Marketplace: %s\n", marketName)
		if p.Homepage != "" {
			fmt.Printf("Homepage: %s\n", p.Homepage)
		}
		if len(p.Keywords) > 0 {
			fmt.Printf("Keywords: %s\n", strings.Join(p.Keywords, ", "))
		}

		// Get available versions
		versions, err := resolver.GetAvailableVersions(pluginPath)
		if err == nil && len(versions) > 0 {
			fmt.Printf("\nAvailable versions:\n")
			for _, v := range versions {
				fmt.Printf("  - %s\n", v)
			}
		}

		// Check if installed
		installed, err := configMgr.LoadInstalledPlugins()
		if err == nil {
			key := fmt.Sprintf("%s@%s", p.Name, marketName)
			if records, ok := installed.Plugins[key]; ok && len(records) > 0 {
				record := records[0]
				fmt.Printf("\nInstalled:\n")
				fmt.Printf("  Version: %s\n", record.Version)
				fmt.Printf("  Scope: %s\n", record.Scope)
				fmt.Printf("  Path: %s\n", record.InstallPath)
				fmt.Printf("  Installed: %s\n", record.InstalledAt.Format("2006-01-02 15:04:05"))
			}
		}
	},
}


