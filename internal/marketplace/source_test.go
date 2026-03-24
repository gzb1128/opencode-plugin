package marketplace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMarketplaceSource(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected *MarketSource
		wantErr  bool
	}{
		{
			name: "GitHub shorthand format",
			url:  "opencode/plugins-official",
			expected: &MarketSource{
				Type: string(SourceTypeGitHub),
				Repo: "opencode/plugins-official",
				URL:  "https://github.com/opencode/plugins-official.git",
			},
			wantErr: false,
		},
		{
			name: "GitHub SSH format",
			url:  "git@github.com:opencode/plugins-official.git",
			expected: &MarketSource{
				Type: string(SourceTypeGitHub),
				Repo: "opencode/plugins-official",
				URL:  "git@github.com:opencode/plugins-official.git",
			},
			wantErr: false,
		},
		{
			name: "Git HTTPS URL",
			url:  "https://github.com/opencode/plugins-official.git",
			expected: &MarketSource{
				Type: string(SourceTypeGit),
				URL:  "https://github.com/opencode/plugins-official.git",
			},
			wantErr: false,
		},
		{
			name: "Git SSH URL (non-GitHub)",
			url:  "git@gitlab.com:opencode/plugins-official.git",
			expected: &MarketSource{
				Type: string(SourceTypeGit),
				URL:  "git@gitlab.com:opencode/plugins-official.git",
			},
			wantErr: false,
		},
		{
			name: "marketplace.json URL",
			url:  "https://example.com/marketplace.json",
			expected: &MarketSource{
				Type: string(SourceTypeJSONURL),
				URL:  "https://example.com/marketplace.json",
			},
			wantErr: false,
		},
		{
			name: "Local path",
			url:  "/tmp/test-marketplace",
			expected: &MarketSource{
				Type: string(SourceTypeLocal),
				Path: "/tmp/test-marketplace",
			},
			wantErr: false,
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "Invalid format",
			url:      "not-a-valid-format",
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Local path" {
				tmpDir := tt.url
				os.MkdirAll(tmpDir, 0755)
				defer os.RemoveAll(tmpDir)
				tt.expected.Path, _ = filepath.Abs(tmpDir)
			}

			result, err := ParseMarketplaceSource(tt.url)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseMarketplaceSource() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseMarketplaceSource() unexpected error: %v", err)
				return
			}

			if result.Type != tt.expected.Type {
				t.Errorf("Type = %v, want %v", result.Type, tt.expected.Type)
			}

			if result.Repo != tt.expected.Repo {
				t.Errorf("Repo = %v, want %v", result.Repo, tt.expected.Repo)
			}

			if result.URL != tt.expected.URL {
				t.Errorf("URL = %v, want %v", result.URL, tt.expected.URL)
			}

			if result.Path != tt.expected.Path {
				t.Errorf("Path = %v, want %v", result.Path, tt.expected.Path)
			}
		})
	}
}
