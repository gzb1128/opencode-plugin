package plugin

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/opencode/plugin-cli/internal/plugin"
)

type resolveAction string

const (
	actionDisable resolveAction = "disable"
	actionEnable  resolveAction = "enable"
	actionRemove  resolveAction = "remove"
	actionUpdate  resolveAction = "update"
	actionInstall resolveAction = "install"
)

func resolveMarketName(installer *plugin.Installer, pluginSpec string, action resolveAction) (pluginName, marketName string, resolved bool) {
	if idx := strings.Index(pluginSpec, "@"); idx > 0 {
		return pluginSpec[:idx], pluginSpec[idx+1:], true
	}

	pluginName = pluginSpec

	installed, err := installer.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to list installed plugins: %v\n", err)
		os.Exit(1)
	}

	var matches []string
	for key := range installed {
		if strings.HasPrefix(key, pluginName+"@") {
			matches = append(matches, key)
		}
	}

	if len(matches) > 1 {
		sort.Strings(matches)
		fmt.Fprintf(os.Stderr, "Error: Multiple installations of %s found:\n", pluginName)
		for _, match := range matches {
			fmt.Fprintf(os.Stderr, "  - %s\n", match)
		}
		fmt.Fprintf(os.Stderr, "\nPlease specify which one to %s:\n", action)
		fmt.Fprintf(os.Stderr, "  opencode-plugin plugin %s %s\n", action, matches[0])
		os.Exit(1)
	}

	if len(matches) == 1 {
		key := matches[0]
		marketName = strings.TrimPrefix(key, pluginName+"@")
		return pluginName, marketName, true
	}

	return pluginName, "", false
}
