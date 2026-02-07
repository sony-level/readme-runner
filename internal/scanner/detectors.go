// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// File type detection logic

package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// skipDirs contains directories to skip during scanning
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
	".next":        true,
	".nuxt":        true,
	"coverage":     true,
	".cache":       true,
}

// detectFileType returns the file type constant for a given filename
func detectFileType(name, nameLower, fullPath string) string {
	// Docker files
	switch nameLower {
	case "dockerfile":
		return FileTypeDockerfile
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		return FileTypeCompose
	}

	// Node.js files
	switch nameLower {
	case "package.json":
		return FileTypePackageJSON
	case "package-lock.json":
		return FileTypePackageLock
	case "yarn.lock":
		return FileTypeYarnLock
	case "pnpm-lock.yaml":
		return FileTypePnpmLock
	case "bun.lockb":
		return FileTypeBunLock
	}

	// Python files
	switch nameLower {
	case "pyproject.toml":
		return FileTypePyProject
	case "requirements.txt":
		return FileTypeRequirements
	case "setup.py":
		return FileTypeSetupPy
	case "pipfile":
		return FileTypePipfile
	case "poetry.lock":
		return FileTypePoetryLock
	// Python entry points (only detect at root level, fullPath check)
	case "main.py":
		return FileTypePyMain
	case "app.py":
		return FileTypePyApp
	case "run.py":
		return FileTypePyRun
	case "manage.py":
		return FileTypePyManage
	case "wsgi.py":
		return FileTypePyWsgi
	case "asgi.py":
		return FileTypePyAsgi
	case "__main__.py":
		return FileTypePyDunder
	}

	// Go files
	switch nameLower {
	case "go.mod":
		return FileTypeGoMod
	case "go.sum":
		return FileTypeGoSum
	}

	// Rust files
	switch nameLower {
	case "cargo.toml":
		return FileTypeCargoToml
	case "cargo.lock":
		return FileTypeCargoLock
	}

	// Java/JVM files
	switch nameLower {
	case "pom.xml":
		return FileTypePomXML
	case "build.gradle":
		return FileTypeBuildGradle
	case "build.gradle.kts":
		return FileTypeGradleKts
	case "settings.gradle", "settings.gradle.kts":
		return FileTypeSettingsGradle
	}

	// Make/Build files
	switch nameLower {
	case "makefile":
		return FileTypeMakefile
	case "cmakelists.txt":
		return FileTypeCMakeLists
	}

	// Ruby files
	if nameLower == "gemfile" {
		return FileTypeGemfile
	}

	// PHP files
	if nameLower == "composer.json" {
		return FileTypeComposerJSON
	}

	// .NET files (check by extension)
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".csproj":
		return FileTypeCSProj
	case ".fsproj":
		return FileTypeFSProj
	case ".sln":
		return FileTypeSolution
	}

	// Check for Kubernetes manifests (YAML files with k8s content)
	if ext == ".yaml" || ext == ".yml" {
		if isKubernetesManifest(fullPath) {
			return FileTypeK8sManifest
		}
	}

	return ""
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

// isKubernetesManifest checks if a YAML file is a Kubernetes manifest
func isKubernetesManifest(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	hasApiVersion := false
	hasKind := false

	for scanner.Scan() && lineNum < 20 {
		line := strings.TrimSpace(scanner.Text())
		lineLower := strings.ToLower(line)

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Check for apiVersion
		if strings.HasPrefix(lineLower, "apiversion:") {
			hasApiVersion = true
		}

		// Check for kind
		if strings.HasPrefix(lineLower, "kind:") {
			hasKind = true
		}

		// Early return if both found
		if hasApiVersion && hasKind {
			return true
		}

		lineNum++
	}

	return hasApiVersion && hasKind
}

// ShouldSkipDir checks if a directory should be skipped during scanning
func ShouldSkipDir(name string) bool {
	return skipDirs[name] || strings.HasPrefix(name, ".")
}
