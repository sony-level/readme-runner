// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Main scanner logic

package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Scan performs a file system scan of the workspace
func Scan(config *ScanConfig) (*ScanResult, error) {
	if config == nil {
		return nil, fmt.Errorf("scan config cannot be nil")
	}

	if config.RootPath == "" {
		return nil, fmt.Errorf("root path cannot be empty")
	}

	// Validate root path exists
	info, err := os.Stat(config.RootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access root path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", config.RootPath)
	}

	// Set defaults
	if config.MaxDepth == 0 {
		config.MaxDepth = 3
	}

	startTime := time.Now()

	result := &ScanResult{
		RootPath:     config.RootPath,
		ProjectFiles: make(map[string][]string),
		Errors:       []error{},
	}

	// Walk the directory tree
	err = filepath.WalkDir(config.RootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, err)
			return nil
		}

		// Calculate depth
		relPath, _ := filepath.Rel(config.RootPath, path)
		depth := 0
		if relPath != "." {
			depth = strings.Count(relPath, string(os.PathSeparator)) + 1
		}

		// Handle directories
		if d.IsDir() {
			if path == config.RootPath {
				return nil
			}

			if depth > config.MaxDepth {
				return filepath.SkipDir
			}

			if ShouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}

			result.TotalDirs++
			return nil
		}

		// Skip symlinks unless configured
		if d.Type()&os.ModeSymlink != 0 && !config.FollowLinks {
			return nil
		}

		// Skip if too deep
		if depth > config.MaxDepth {
			return nil
		}

		// Process files
		result.TotalFiles++
		detectProjectFile(path, relPath, result)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	result.ScanDuration = time.Since(startTime)

	return result, nil
}

// detectProjectFile identifies and categorizes project files
func detectProjectFile(path, relPath string, result *ScanResult) {
	name := filepath.Base(path)
	nameLower := strings.ToLower(name)

	// Detect README (prefer root-level)
	if isReadmeFile(nameLower) {
		if result.ReadmeFile == nil || !strings.Contains(relPath, string(os.PathSeparator)) {
			readme, err := ParseReadme(path, relPath)
			if err == nil {
				result.ReadmeFile = readme
			} else {
				result.Errors = append(result.Errors, fmt.Errorf("failed to parse README: %w", err))
			}
		}
		return
	}

	// Detect project files
	fileType := detectFileType(name, nameLower, path)
	if fileType != "" {
		result.ProjectFiles[fileType] = append(result.ProjectFiles[fileType], relPath)
	}
}
