package market

import "testing"

func TestGetMarketURLPrefersURLOverRepo(t *testing.T) {
	market := map[string]interface{}{
		"source": "github",
		"repo":   "owner/repo",
		"url":    "git@github.com:owner/repo.git",
	}

	got := getMarketURL(market)
	want := "git@github.com:owner/repo.git"
	if got != want {
		t.Fatalf("getMarketURL() = %q, want %q", got, want)
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
