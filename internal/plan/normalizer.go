// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Plan normalization for OS and lockfile compatibility

package plan

import (
	"runtime"
	"strings"

	"github.com/sony-level/readme-runner/internal/llm"
	"github.com/sony-level/readme-runner/internal/scanner"
)

// Normalizer adjusts plans for the current environment
type Normalizer struct {
	os      string
	profile *scanner.ProjectProfile
}

// NewNormalizer creates a new plan normalizer
func NewNormalizer(profile *scanner.ProjectProfile) *Normalizer {
	return &Normalizer{
		os:      runtime.GOOS,
		profile: profile,
	}
}

// NewNormalizerWithOS creates a normalizer for a specific OS
func NewNormalizerWithOS(os string, profile *scanner.ProjectProfile) *Normalizer {
	return &Normalizer{
		os:      os,
		profile: profile,
	}
}

// Normalize adjusts the plan for the current environment
func (n *Normalizer) Normalize(plan *llm.RunPlan) *llm.RunPlan {
	normalized := *plan
	normalized.Steps = make([]llm.Step, len(plan.Steps))

	for i, step := range plan.Steps {
		normalized.Steps[i] = n.normalizeStep(step)
	}

	// Normalize prerequisites
	normalized.Prerequisites = n.normalizePrerequisites(plan.Prerequisites)

	return &normalized
}

// normalizeStep adjusts a single step
func (n *Normalizer) normalizeStep(step llm.Step) llm.Step {
	normalized := step

	// Normalize package manager commands based on lockfiles
	normalized.Cmd = n.normalizePackageCommand(step.Cmd)

	// Normalize Docker commands
	normalized.Cmd = n.normalizeDockerCommand(normalized.Cmd)

	// Normalize paths for OS
	normalized.Cmd = n.normalizePathSeparators(normalized.Cmd)
	normalized.Cwd = n.normalizePathSeparators(normalized.Cwd)

	// Ensure cwd is set
	if normalized.Cwd == "" {
		normalized.Cwd = "."
	}

	return normalized
}

// normalizePackageCommand adjusts package manager commands based on lockfiles
func (n *Normalizer) normalizePackageCommand(cmd string) string {
	if n.profile == nil {
		return cmd
	}

	// Node.js normalization
	if strings.HasPrefix(cmd, "npm install") && n.hasPackageFile("package-lock.json") {
		return strings.Replace(cmd, "npm install", "npm ci", 1)
	}

	if strings.HasPrefix(cmd, "yarn install") && n.hasPackageFile("yarn.lock") {
		if !strings.Contains(cmd, "--frozen-lockfile") {
			return cmd + " --frozen-lockfile"
		}
	}

	if strings.HasPrefix(cmd, "pnpm install") && n.hasPackageFile("pnpm-lock.yaml") {
		if !strings.Contains(cmd, "--frozen-lockfile") {
			return cmd + " --frozen-lockfile"
		}
	}

	// Python normalization
	if strings.HasPrefix(cmd, "pip install -r") && n.hasPackageFile("requirements.txt") {
		// Prefer --no-cache-dir for reproducibility
		if !strings.Contains(cmd, "--no-cache-dir") {
			return cmd + " --no-cache-dir"
		}
	}

	// Go normalization
	if strings.HasPrefix(cmd, "go build") && !strings.Contains(cmd, "-v") {
		// Add verbose flag for better output
		return strings.Replace(cmd, "go build", "go build -v", 1)
	}

	return cmd
}

// normalizeDockerCommand adjusts Docker commands
func (n *Normalizer) normalizeDockerCommand(cmd string) string {
	// Prefer "docker compose" over "docker-compose"
	if strings.HasPrefix(cmd, "docker-compose ") {
		return strings.Replace(cmd, "docker-compose ", "docker compose ", 1)
	}

	// Add --build flag to docker compose up if not present for dev usage
	if strings.HasPrefix(cmd, "docker compose up") && !strings.Contains(cmd, "--build") {
		// Only for run steps, not explicit build steps
		if !strings.Contains(cmd, "-d") {
			// Interactive mode - might want to rebuild
		}
	}

	return cmd
}

// normalizePathSeparators adjusts path separators for the current OS
func (n *Normalizer) normalizePathSeparators(path string) string {
	if n.os == "windows" {
		return strings.ReplaceAll(path, "/", "\\")
	}
	return strings.ReplaceAll(path, "\\", "/")
}

// normalizePrerequisites adjusts prerequisites for the environment
func (n *Normalizer) normalizePrerequisites(prereqs []llm.Prerequisite) []llm.Prerequisite {
	normalized := make([]llm.Prerequisite, 0, len(prereqs))

	for _, prereq := range prereqs {
		// Handle docker-compose -> docker compose transition
		if prereq.Name == "docker-compose" {
			// Check if docker compose is available (bundled with docker)
			prereq.Name = "docker"
			prereq.Reason = "Docker (with compose plugin) required"
		}

		// Deduplicate
		exists := false
		for _, p := range normalized {
			if p.Name == prereq.Name {
				exists = true
				break
			}
		}
		if !exists {
			normalized = append(normalized, prereq)
		}
	}

	return normalized
}

// hasPackageFile checks if a package file exists in the profile
func (n *Normalizer) hasPackageFile(filename string) bool {
	if n.profile == nil {
		return false
	}

	for _, pkg := range n.profile.Packages {
		if pkg == filename || strings.HasSuffix(pkg, "/"+filename) {
			return true
		}
	}
	return false
}

// hasTool checks if a tool is detected in the profile
func (n *Normalizer) hasTool(toolName string) bool {
	if n.profile == nil {
		return false
	}

	for _, tool := range n.profile.Tools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// hasContainer checks if a container file exists in the profile
func (n *Normalizer) hasContainer(filename string) bool {
	if n.profile == nil {
		return false
	}

	for _, container := range n.profile.Containers {
		if container == filename || strings.HasSuffix(container, "/"+filename) {
			return true
		}
	}
	return false
}

// SuggestDockerPreference returns true if Docker should be preferred
func (n *Normalizer) SuggestDockerPreference() bool {
	if n.profile == nil {
		return false
	}

	// Check for Docker-related files
	dockerFiles := []string{
		"Dockerfile",
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, df := range dockerFiles {
		if n.hasContainer(df) {
			return true
		}
	}

	return false
}

// GetDetectedPackageManager returns the primary package manager
func (n *Normalizer) GetDetectedPackageManager() string {
	if n.profile == nil {
		return ""
	}

	// Priority order based on lockfiles
	lockfileOrder := []struct {
		file    string
		manager string
	}{
		{"bun.lockb", "bun"},
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
		{"poetry.lock", "poetry"},
		{"Pipfile.lock", "pipenv"},
		{"Cargo.lock", "cargo"},
		{"go.sum", "go"},
	}

	for _, lf := range lockfileOrder {
		if n.hasPackageFile(lf.file) {
			return lf.manager
		}
	}

	return ""
}
