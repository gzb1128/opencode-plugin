package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var githubShorthandRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`)

func ParseMarketplaceSource(url string) (MarketSource, error) {
	if url == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}

	if _, err := os.Stat(url); err == nil {
		absPath, _ := filepath.Abs(url)

		info, err := os.Stat(absPath)
		if err == nil && !info.IsDir() {
			return &FileMarketSource{
				Path: absPath,
			}, nil
		}

		return &DirectoryMarketSource{
			Path: absPath,
		}, nil
	}

	if matched := githubShorthandRegex.MatchString(url); matched {
		return &GitHubMarketSource{
			Repo: url,
		}, nil
	}

	if strings.HasPrefix(url, "https://github.com/") || strings.HasPrefix(url, "http://github.com/") {
		repo := extractRepoFromGitHubURL(url)
		if repo != "" {
			return &GitHubMarketSource{
				Repo: repo,
				URL:  url,
			}, nil
		}
	}

	if strings.HasPrefix(url, "git@github.com:") {
		repo := strings.TrimPrefix(url, "git@github.com:")
		repo = strings.TrimSuffix(repo, ".git")
		return &GitHubMarketSource{
			Repo: repo,
			URL:  url,
		}, nil
	}

	if strings.HasSuffix(url, "marketplace.json") {
		return &URLMarketSource{
			URL: url,
		}, nil
	}

	if strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return &GitMarketSource{
			URL: url,
		}, nil
	}

	return nil, fmt.Errorf("unsupported marketplace source format: %s", url)
}

func extractRepoFromGitHubURL(url string) string {
	var path string
	if strings.HasPrefix(url, "https://github.com/") {
		path = strings.TrimPrefix(url, "https://github.com/")
	} else if strings.HasPrefix(url, "http://github.com/") {
		path = strings.TrimPrefix(url, "http://github.com/")
	}
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return ""
}
