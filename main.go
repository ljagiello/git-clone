package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func baseDir() (string, error) {
	if env := os.Getenv("GIT_CLONE_ROOT"); env != "" {
		abs, err := filepath.Abs(env)
		if err != nil {
			return "", fmt.Errorf("invalid GIT_CLONE_ROOT: %w", err)
		}
		return abs, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, "code"), nil
}

// extractRepoPath parses the input URL and returns:
//   - repoPath: the host/org/repo directory structure
//   - cloneURL: the normalized URL to pass to git clone
func extractRepoPath(inputURL string) (repoPath string, cloneURL string, err error) {
	// Handle SSH-style URLs: [user@]host:org/repo.git
	// Disambiguate from host:port/path by checking if the part after : starts with an ASCII digit.
	if strings.Contains(inputURL, ":") && !strings.Contains(inputURL, "://") {
		parts := strings.SplitN(inputURL, ":", 2)
		afterColon := parts[1]

		// If part after : starts with an ASCII digit, it's host:port/path, not SSH
		if len(afterColon) > 0 && afterColon[0] >= '0' && afterColon[0] <= '9' {
			// Treat as bare URL with port — fall through to HTTP parsing
			inputURL = "https://" + inputURL
		} else {
			host := parts[0]
			if idx := strings.LastIndex(host, "@"); idx != -1 {
				host = host[idx+1:]
			}
			path := strings.TrimSuffix(strings.Trim(afterColon, "/"), ".git")
			if host == "" || path == "" || !strings.Contains(path, "/") {
				return "", "", fmt.Errorf("invalid SSH URL (expected [user@]host:org/repo): %s", inputURL)
			}
			if err := validatePathComponents(path); err != nil {
				return "", "", err
			}
			return host + "/" + path, inputURL, nil
		}
	}

	// If no scheme is present, prepend https://
	if !strings.Contains(inputURL, "://") {
		inputURL = "https://" + inputURL
	}

	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Host == "" {
		return "", "", fmt.Errorf("URL missing host: %s", inputURL)
	}

	// Use Hostname() to strip port numbers for directory naming
	host := parsedURL.Hostname()

	path := strings.Trim(parsedURL.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" || !strings.Contains(path, "/") {
		return "", "", fmt.Errorf("URL must contain org/repo path: %s", inputURL)
	}
	if err := validatePathComponents(path); err != nil {
		return "", "", err
	}

	return host + "/" + path, inputURL, nil
}

// validatePathComponents rejects paths containing ".." or empty components,
// which are never valid org/repo names.
func validatePathComponents(path string) error {
	for _, component := range strings.Split(path, "/") {
		if component == "" {
			return fmt.Errorf("empty path component in: %s", path)
		}
		if component == ".." {
			return fmt.Errorf("invalid path component '..': %s", path)
		}
	}
	return nil
}

func gitClone(cloneURL, targetDir string) error {
	cmd := exec.Command("git", "clone", cloneURL)
	cmd.Dir = targetDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr // git output goes to stderr; stdout reserved for path output
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// shouldCleanup checks whether the partial clone at fullTarget should be
// removed. It only returns true if the remote URL matches our intent AND
// the clone is not recoverable (HEAD doesn't resolve to a valid commit).
func shouldCleanup(fullTarget, expectedURL string) bool {
	// Verify remote matches our clone URL
	urlCmd := exec.Command("git", "-C", fullTarget, "config", "--get", "remote.origin.url")
	out, err := urlCmd.Output()
	if err != nil {
		return false
	}
	if strings.TrimSpace(string(out)) != expectedURL {
		return false
	}

	// If HEAD resolves, the repo has all objects and is recoverable
	// (e.g., checkout failed but data is intact) — don't delete it
	headCmd := exec.Command("git", "-C", fullTarget, "rev-parse", "--verify", "HEAD")
	if headCmd.Run() == nil {
		return false
	}

	return true
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: git-clone <repository-url>")
		os.Exit(1)
	}

	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "Usage: git-clone <repository-url>")
		fmt.Fprintf(os.Stderr, "unexpected extra arguments: %s\n", strings.Join(os.Args[2:], " "))
		os.Exit(1)
	}

	arg := os.Args[1]
	if strings.HasPrefix(arg, "-") {
		fmt.Fprintln(os.Stderr, "Usage: git-clone <repository-url>")
		os.Exit(1)
	}

	repoPath, cloneURL, err := extractRepoPath(arg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	base, err := baseDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Resolve and verify the target stays within base directory
	fullTarget := filepath.Clean(filepath.Join(base, repoPath))
	baseClean := filepath.Clean(base) + string(os.PathSeparator)
	if !strings.HasPrefix(fullTarget+string(os.PathSeparator), baseClean) {
		fmt.Fprintf(os.Stderr, "resolved path escapes base directory: %s\n", fullTarget)
		os.Exit(1)
	}

	if info, err := os.Stat(fullTarget); err == nil {
		kind := "path"
		if info.IsDir() {
			kind = "directory"
		}
		fmt.Fprintf(os.Stderr, "already exists (%s): %s\n", kind, fullTarget)
		os.Exit(1)
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "cannot access %s: %v\n", fullTarget, err)
		os.Exit(1)
	}

	// Parent dir (host/org) — git clone will create the repo dir itself
	parentDir := filepath.Dir(fullTarget)

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := gitClone(cloneURL, parentDir); err != nil {
		// Clean up if the directory was created by our clone attempt and is not recoverable
		if _, statErr := os.Stat(fullTarget); statErr == nil {
			if shouldCleanup(fullTarget, cloneURL) {
				if rmErr := os.RemoveAll(fullTarget); rmErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to clean up partial clone: %v\n", rmErr)
				}
			}
		}

		fmt.Fprintln(os.Stderr, err)
		exitCode := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		os.Exit(exitCode)
	}

	fmt.Println(fullTarget)
}
