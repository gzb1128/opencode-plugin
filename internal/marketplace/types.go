package marketplace

type SourceType string

const (
	SourceTypeLocal     SourceType = "local"
	SourceTypeGitHub    SourceType = "github"
	SourceTypeGit       SourceType = "git"
	SourceTypeGitSubdir SourceType = "git-subdir"
	SourceTypeURL       SourceType = "url"
	SourceTypeNpm       SourceType = "npm"
	SourceTypePip       SourceType = "pip"
	SourceTypeFile      SourceType = "file"
	SourceTypeDirectory SourceType = "directory"
)

type PluginSource interface {
	SourceType() string
}

type LocalSource struct {
	Path string
}

func (s *LocalSource) SourceType() string { return string(SourceTypeLocal) }

type GitHubSource struct {
	Repo string
	Ref  string
	SHA  string
}

func (s *GitHubSource) SourceType() string { return string(SourceTypeGitHub) }

type GitSource struct {
	URL string
	Ref string
	SHA string
}

func (s *GitSource) SourceType() string { return string(SourceTypeGit) }

type GitSubdirSource struct {
	URL     string
	SubPath string
	Ref     string
	SHA     string
}

func (s *GitSubdirSource) SourceType() string { return string(SourceTypeGitSubdir) }

type URLSource struct {
	URL string
	Ref string
	SHA string
}

func (s *URLSource) SourceType() string { return string(SourceTypeURL) }

type NpmSource struct {
	Package  string
	Version  string
	Registry string
}

func (s *NpmSource) SourceType() string { return string(SourceTypeNpm) }

type PipSource struct {
	Package  string
	Version  string
	Registry string
}

func (s *PipSource) SourceType() string { return string(SourceTypePip) }

type MarketSource interface {
	SourceType() string
	InstallLocation() string
	SetInstallLocation(string)
}

type marketSourceBase struct {
	installLocation string
}

func (b *marketSourceBase) InstallLocation() string       { return b.installLocation }
func (b *marketSourceBase) SetInstallLocation(loc string) { b.installLocation = loc }

type GitHubMarketSource struct {
	marketSourceBase
	Repo        string
	URL         string
	Ref         string
	Path        string
	SparsePaths []string
}

func (s *GitHubMarketSource) SourceType() string { return string(SourceTypeGitHub) }

type GitMarketSource struct {
	marketSourceBase
	URL         string
	Ref         string
	Path        string
	SparsePaths []string
}

func (s *GitMarketSource) SourceType() string { return string(SourceTypeGit) }

type URLMarketSource struct {
	marketSourceBase
	URL     string
	Headers map[string]string
}

func (s *URLMarketSource) SourceType() string { return string(SourceTypeURL) }

type LocalMarketSource struct {
	marketSourceBase
	Path string
}

func (s *LocalMarketSource) SourceType() string { return string(SourceTypeLocal) }

type FileMarketSource struct {
	marketSourceBase
	Path string
}

func (s *FileMarketSource) SourceType() string { return string(SourceTypeFile) }

type DirectoryMarketSource struct {
	marketSourceBase
	Path string
}

func (s *DirectoryMarketSource) SourceType() string { return string(SourceTypeDirectory) }

type Marketplace struct {
	Name                      string               `json:"name"`
	Description               string               `json:"description,omitempty"`
	Owner                     *Owner               `json:"owner,omitempty"`
	Plugins                   []Plugin             `json:"plugins"`
	ForceRemoveDeletedPlugins bool                 `json:"forceRemoveDeletedPlugins,omitempty"`
	Metadata                  *MarketplaceMetadata `json:"metadata,omitempty"`
	AllowCrossMarketplaceDeps []string             `json:"allowCrossMarketplaceDependenciesOn,omitempty"`
}

type MarketplaceMetadata struct {
	PluginRoot  string `json:"pluginRoot,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
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
	Source      interface{} `json:"source"`
	Homepage    string      `json:"homepage,omitempty"`
	Keywords    []string    `json:"keywords,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Strict      *bool       `json:"strict,omitempty"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

func MarketSourceToConfig(ms MarketSource) map[string]interface{} {
	result := map[string]interface{}{
		"source":          ms.SourceType(),
		"installLocation": ms.InstallLocation(),
	}

	switch s := ms.(type) {
	case *GitHubMarketSource:
		result["repo"] = s.Repo
		if s.URL != "" {
			result["url"] = s.URL
		} else {
			result["url"] = "https://github.com/" + s.Repo + ".git"
		}
		if s.Ref != "" {
			result["ref"] = s.Ref
		}
		if s.Path != "" {
			result["path"] = s.Path
		}
		if len(s.SparsePaths) > 0 {
			result["sparsePaths"] = s.SparsePaths
		}
	case *GitMarketSource:
		result["url"] = s.URL
		if s.Ref != "" {
			result["ref"] = s.Ref
		}
		if s.Path != "" {
			result["path"] = s.Path
		}
		if len(s.SparsePaths) > 0 {
			result["sparsePaths"] = s.SparsePaths
		}
	case *URLMarketSource:
		result["url"] = s.URL
		if len(s.Headers) > 0 {
			result["headers"] = s.Headers
		}
	case *LocalMarketSource:
		result["path"] = s.Path
	case *FileMarketSource:
		result["path"] = s.Path
	case *DirectoryMarketSource:
		result["path"] = s.Path
	}

	return result
}

func NewMarketSourceFromConfig(cfg map[string]interface{}) MarketSource {
	sourceType, _ := cfg["source"].(string)
	installLoc, _ := cfg["installLocation"].(string)

	switch sourceType {
	case string(SourceTypeGitHub):
		ms := &GitHubMarketSource{}
		ms.installLocation = installLoc
		ms.Repo, _ = cfg["repo"].(string)
		ms.URL, _ = cfg["url"].(string)
		ms.Ref, _ = cfg["ref"].(string)
		ms.Path, _ = cfg["path"].(string)
		ms.SparsePaths = configStringSlice(cfg["sparsePaths"])
		return ms
	case string(SourceTypeGit):
		ms := &GitMarketSource{}
		ms.installLocation = installLoc
		ms.URL, _ = cfg["url"].(string)
		ms.Ref, _ = cfg["ref"].(string)
		ms.Path, _ = cfg["path"].(string)
		ms.SparsePaths = configStringSlice(cfg["sparsePaths"])
		return ms
	case string(SourceTypeURL):
		ms := &URLMarketSource{}
		ms.installLocation = installLoc
		ms.URL, _ = cfg["url"].(string)
		ms.Headers = configStringMap(cfg["headers"])
		return ms
	case string(SourceTypeFile):
		ms := &FileMarketSource{}
		ms.installLocation = installLoc
		ms.Path, _ = cfg["path"].(string)
		return ms
	case string(SourceTypeDirectory):
		ms := &DirectoryMarketSource{}
		ms.installLocation = installLoc
		ms.Path, _ = cfg["path"].(string)
		return ms
	case string(SourceTypeLocal):
		ms := &LocalMarketSource{}
		ms.installLocation = installLoc
		ms.Path, _ = cfg["path"].(string)
		return ms
	default:
		ms := &LocalMarketSource{}
		ms.installLocation = installLoc
		return ms
	}
}

func GetMarketSourcePath(ms MarketSource) string {
	switch s := ms.(type) {
	case *LocalMarketSource:
		return s.Path
	case *FileMarketSource:
		return s.Path
	case *DirectoryMarketSource:
		return s.Path
	default:
		return ""
	}
}

func GetMarketSourceRepo(ms MarketSource) string {
	if s, ok := ms.(*GitHubMarketSource); ok {
		return s.Repo
	}
	return ""
}

func GetMarketSourceURL(ms MarketSource) string {
	switch s := ms.(type) {
	case *GitHubMarketSource:
		if s.URL != "" {
			return s.URL
		}
		return "https://github.com/" + s.Repo + ".git"
	case *GitMarketSource:
		return s.URL
	case *URLMarketSource:
		return s.URL
	default:
		return ""
	}
}

func configStringSlice(raw interface{}) []string {
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...)
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

func configStringMap(raw interface{}) map[string]string {
	switch v := raw.(type) {
	case map[string]string:
		result := make(map[string]string, len(v))
		for key, value := range v {
			result[key] = value
		}
		return result
	case map[string]interface{}:
		result := make(map[string]string, len(v))
		for key, value := range v {
			if s, ok := value.(string); ok {
				result[key] = s
			}
		}
		return result
	default:
		return nil
	}
}
