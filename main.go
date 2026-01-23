package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func baseDir() (string, error) {
	if env := os.Getenv("GIT_CLONE_ROOT"); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, "code"), nil
}

func extractRepoPath(inputURL string) (string, error) {
	// Handle SSH-style URLs: git@host:org/repo.git
	if strings.Contains(inputURL, ":") && !strings.Contains(inputURL, "://") {
		parts := strings.SplitN(inputURL, ":", 2)
		host := strings.TrimPrefix(parts[0], "git@")
		path := strings.TrimSuffix(strings.Trim(parts[1], "/"), ".git")
		if host == "" || path == "" {
			return "", fmt.Errorf("invalid SSH URL: %s", inputURL)
		}
		return host + "/" + path, nil
	}

	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Host == "" {
		return "", fmt.Errorf("URL missing host: %s", inputURL)
	}

	path := strings.Trim(parsedURL.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" || !strings.Contains(path, "/") {
		return "", fmt.Errorf("URL must contain org/repo path: %s", inputURL)
	}

	return parsedURL.Host + "/" + path, nil
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

	// Parent dir (host/org) — git clone will create the repo dir itself
	parentDir := filepath.Join(base, filepath.Dir(repoPath))

	fullTarget := filepath.Join(base, repoPath)
	if info, err := os.Stat(fullTarget); err == nil && info.IsDir() {
		fmt.Fprintf(os.Stderr, "already exists: %s\n", fullTarget)
		os.Exit(1)
	}

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := gitClone(repoURL, parentDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
