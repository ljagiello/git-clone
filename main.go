package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	HomeDir = "/Users/lcf/code"
)

func extractGitHubRepo(inputURL string) (string, error) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", err
	}

	path := strings.Trim(parsedURL.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	return parsedURL.Host + "/" + path, nil
}

func ensureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

func removeLastDir(inputPath string) string {
	return filepath.Dir(inputPath)
}

func gitClone(repoURL string, dirPath string) error {
	if err := os.Chdir(dirPath); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	cmd := exec.Command("git", "clone", repoURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, output)
	}

	fmt.Println("Git clone successful. Output:\n", string(output))
	return nil
}

func main() {
	projectToClone := os.Args[1:]

	if len(projectToClone) == 0 || projectToClone[0] == "" {
		println("Please provide a project to clone.")
		os.Exit(1)
	}

	path, err := extractGitHubRepo(projectToClone[0])
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}

	topDir := HomeDir + "/" + removeLastDir(path)

	err = ensureDir(topDir)
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}

	err = gitClone(projectToClone[0], topDir)
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
}
