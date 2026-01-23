package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
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
	// Disambiguate from host:port/path by checking if the part after : starts with a digit.
	if strings.Contains(inputURL, ":") && !strings.Contains(inputURL, "://") {
		parts := strings.SplitN(inputURL, ":", 2)
		afterColon := parts[1]

		// If part after : starts with a digit, it's host:port/path, not SSH
		if len(afterColon) > 0 && unicode.IsDigit(rune(afterColon[0])) {
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

	return host + "/" + path, inputURL, nil
}

func gitClone(cloneURL, targetDir string) error {
	cmd := exec.Command("git", "clone", cloneURL)
	cmd.Dir = targetDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: git-clone <repository-url>")
		os.Exit(1)
	}

	repoPath, cloneURL, err := extractRepoPath(os.Args[1])
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
		// Clean up partial clone directory if it was created
		if _, statErr := os.Stat(fullTarget); statErr == nil {
			os.RemoveAll(fullTarget)
		}

		fmt.Fprintln(os.Stderr, err)
		exitCode := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		os.Exit(exitCode)
	}
}
