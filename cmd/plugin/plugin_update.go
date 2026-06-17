package plugin

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/plugin"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update [<plugin-name>[@<marketplace>]]",
	Short: "Update installed plugins",
	Long: `Update one or all installed plugins to their latest versions.

If no plugin is specified, updates all installed plugins.

Examples:
  opencode-plugin plugin update
  opencode-plugin plugin update my-plugin
  opencode-plugin plugin update my-plugin@my-market
  opencode-plugin plugin update --force my-plugin`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")
		configMgr, err := config.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize config: %v\n", err)
			os.Exit(1)
		}

		installer := plugin.NewInstaller(configMgr)
		installed, err := installer.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to list installed plugins: %v\n", err)
			os.Exit(1)
		}

		if len(installed) == 0 {
			fmt.Println("No plugins installed.")
			return
		}

		if len(args) == 0 {
			fmt.Printf("Updating all installed plugins (%d)...\n\n", len(installed))
			updated := 0
			failed := 0

			for key, records := range installed {
				if len(records) == 0 {
					continue
				}

				idx := strings.LastIndex(key, "@")
				if idx <= 0 {
					continue
				}
				pluginName := key[:idx]
				marketName := key[idx+1:]

				fmt.Printf("Updating %s...\n", key)
				if err := updatePlugin(installer, configMgr, pluginName, marketName, force, &records[0]); err != nil {
					fmt.Fprintf(os.Stderr, "  Error: %v\n\n", err)
					failed++
				} else {
					updated++
				}
			}

			fmt.Printf("\n✓ Updated %d plugins, %d failed\n", updated, failed)
		} else {
			pluginName, marketName, resolved := resolveMarketName(installer, args[0], actionUpdate)
			if !resolved {
				fmt.Fprintf(os.Stderr, "Error: Plugin %s not found in installed list\n", pluginName)
				os.Exit(1)
			}

			if err := updatePlugin(installer, configMgr, pluginName, marketName, force); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("\n✓ Successfully updated plugin: %s@%s\n", pluginName, marketName)
		}
	},
}

func updatePlugin(installer *plugin.Installer, configMgr *config.Manager, pluginName, marketName string, force bool, preloadedRecord ...*config.InstallRecord) error {
	key := fmt.Sprintf("%s@%s", pluginName, marketName)

	// 读取已有 record，决定保留 disabled 状态
	var existingRecord *config.InstallRecord
	if len(preloadedRecord) > 0 && preloadedRecord[0] != nil {
		existingRecord = preloadedRecord[0]
	} else if record, err := configMgr.GetInstallRecord(key); err == nil {
		existingRecord = record
	}

	wasDisabled := existingRecord != nil && existingRecord.Disabled

	opts := plugin.InstallOptions{
		MarketName: marketName,
		Version:    "",
		Scope:      "user",
		Force:      force,
		Disabled:   wasDisabled,
	}

	// Update() 内部做了 "先 materialize 再 swap" 的两阶段提交：
	// 下载失败时旧版本完整保留，不会出现 plugin 被删除后无法恢复的情况。
	return installer.Update(pluginName, opts)
}
