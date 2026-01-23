package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractRepoPath(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPath     string
		wantCloneURL string
		wantErr      bool
	}{
		// HTTPS URLs
		{
			name:         "https with .git suffix",
			input:        "https://github.com/org/repo.git",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "https://github.com/org/repo.git",
		},
		{
			name:         "https without .git suffix",
			input:        "https://github.com/org/repo",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "https://github.com/org/repo",
		},
		{
			name:         "http scheme",
			input:        "http://github.com/org/repo",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "http://github.com/org/repo",
		},
		{
			name:         "https with trailing slash",
			input:        "https://github.com/org/repo/",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "https://github.com/org/repo/",
		},
		{
			name:         "nested path (org/suborg/repo)",
			input:        "https://github.com/org/suborg/repo.git",
			wantPath:     "github.com/org/suborg/repo",
			wantCloneURL: "https://github.com/org/suborg/repo.git",
		},

		// SSH URLs
		{
			name:         "ssh git@host:org/repo.git",
			input:        "git@github.com:org/repo.git",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "git@github.com:org/repo.git",
		},
		{
			name:         "ssh git@host:org/repo without .git",
			input:        "git@github.com:org/repo",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "git@github.com:org/repo",
		},
		{
			name:         "ssh deploy@host:org/repo.git",
			input:        "deploy@github.com:org/repo.git",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "deploy@github.com:org/repo.git",
		},
		{
			name:         "ssh host:org/repo (no user)",
			input:        "github.com:org/repo.git",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "github.com:org/repo.git",
		},

		// Bare URLs (no scheme)
		{
			name:         "bare host/org/repo",
			input:        "github.com/org/repo",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "https://github.com/org/repo",
		},
		{
			name:         "bare host/org/repo.git",
			input:        "github.com/org/repo.git",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "https://github.com/org/repo.git",
		},

		// Port handling
		{
			name:         "https with port 443",
			input:        "https://github.com:443/org/repo",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "https://github.com:443/org/repo",
		},
		{
			name:         "https with custom port",
			input:        "https://git.example.com:8080/org/repo",
			wantPath:     "git.example.com/org/repo",
			wantCloneURL: "https://git.example.com:8080/org/repo",
		},
		{
			name:         "bare host:port/org/repo treated as HTTP not SSH",
			input:        "git.example.com:8080/org/repo",
			wantPath:     "git.example.com/org/repo",
			wantCloneURL: "https://git.example.com:8080/org/repo",
		},

		// ssh:// scheme
		{
			name:         "ssh:// scheme URL",
			input:        "ssh://git@github.com/org/repo.git",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "ssh://git@github.com/org/repo.git",
		},

		// Error cases
		{
			name:    "https missing path",
			input:   "https://github.com/",
			wantErr: true,
		},
		{
			name:    "https only org (no repo)",
			input:   "https://github.com/org",
			wantErr: true,
		},
		{
			name:    "ssh missing path",
			input:   "git@github.com:",
			wantErr: true,
		},
		{
			name:    "ssh only repo (no org)",
			input:   "git@github.com:repo.git",
			wantErr: true,
		},
		{
			name:    "ssh empty host",
			input:   "git@:org/repo.git",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:         "path traversal in https URL",
			input:        "https://evil.com/../../etc/passwd",
			wantPath:     "evil.com/../../etc/passwd",
			wantCloneURL: "https://evil.com/../../etc/passwd",
			// extractRepoPath itself allows this; containment is checked in main
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotCloneURL, err := extractRepoPath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("extractRepoPath(%q) = (%q, %q), want error", tt.input, gotPath, gotCloneURL)
				}
				return
			}
			if err != nil {
				t.Errorf("extractRepoPath(%q) unexpected error: %v", tt.input, err)
				return
			}
			if gotPath != tt.wantPath {
				t.Errorf("extractRepoPath(%q) path = %q, want %q", tt.input, gotPath, tt.wantPath)
			}
			if gotCloneURL != tt.wantCloneURL {
				t.Errorf("extractRepoPath(%q) cloneURL = %q, want %q", tt.input, gotCloneURL, tt.wantCloneURL)
			}
		})
	}
}

func TestBaseDir(t *testing.T) {
	t.Run("uses GIT_CLONE_ROOT when set", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("GIT_CLONE_ROOT", dir)

		got, err := baseDir()
		if err != nil {
			t.Fatalf("baseDir() error: %v", err)
		}
		abs, _ := filepath.Abs(dir)
		if got != abs {
			t.Errorf("baseDir() = %q, want %q", got, abs)
		}
	})

	t.Run("resolves relative GIT_CLONE_ROOT to absolute", func(t *testing.T) {
		t.Setenv("GIT_CLONE_ROOT", "./relative/path")

		got, err := baseDir()
		if err != nil {
			t.Fatalf("baseDir() error: %v", err)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("baseDir() = %q, want absolute path", got)
		}
	})

	t.Run("falls back to $HOME/code when env unset", func(t *testing.T) {
		t.Setenv("GIT_CLONE_ROOT", "")

		got, err := baseDir()
		if err != nil {
			t.Fatalf("baseDir() error: %v", err)
		}
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, "code")
		if got != want {
			t.Errorf("baseDir() = %q, want %q", got, want)
		}
	})
}

func TestPathContainment(t *testing.T) {
	base := "/home/user/code"

	tests := []struct {
		name     string
		repoPath string
		escapes  bool
	}{
		{
			name:     "normal path",
			repoPath: "github.com/org/repo",
			escapes:  false,
		},
		{
			name:     "traversal escapes base",
			repoPath: "evil.com/../../etc/passwd",
			escapes:  true,
		},
		{
			name:     "traversal that stays inside",
			repoPath: "github.com/org/../org/repo",
			escapes:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullTarget := filepath.Clean(filepath.Join(base, tt.repoPath))
			baseClean := filepath.Clean(base) + string(os.PathSeparator)
			escaped := !strings.HasPrefix(fullTarget+string(os.PathSeparator), baseClean)

			if escaped != tt.escapes {
				t.Errorf("path %q: escaped=%v, want escaped=%v (resolved: %s)",
					tt.repoPath, escaped, tt.escapes, fullTarget)
			}
		})
	}
}
