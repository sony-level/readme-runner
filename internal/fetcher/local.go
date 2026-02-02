// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Local project copying implementation

package fetcher

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Default patterns to skip when copying
var defaultSkipPatterns = []string{
	".git",
	".rr-temp",
	"node_modules",
	"__pycache__",
	".venv",
	"venv",
	"target", // Rust
	"vendor", // Go (optional, but often large)
	".expo", // Expo
	".expo-build", // Expo
}

// fetchFromLocal copies a local directory to the destination
func fetchFromLocal(config *FetchConfig) (*FetchResult, error) {
	// Validate source path
	if err := ValidateLocalPath(config.Source); err != nil {
		return nil, err
	}

	// Resolve absolute paths
	srcPath, err := filepath.Abs(config.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source path: %w", err)
	}

	// Load gitignore patterns if present
	ignorePatterns := loadGitignore(srcPath)
	ignorePatterns = append(ignorePatterns, defaultSkipPatterns...)

	if config.Verbose {
		fmt.Fprintf(config.Progress, "Copying local project: %s\n", srcPath)
		fmt.Fprintf(config.Progress, "Destination: %s\n", config.Destination)
		if len(ignorePatterns) > 0 {
			fmt.Fprintf(config.Progress, "Ignoring %d patterns\n", len(ignorePatterns))
		}
	}

	// Check if source is a git repository
	isGitRepo := isGitRepository(srcPath)

	// Copy directory
	var filesCopied int
	var bytesCopied int64

	err = walkDir(srcPath, func(path string, info os.FileInfo) error {
		// Get relative path from source
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Check if should skip this path
		if shouldSkip(relPath, ignorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Destination path
		destPath := filepath.Join(config.Destination, relPath)

		if info.IsDir() {
			// Create directory
			if err := os.MkdirAll(destPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}
			return nil
		}

		// Copy file
		copied, err := copyFile(path, destPath, info)
		if err != nil {
			return fmt.Errorf("failed to copy %s: %w", relPath, err)
		}

		filesCopied++
		bytesCopied += copied

		return nil
	})

	if err != nil {
		// Clean up partial copy on failure
		_ = os.RemoveAll(config.Destination)
		return nil, fmt.Errorf("failed to copy project: %w", err)
	}

	if config.Verbose {
		fmt.Fprintf(config.Progress, "Copied %d files (%d bytes)\n", filesCopied, bytesCopied)
	}

	return &FetchResult{
		Source:      config.Source,
		Destination: config.Destination,
		SourceType:  SourceTypeLocal,
		IsGitRepo:   isGitRepo,
		FilesCopied: filesCopied,
		BytesCopied: bytesCopied,
	}, nil
}

// walkDir walks a directory tree, calling walkFn for each file or directory
func walkDir(root string, walkFn func(path string, info os.FileInfo) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return walkFn(path, info)
	})
}

// shouldSkip checks if a path should be skipped based on patterns
func shouldSkip(relPath string, patterns []string) bool {
	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	for _, pattern := range patterns {
		// Exact match
		if relPath == pattern {
			return true
		}

		// Check if any path component matches
		parts := strings.Split(relPath, "/")
		for _, part := range parts {
			if part == pattern {
				return true
			}
		}

		// Prefix match for directories
		if strings.HasPrefix(relPath, pattern+"/") {
			return true
		}
	}

	return false
}

// loadGitignore loads patterns from .gitignore file
func loadGitignore(dir string) []string {
	gitignorePath := filepath.Join(dir, ".gitignore")
	file, err := os.Open(gitignorePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Remove trailing slashes for directory patterns
		line = strings.TrimSuffix(line, "/")
		patterns = append(patterns, line)
	}

	return patterns
}

// isGitRepository checks if a directory is a git repository
func isGitRepository(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// copyFile copies a single file preserving permissions
func copyFile(src, dst string, info os.FileInfo) (int64, error) {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return 0, err
	}

	// Handle symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return 0, err
		}
		return 0, os.Symlink(target, dst)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return 0, err
	}
	defer dstFile.Close()

	// Copy contents
	written, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return 0, err
	}

	return written, nil
}
