// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Tests for ProjectProfile and signal detection

package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sony-level/readme-runner/internal/scanner"
)

func TestProjectProfile_NodeProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-node-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Node.js project files
	createFile(t, tmpDir, "package.json", `{"name": "test", "version": "1.0.0"}`)
	createFile(t, tmpDir, "package-lock.json", `{}`)
	createFile(t, tmpDir, "README.md", "# Test Project\n\n## Installation\n\nnpm install")

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	profile := result.Profile
	if profile == nil {
		t.Fatal("Profile should not be nil")
	}

	// Verify basic fields
	if profile.Root != tmpDir {
		t.Errorf("Root = %s, want %s", profile.Root, tmpDir)
	}

	if !profile.Readme {
		t.Error("Readme should be true")
	}

	// Verify stack detection
	if profile.Stack != "node" {
		t.Errorf("Stack = %s, want node", profile.Stack)
	}

	// Verify languages
	if !containsString(profile.Languages, "javascript") {
		t.Error("JavaScript should be detected")
	}

	// Verify tools
	if !containsString(profile.Tools, "npm") {
		t.Error("npm should be detected")
	}

	// Verify packages
	if !containsString(profile.Packages, "package.json") {
		t.Error("package.json should be in packages")
	}

	// Verify signals
	if !containsString(profile.Signals, "package.json") {
		t.Error("package.json should be in signals")
	}
}

func TestProjectProfile_DockerProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-docker-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Docker project files
	createFile(t, tmpDir, "Dockerfile", "FROM node:18\nWORKDIR /app")
	createFile(t, tmpDir, "docker-compose.yml", "version: '3'\nservices:\n  app:\n    build: .")
	createFile(t, tmpDir, "package.json", `{"name": "test"}`)

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	profile := result.Profile

	// Docker should be primary stack when present
	if profile.Stack != "docker" {
		t.Errorf("Stack = %s, want docker", profile.Stack)
	}

	// Verify containers
	if !containsString(profile.Containers, "Dockerfile") {
		t.Error("Dockerfile should be in containers")
	}
	if !containsString(profile.Containers, "docker-compose.yml") {
		t.Error("docker-compose.yml should be in containers")
	}

	// Verify tools
	if !containsString(profile.Tools, "docker") {
		t.Error("docker should be in tools")
	}
	if !containsString(profile.Tools, "docker-compose") {
		t.Error("docker-compose should be in tools")
	}
}

func TestProjectProfile_PythonProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-python-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Python project files
	createFile(t, tmpDir, "pyproject.toml", "[project]\nname = \"test\"")
	createFile(t, tmpDir, "requirements.txt", "flask>=2.0.0")

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	profile := result.Profile

	if profile.Stack != "python" {
		t.Errorf("Stack = %s, want python", profile.Stack)
	}

	if !containsString(profile.Languages, "python") {
		t.Error("python should be in languages")
	}

	// Should detect both pip and poetry
	if !containsString(profile.Tools, "pip") && !containsString(profile.Tools, "poetry") {
		t.Error("pip or poetry should be in tools")
	}
}

func TestProjectProfile_GoProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-go-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Go project files
	createFile(t, tmpDir, "go.mod", "module test\n\ngo 1.21")
	createFile(t, tmpDir, "go.sum", "")

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	profile := result.Profile

	if profile.Stack != "go" {
		t.Errorf("Stack = %s, want go", profile.Stack)
	}

	if !containsString(profile.Languages, "go") {
		t.Error("go should be in languages")
	}

	if !containsString(profile.Tools, "go") {
		t.Error("go should be in tools")
	}
}

func TestProjectProfile_RustProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-rust-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Rust project files
	createFile(t, tmpDir, "Cargo.toml", "[package]\nname = \"test\"\nversion = \"0.1.0\"")
	createFile(t, tmpDir, "Cargo.lock", "")

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	profile := result.Profile

	if profile.Stack != "rust" {
		t.Errorf("Stack = %s, want rust", profile.Stack)
	}

	if !containsString(profile.Languages, "rust") {
		t.Error("rust should be in languages")
	}

	if !containsString(profile.Tools, "cargo") {
		t.Error("cargo should be in tools")
	}
}

func TestProjectProfile_MixedProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-mixed-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mixed project with multiple stacks
	createFile(t, tmpDir, "package.json", `{"name": "frontend"}`)
	createFile(t, tmpDir, "go.mod", "module backend\n\ngo 1.21")
	createFile(t, tmpDir, "Makefile", "all:\n\tgo build")

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	profile := result.Profile

	// Should detect multiple languages
	if len(profile.Languages) < 2 {
		t.Errorf("Expected at least 2 languages, got %d: %v", len(profile.Languages), profile.Languages)
	}

	// Should detect multiple tools
	if len(profile.Tools) < 2 {
		t.Errorf("Expected at least 2 tools, got %d: %v", len(profile.Tools), profile.Tools)
	}

	// Make should be detected
	if !containsString(profile.Tools, "make") {
		t.Error("make should be in tools")
	}
}

func TestProjectProfile_NoReadme(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-noreadme-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create project without README
	createFile(t, tmpDir, "package.json", `{"name": "test"}`)

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	profile := result.Profile

	if profile.Readme {
		t.Error("Readme should be false when no README exists")
	}
}

func TestProjectProfile_EmptyProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-empty-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	profile := result.Profile

	if profile.Stack != "unknown" {
		t.Errorf("Stack = %s, want unknown for empty project", profile.Stack)
	}

	if len(profile.Languages) != 0 {
		t.Errorf("Expected 0 languages for empty project, got %d", len(profile.Languages))
	}
}

func TestReadme_Truncation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "readme-truncate-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a large README (> 512KB)
	largeContent := strings.Repeat("# Large README\n\nThis is a line of content for testing.\n", 20000)
	createFile(t, tmpDir, "README.md", largeContent)

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	readme := result.ReadmeFile
	if readme == nil {
		t.Fatal("README should be found")
	}

	if !readme.Truncated {
		t.Error("README should be marked as truncated")
	}

	if readme.OriginalSize <= scanner.MaxReadmeSize {
		t.Errorf("OriginalSize (%d) should be > MaxReadmeSize (%d)",
			readme.OriginalSize, scanner.MaxReadmeSize)
	}

	if int64(len(readme.Content)) >= readme.OriginalSize {
		t.Errorf("Content length (%d) should be < OriginalSize (%d)",
			len(readme.Content), readme.OriginalSize)
	}
}

func TestReadme_SmallFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "readme-small-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := "# Small README\n\nThis is a small file.\n"
	createFile(t, tmpDir, "README.md", content)

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	readme := result.ReadmeFile
	if readme == nil {
		t.Fatal("README should be found")
	}

	if readme.Truncated {
		t.Error("Small README should not be truncated")
	}

	if readme.Content != content {
		t.Errorf("Content mismatch: got %q, want %q", readme.Content, content)
	}
}

func TestDetectSignals_UniqueValues(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "signals-unique-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create project with package.json (which adds npm to tools)
	createFile(t, tmpDir, "package.json", `{}`)
	createFile(t, tmpDir, "package-lock.json", `{}`)

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	profile := result.Profile

	// Check that npm appears only once in tools
	npmCount := 0
	for _, tool := range profile.Tools {
		if tool == "npm" {
			npmCount++
		}
	}
	if npmCount > 1 {
		t.Errorf("npm appears %d times in tools, should appear only once", npmCount)
	}
}

// Helper functions

func createFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create file %s: %v", name, err)
	}
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
