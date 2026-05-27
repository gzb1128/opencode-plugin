package market

import "testing"

func TestGetMarketURLPrefersRepoForGithub(t *testing.T) {
	market := map[string]interface{}{
		"source": "github",
		"repo":   "owner/repo",
		"url":    "git@github.com:owner/repo.git",
	}

	got := getMarketURL(market)
	want := "owner/repo"
	if got != want {
		t.Fatalf("getMarketURL() = %q, want %q", got, want)
	}
}

func TestGetMarketURLSourceAware(t *testing.T) {
	tests := []struct {
		name string
		market map[string]interface{}
		want string
	}{
		{
			name: "github returns repo",
			market: map[string]interface{}{
				"source": "github",
				"repo":   "owner/repo",
				"url":    "https://github.com/owner/repo.git",
			},
			want: "owner/repo",
		},
		{
			name: "git returns url",
			market: map[string]interface{}{
				"source": "git",
				"url":    "https://gitlab.com/org/repo.git",
			},
			want: "https://gitlab.com/org/repo.git",
		},
		{
			name: "url returns url",
			market: map[string]interface{}{
				"source": "url",
				"url":    "https://example.com/marketplace.json",
			},
			want: "https://example.com/marketplace.json",
		},
		{
			name: "file returns path",
			market: map[string]interface{}{
				"source": "file",
				"path":   "/tmp/marketplace.json",
			},
			want: "/tmp/marketplace.json",
		},
		{
			name: "directory returns path",
			market: map[string]interface{}{
				"source": "directory",
				"path":   "/tmp/market",
			},
			want: "/tmp/market",
		},
		{
			name: "local returns path",
			market: map[string]interface{}{
				"source": "local",
				"path":   "/tmp/market",
			},
			want: "/tmp/market",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMarketURL(tt.market)
			if got != tt.want {
				t.Errorf("getMarketURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsLocalMarketType(t *testing.T) {
	for _, marketType := range []string{"local", "directory", "file"} {
		t.Run(marketType, func(t *testing.T) {
			if !isLocalMarketType(marketType) {
				t.Fatalf("isLocalMarketType(%q) = false, want true", marketType)
			}
		})
	}

	if isLocalMarketType("github") {
		t.Fatal("isLocalMarketType(\"github\") = true, want false")
	}
}
