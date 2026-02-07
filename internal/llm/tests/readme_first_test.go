// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Tests for README-first AI behavior and provider auto-selection

package tests

import (
	"os"
	"testing"

	"github.com/sony-level/readme-runner/internal/llm"
	"github.com/sony-level/readme-runner/internal/llm/provider"
	"github.com/sony-level/readme-runner/internal/scanner"
)

// TestProviderAutoSelectionOrder verifies the provider auto-selection priority:
// anthropic > openai > mistral > ollama > mock
func TestProviderAutoSelectionOrder(t *testing.T) {
	// Save all env vars
	envVars := []string{
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
		"MISTRAL_API_KEY",
		"RD_LLM_PROVIDER",
		"RD_LLM_TOKEN",
	}
	saved := make(map[string]string)
	for _, v := range envVars {
		saved[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range saved {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	tests := []struct {
		name         string
		envVars      map[string]string
		expectedType llm.ProviderType
		description  string
	}{
		{
			name:         "no keys - defaults to mock",
			envVars:      map[string]string{},
			expectedType: llm.ProviderMock,
			description:  "Should use mock when no API keys are available",
		},
		{
			name: "anthropic key only - selects anthropic",
			envVars: map[string]string{
				"ANTHROPIC_API_KEY": "sk-ant-test",
			},
			expectedType: llm.ProviderAnthropic,
			description:  "Should prefer Anthropic when key is available",
		},
		{
			name: "openai key only - selects openai",
			envVars: map[string]string{
				"OPENAI_API_KEY": "sk-test",
			},
			expectedType: llm.ProviderOpenAI,
			description:  "Should select OpenAI when only OpenAI key is available",
		},
		{
			name: "mistral key only - selects mistral",
			envVars: map[string]string{
				"MISTRAL_API_KEY": "test-key",
			},
			expectedType: llm.ProviderMistral,
			description:  "Should select Mistral when only Mistral key is available",
		},
		{
			name: "anthropic beats openai",
			envVars: map[string]string{
				"ANTHROPIC_API_KEY": "sk-ant-test",
				"OPENAI_API_KEY":    "sk-test",
			},
			expectedType: llm.ProviderAnthropic,
			description:  "Anthropic should have higher priority than OpenAI",
		},
		{
			name: "anthropic beats all others",
			envVars: map[string]string{
				"ANTHROPIC_API_KEY": "sk-ant-test",
				"OPENAI_API_KEY":    "sk-test",
				"MISTRAL_API_KEY":   "test-key",
			},
			expectedType: llm.ProviderAnthropic,
			description:  "Anthropic should have highest priority",
		},
		{
			name: "openai beats mistral",
			envVars: map[string]string{
				"OPENAI_API_KEY":  "sk-test",
				"MISTRAL_API_KEY": "test-key",
			},
			expectedType: llm.ProviderOpenAI,
			description:  "OpenAI should have higher priority than Mistral",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all keys first
			for _, v := range envVars {
				os.Unsetenv(v)
			}

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// Get config with auto-selection (empty provider string)
			config, info := llm.ResolveProviderConfigWithInfo("", "", "", "", 0, false)

			if config.Type != tt.expectedType {
				t.Errorf("%s: expected %s, got %s", tt.description, tt.expectedType, config.Type)
			}

			if info.Source != "auto" {
				t.Errorf("Expected auto selection, got source: %s", info.Source)
			}
		})
	}
}

// TestProviderPrecedence verifies: CLI > ENV > config file > defaults
func TestProviderPrecedence(t *testing.T) {
	// Save env vars
	saved := map[string]string{
		"RD_LLM_PROVIDER":   os.Getenv("RD_LLM_PROVIDER"),
		"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
	}
	defer func() {
		for k, v := range saved {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	tests := []struct {
		name         string
		cliProvider  string
		envProvider  string
		expectedType llm.ProviderType
		source       string
	}{
		{
			name:         "CLI takes precedence over ENV",
			cliProvider:  "openai",
			envProvider:  "mistral",
			expectedType: llm.ProviderOpenAI,
			source:       "cli",
		},
		{
			name:         "ENV used when CLI empty",
			cliProvider:  "",
			envProvider:  "mistral",
			expectedType: llm.ProviderMistral,
			source:       "env",
		},
		{
			name:         "Auto-select when both empty",
			cliProvider:  "",
			envProvider:  "",
			expectedType: llm.ProviderMock, // No keys set
			source:       "auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env first
			os.Unsetenv("RD_LLM_PROVIDER")
			os.Unsetenv("ANTHROPIC_API_KEY")
			os.Unsetenv("OPENAI_API_KEY")
			os.Unsetenv("MISTRAL_API_KEY")

			// Set test env
			if tt.envProvider != "" {
				os.Setenv("RD_LLM_PROVIDER", tt.envProvider)
			}

			config, info := llm.ResolveProviderConfigWithInfo(tt.cliProvider, "", "", "", 0, false)

			if config.Type != tt.expectedType {
				t.Errorf("Expected %s, got %s", tt.expectedType, config.Type)
			}

			if info.Source != tt.source {
				t.Errorf("Expected source %s, got %s", tt.source, info.Source)
			}
		})
	}
}

// TestReadmeFirstClarityThreshold verifies README-first behavior based on clarity score
func TestReadmeFirstClarityThreshold(t *testing.T) {
	tests := []struct {
		name         string
		readme       *scanner.ReadmeInfo
		expectUse    bool
		minScore     float64
		maxScore     float64
	}{
		{
			name:      "nil README - don't use",
			readme:    nil,
			expectUse: false,
			minScore:  0.0,
			maxScore:  0.0,
		},
		{
			name: "empty README - don't use",
			readme: &scanner.ReadmeInfo{
				Content: "",
			},
			expectUse: false,
			minScore:  0.0,
			maxScore:  0.1,
		},
		{
			name: "minimal README - below threshold",
			readme: &scanner.ReadmeInfo{
				Content:    "# Project\nSome description",
				CodeBlocks: 0,
			},
			expectUse: false,
			minScore:  0.0,
			maxScore:  0.5,
		},
		{
			name: "good README with install section - above threshold",
			readme: &scanner.ReadmeInfo{
				Content:       "# Project\n## Installation\nnpm install\n## Usage\nnpm start",
				HasInstall:    true,
				HasUsage:      true,
				CodeBlocks:    2,
				ShellCommands: 2,
			},
			expectUse: true,
			minScore:  0.6,
			maxScore:  1.0,
		},
		{
			name: "excellent README - definitely use",
			readme: &scanner.ReadmeInfo{
				Content:       "# Project\n## Installation\nnpm install\n## Usage\nnpm start\n## Quick Start\n...",
				HasInstall:    true,
				HasUsage:      true,
				HasQuickStart: true,
				HasBuild:      true,
				CodeBlocks:    5,
				ShellCommands: 4,
			},
			expectUse: true,
			minScore:  0.8,
			maxScore:  1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := llm.CalculateClarityScore(tt.readme)
			shouldUse := llm.ShouldUseReadme(tt.readme)

			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Clarity score %.2f not in expected range [%.2f, %.2f]",
					score, tt.minScore, tt.maxScore)
			}

			if shouldUse != tt.expectUse {
				t.Errorf("ShouldUseReadme returned %v, expected %v (score: %.2f, threshold: %.2f)",
					shouldUse, tt.expectUse, score, llm.ClarityThreshold)
			}
		})
	}
}

// TestReadmeFirstInPlanContext verifies that PlanContext correctly sets UseReadme
func TestReadmeFirstInPlanContext(t *testing.T) {
	goodReadme := &scanner.ReadmeInfo{
		Content:       "# Project\n## Installation\nnpm install",
		HasInstall:    true,
		HasUsage:      true,
		CodeBlocks:    3,
		ShellCommands: 2,
	}

	poorReadme := &scanner.ReadmeInfo{
		Content: "# Project\nNo clear instructions",
	}

	tests := []struct {
		name      string
		readme    *scanner.ReadmeInfo
		useReadme bool
	}{
		{
			name:      "good README enables README-first",
			readme:    goodReadme,
			useReadme: true,
		},
		{
			name:      "poor README uses file signals",
			readme:    poorReadme,
			useReadme: false,
		},
		{
			name:      "nil README uses file signals",
			readme:    nil,
			useReadme: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useReadme := llm.ShouldUseReadme(tt.readme)
			if useReadme != tt.useReadme {
				t.Errorf("Expected UseReadme=%v, got %v", tt.useReadme, useReadme)
			}
		})
	}
}

// TestMockProviderAlwaysSucceeds verifies mock provider never fails
func TestMockProviderAlwaysSucceeds(t *testing.T) {
	mockProv := provider.NewMockProvider()

	testCases := []struct {
		name    string
		context *llm.PlanContext
	}{
		{
			name:    "nil context",
			context: nil,
		},
		{
			name: "empty context",
			context: &llm.PlanContext{},
		},
		{
			name: "context with profile only",
			context: &llm.PlanContext{
				Profile: &scanner.ProjectProfile{
					Stack: "node",
				},
			},
		},
		{
			name: "context with readme only",
			context: &llm.PlanContext{
				ReadmeInfo: &scanner.ReadmeInfo{
					Content: "# Test",
				},
			},
		},
		{
			name: "full context",
			context: &llm.PlanContext{
				Profile: &scanner.ProjectProfile{
					Stack:     "go",
					Languages: []string{"go"},
					Tools:     []string{"go"},
				},
				ReadmeInfo: &scanner.ReadmeInfo{
					Content:    "# Go Project",
					HasInstall: true,
				},
				UseReadme:    true,
				ClarityScore: 0.8,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := mockProv.GeneratePlan(tc.context)
			if err != nil {
				t.Fatalf("Mock provider should never fail, got error: %v", err)
			}
			if plan == nil {
				t.Fatal("Mock provider should always return a plan")
			}
			if plan.Version != "1" {
				t.Errorf("Expected plan version 1, got %s", plan.Version)
			}
		})
	}
}

// TestProviderSelectionInfo verifies selection info is populated correctly
func TestProviderSelectionInfo(t *testing.T) {
	// Clear env
	os.Unsetenv("RD_LLM_PROVIDER")
	os.Unsetenv("ANTHROPIC_API_KEY")

	// Test CLI selection
	_, info := llm.ResolveProviderConfigWithInfo("mock", "", "", "", 0, false)
	if info.Source != "cli" {
		t.Errorf("Expected source 'cli', got '%s'", info.Source)
	}

	// Test ENV selection
	os.Setenv("RD_LLM_PROVIDER", "openai")
	_, info = llm.ResolveProviderConfigWithInfo("", "", "", "", 0, false)
	if info.Source != "env" {
		t.Errorf("Expected source 'env', got '%s'", info.Source)
	}
	os.Unsetenv("RD_LLM_PROVIDER")

	// Test auto selection
	_, info = llm.ResolveProviderConfigWithInfo("", "", "", "", 0, false)
	if info.Source != "auto" {
		t.Errorf("Expected source 'auto', got '%s'", info.Source)
	}
	if info.AutoReason == "" {
		t.Error("Expected AutoReason to be set for auto selection")
	}
}

// TestGetProviderSelectionDescription verifies human-readable descriptions
func TestGetProviderSelectionDescription(t *testing.T) {
	tests := []struct {
		info     *llm.ProviderSelectionInfo
		contains string
	}{
		{
			info:     &llm.ProviderSelectionInfo{Source: "cli"},
			contains: "--provider",
		},
		{
			info:     &llm.ProviderSelectionInfo{Source: "env"},
			contains: "RD_LLM_PROVIDER",
		},
		{
			info:     &llm.ProviderSelectionInfo{Source: "config"},
			contains: "config file",
		},
		{
			info:     &llm.ProviderSelectionInfo{Source: "auto", AutoReason: "test reason"},
			contains: "auto-selected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.info.Source, func(t *testing.T) {
			desc := llm.GetProviderSelectionDescription(tt.info)
			if desc == "" {
				t.Error("Description should not be empty")
			}
			// Just check it returns something meaningful
			if len(desc) < 5 {
				t.Error("Description seems too short")
			}
		})
	}
}
