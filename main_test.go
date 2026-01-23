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

		// Host case normalization
		{
			name:         "https uppercase host normalized",
			input:        "https://GitHub.Com/Org/repo",
			wantPath:     "github.com/Org/repo",
			wantCloneURL: "https://GitHub.Com/Org/repo",
		},
		{
			name:         "ssh uppercase host normalized",
			input:        "git@GitHub.COM:org/repo.git",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "git@GitHub.COM:org/repo.git",
		},
		{
			name:         "bare uppercase host normalized",
			input:        "GitHub.com/org/repo",
			wantPath:     "github.com/org/repo",
			wantCloneURL: "https://GitHub.com/org/repo",
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

		// Empty path components rejected
		{
			name:    "double slash in https URL",
			input:   "https://github.com/org//repo",
			wantErr: true,
		},
		{
			name:    "double slash in ssh URL",
			input:   "git@github.com:org//repo",
			wantErr: true,
		},

		// Path traversal rejected at extractRepoPath level
		{
			name:    "path traversal in https URL",
			input:   "https://evil.com/../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "path traversal in ssh URL",
			input:   "git@evil.com:../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "dotdot in middle of https path",
			input:   "https://github.com/org/../other/repo",
			wantErr: true,
		},
		{
			name:    "dotdot in middle of ssh path",
			input:   "git@github.com:org/../other/repo",
			wantErr: true,
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

func TestValidatePathComponents(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "valid org/repo", path: "org/repo", wantErr: false},
		{name: "valid nested", path: "org/sub/repo", wantErr: false},
		{name: "dotdot at start", path: "../org/repo", wantErr: true},
		{name: "dotdot in middle", path: "org/../repo", wantErr: true},
		{name: "dotdot at end", path: "org/repo/..", wantErr: true},
		{name: "single dot is fine", path: "org/./repo", wantErr: false},
		{name: "dotdot as substring is fine", path: "org/..repo/foo", wantErr: false},
		{name: "empty component (double slash)", path: "org//repo", wantErr: true},
		{name: "empty component at start", path: "/org/repo", wantErr: true},
		{name: "empty component at end", path: "org/repo/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathComponents(tt.path)
			if tt.wantErr && err == nil {
				t.Errorf("validatePathComponents(%q) = nil, want error", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validatePathComponents(%q) = %v, want nil", tt.path, err)
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

func TestRemoveEmptyParents(t *testing.T) {
	t.Run("removes empty parents up to base", func(t *testing.T) {
		base := t.TempDir()
		nested := filepath.Join(base, "a", "b", "c")
		if err := os.MkdirAll(nested, 0755); err != nil {
			t.Fatal(err)
		}

		removeEmptyParents(nested, base)

		// All empty dirs should be gone
		if _, err := os.Stat(filepath.Join(base, "a")); !os.IsNotExist(err) {
			t.Errorf("expected 'a' to be removed, but it still exists")
		}
	})

	t.Run("stops at non-empty parent", func(t *testing.T) {
		base := t.TempDir()
		nested := filepath.Join(base, "a", "b", "c")
		if err := os.MkdirAll(nested, 0755); err != nil {
			t.Fatal(err)
		}
		// Put a file in "a/b" so it's not empty after "c" is removed
		f, err := os.Create(filepath.Join(base, "a", "b", "keep.txt"))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		removeEmptyParents(nested, base)

		// "c" should be removed but "b" and "a" should remain
		if _, err := os.Stat(nested); !os.IsNotExist(err) {
			t.Errorf("expected 'c' to be removed")
		}
		if _, err := os.Stat(filepath.Join(base, "a", "b")); err != nil {
			t.Errorf("expected 'a/b' to still exist: %v", err)
		}
	})

	t.Run("never removes base directory", func(t *testing.T) {
		base := t.TempDir()

		removeEmptyParents(base, base)

		if _, err := os.Stat(base); err != nil {
			t.Errorf("base directory should not be removed: %v", err)
		}
	})
}
