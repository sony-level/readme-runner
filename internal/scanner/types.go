// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Scanner types and constants

package scanner

import "time"

// File type constants for detection
const (
	FileTypeReadme      = "readme"
	FileTypePackageJSON = "package.json"
	FileTypeGoMod       = "go.mod"
	FileTypePyProject   = "pyproject.toml"
	FileTypeCargoToml   = "Cargo.toml"
	FileTypeDockerfile  = "Dockerfile"
	FileTypeCompose     = "docker-compose"
	FileTypeMakefile    = "Makefile"
	FileTypeRequirements = "requirements.txt"
)

// ScanConfig holds configuration for scanning
type ScanConfig struct {
	RootPath    string // Root directory to scan (workspace.RepoPath())
	MaxDepth    int    // Maximum directory depth (default: 3)
	FollowLinks bool   // Follow symbolic links (default: false)
	Verbose     bool   // Enable verbose output
}

// ScanResult contains all detected files and metadata
type ScanResult struct {
	RootPath     string              // Scanned root path
	ReadmeFile   *ReadmeInfo         // Primary README.md info
	ProjectFiles map[string][]string // Map of file type to paths
	TotalFiles   int                 // Total files scanned
	TotalDirs    int                 // Total directories scanned
	ScanDuration time.Duration       // Time taken to scan
	Errors       []error             // Non-fatal errors during scan
}

// ReadmeInfo contains README.md metadata
type ReadmeInfo struct {
	Path          string   // Absolute path to README
	RelPath       string   // Relative path from root
	Size          int64    // File size in bytes
	Content       string   // Full content of README (for AI)
	Sections      []string // Detected section headers
	HasInstall    bool     // Has installation section
	HasUsage      bool     // Has usage section
	HasBuild      bool     // Has build section
	HasQuickStart bool     // Has quick start section
	CodeBlocks    int      // Number of code blocks
	ShellCommands int      // Number of shell command blocks
}

// HasProjectFile checks if a specific file type was detected
func (r *ScanResult) HasProjectFile(fileType string) bool {
	paths, ok := r.ProjectFiles[fileType]
	return ok && len(paths) > 0
}

// GetProjectFiles returns all paths for a given file type
func (r *ScanResult) GetProjectFiles(fileType string) []string {
	if paths, ok := r.ProjectFiles[fileType]; ok {
		return paths
	}
	return nil
}

// DetectedStacks returns a list of detected technology stacks
func (r *ScanResult) DetectedStacks() []string {
	var stacks []string

	if r.HasProjectFile(FileTypeDockerfile) || r.HasProjectFile(FileTypeCompose) {
		stacks = append(stacks, "docker")
	}
	if r.HasProjectFile(FileTypePackageJSON) {
		stacks = append(stacks, "node")
	}
	if r.HasProjectFile(FileTypeGoMod) {
		stacks = append(stacks, "go")
	}
	if r.HasProjectFile(FileTypePyProject) || r.HasProjectFile(FileTypeRequirements) {
		stacks = append(stacks, "python")
	}
	if r.HasProjectFile(FileTypeCargoToml) {
		stacks = append(stacks, "rust")
	}

	return stacks
}
