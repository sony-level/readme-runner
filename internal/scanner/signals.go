// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Signal detection for project profiling

package scanner

import (
	"path/filepath"
	"sort"
)

// DetectSignals creates a ProjectProfile from scan results
func DetectSignals(result *ScanResult) *ProjectProfile {
	profile := &ProjectProfile{
		Root:       result.RootPath,
		Readme:     result.ReadmeFile != nil,
		Languages:  []string{},
		Tools:      []string{},
		Signals:    []string{},
		Containers: []string{},
		Packages:   []string{},
	}

	// Collect signals and detect tools from project files
	for fileType, files := range result.ProjectFiles {
		for _, file := range files {
			baseName := filepath.Base(file)
			profile.Signals = append(profile.Signals, baseName)

			detectToolsFromFile(fileType, baseName, profile)
		}
	}

	// Detect languages from project files
	profile.Languages = detectLanguages(result.ProjectFiles)

	// Determine primary stack
	profile.Stack = determinePrimaryStack(profile)

	// Sort and deduplicate all slices
	profile.Languages = uniqueSortedStrings(profile.Languages)
	profile.Tools = uniqueSortedStrings(profile.Tools)
	profile.Signals = uniqueSortedStrings(profile.Signals)
	profile.Containers = uniqueSortedStrings(profile.Containers)
	profile.Packages = uniqueSortedStrings(profile.Packages)

	return profile
}

// detectToolsFromFile identifies tools and categorizes files by type
func detectToolsFromFile(fileType, baseName string, profile *ProjectProfile) {
	switch fileType {
	// Node.js
	case FileTypePackageJSON:
		profile.Tools = append(profile.Tools, "npm")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypePackageLock:
		profile.Tools = append(profile.Tools, "npm")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypeYarnLock:
		profile.Tools = append(profile.Tools, "yarn")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypePnpmLock:
		profile.Tools = append(profile.Tools, "pnpm")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypeBunLock:
		profile.Tools = append(profile.Tools, "bun")
		profile.Packages = append(profile.Packages, baseName)

	// Python
	case FileTypePyProject:
		profile.Tools = append(profile.Tools, "poetry")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypeRequirements:
		profile.Tools = append(profile.Tools, "pip")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypeSetupPy:
		profile.Tools = append(profile.Tools, "pip")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypePipfile:
		profile.Tools = append(profile.Tools, "pipenv")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypePoetryLock:
		profile.Tools = append(profile.Tools, "poetry")
		profile.Packages = append(profile.Packages, baseName)

	// Go
	case FileTypeGoMod:
		profile.Tools = append(profile.Tools, "go")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypeGoSum:
		profile.Packages = append(profile.Packages, baseName)

	// Rust
	case FileTypeCargoToml:
		profile.Tools = append(profile.Tools, "cargo")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypeCargoLock:
		profile.Packages = append(profile.Packages, baseName)

	// .NET
	case FileTypeCSProj, FileTypeFSProj:
		profile.Tools = append(profile.Tools, "dotnet")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypeSolution:
		profile.Tools = append(profile.Tools, "dotnet")
		profile.Packages = append(profile.Packages, baseName)

	// Java/JVM
	case FileTypePomXML:
		profile.Tools = append(profile.Tools, "maven")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypeBuildGradle, FileTypeGradleKts:
		profile.Tools = append(profile.Tools, "gradle")
		profile.Packages = append(profile.Packages, baseName)
	case FileTypeSettingsGradle:
		profile.Packages = append(profile.Packages, baseName)

	// Docker
	case FileTypeDockerfile:
		profile.Tools = append(profile.Tools, "docker")
		profile.Containers = append(profile.Containers, baseName)
	case FileTypeCompose:
		profile.Tools = append(profile.Tools, "docker-compose")
		profile.Containers = append(profile.Containers, baseName)

	// Kubernetes
	case FileTypeK8sManifest:
		profile.Tools = append(profile.Tools, "kubernetes")
		profile.Containers = append(profile.Containers, baseName)

	// Make/Build
	case FileTypeMakefile:
		profile.Tools = append(profile.Tools, "make")
	case FileTypeCMakeLists:
		profile.Tools = append(profile.Tools, "cmake")

	// Ruby
	case FileTypeGemfile:
		profile.Tools = append(profile.Tools, "bundler")
		profile.Packages = append(profile.Packages, baseName)

	// PHP
	case FileTypeComposerJSON:
		profile.Tools = append(profile.Tools, "composer")
		profile.Packages = append(profile.Packages, baseName)
	}
}

// detectLanguages infers programming languages from project files
func detectLanguages(files map[string][]string) []string {
	languages := make(map[string]bool)

	// Detect from project file types
	if _, ok := files[FileTypePackageJSON]; ok {
		languages["javascript"] = true
	}
	if _, ok := files[FileTypeGoMod]; ok {
		languages["go"] = true
	}
	if _, ok := files[FileTypeCargoToml]; ok {
		languages["rust"] = true
	}
	if _, ok := files[FileTypePyProject]; ok {
		languages["python"] = true
	}
	if _, ok := files[FileTypeRequirements]; ok {
		languages["python"] = true
	}
	if _, ok := files[FileTypeSetupPy]; ok {
		languages["python"] = true
	}
	if _, ok := files[FileTypePipfile]; ok {
		languages["python"] = true
	}
	if _, ok := files[FileTypePomXML]; ok {
		languages["java"] = true
	}
	if _, ok := files[FileTypeBuildGradle]; ok {
		languages["java"] = true
	}
	if _, ok := files[FileTypeGradleKts]; ok {
		languages["kotlin"] = true
	}
	if _, ok := files[FileTypeCSProj]; ok {
		languages["csharp"] = true
	}
	if _, ok := files[FileTypeFSProj]; ok {
		languages["fsharp"] = true
	}
	if _, ok := files[FileTypeGemfile]; ok {
		languages["ruby"] = true
	}
	if _, ok := files[FileTypeComposerJSON]; ok {
		languages["php"] = true
	}

	return mapKeysToSlice(languages)
}

// determinePrimaryStack selects the main stack based on priority
func determinePrimaryStack(profile *ProjectProfile) string {
	// Priority order: containers first, then languages
	if containsString(profile.Containers, "Dockerfile") ||
		containsString(profile.Tools, "docker-compose") {
		return "docker"
	}
	if containsString(profile.Tools, "npm") ||
		containsString(profile.Tools, "yarn") ||
		containsString(profile.Tools, "pnpm") ||
		containsString(profile.Tools, "bun") {
		return "node"
	}
	if containsString(profile.Tools, "go") {
		return "go"
	}
	if containsString(profile.Tools, "cargo") {
		return "rust"
	}
	if containsString(profile.Tools, "pip") ||
		containsString(profile.Tools, "poetry") ||
		containsString(profile.Tools, "pipenv") {
		return "python"
	}
	if containsString(profile.Tools, "maven") ||
		containsString(profile.Tools, "gradle") {
		return "java"
	}
	if containsString(profile.Tools, "dotnet") {
		return "dotnet"
	}
	if containsString(profile.Tools, "bundler") {
		return "ruby"
	}
	if containsString(profile.Tools, "composer") {
		return "php"
	}
	if containsString(profile.Tools, "kubernetes") {
		return "kubernetes"
	}

	return "unknown"
}

// Helper functions

// containsString checks if a slice contains a specific string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// uniqueSortedStrings returns a sorted slice with duplicates removed
func uniqueSortedStrings(slice []string) []string {
	if len(slice) == 0 {
		return slice
	}

	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))

	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	sort.Strings(result)
	return result
}

// mapKeysToSlice converts map keys to a slice
func mapKeysToSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
