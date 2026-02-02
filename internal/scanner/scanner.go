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

// Default directories to skip during scanning
var skipDirs = map[string]bool{
	".git":         true,
	".svn":         true,
	".hg":          true,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"target":       true,
	".rr-temp":     true,
	".idea":        true,
	".vscode":      true,
	"dist":         true,
	"build":        true,
	".expo":        true,
}

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
			// Collect non-fatal errors
			result.Errors = append(result.Errors, err)
			return nil // Continue scanning
		}

		// Calculate depth
		relPath, _ := filepath.Rel(config.RootPath, path)
		depth := 0
		if relPath != "." {
			depth = strings.Count(relPath, string(os.PathSeparator)) + 1
		}

		// Handle directories
		if d.IsDir() {
			// Skip root directory from counting
			if path == config.RootPath {
				return nil
			}

			// Skip if too deep
			if depth > config.MaxDepth {
				return filepath.SkipDir
			}

			// Skip hidden directories and known skip dirs
			name := d.Name()
			if strings.HasPrefix(name, ".") || skipDirs[name] {
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

	// Detect README (first one found at root level wins)
	if isReadmeFile(nameLower) {
		// Prefer root-level README
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

	// Detect other project files
	switch nameLower {
	case "package.json":
		result.ProjectFiles[FileTypePackageJSON] = append(result.ProjectFiles[FileTypePackageJSON], relPath)
	case "go.mod":
		result.ProjectFiles[FileTypeGoMod] = append(result.ProjectFiles[FileTypeGoMod], relPath)
	case "pyproject.toml", "setup.py":
		result.ProjectFiles[FileTypePyProject] = append(result.ProjectFiles[FileTypePyProject], relPath)
	case "requirements.txt":
		result.ProjectFiles[FileTypeRequirements] = append(result.ProjectFiles[FileTypeRequirements], relPath)
	case "cargo.toml":
		result.ProjectFiles[FileTypeCargoToml] = append(result.ProjectFiles[FileTypeCargoToml], relPath)
	case "dockerfile":
		result.ProjectFiles[FileTypeDockerfile] = append(result.ProjectFiles[FileTypeDockerfile], relPath)
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		result.ProjectFiles[FileTypeCompose] = append(result.ProjectFiles[FileTypeCompose], relPath)
	case "makefile":
		result.ProjectFiles[FileTypeMakefile] = append(result.ProjectFiles[FileTypeMakefile], relPath)
	}
}

// isReadmeFile checks if filename matches README pattern
func isReadmeFile(name string) bool {
	name = strings.ToLower(name)
	return name == "readme.md" ||
		name == "readme.markdown" ||
		name == "readme.txt" ||
		name == "readme" ||
		name == "readme.rst"
}
