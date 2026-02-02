// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Scanner types and constants

package scanner

import (
	"sort"
	"time"
)

// File type constants for detection
const (
	// README
	FileTypeReadme = "readme"

	// Docker
	FileTypeDockerfile = "Dockerfile"
	FileTypeCompose    = "docker-compose"

	// Node.js
	FileTypePackageJSON  = "package.json"
	FileTypePackageLock  = "package-lock.json"
	FileTypeYarnLock     = "yarn.lock"
	FileTypePnpmLock     = "pnpm-lock.yaml"
	FileTypeBunLock      = "bun.lockb"

	// Python
	FileTypePyProject    = "pyproject.toml"
	FileTypeRequirements = "requirements.txt"
	FileTypeSetupPy      = "setup.py"
	FileTypePipfile      = "Pipfile"
	FileTypePoetryLock   = "poetry.lock"

	// Go
	FileTypeGoMod = "go.mod"
	FileTypeGoSum = "go.sum"

	// Rust
	FileTypeCargoToml = "Cargo.toml"
	FileTypeCargoLock = "Cargo.lock"

	// .NET
	FileTypeCSProj     = "csproj"
	FileTypeFSProj     = "fsproj"
	FileTypeSolution   = "sln"
	FileTypeNugetPkg   = "packages.config"

	// Java/JVM
	FileTypePomXML     = "pom.xml"
	FileTypeBuildGradle = "build.gradle"
	FileTypeGradleKts  = "build.gradle.kts"
	FileTypeSettingsGradle = "settings.gradle"

	// Kubernetes
	FileTypeK8sManifest = "k8s-manifest"

	// Make/Build
	FileTypeMakefile   = "Makefile"
	FileTypeCMakeLists = "CMakeLists.txt"

	// Other
	FileTypeGemfile    = "Gemfile"
	FileTypeComposerJSON = "composer.json"
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
	RootPath        string              // Scanned root path
	ReadmeFile      *ReadmeInfo         // Primary README.md info
	ProjectFiles    map[string][]string // Map of file type to paths
	TotalFiles      int                 // Total files scanned
	TotalDirs       int                 // Total directories scanned
	ScanDuration    time.Duration       // Time taken to scan
	Errors          []error             // Non-fatal errors during scan
	PackageManagers []string            // Detected package managers
	BuildTools      []string            // Detected build tools
	Profile         *ProjectProfile     // Project profile with signals
}

// ProjectProfile contains project metadata for AI processing
type ProjectProfile struct {
	Root       string   `json:"root"`       // Root path of project
	Readme     bool     `json:"readme"`     // Has README file
	Languages  []string `json:"languages"`  // Detected programming languages
	Tools      []string `json:"tools"`      // Detected tools (npm, docker, etc.)
	Signals    []string `json:"signals"`    // Key file signals detected
	Stack      string   `json:"stack"`      // Primary technology stack
	Containers []string `json:"containers"` // Container/orchestration files
	Packages   []string `json:"packages"`   // Package manifest files
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
	Truncated     bool     // Whether content was truncated
	OriginalSize  int64    // Original size before truncation
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
	stacks := make(map[string]bool)

	// Docker
	if r.HasProjectFile(FileTypeDockerfile) || r.HasProjectFile(FileTypeCompose) {
		stacks["docker"] = true
	}

	// Node.js with package manager detection
	if r.HasProjectFile(FileTypePackageJSON) {
		stacks["node"] = true
		// Detect specific package manager
		switch {
		case r.HasProjectFile(FileTypePnpmLock):
			stacks["pnpm"] = true
		case r.HasProjectFile(FileTypeYarnLock):
			stacks["yarn"] = true
		case r.HasProjectFile(FileTypeBunLock):
			stacks["bun"] = true
		default:
			stacks["npm"] = true
		}
	}

	// Go
	if r.HasProjectFile(FileTypeGoMod) {
		stacks["go"] = true
	}

	// Python with tool detection
	if r.HasProjectFile(FileTypePyProject) || r.HasProjectFile(FileTypeRequirements) ||
		r.HasProjectFile(FileTypeSetupPy) || r.HasProjectFile(FileTypePipfile) {
		stacks["python"] = true
		// Detect package manager
		switch {
		case r.HasProjectFile(FileTypePoetryLock) || r.HasProjectFile(FileTypePyProject):
			stacks["poetry"] = true
		case r.HasProjectFile(FileTypePipfile):
			stacks["pipenv"] = true
		default:
			stacks["pip"] = true
		}
	}

	// Rust
	if r.HasProjectFile(FileTypeCargoToml) {
		stacks["rust"] = true
		stacks["cargo"] = true
	}

	// .NET
	if r.HasProjectFile(FileTypeCSProj) || r.HasProjectFile(FileTypeFSProj) || r.HasProjectFile(FileTypeSolution) {
		stacks["dotnet"] = true
	}

	// Java/JVM
	if r.HasProjectFile(FileTypePomXML) {
		stacks["java"] = true
		stacks["maven"] = true
	}
	if r.HasProjectFile(FileTypeBuildGradle) || r.HasProjectFile(FileTypeGradleKts) {
		stacks["java"] = true
		stacks["gradle"] = true
	}

	// Kubernetes
	if r.HasProjectFile(FileTypeK8sManifest) {
		stacks["kubernetes"] = true
	}

	// Ruby
	if r.HasProjectFile(FileTypeGemfile) {
		stacks["ruby"] = true
		stacks["bundler"] = true
	}

	// PHP
	if r.HasProjectFile(FileTypeComposerJSON) {
		stacks["php"] = true
		stacks["composer"] = true
	}

	// Convert map to sorted slice
	result := make([]string, 0, len(stacks))
	for stack := range stacks {
		result = append(result, stack)
	}
	sort.Strings(result)
	return result
}

// PrimaryStack returns the main technology stack (first non-tool stack)
func (r *ScanResult) PrimaryStack() string {
	// Priority order for primary stack detection
	priorities := []string{"docker", "node", "go", "python", "rust", "java", "dotnet", "ruby", "php"}

	for _, stack := range priorities {
		if r.HasStack(stack) {
			return stack
		}
	}
	return "unknown"
}

// HasStack checks if a specific stack was detected
func (r *ScanResult) HasStack(stack string) bool {
	for _, s := range r.DetectedStacks() {
		if s == stack {
			return true
		}
	}
	return false
}
