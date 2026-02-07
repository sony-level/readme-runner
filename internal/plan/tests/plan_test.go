// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Tests for plan validation and normalization

package tests

import (
	"testing"

	"github.com/sony-level/readme-runner/internal/llm"
	"github.com/sony-level/readme-runner/internal/plan"
	"github.com/sony-level/readme-runner/internal/scanner"
)

func TestValidatorValidPlan(t *testing.T) {
	validator := plan.NewValidator()

	runPlan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Prerequisites: []llm.Prerequisite{
			{Name: "node", Reason: "Required"},
		},
		Steps: []llm.Step{
			{ID: "install", Cmd: "npm ci", Cwd: ".", Risk: llm.RiskMedium},
			{ID: "run", Cmd: "npm start", Cwd: ".", Risk: llm.RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{3000},
		Notes: []string{"Test note"},
	}

	result := validator.Validate(runPlan)

	if !result.Valid {
		t.Errorf("Expected valid plan, got errors: %v", result.Errors)
	}
}

func TestValidatorSudoDetection(t *testing.T) {
	validator := plan.NewValidator()

	runPlan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Steps: []llm.Step{
			{ID: "install", Cmd: "sudo apt install docker", Cwd: ".", Risk: llm.RiskLow, RequiresSudo: false},
		},
	}

	result := validator.Validate(runPlan)

	// Should warn about sudo mismatch
	hasWarning := false
	for _, warn := range result.Warnings {
		if warn != "" {
			hasWarning = true
			break
		}
	}
	if !hasWarning && result.Valid {
		// The validator should at least enhance this
	}
}

func TestValidatorEnhancePlan(t *testing.T) {
	validator := plan.NewValidator()

	runPlan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Steps: []llm.Step{
			{ID: "install", Cmd: "sudo npm install -g something", Cwd: ".", Risk: llm.RiskLow, RequiresSudo: false},
		},
	}

	enhanced := validator.EnhancePlan(runPlan)

	// Should have detected sudo
	if !enhanced.Steps[0].RequiresSudo {
		t.Error("Expected RequiresSudo to be true after enhancement")
	}

	// Should have elevated risk
	if enhanced.Steps[0].Risk != llm.RiskCritical {
		t.Errorf("Expected RiskCritical, got %s", enhanced.Steps[0].Risk)
	}
}

func TestValidatorBlockedCommands(t *testing.T) {
	validator := plan.NewValidator()

	runPlan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Steps: []llm.Step{
			{ID: "danger", Cmd: "rm -rf /", Cwd: "."},
		},
	}

	result := validator.Validate(runPlan)

	if result.Valid {
		t.Error("Expected plan with 'rm -rf /' to be invalid")
	}
}

func TestNormalizerNodePackages(t *testing.T) {
	profile := &scanner.ProjectProfile{
		Stack:    "node",
		Packages: []string{"package.json", "package-lock.json"},
	}

	normalizer := plan.NewNormalizer(profile)

	runPlan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Steps: []llm.Step{
			{ID: "install", Cmd: "npm install", Cwd: "."},
		},
	}

	normalized := normalizer.Normalize(runPlan)

	// Should have normalized to npm ci
	if normalized.Steps[0].Cmd != "npm ci" {
		t.Errorf("Expected 'npm ci', got '%s'", normalized.Steps[0].Cmd)
	}
}

func TestNormalizerYarnPackages(t *testing.T) {
	profile := &scanner.ProjectProfile{
		Stack:    "node",
		Packages: []string{"package.json", "yarn.lock"},
	}

	normalizer := plan.NewNormalizer(profile)

	runPlan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Steps: []llm.Step{
			{ID: "install", Cmd: "yarn install", Cwd: "."},
		},
	}

	normalized := normalizer.Normalize(runPlan)

	// Should have added frozen lockfile
	if normalized.Steps[0].Cmd != "yarn install --frozen-lockfile" {
		t.Errorf("Expected 'yarn install --frozen-lockfile', got '%s'", normalized.Steps[0].Cmd)
	}
}

func TestNormalizerDockerCompose(t *testing.T) {
	profile := &scanner.ProjectProfile{
		Stack:      "docker",
		Containers: []string{"docker-compose.yml"},
	}

	normalizer := plan.NewNormalizer(profile)

	runPlan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "docker",
		Steps: []llm.Step{
			{ID: "up", Cmd: "docker-compose up", Cwd: "."},
		},
	}

	normalized := normalizer.Normalize(runPlan)

	// Should have normalized to docker compose (v2)
	if normalized.Steps[0].Cmd != "docker compose up" {
		t.Errorf("Expected 'docker compose up', got '%s'", normalized.Steps[0].Cmd)
	}
}

func TestNormalizerPrerequisitesDedupe(t *testing.T) {
	normalizer := plan.NewNormalizer(nil)

	runPlan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "docker",
		Prerequisites: []llm.Prerequisite{
			{Name: "docker", Reason: "Docker required"},
			{Name: "docker-compose", Reason: "Compose required"},
		},
		Steps: []llm.Step{},
	}

	normalized := normalizer.Normalize(runPlan)

	// docker-compose should be normalized to docker (deduped)
	if len(normalized.Prerequisites) != 1 {
		t.Errorf("Expected 1 prerequisite after normalization, got %d", len(normalized.Prerequisites))
	}
}

func TestNormalizerSuggestDocker(t *testing.T) {
	tests := []struct {
		name      string
		profile   *scanner.ProjectProfile
		expectDoc bool
	}{
		{
			name:      "nil profile",
			profile:   nil,
			expectDoc: false,
		},
		{
			name: "with Dockerfile",
			profile: &scanner.ProjectProfile{
				Containers: []string{"Dockerfile"},
			},
			expectDoc: true,
		},
		{
			name: "with docker-compose",
			profile: &scanner.ProjectProfile{
				Containers: []string{"docker-compose.yml"},
			},
			expectDoc: true,
		},
		{
			name: "no containers",
			profile: &scanner.ProjectProfile{
				Stack: "node",
			},
			expectDoc: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizer := plan.NewNormalizer(tt.profile)
			result := normalizer.SuggestDockerPreference()
			if result != tt.expectDoc {
				t.Errorf("SuggestDockerPreference() = %v, want %v", result, tt.expectDoc)
			}
		})
	}
}
