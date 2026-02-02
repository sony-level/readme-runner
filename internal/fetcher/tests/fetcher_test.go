// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Fetcher tests

package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sony-level/readme-runner/internal/fetcher"
)

func TestDetectSourceType(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected string
	}{
		// GitHub URLs
		{"github https", "https://github.com/user/repo", fetcher.SourceTypeGitHub},
		{"github https with .git", "https://github.com/user/repo.git", fetcher.SourceTypeGitHub},
		{"github https trailing slash", "https://github.com/user/repo/", fetcher.SourceTypeGitHub},
		{"github ssh", "git@github.com:user/repo.git", fetcher.SourceTypeGitHub},
		{"github ssh no .git", "git@github.com:user/repo", fetcher.SourceTypeGitHub},

		// GitLab URLs
		{"gitlab https", "https://gitlab.com/user/repo", fetcher.SourceTypeGitLab},
		{"gitlab https with .git", "https://gitlab.com/user/repo.git", fetcher.SourceTypeGitLab},
		{"gitlab ssh", "git@gitlab.com:user/repo.git", fetcher.SourceTypeGitLab},

		// Local paths
		{"current dir", ".", fetcher.SourceTypeLocal},
		{"parent dir", "..", fetcher.SourceTypeLocal},
		{"relative path", "./some/path", fetcher.SourceTypeLocal},
		{"parent relative", "../other/path", fetcher.SourceTypeLocal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fetcher.DetectSourceType(tt.source)
			if result != tt.expected {
				t.Errorf("DetectSourceType(%q) = %q, want %q", tt.source, result, tt.expected)
			}
		})
	}
}

func TestIsGitHubURL(t *testing.T) {
	validURLs := []string{
		"https://github.com/user/repo",
		"https://github.com/user/repo.git",
		"https://github.com/user/repo/",
		"http://github.com/user/repo",
		"git@github.com:user/repo.git",
		"git@github.com:user/repo",
	}

	for _, url := range validURLs {
		if !fetcher.IsGitHubURL(url) {
			t.Errorf("IsGitHubURL(%q) = false, want true", url)
		}
	}

	invalidURLs := []string{
		"https://gitlab.com/user/repo",
		"https://bitbucket.org/user/repo",
		"git@gitlab.com:user/repo.git",
		".",
		"./local/path",
		"/absolute/path",
	}

	for _, url := range invalidURLs {
		if fetcher.IsGitHubURL(url) {
			t.Errorf("IsGitHubURL(%q) = true, want false", url)
		}
	}
}

func TestIsGitLabURL(t *testing.T) {
	validURLs := []string{
		"https://gitlab.com/user/repo",
		"https://gitlab.com/user/repo.git",
		"git@gitlab.com:user/repo.git",
	}

	for _, url := range validURLs {
		if !fetcher.IsGitLabURL(url) {
			t.Errorf("IsGitLabURL(%q) = false, want true", url)
		}
	}

	invalidURLs := []string{
		"https://github.com/user/repo",
		"git@github.com:user/repo.git",
	}

	for _, url := range invalidURLs {
		if fetcher.IsGitLabURL(url) {
			t.Errorf("IsGitLabURL(%q) = true, want false", url)
		}
	}
}

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		owner    string
		repo     string
		platform string
	}{
		{"github https", "https://github.com/sony-level/readme-runner", "sony-level", "readme-runner", fetcher.SourceTypeGitHub},
		{"github https .git", "https://github.com/sony-level/readme-runner.git", "sony-level", "readme-runner", fetcher.SourceTypeGitHub},
		{"github ssh", "git@github.com:sony-level/readme-runner.git", "sony-level", "readme-runner", fetcher.SourceTypeGitHub},
		{"gitlab https", "https://gitlab.com/user/project", "user", "project", fetcher.SourceTypeGitLab},
		{"gitlab ssh", "git@gitlab.com:user/project.git", "user", "project", fetcher.SourceTypeGitLab},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fetcher.ParseGitURL(tt.url)
			if err != nil {
				t.Fatalf("ParseGitURL(%q) error = %v", tt.url, err)
			}
			if info.Owner != tt.owner {
				t.Errorf("Owner = %q, want %q", info.Owner, tt.owner)
			}
			if info.Repo != tt.repo {
				t.Errorf("Repo = %q, want %q", info.Repo, tt.repo)
			}
			if info.Platform != tt.platform {
				t.Errorf("Platform = %q, want %q", info.Platform, tt.platform)
			}
		})
	}
}

func TestParseGitURL_Invalid(t *testing.T) {
	invalidURLs := []string{
		"https://bitbucket.org/user/repo",
		"not-a-url",
		"",
	}

	for _, url := range invalidURLs {
		_, err := fetcher.ParseGitURL(url)
		if err == nil {
			t.Errorf("ParseGitURL(%q) should return error", url)
		}
	}
}

func TestNormalizeGitURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/user/repo", "https://github.com/user/repo.git"},
		{"git@github.com:user/repo.git", "https://github.com/user/repo.git"},
		{"https://gitlab.com/user/repo", "https://gitlab.com/user/repo.git"},
		{"git@gitlab.com:user/repo.git", "https://gitlab.com/user/repo.git"},
	}

	for _, tt := range tests {
		result := fetcher.NormalizeGitURL(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeGitURL(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestValidateLocalPath(t *testing.T) {
	// Create temp directory for testing
	tmpDir, err := os.MkdirTemp("", "fetcher-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Valid path
	if err := fetcher.ValidateLocalPath(tmpDir); err != nil {
		t.Errorf("ValidateLocalPath(%q) error = %v, want nil", tmpDir, err)
	}

	// Non-existent path
	nonExistent := filepath.Join(tmpDir, "does-not-exist")
	if err := fetcher.ValidateLocalPath(nonExistent); err == nil {
		t.Errorf("ValidateLocalPath(%q) should return error for non-existent path", nonExistent)
	}

	// File instead of directory
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := fetcher.ValidateLocalPath(filePath); err == nil {
		t.Errorf("ValidateLocalPath(%q) should return error for file", filePath)
	}
}

func TestFetchFromLocal(t *testing.T) {
	// Create source directory with some files
	srcDir, err := os.MkdirTemp("", "fetcher-src-*")
	if err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	// Create destination directory
	dstDir, err := os.MkdirTemp("", "fetcher-dst-*")
	if err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	// Create test files in source
	testFiles := map[string]string{
		"file1.txt":        "content1",
		"subdir/file2.txt": "content2",
		"subdir/file3.go":  "package main",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(srcDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create a .git directory that should be skipped
	gitDir := filepath.Join(srcDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0644); err != nil {
		t.Fatalf("Failed to create git config: %v", err)
	}

	// Fetch
	destPath := filepath.Join(dstDir, "repo")
	config := &fetcher.FetchConfig{
		Source:      srcDir,
		Destination: destPath,
		Verbose:     false,
	}

	result, err := fetcher.Fetch(config)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	// Verify result
	if result.SourceType != fetcher.SourceTypeLocal {
		t.Errorf("SourceType = %q, want %q", result.SourceType, fetcher.SourceTypeLocal)
	}
	if result.FilesCopied != 3 {
		t.Errorf("FilesCopied = %d, want 3", result.FilesCopied)
	}

	// Verify files were copied
	for path, content := range testFiles {
		fullPath := filepath.Join(destPath, path)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("Failed to read copied file %s: %v", path, err)
			continue
		}
		if string(data) != content {
			t.Errorf("File %s content = %q, want %q", path, string(data), content)
		}
	}

	// Verify .git was NOT copied
	gitDstDir := filepath.Join(destPath, ".git")
	if _, err := os.Stat(gitDstDir); !os.IsNotExist(err) {
		t.Error(".git directory should not be copied")
	}
}

func TestFetch_Validation(t *testing.T) {
	// Nil config
	_, err := fetcher.Fetch(nil)
	if err == nil {
		t.Error("Fetch(nil) should return error")
	}

	// Empty source
	_, err = fetcher.Fetch(&fetcher.FetchConfig{
		Source:      "",
		Destination: "/tmp/dest",
	})
	if err == nil {
		t.Error("Fetch with empty source should return error")
	}

	// Empty destination
	_, err = fetcher.Fetch(&fetcher.FetchConfig{
		Source:      ".",
		Destination: "",
	})
	if err == nil {
		t.Error("Fetch with empty destination should return error")
	}
}
