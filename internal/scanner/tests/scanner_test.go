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

	if result.ReadmeFile == nil {
		t.Error("README.md not detected")
	}

	if !result.HasProjectFile(scanner.FileTypePackageJSON) {
		t.Error("package.json not detected")
	}

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

	deepPath := filepath.Join(tmpDir, "level1", "level2", "level3", "level4")
	os.MkdirAll(deepPath, 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "l1.txt"), []byte("l1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "level2", "l2.txt"), []byte("l2"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "level2", "level3", "l3.txt"), []byte("l3"), 0644)
	os.WriteFile(filepath.Join(deepPath, "l4.txt"), []byte("l4"), 0644)

	config := &scanner.ScanConfig{
		RootPath: tmpDir,
		MaxDepth: 3,
	}

	result, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3 (MaxDepth=3)", result.TotalFiles)
	}
}

func TestScan_SkipHiddenDirs(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-hidden-*")
	defer os.RemoveAll(tmpDir)

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

	if result.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2 (hidden dir skipped)", result.TotalFiles)
	}
}

func TestScan_DetectsAllFileTypes(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-types-*")
	defer os.RemoveAll(tmpDir)

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

	if !contains(stacks, "docker") {
		t.Error("Docker stack not detected")
	}
	if !contains(stacks, "node") {
		t.Error("Node stack not detected")
	}
}

func TestScan_NodePackageManagers(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected []string
	}{
		{
			name:     "npm project",
			files:    map[string]string{"package.json": `{"name": "test"}`},
			expected: []string{"node", "npm"},
		},
		{
			name:     "yarn project",
			files:    map[string]string{"package.json": `{"name": "test"}`, "yarn.lock": "# yarn"},
			expected: []string{"node", "yarn"},
		},
		{
			name:     "pnpm project",
			files:    map[string]string{"package.json": `{"name": "test"}`, "pnpm-lock.yaml": "# pnpm"},
			expected: []string{"node", "pnpm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := createProjectWithFiles(t, tt.files)
			defer os.RemoveAll(tmpDir)

			result, err := scanner.Scan(&scanner.ScanConfig{RootPath: tmpDir, MaxDepth: 2})
			if err != nil {
				t.Fatalf("Scan() error = %v", err)
			}

			stacks := result.DetectedStacks()
			for _, expected := range tt.expected {
				if !contains(stacks, expected) {
					t.Errorf("Expected stack %s not found in %v", expected, stacks)
				}
			}
		})
	}
}

func TestScan_PythonPackageManagers(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected []string
	}{
		{
			name:     "pip project",
			files:    map[string]string{"requirements.txt": "flask"},
			expected: []string{"python", "pip"},
		},
		{
			name:     "poetry project",
			files:    map[string]string{"pyproject.toml": "[tool.poetry]"},
			expected: []string{"python", "poetry"},
		},
		{
			name:     "pipenv project",
			files:    map[string]string{"Pipfile": "[[source]]"},
			expected: []string{"python", "pipenv"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := createProjectWithFiles(t, tt.files)
			defer os.RemoveAll(tmpDir)

			result, err := scanner.Scan(&scanner.ScanConfig{RootPath: tmpDir, MaxDepth: 2})
			if err != nil {
				t.Fatalf("Scan() error = %v", err)
			}

			stacks := result.DetectedStacks()
			for _, expected := range tt.expected {
				if !contains(stacks, expected) {
					t.Errorf("Expected stack %s not found in %v", expected, stacks)
				}
			}
		})
	}
}

func TestScan_JavaBuildTools(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected []string
	}{
		{
			name:     "maven project",
			files:    map[string]string{"pom.xml": "<project></project>"},
			expected: []string{"java", "maven"},
		},
		{
			name:     "gradle project",
			files:    map[string]string{"build.gradle": "plugins {}"},
			expected: []string{"java", "gradle"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := createProjectWithFiles(t, tt.files)
			defer os.RemoveAll(tmpDir)

			result, err := scanner.Scan(&scanner.ScanConfig{RootPath: tmpDir, MaxDepth: 2})
			if err != nil {
				t.Fatalf("Scan() error = %v", err)
			}

			stacks := result.DetectedStacks()
			for _, expected := range tt.expected {
				if !contains(stacks, expected) {
					t.Errorf("Expected stack %s not found in %v", expected, stacks)
				}
			}
		})
	}
}

func TestScan_DotNetProject(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-dotnet-*")
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "MyApp.csproj"), []byte("<Project></Project>"), 0644)

	result, err := scanner.Scan(&scanner.ScanConfig{RootPath: tmpDir, MaxDepth: 2})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if !result.HasStack("dotnet") {
		t.Error(".NET stack not detected")
	}
}

func TestScan_RustProject(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-rust-*")
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]"), 0644)

	result, err := scanner.Scan(&scanner.ScanConfig{RootPath: tmpDir, MaxDepth: 2})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	stacks := result.DetectedStacks()
	if !contains(stacks, "rust") {
		t.Error("Rust stack not detected")
	}
	if !contains(stacks, "cargo") {
		t.Error("Cargo not detected")
	}
}

func TestScan_KubernetesManifest(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-k8s-*")
	defer os.RemoveAll(tmpDir)

	deployment := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: 1`
	os.WriteFile(filepath.Join(tmpDir, "deployment.yaml"), []byte(deployment), 0644)

	result, err := scanner.Scan(&scanner.ScanConfig{RootPath: tmpDir, MaxDepth: 2})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if !result.HasProjectFile(scanner.FileTypeK8sManifest) {
		t.Error("Kubernetes manifest not detected")
	}

	if !result.HasStack("kubernetes") {
		t.Error("Kubernetes stack not detected")
	}
}

func TestScan_NonK8sYAML(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-yaml-*")
	defer os.RemoveAll(tmpDir)

	yamlContent := `name: test
version: 1.0
config:
  debug: true`
	os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(yamlContent), 0644)

	result, err := scanner.Scan(&scanner.ScanConfig{RootPath: tmpDir, MaxDepth: 2})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.HasProjectFile(scanner.FileTypeK8sManifest) {
		t.Error("Regular YAML incorrectly detected as Kubernetes manifest")
	}
}

func TestScan_PrimaryStack(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected string
	}{
		{
			name:     "docker takes priority",
			files:    map[string]string{"Dockerfile": "FROM node", "package.json": "{}"},
			expected: "docker",
		},
		{
			name:     "node project",
			files:    map[string]string{"package.json": "{}"},
			expected: "node",
		},
		{
			name:     "go project",
			files:    map[string]string{"go.mod": "module test"},
			expected: "go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := createProjectWithFiles(t, tt.files)
			defer os.RemoveAll(tmpDir)

			result, err := scanner.Scan(&scanner.ScanConfig{RootPath: tmpDir, MaxDepth: 2})
			if err != nil {
				t.Fatalf("Scan() error = %v", err)
			}

			if result.PrimaryStack() != tt.expected {
				t.Errorf("PrimaryStack() = %s, want %s", result.PrimaryStack(), tt.expected)
			}
		})
	}
}

func TestParseReadme(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "scanner-readme-*")
	defer os.RemoveAll(tmpDir)

	readmePath := filepath.Join(tmpDir, "README.md")
	content := "# Test Project\n\n## Installation\n\n```bash\nnpm install\n```\n\n## Usage\n\n```sh\nnpm start\n```\n\n## Build\n\n```go\ngo build\n```\n"
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
	if readme.Content == "" {
		t.Error("README content not loaded")
	}
}

func TestExtractCodeBlocks(t *testing.T) {
	content := "# Test\n\n```bash\nnpm install\n```\n\n```python\nprint(\"hello\")\n```\n"

	blocks := scanner.ExtractCodeBlocks(content)

	if len(blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(blocks))
	}

	if blocks[0].Language != "bash" || !blocks[0].IsShell {
		t.Errorf("blocks[0] = {%s, %v}, want {bash, true}", blocks[0].Language, blocks[0].IsShell)
	}

	if blocks[1].Language != "python" || blocks[1].IsShell {
		t.Errorf("blocks[1] = {%s, %v}, want {python, false}", blocks[1].Language, blocks[1].IsShell)
	}
}

func TestGetShellCommands(t *testing.T) {
	content := "# Test\n\n```bash\n$ npm install\nnpm run build\n# comment\n```\n"

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

// Helper functions
func createTestProject(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatal(err)
	}

	readmeContent := "# Test Project\n\n## Installation\n\n```bash\nnpm install\n```\n"
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(readmeContent), 0644)
	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"name": "test"}`), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src", "index.js"), []byte("console.log('hello')"), 0644)

	return tmpDir
}

func createProjectWithFiles(t *testing.T, files map[string]string) string {
	tmpDir, err := os.MkdirTemp("", "project-test-*")
	if err != nil {
		t.Fatal(err)
	}

	for name, content := range files {
		fullPath := filepath.Join(tmpDir, name)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	return tmpDir
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
