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

func extractRepoPath(inputURL string) (string, error) {
	// Handle SSH-style URLs: [user@]host:org/repo.git
	if strings.Contains(inputURL, ":") && !strings.Contains(inputURL, "://") {
		parts := strings.SplitN(inputURL, ":", 2)
		host := parts[0]
		if idx := strings.LastIndex(host, "@"); idx != -1 {
			host = host[idx+1:]
		}
		path := strings.TrimSuffix(strings.Trim(parts[1], "/"), ".git")
		if host == "" || path == "" || !strings.Contains(path, "/") {
			return "", fmt.Errorf("invalid SSH URL (expected [user@]host:org/repo): %s", inputURL)
		}
		return host + "/" + path, nil
	}

	// If no scheme is present, prepend https://
	if !strings.Contains(inputURL, "://") {
		inputURL = "https://" + inputURL
	}

	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Host == "" {
		return "", fmt.Errorf("URL missing host: %s", inputURL)
	}

	// Use Hostname() to strip port numbers
	host := parsedURL.Hostname()

	path := strings.Trim(parsedURL.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" || !strings.Contains(path, "/") {
		return "", fmt.Errorf("URL must contain org/repo path: %s", inputURL)
	}

	return host + "/" + path, nil
}

func gitClone(repoURL, targetDir string) error {
	cmd := exec.Command("git", "clone", repoURL)
	cmd.Dir = targetDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: git-clone <repository-url>")
		os.Exit(1)
	}

	repoURL := os.Args[1]

	repoPath, err := extractRepoPath(repoURL)
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

	if _, err := os.Stat(fullTarget); err == nil {
		fmt.Fprintf(os.Stderr, "already exists: %s\n", fullTarget)
		os.Exit(1)
	}

	// Parent dir (host/org) — git clone will create the repo dir itself
	parentDir := filepath.Dir(fullTarget)

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := gitClone(repoURL, parentDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		exitCode := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		os.Exit(exitCode)
	}
}
