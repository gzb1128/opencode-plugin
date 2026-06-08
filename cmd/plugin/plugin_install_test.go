package plugin

import (
	"testing"
	"time"

	"github.com/opencode/plugin-cli/internal/config"
)

func TestFindDisabledInstallMatches(t *testing.T) {
	installed := map[string][]config.InstallRecord{
		"test-plugin@market-a": {{Version: "1.0.0", Disabled: true, DisabledAt: time.Now()}},
		"test-plugin@market-b": {{Version: "2.0.0", Disabled: true, DisabledAt: time.Now()}},
		"test-plugin@market-c": {{Version: "1.0.0", Disabled: false}},
		"other@market-a":       {{Version: "1.0.0", Disabled: true, DisabledAt: time.Now()}},
	}

	tests := []struct {
		name       string
		pluginName string
		marketName string
		version    string
		want       []string
	}{
		{
			name:       "filters by explicit marketplace",
			pluginName: "test-plugin",
			marketName: "market-b",
			want:       []string{"test-plugin@market-b"},
		},
		{
			name:       "filters by requested version",
			pluginName: "test-plugin",
			version:    "1.0.0",
			want:       []string{"test-plugin@market-a"},
		},
		{
			name:       "returns all disabled matches when ambiguous",
			pluginName: "test-plugin",
			want:       []string{"test-plugin@market-a", "test-plugin@market-b"},
		},
		{
			name:       "does not match enabled installations",
			pluginName: "test-plugin",
			marketName: "market-c",
			want:       nil,
		},
		{
			name:       "matches scoped plugin names with LastIndex",
			pluginName: "@scope/tool",
			marketName: "market-a",
			want:       []string{"@scope/tool@market-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scoped := map[string][]config.InstallRecord{
				"@scope/tool@market-a": {{Version: "1.0.0", Disabled: true, DisabledAt: time.Now()}},
				"test-plugin@market-a": {{Version: "1.0.0", Disabled: true, DisabledAt: time.Now()}},
			}
			src := installed
			if tt.pluginName == "@scope/tool" {
				src = scoped
			}
			got := findDisabledInstallMatches(src, tt.pluginName, tt.marketName, tt.version)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}
