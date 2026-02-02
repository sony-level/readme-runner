// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Main fetcher logic

package fetcher

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Fetch fetches a project from a GitHub/GitLab URL or local path
func Fetch(config *FetchConfig) (*FetchResult, error) {
	if config == nil {
		return nil, fmt.Errorf("fetch config is nil")
	}

	if config.Source == "" {
		return nil, fmt.Errorf("source is empty")
	}

	if config.Destination == "" {
		return nil, fmt.Errorf("destination is empty")
	}

	// Set default progress writer
	if config.Progress == nil {
		config.Progress = io.Discard
	}

	// Detect source type
	sourceType := DetectSourceType(config.Source)

	switch sourceType {
	case SourceTypeGitHub, SourceTypeGitLab:
		return fetchFromGit(config, sourceType)
	case SourceTypeLocal:
		return fetchFromLocal(config)
	default:
		return nil, fmt.Errorf("unknown source type for: %s", config.Source)
	}
}

// DetectSourceType determines if the source is a GitHub/GitLab URL or local path
func DetectSourceType(source string) string {
	// Check for GitHub URL patterns
	if IsGitHubURL(source) {
		return SourceTypeGitHub
	}

	// Check for GitLab URL patterns
	if IsGitLabURL(source) {
		return SourceTypeGitLab
	}

	// Check if it's a local path
	if isLocalPath(source) {
		return SourceTypeLocal
	}

	return SourceTypeUnknown
}

// IsGitHubURL checks if the source is a valid GitHub URL
func IsGitHubURL(source string) bool {
	return githubHTTPSPattern.MatchString(source) || githubSSHPattern.MatchString(source)
}

// IsGitLabURL checks if the source is a valid GitLab URL
func IsGitLabURL(source string) bool {
	return gitlabHTTPSPattern.MatchString(source) || gitlabSSHPattern.MatchString(source)
}

// ParseGitURL extracts owner and repo from a GitHub or GitLab URL
func ParseGitURL(url string) (*GitRepoInfo, error) {
	// Try GitHub HTTPS pattern
	if matches := githubHTTPSPattern.FindStringSubmatch(url); matches != nil {
		return &GitRepoInfo{
			Owner:    matches[1],
			Repo:     strings.TrimSuffix(matches[2], ".git"),
			URL:      url,
			Platform: SourceTypeGitHub,
		}, nil
	}

	// Try GitHub SSH pattern
	if matches := githubSSHPattern.FindStringSubmatch(url); matches != nil {
		return &GitRepoInfo{
			Owner:    matches[1],
			Repo:     strings.TrimSuffix(matches[2], ".git"),
			URL:      url,
			Platform: SourceTypeGitHub,
		}, nil
	}

	// Try GitLab HTTPS pattern
	if matches := gitlabHTTPSPattern.FindStringSubmatch(url); matches != nil {
		return &GitRepoInfo{
			Owner:    matches[1],
			Repo:     strings.TrimSuffix(matches[2], ".git"),
			URL:      url,
			Platform: SourceTypeGitLab,
		}, nil
	}

	// Try GitLab SSH pattern
	if matches := gitlabSSHPattern.FindStringSubmatch(url); matches != nil {
		return &GitRepoInfo{
			Owner:    matches[1],
			Repo:     strings.TrimSuffix(matches[2], ".git"),
			URL:      url,
			Platform: SourceTypeGitLab,
		}, nil
	}

	return nil, fmt.Errorf("invalid git URL: %s", url)
}

// isLocalPath checks if the source appears to be a local path
func isLocalPath(source string) bool {
	// Absolute path
	if filepath.IsAbs(source) {
		return true
	}

	// Relative path indicators
	if source == "." || source == ".." {
		return true
	}

	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		return true
	}

	// Check if it exists on the filesystem
	if _, err := os.Stat(source); err == nil {
		return true
	}

	// Could be a relative path that doesn't exist yet
	// but doesn't look like a URL
	if !strings.Contains(source, "://") && !strings.Contains(source, "@") {
		return true
	}

	return false
}

// ValidateLocalPath validates that a local path exists and is readable
func ValidateLocalPath(path string) error {
	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", absPath)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: %s", absPath)
		}
		return fmt.Errorf("failed to stat path: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	return nil
}

// NormalizeGitURL converts various git URL formats to HTTPS
func NormalizeGitURL(url string) string {
	info, err := ParseGitURL(url)
	if err != nil {
		return url
	}

	switch info.Platform {
	case SourceTypeGitHub:
		return fmt.Sprintf("https://github.com/%s/%s.git", info.Owner, info.Repo)
	case SourceTypeGitLab:
		return fmt.Sprintf("https://gitlab.com/%s/%s.git", info.Owner, info.Repo)
	default:
		return url
	}
}
