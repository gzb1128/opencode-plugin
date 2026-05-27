package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var githubShorthandRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`)

func splitSourceRef(input string) (base string, ref string) {
	input = strings.TrimSpace(input)
	if idx := strings.LastIndex(input, "#"); idx >= 0 {
		return input[:idx], input[idx+1:]
	}
	if idx := strings.LastIndex(input, "@"); idx >= 0 {
		candidate := input[:idx]
		if githubShorthandRegex.MatchString(candidate) {
			return candidate, input[idx+1:]
		}
	}
	return input, ""
}

func ParseMarketplaceSource(url string) (MarketSource, error) {
	url = strings.TrimSpace(url)

	if url == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}

	if strings.HasPrefix(url, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to expand home directory: %w", err)
		}
		expanded := filepath.Join(home, url[2:])
		info, err := os.Stat(expanded)
		if err == nil {
			if !info.IsDir() {
				return &FileMarketSource{Path: expanded}, nil
			}
			return &DirectoryMarketSource{Path: expanded}, nil
		}
		return nil, fmt.Errorf("home-relative path does not exist: %s", expanded)
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

	base, ref := splitSourceRef(url)

	if matched := githubShorthandRegex.MatchString(base); matched {
		return &GitHubMarketSource{
			Repo: base,
			Ref:  ref,
		}, nil
	}

	if strings.HasPrefix(base, "https://github.com/") || strings.HasPrefix(base, "http://github.com/") {
		var gitURL string
		if strings.HasSuffix(base, ".git") {
			gitURL = base
		} else {
			gitURL = base + ".git"
		}
		return &GitMarketSource{
			URL: gitURL,
			Ref: ref,
		}, nil
	}

	if strings.HasPrefix(base, "git@github.com:") {
		return &GitMarketSource{
			URL: base,
			Ref: ref,
		}, nil
	}

	if strings.HasSuffix(base, "marketplace.json") {
		return &URLMarketSource{
			URL: base,
		}, nil
	}

	if strings.HasPrefix(base, "git@") || strings.HasPrefix(base, "https://") || strings.HasPrefix(base, "http://") || isSSHURL(base) {
		return &GitMarketSource{
			URL: base,
			Ref: ref,
		}, nil
	}

	return nil, fmt.Errorf("unsupported marketplace source format: %s", url)
}

func isSSHURL(s string) bool {
	at := strings.Index(s, "@")
	if at < 0 {
		return false
	}
	rest := s[at+1:]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return false
	}
	host := rest[:colon]
	return !strings.Contains(host, "/")
}
