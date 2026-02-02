// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Scanner tests

package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sony-level/readme-runner/internal/scanner"
)

func TestScan_BasicStructure(t *testing.T) {
	tmpDir := createTestProject(t)
	defer os.RemoveAll(tmpDir)

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
		Verbose:  false,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Verify README was found
	if result.ReadmeFile == nil {
		t.Error("README.md not detected")
	}

	// Verify package.json detected
	if !result.HasProjectFile(scanner.FileTypePackageJSON) {
		t.Error("package.json not detected")
	}

	// Verify total files
	if result.TotalFiles < 2 {
		t.Errorf("TotalFiles = %d, want >= 2", result.TotalFiles)
	}
}

func TestScan_NilConfig(t *testing.T) {
	_, err := scanner.Scan(nil)
	if err == nil {
		t.Error("Scan(nil) should return error")
	}
}

func TestScan_EmptyRootPath(t *testing.T) {
	_, err := scanner.Scan(&scanner.ScanConfig{RootPath: ""})
	if err == nil {
		t.Error("Scan with empty root path should return error")
	}
}

func TestScan_InvalidPath(t *testing.T) {
	_, err := scanner.Scan(&scanner.ScanConfig{RootPath: "/nonexistent/path"})
	if err == nil {
		t.Error("Scan with invalid path should return error")
	}
}

func TestScan_MaxDepth(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-depth-*")
	defer os.RemoveAll(tmpDir)

	// Create nested structure: root/level1/level2/level3/level4
	deepPath := filepath.Join(tmpDir, "level1", "level2", "level3", "level4")
	os.MkdirAll(deepPath, 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "l1.txt"), []byte("l1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "level2", "l2.txt"), []byte("l2"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "level2", "level3", "l3.txt"), []byte("l3"), 0644)
	os.WriteFile(filepath.Join(deepPath, "l4.txt"), []byte("l4"), 0644)

	// Scan with MaxDepth=3
	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// With MaxDepth=3, should find root.txt (depth 1), l1.txt (depth 2), l2.txt (depth 3)
	// but not l3.txt (depth 4) or l4.txt (depth 5)
	if result.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3 (MaxDepth=3)", result.TotalFiles)
	}
}

func TestScan_SkipHiddenDirs(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-hidden-*")
	defer os.RemoveAll(tmpDir)

	// Create structure with hidden dir
	os.MkdirAll(filepath.Join(tmpDir, ".hidden"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "visible"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".hidden", "secret.txt"), []byte("secret"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "visible", "public.txt"), []byte("public"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0644)

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Should find root.txt and public.txt, but not secret.txt
	if result.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2 (hidden dir skipped)", result.TotalFiles)
	}
}

func TestScan_DetectsAllFileTypes(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-types-*")
	defer os.RemoveAll(tmpDir)

	// Create various project files
	files := map[string]string{
		"README.md":          "# Test",
		"package.json":       "{}",
		"go.mod":             "module test",
		"pyproject.toml":     "[project]",
		"requirements.txt":   "flask",
		"Cargo.toml":         "[package]",
		"Dockerfile":         "FROM alpine",
		"docker-compose.yml": "version: '3'",
		"Makefile":           "all:",
	}

	for name, content := range files {
		os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
	}

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 1,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Verify all file types detected
	expectedTypes := []string{
		scanner.FileTypePackageJSON,
		scanner.FileTypeGoMod,
		scanner.FileTypePyProject,
		scanner.FileTypeRequirements,
		scanner.FileTypeCargoToml,
		scanner.FileTypeDockerfile,
		scanner.FileTypeCompose,
		scanner.FileTypeMakefile,
	}

	for _, fileType := range expectedTypes {
		if !result.HasProjectFile(fileType) {
			t.Errorf("%s not detected", fileType)
		}
	}

	// Verify README
	if result.ReadmeFile == nil {
		t.Error("README.md not detected")
	}
}

func TestScan_DetectedStacks(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-stacks-*")
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte("FROM node"), 0644)

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 1,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	stacks := result.DetectedStacks()

	// Should detect docker and node
	hasDocker := false
	hasNode := false
	for _, stack := range stacks {
		if stack == "docker" {
			hasDocker = true
		}
		if stack == "node" {
			hasNode = true
		}
	}

	if !hasDocker {
		t.Error("Docker stack not detected")
	}
	if !hasNode {
		t.Error("Node stack not detected")
	}
}

func TestParseReadme(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-readme-*")
	defer os.RemoveAll(tmpDir)

	readmePath := filepath.Join(tmpDir, "README.md")
	content := "# Test Project\n\n## Installation\n\nRun the following:\n\n```bash\nnpm install\nnpm run build\n```\n\n## Usage\n\nStart the app:\n\n```sh\nnpm start\n```\n\n## Build\n\n```go\ngo build\n```"

	os.WriteFile(readmePath, []byte(content), 0644)

	readme, err := scanner.ParseReadme(readmePath, "README.md")
	if err != nil {
		t.Fatalf("ParseReadme() error = %v", err)
	}

	if !readme.HasInstall {
		t.Error("Installation section not detected")
	}

	if !readme.HasUsage {
		t.Error("Usage section not detected")
	}

	if !readme.HasBuild {
		t.Error("Build section not detected")
	}

	if readme.CodeBlocks != 3 {
		t.Errorf("CodeBlocks = %d, want 3", readme.CodeBlocks)
	}

	if readme.ShellCommands != 2 {
		t.Errorf("ShellCommands = %d, want 2", readme.ShellCommands)
	}

	if len(readme.Sections) != 4 {
		t.Errorf("Sections count = %d, want 4", len(readme.Sections))
	}
}

func TestExtractCodeBlocks(t *testing.T) {
	content := "# Test\n\n```bash\nnpm install\n```\n\n```python\nprint(\"hello\")\n```"

	blocks := scanner.ExtractCodeBlocks(content)

	if len(blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(blocks))
	}

	if blocks[0].Language != "bash" {
		t.Errorf("blocks[0].Language = %q, want bash", blocks[0].Language)
	}

	if !blocks[0].IsShell {
		t.Error("blocks[0].IsShell should be true")
	}

	if blocks[1].Language != "python" {
		t.Errorf("blocks[1].Language = %q, want python", blocks[1].Language)
	}

	if blocks[1].IsShell {
		t.Error("blocks[1].IsShell should be false")
	}
}

func TestGetShellCommands(t *testing.T) {
	content := "# Test\n\n```bash\n$ npm install\nnpm run build\n# This is a comment\n```\n\n```python\nprint(\"not shell\")\n```"

	commands := scanner.GetShellCommands(content)

	if len(commands) != 2 {
		t.Fatalf("len(commands) = %d, want 2", len(commands))
	}

	if commands[0] != "npm install" {
		t.Errorf("commands[0] = %q, want 'npm install'", commands[0])
	}

	if commands[1] != "npm run build" {
		t.Errorf("commands[1] = %q, want 'npm run build'", commands[1])
	}
}


func createTestProject(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// Create README
	readmeContent := "# Test Project\n\n## Installation\n\n```bash\nnpm install\n```\n"
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(readmeContent), 0644)

	// Create package.json
	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"name": "test"}`), 0644)

	// Create src directory
	os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src", "index.js"), []byte("console.log('hello')"), 0644)

	return tmpDir
}
