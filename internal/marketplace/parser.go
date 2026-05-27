package marketplace

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	nonASCII = regexp.MustCompile(`[^\x20-\x7E]`)
	shaRegex = regexp.MustCompile(`^[a-f0-9]{7,40}$`)
)

func ParseMarketplaceIndex(path string) (*Marketplace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read marketplace.json: %w", err)
	}

	var marketplace Marketplace
	if err := json.Unmarshal(data, &marketplace); err != nil {
		return nil, fmt.Errorf("failed to parse marketplace.json: %w", err)
	}

	if err := ValidateMarketplaceName(marketplace.Name); err != nil {
		return nil, err
	}

	for i, plugin := range marketplace.Plugins {
		src, err := parsePluginSource(plugin.Source)
		if err != nil {
			return nil, fmt.Errorf("failed to parse source for plugin %s: %w", plugin.Name, err)
		}
		marketplace.Plugins[i].Source = src
	}

	return &marketplace, nil
}

func ValidateMarketplaceName(name string) error {
	if name == "" {
		return fmt.Errorf("marketplace.json must have a 'name' field")
	}
	if strings.Contains(name, " ") {
		return fmt.Errorf("marketplace name cannot contain spaces: %q", name)
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("marketplace name cannot contain path separators: %q", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("marketplace name cannot contain '..': %q", name)
	}
	if name == "." {
		return fmt.Errorf("marketplace name cannot be '.'")
	}
	if nonASCII.MatchString(name) {
		return fmt.Errorf("marketplace name cannot contain non-ASCII characters: %q", name)
	}
	return nil
}

func parsePluginSource(raw interface{}) (PluginSource, error) {
	switch v := raw.(type) {
	case string:
		return &LocalSource{Path: v}, nil

	case map[string]interface{}:
		sourceType, ok := v["source"].(string)
		if !ok {
			return nil, fmt.Errorf("source must have a 'source' field")
		}

		switch sourceType {
		case string(SourceTypeGitHub):
			repo, _ := v["repo"].(string)
			if repo == "" {
				return nil, fmt.Errorf("github source must have a 'repo' field")
			}
			sha, _ := v["sha"].(string)
			if sha != "" {
				if err := validateSHA(sha); err != nil {
					return nil, err
				}
			}
			return &GitHubSource{
				Repo: repo,
				Ref:  optionalString(v, "ref"),
				SHA:  sha,
			}, nil

		case string(SourceTypeGit):
			url, _ := v["url"].(string)
			if url == "" {
				return nil, fmt.Errorf("git source must have a 'url' field")
			}
			sha, _ := v["sha"].(string)
			if sha != "" {
				if err := validateSHA(sha); err != nil {
					return nil, err
				}
			}
			return &GitSource{
				URL: url,
				Ref: optionalString(v, "ref"),
				SHA: sha,
			}, nil

		case string(SourceTypeGitSubdir):
			url := resolveGitSubdirURL(v)
			if url == "" {
				return nil, fmt.Errorf("git-subdir source must have a 'url' or 'repo' field")
			}
			subPath, _ := v["path"].(string)
			if subPath == "" {
				return nil, fmt.Errorf("git-subdir source must have a 'path' field")
			}
			sha, _ := v["sha"].(string)
			if sha != "" {
				if err := validateSHA(sha); err != nil {
					return nil, err
				}
			}
			return &GitSubdirSource{
				URL:     url,
				SubPath: subPath,
				Ref:     optionalString(v, "ref"),
				SHA:     sha,
			}, nil

		case string(SourceTypeURL):
			url, _ := v["url"].(string)
			if url == "" {
				return nil, fmt.Errorf("url source must have a 'url' field")
			}
			sha, _ := v["sha"].(string)
			if sha != "" {
				if err := validateSHA(sha); err != nil {
					return nil, err
				}
			}
			return &URLSource{
				URL: url,
				Ref: optionalString(v, "ref"),
				SHA: sha,
			}, nil

		case string(SourceTypeNpm):
			pkg, _ := v["package"].(string)
			if pkg == "" {
				return nil, fmt.Errorf("npm source must have a 'package' field")
			}
			return &NpmSource{
				Package:  pkg,
				Version:  optionalString(v, "version"),
				Registry: optionalString(v, "registry"),
			}, nil

		case string(SourceTypePip):
			pkg, _ := v["package"].(string)
			if pkg == "" {
				return nil, fmt.Errorf("pip source must have a 'package' field")
			}
			return &PipSource{
				Package:  pkg,
				Version:  optionalString(v, "version"),
				Registry: optionalString(v, "registry"),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported source type: %s", sourceType)
		}

	default:
		return nil, fmt.Errorf("invalid source format")
	}
}

func resolveGitSubdirURL(v map[string]interface{}) string {
	url, _ := v["url"].(string)
	repo, _ := v["repo"].(string)

	if url == "" && repo != "" {
		url = "https://github.com/" + repo + ".git"
	}

	if url != "" && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "git@") {
		url = "https://github.com/" + url + ".git"
	}

	return url
}

func validateSHA(sha string) error {
	if !shaRegex.MatchString(sha) {
		return fmt.Errorf("SHA must be a 7-40 character lowercase hex string, got: %q", sha)
	}
	return nil
}

func optionalString(v map[string]interface{}, key string) string {
	s, _ := v[key].(string)
	return s
}
