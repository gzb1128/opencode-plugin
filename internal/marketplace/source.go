package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type SourceType string

const (
	SourceTypeGitHub  SourceType = "github"
	SourceTypeGit     SourceType = "git"
	SourceTypeJSONURL SourceType = "json-url"
	SourceTypeLocal   SourceType = "local"
)

func ParseMarketplaceSource(url string) (*MarketSource, error) {
	if url == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}

	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`, url); matched {
		return &MarketSource{
			Type: string(SourceTypeGitHub),
			Repo: url,
			URL:  fmt.Sprintf("https://github.com/%s.git", url),
		}, nil
	}

	if strings.HasPrefix(url, "git@github.com:") {
		repo := strings.TrimPrefix(url, "git@github.com:")
		repo = strings.TrimSuffix(repo, ".git")
		return &MarketSource{
			Type: string(SourceTypeGitHub),
			Repo: repo,
			URL:  url,
		}, nil
	}

	if strings.HasSuffix(url, "marketplace.json") {
		return &MarketSource{
			Type: string(SourceTypeJSONURL),
			URL:  url,
		}, nil
	}

	if strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return &MarketSource{
			Type: string(SourceTypeGit),
			URL:  url,
		}, nil
	}

	if _, err := os.Stat(url); err == nil {
		absPath, _ := filepath.Abs(url)
		return &MarketSource{
			Type: string(SourceTypeLocal),
			Path: absPath,
		}, nil
	}

	return nil, fmt.Errorf("unsupported marketplace source format: %s", url)
}
