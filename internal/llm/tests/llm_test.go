// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Tests for LLM providers

package tests

import (
	"testing"

	"github.com/sony-level/readme-runner/internal/llm"
	"github.com/sony-level/readme-runner/internal/llm/provider"
	"github.com/sony-level/readme-runner/internal/scanner"
)

func TestMockProvider(t *testing.T) {
	prov := provider.NewMockProvider()

	if prov.Name() != "mock" {
		t.Errorf("Expected name 'mock', got '%s'", prov.Name())
	}

	// Test with Go profile
	ctx := &llm.PlanContext{
		Profile: &scanner.ProjectProfile{
			Stack:     "go",
			Languages: []string{"go"},
			Tools:     []string{"go"},
			Packages:  []string{"go.mod", "go.sum"},
		},
	}

	plan, err := prov.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan.ProjectType != "go" {
		t.Errorf("Expected project_type 'go', got '%s'", plan.ProjectType)
	}

	if plan.Version != "1" {
		t.Errorf("Expected version '1', got '%s'", plan.Version)
	}

	if len(plan.Steps) < 1 {
		t.Error("Expected at least 1 step")
	}
}

func TestMockProviderNodeStack(t *testing.T) {
	prov := provider.NewMockProvider()

	ctx := &llm.PlanContext{
		Profile: &scanner.ProjectProfile{
			Stack:     "node",
			Languages: []string{"javascript"},
			Tools:     []string{"npm"},
			Packages:  []string{"package.json", "package-lock.json"},
		},
	}

	plan, err := prov.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan.ProjectType != "node" {
		t.Errorf("Expected project_type 'node', got '%s'", plan.ProjectType)
	}

	// Check that npm ci is used when lockfile is present
	hasNpmCi := false
	for _, step := range plan.Steps {
		if step.Cmd == "npm ci" {
			hasNpmCi = true
			break
		}
	}
	if !hasNpmCi {
		t.Error("Expected 'npm ci' command when package-lock.json is present")
	}
}

func TestMockProviderDockerStack(t *testing.T) {
	prov := provider.NewMockProvider()

	ctx := &llm.PlanContext{
		Profile: &scanner.ProjectProfile{
			Stack:      "docker",
			Containers: []string{"docker-compose.yml"},
		},
	}

	plan, err := prov.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan.ProjectType != "docker" {
		t.Errorf("Expected project_type 'docker', got '%s'", plan.ProjectType)
	}

	// Check for docker compose commands
	hasComposeCmd := false
	for _, step := range plan.Steps {
		if step.Cmd == "docker compose build" || step.Cmd == "docker compose up" {
			hasComposeCmd = true
			break
		}
	}
	if !hasComposeCmd {
		t.Error("Expected docker compose command for docker-compose.yml")
	}
}

func TestClarityScore(t *testing.T) {
	tests := []struct {
		name     string
		readme   *scanner.ReadmeInfo
		minScore float64
		maxScore float64
	}{
		{
			name:     "nil readme",
			readme:   nil,
			minScore: 0.0,
			maxScore: 0.0,
		},
		{
			name: "empty readme",
			readme: &scanner.ReadmeInfo{
				Content: "",
			},
			minScore: 0.0,
			maxScore: 0.1,
		},
		{
			name: "good readme",
			readme: &scanner.ReadmeInfo{
				Content:       "# Project\n## Installation\nnpm install\n## Usage\nnpm start",
				HasInstall:    true,
				HasUsage:      true,
				HasQuickStart: true,
				CodeBlocks:    3,
				ShellCommands: 2,
			},
			minScore: 0.8,
			maxScore: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := llm.CalculateClarityScore(tt.readme)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Score %.2f not in range [%.2f, %.2f]", score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestRunPlanValidation(t *testing.T) {
	tests := []struct {
		name    string
		plan    *llm.RunPlan
		wantErr bool
	}{
		{
			name: "valid plan",
			plan: &llm.RunPlan{
				Version:     "1",
				ProjectType: "node",
				Steps: []llm.Step{
					{ID: "install", Cmd: "npm ci", Cwd: "."},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid version",
			plan: &llm.RunPlan{
				Version:     "2",
				ProjectType: "node",
				Steps:       []llm.Step{},
			},
			wantErr: true,
		},
		{
			name: "invalid project type",
			plan: &llm.RunPlan{
				Version:     "1",
				ProjectType: "invalid",
				Steps:       []llm.Step{},
			},
			wantErr: true,
		},
		{
			name: "empty command",
			plan: &llm.RunPlan{
				Version:     "1",
				ProjectType: "node",
				Steps: []llm.Step{
					{ID: "install", Cmd: "", Cwd: "."},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.plan.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProviderConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *llm.ProviderConfig
		wantErr error
	}{
		{
			name:    "mock provider",
			config:  &llm.ProviderConfig{Type: llm.ProviderMock},
			wantErr: nil,
		},
		{
			name:    "copilot provider (deprecated but valid)",
			config:  &llm.ProviderConfig{Type: llm.ProviderCopilot},
			wantErr: nil,
		},
		{
			name: "http provider without endpoint",
			config: &llm.ProviderConfig{
				Type:  llm.ProviderHTTP,
				Token: "test-token",
			},
			wantErr: llm.ErrMissingEndpoint,
		},
		{
			name: "http provider with endpoint (token optional)",
			config: &llm.ProviderConfig{
				Type:     llm.ProviderHTTP,
				Endpoint: "http://localhost:8080",
			},
			wantErr: nil, // Token is now optional for HTTP provider
		},
		{
			name: "valid http provider with token",
			config: &llm.ProviderConfig{
				Type:     llm.ProviderHTTP,
				Endpoint: "http://localhost:8080",
				Token:    "test-token",
			},
			wantErr: nil,
		},
		{
			name:    "openai provider (validates in constructor)",
			config:  &llm.ProviderConfig{Type: llm.ProviderOpenAI},
			wantErr: nil,
		},
		{
			name:    "anthropic provider (validates in constructor)",
			config:  &llm.ProviderConfig{Type: llm.ProviderAnthropic},
			wantErr: nil,
		},
		{
			name:    "ollama provider (no token needed)",
			config:  &llm.ProviderConfig{Type: llm.ProviderOllama},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		token    string
		expected string
	}{
		{"", ""},
		{"short", "[REDACTED]"},
		{"12345678", "[REDACTED]"},
		{"123456789", "1234...6789"},
		{"abcdefghijklmnop", "abcd...mnop"},
	}

	for _, tt := range tests {
		result := llm.MaskToken(tt.token)
		if result != tt.expected {
			t.Errorf("MaskToken(%q) = %q, want %q", tt.token, result, tt.expected)
		}
	}
}
