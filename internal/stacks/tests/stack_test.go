// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Stack detection tests

package tests

import (
	"testing"

	"github.com/sony-level/readme-runner/internal/scanner"
	"github.com/sony-level/readme-runner/internal/stacks"
)

func TestDockerDetector_ComposeProject(t *testing.T) {
	detector := stacks.NewDockerDetector()

	profile := &scanner.ProjectProfile{
		Signals:    []string{"docker-compose.yml", "Dockerfile"},
		Tools:      []string{"docker", "docker-compose"},
		Containers: []string{"Dockerfile", "docker-compose.yml"},
	}

	match, found := detector.Detect(profile)
	if !found {
		t.Fatal("Docker should be detected")
	}

	if match.Name != "docker" {
		t.Errorf("Name = %s, want docker", match.Name)
	}

	if match.Priority != stacks.PriorityDocker {
		t.Errorf("Priority = %d, want %d", match.Priority, stacks.PriorityDocker)
	}

	if match.Confidence <= 0 {
		t.Error("Confidence should be > 0")
	}

	if len(match.Reasons) == 0 {
		t.Error("Should have reasons")
	}
}

func TestDockerDetector_DockerfileOnly(t *testing.T) {
	detector := stacks.NewDockerDetector()

	profile := &scanner.ProjectProfile{
		Signals:    []string{"Dockerfile"},
		Containers: []string{"Dockerfile"},
	}

	match, found := detector.Detect(profile)
	if !found {
		t.Fatal("Docker should be detected with Dockerfile only")
	}

	if match.Name != "docker" {
		t.Errorf("Name = %s, want docker", match.Name)
	}
}

func TestDockerDetector_NoDocker(t *testing.T) {
	detector := stacks.NewDockerDetector()

	profile := &scanner.ProjectProfile{
		Signals: []string{"package.json"},
		Tools:   []string{"npm"},
	}

	_, found := detector.Detect(profile)
	if found {
		t.Error("Docker should not be detected without Dockerfile or compose")
	}
}

func TestNodeDetector_NpmProject(t *testing.T) {
	detector := stacks.NewNodeDetector()

	profile := &scanner.ProjectProfile{
		Signals:   []string{"package.json", "package-lock.json"},
		Packages:  []string{"package.json", "package-lock.json"},
		Tools:     []string{"npm"},
		Languages: []string{"javascript"},
	}

	match, found := detector.Detect(profile)
	if !found {
		t.Fatal("Node should be detected")
	}

	if match.Name != "node" {
		t.Errorf("Name = %s, want node", match.Name)
	}

	// Check that npm is mentioned in reasons
	hasNpmReason := false
	for _, reason := range match.Reasons {
		if containsString(reason, "npm") {
			hasNpmReason = true
			break
		}
	}
	if !hasNpmReason {
		t.Error("Should mention npm in reasons")
	}
}

func TestNodeDetector_YarnProject(t *testing.T) {
	detector := stacks.NewNodeDetector()

	profile := &scanner.ProjectProfile{
		Signals:  []string{"package.json", "yarn.lock"},
		Packages: []string{"package.json", "yarn.lock"},
		Tools:    []string{"yarn"},
	}

	match, found := detector.Detect(profile)
	if !found {
		t.Fatal("Node should be detected")
	}

	// Check that yarn is mentioned in reasons
	hasYarnReason := false
	for _, reason := range match.Reasons {
		if containsString(reason, "Yarn") || containsString(reason, "yarn") {
			hasYarnReason = true
			break
		}
	}
	if !hasYarnReason {
		t.Error("Should mention Yarn in reasons")
	}
}

func TestNodeDetector_TypeScriptProject(t *testing.T) {
	detector := stacks.NewNodeDetector()

	profile := &scanner.ProjectProfile{
		Signals:   []string{"package.json", "tsconfig.json"},
		Packages:  []string{"package.json"},
		Languages: []string{"typescript"},
	}

	match, found := detector.Detect(profile)
	if !found {
		t.Fatal("Node should be detected")
	}

	// Check that TypeScript is mentioned
	hasTsReason := false
	for _, reason := range match.Reasons {
		if containsString(reason, "TypeScript") {
			hasTsReason = true
			break
		}
	}
	if !hasTsReason {
		t.Error("Should mention TypeScript in reasons")
	}

	// Check tsconfig.json is in signals
	hasTsConfig := false
	for _, signal := range match.Signals {
		if signal == "tsconfig.json" {
			hasTsConfig = true
			break
		}
	}
	if !hasTsConfig {
		t.Error("Should have tsconfig.json in signals")
	}
}

func TestNodeDetector_NoPackageJson(t *testing.T) {
	detector := stacks.NewNodeDetector()

	profile := &scanner.ProjectProfile{
		Signals: []string{"index.js"},
	}

	_, found := detector.Detect(profile)
	if found {
		t.Error("Node should not be detected without package.json")
	}
}

func TestPythonDetector_PyProjectToml(t *testing.T) {
	detector := stacks.NewPythonDetector()

	profile := &scanner.ProjectProfile{
		Signals:   []string{"pyproject.toml", "poetry.lock"},
		Packages:  []string{"pyproject.toml", "poetry.lock"},
		Tools:     []string{"poetry"},
		Languages: []string{"python"},
	}

	match, found := detector.Detect(profile)
	if !found {
		t.Fatal("Python should be detected")
	}

	if match.Name != "python" {
		t.Errorf("Name = %s, want python", match.Name)
	}

	// Check Poetry is mentioned
	hasPoetryReason := false
	for _, reason := range match.Reasons {
		if containsString(reason, "Poetry") {
			hasPoetryReason = true
			break
		}
	}
	if !hasPoetryReason {
		t.Error("Should mention Poetry in reasons")
	}
}

func TestPythonDetector_RequirementsTxt(t *testing.T) {
	detector := stacks.NewPythonDetector()

	profile := &scanner.ProjectProfile{
		Signals:  []string{"requirements.txt"},
		Packages: []string{"requirements.txt"},
		Tools:    []string{"pip"},
	}

	match, found := detector.Detect(profile)
	if !found {
		t.Fatal("Python should be detected")
	}

	// Check pip requirements is mentioned
	hasPipReason := false
	for _, reason := range match.Reasons {
		if containsString(reason, "pip") || containsString(reason, "requirements") {
			hasPipReason = true
			break
		}
	}
	if !hasPipReason {
		t.Error("Should mention pip requirements in reasons")
	}
}

func TestGoDetector_GoMod(t *testing.T) {
	detector := stacks.NewGoDetector()

	profile := &scanner.ProjectProfile{
		Signals:   []string{"go.mod", "go.sum", "Makefile"},
		Packages:  []string{"go.mod", "go.sum"},
		Tools:     []string{"go", "make"},
		Languages: []string{"go"},
	}

	match, found := detector.Detect(profile)
	if !found {
		t.Fatal("Go should be detected")
	}

	if match.Name != "go" {
		t.Errorf("Name = %s, want go", match.Name)
	}
}

func TestGoDetector_NoGoMod(t *testing.T) {
	detector := stacks.NewGoDetector()

	profile := &scanner.ProjectProfile{
		Signals: []string{"main.go"},
	}

	_, found := detector.Detect(profile)
	if found {
		t.Error("Go should not be detected without go.mod")
	}
}

func TestRustDetector_CargoToml(t *testing.T) {
	detector := stacks.NewRustDetector()

	profile := &scanner.ProjectProfile{
		Signals:   []string{"Cargo.toml", "Cargo.lock"},
		Packages:  []string{"Cargo.toml", "Cargo.lock"},
		Tools:     []string{"cargo"},
		Languages: []string{"rust"},
	}

	match, found := detector.Detect(profile)
	if !found {
		t.Fatal("Rust should be detected")
	}

	if match.Name != "rust" {
		t.Errorf("Name = %s, want rust", match.Name)
	}
}

func TestAggregator_SingleStack(t *testing.T) {
	aggregator := stacks.NewAggregator()

	profile := &scanner.ProjectProfile{
		Signals:  []string{"go.mod", "go.sum"},
		Packages: []string{"go.mod", "go.sum"},
		Tools:    []string{"go"},
	}

	result := aggregator.Detect(profile)

	if len(result.Matches) != 1 {
		t.Errorf("Expected 1 match, got %d", len(result.Matches))
	}

	if result.Dominant.Name != "go" {
		t.Errorf("Dominant = %s, want go", result.Dominant.Name)
	}

	if result.IsMixed {
		t.Error("Should not be mixed with single stack")
	}
}

func TestAggregator_DockerDominates(t *testing.T) {
	aggregator := stacks.NewAggregator()

	profile := &scanner.ProjectProfile{
		Signals:    []string{"docker-compose.yml", "Dockerfile", "package.json", "go.mod"},
		Packages:   []string{"package.json", "go.mod"},
		Containers: []string{"Dockerfile", "docker-compose.yml"},
		Tools:      []string{"docker", "docker-compose", "npm", "go"},
	}

	result := aggregator.Detect(profile)

	if result.Dominant.Name != "docker" {
		t.Errorf("Dominant = %s, want docker (Docker Compose should dominate)", result.Dominant.Name)
	}

	if !result.IsMixed {
		t.Error("Should be mixed with multiple stacks")
	}

	if len(result.AllStacks) < 2 {
		t.Errorf("Expected multiple stacks, got %d", len(result.AllStacks))
	}
}

func TestAggregator_MixedEqualPriority(t *testing.T) {
	aggregator := stacks.NewAggregator()

	profile := &scanner.ProjectProfile{
		Signals:  []string{"package.json", "pyproject.toml"},
		Packages: []string{"package.json", "pyproject.toml"},
		Tools:    []string{"npm", "poetry"},
	}

	result := aggregator.Detect(profile)

	if !result.IsMixed {
		t.Error("Should be mixed when Node and Python have equal priority")
	}

	// Should have both node and python in matches
	hasNode := false
	hasPython := false
	for _, match := range result.Matches {
		if match.Name == "node" {
			hasNode = true
		}
		if match.Name == "python" {
			hasPython = true
		}
	}

	if !hasNode || !hasPython {
		t.Error("Should have both node and python detected")
	}
}

func TestAggregator_EmptyProfile(t *testing.T) {
	aggregator := stacks.NewAggregator()

	profile := &scanner.ProjectProfile{}

	result := aggregator.Detect(profile)

	if len(result.Matches) != 0 {
		t.Errorf("Expected 0 matches for empty profile, got %d", len(result.Matches))
	}

	if result.Dominant.Name != "unknown" {
		t.Errorf("Dominant = %s, want unknown for empty profile", result.Dominant.Name)
	}
}

func TestAggregator_NilProfile(t *testing.T) {
	aggregator := stacks.NewAggregator()

	result := aggregator.Detect(nil)

	if len(result.Matches) != 0 {
		t.Errorf("Expected 0 matches for nil profile, got %d", len(result.Matches))
	}
}

func TestAggregator_PriorityOrder(t *testing.T) {
	aggregator := stacks.NewAggregator()

	// Docker has highest priority, then Node/Python, then Go/Rust
	profile := &scanner.ProjectProfile{
		Signals:    []string{"Dockerfile", "package.json", "go.mod"},
		Packages:   []string{"package.json", "go.mod"},
		Containers: []string{"Dockerfile"},
		Tools:      []string{"docker", "npm", "go"},
	}

	result := aggregator.Detect(profile)

	// Docker should be dominant (highest priority)
	if result.Dominant.Name != "docker" {
		t.Errorf("Dominant = %s, want docker (highest priority)", result.Dominant.Name)
	}

	// Matches should be sorted by priority
	if len(result.Matches) >= 2 {
		for i := 0; i < len(result.Matches)-1; i++ {
			if result.Matches[i].Priority < result.Matches[i+1].Priority {
				t.Errorf("Matches not sorted by priority: %s (%d) before %s (%d)",
					result.Matches[i].Name, result.Matches[i].Priority,
					result.Matches[i+1].Name, result.Matches[i+1].Priority)
			}
		}
	}
}

func TestStackMatch_Confidence(t *testing.T) {
	detector := stacks.NewDockerDetector()

	// More signals should increase confidence
	profileMinimal := &scanner.ProjectProfile{
		Signals:    []string{"Dockerfile"},
		Containers: []string{"Dockerfile"},
	}

	profileFull := &scanner.ProjectProfile{
		Signals:    []string{"Dockerfile", "docker-compose.yml", ".dockerignore"},
		Containers: []string{"Dockerfile", "docker-compose.yml"},
		Tools:      []string{"docker", "docker-compose"},
		Readme:     true,
	}

	matchMinimal, _ := detector.Detect(profileMinimal)
	matchFull, _ := detector.Detect(profileFull)

	if matchFull.Confidence <= matchMinimal.Confidence {
		t.Errorf("Full profile confidence (%.2f) should be > minimal (%.2f)",
			matchFull.Confidence, matchMinimal.Confidence)
	}
}

func TestDetectionResult_Explanation(t *testing.T) {
	aggregator := stacks.NewAggregator()

	profile := &scanner.ProjectProfile{
		Signals:    []string{"docker-compose.yml", "package.json"},
		Packages:   []string{"package.json"},
		Containers: []string{"docker-compose.yml"},
		Tools:      []string{"docker-compose", "npm"},
	}

	result := aggregator.Detect(profile)

	if result.Explanation == "" {
		t.Error("Explanation should not be empty")
	}

	// Explanation should mention Docker Compose
	if !containsString(result.Explanation, "Docker") {
		t.Errorf("Explanation should mention Docker: %s", result.Explanation)
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
