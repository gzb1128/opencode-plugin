package marketplace

import "time"

type Marketplace struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Owner       *Owner   `json:"owner,omitempty"`
	Plugins     []Plugin `json:"plugins"`
}

type Owner struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Plugin struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version,omitempty"`
	Category    string      `json:"category,omitempty"`
	Author      *Author     `json:"author,omitempty"`
	Source      interface{} `json:"source"` // string or object
	Homepage    string      `json:"homepage,omitempty"`
	Keywords    []string    `json:"keywords,omitempty"`
}

type PluginSource struct {
	Type string `json:"-"` // "local", "github", "git", "git-subdir", "url"

	Path string `json:"-"` // "./plugins/plugin-name" for local

	Repo string `json:"repo,omitempty"` // "owner/repo" for github

	URL string `json:"url,omitempty"` // "https://..." for git/url

	SubPath string `json:"path,omitempty"` // "plugins/plugin-name" for git-subdir
	Ref     string `json:"ref,omitempty"`  // "main" for git-subdir

	SHA string `json:"sha,omitempty"` // commit SHA (optional)
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

type MarketSource struct {
	Type            string    `json:"source"` // "github", "git", "json-url", "local"
	Repo            string    `json:"repo,omitempty"`
	URL             string    `json:"url,omitempty"`
	Path            string    `json:"path,omitempty"`
	InstallLocation string    `json:"installLocation"`
	LastUpdated     time.Time `json:"lastUpdated"`
}
