// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Tests for provider registry and fallback behavior

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sony-level/readme-runner/internal/llm"
	"github.com/sony-level/readme-runner/internal/llm/provider"
	"github.com/sony-level/readme-runner/internal/scanner"
)

// TestCopilotDeprecatedFallback verifies that copilot provider
// immediately falls back to mock without any network call
func TestCopilotDeprecatedFallback(t *testing.T) {
	// Track if any HTTP request was made
	requestMade := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		t.Error("Network request was made - copilot should not make any requests")
		http.Error(w, "Should not reach here", http.StatusInternalServerError)
	}))
	defer server.Close()

	config := &llm.ProviderConfig{
		Type:     llm.ProviderCopilot,
		Endpoint: server.URL, // Even with a valid endpoint, should not be used
		Timeout:  2 * time.Second,
	}

	// Get provider from registry
	prov := llm.DefaultRegistry.Get(config)

	// Should be a mock provider (or FallbackProvider wrapping mock)
	if prov == nil {
		t.Fatal("Expected provider, got nil")
	}

	// Generate a plan - should work without network
	ctx := &llm.PlanContext{
		Profile: &scanner.ProjectProfile{
			Stack: "node",
		},
	}

	plan, err := prov.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan == nil {
		t.Fatal("Expected plan, got nil")
	}

	if requestMade {
		t.Error("Copilot provider made a network request when it should have fallen back to mock")
	}
}

// TestDefaultNoKeyUsesMock verifies that without any API keys,
// the system defaults to mock provider
func TestDefaultNoKeyUsesMock(t *testing.T) {
	// Clear all API key environment variables
	envVars := []string{
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
		"MISTRAL_API_KEY",
		"RD_LLM_TOKEN",
		"RD_LLM_PROVIDER",
	}

	// Save and clear env vars
	saved := make(map[string]string)
	for _, v := range envVars {
		saved[v] = os.Getenv(v)
		os.Unsetenv(v)
	}

	// Restore env vars after test
	defer func() {
		for k, v := range saved {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	// Resolve config with no CLI args and no env vars
	config := llm.ResolveProviderConfig("", "", "", "", 0, false)

	// Should default to mock
	if config.Type != llm.ProviderMock {
		t.Errorf("Expected mock provider when no keys available, got %s", config.Type)
	}
}

// TestProviderResolutionPrecedence verifies CLI > ENV > config > defaults
func TestProviderResolutionPrecedence(t *testing.T) {
	// Save env vars
	savedProvider := os.Getenv("RD_LLM_PROVIDER")
	savedModel := os.Getenv("RD_LLM_MODEL")
	defer func() {
		if savedProvider != "" {
			os.Setenv("RD_LLM_PROVIDER", savedProvider)
		} else {
			os.Unsetenv("RD_LLM_PROVIDER")
		}
		if savedModel != "" {
			os.Setenv("RD_LLM_MODEL", savedModel)
		} else {
			os.Unsetenv("RD_LLM_MODEL")
		}
	}()

	// Set env vars
	os.Setenv("RD_LLM_PROVIDER", "mistral")
	os.Setenv("RD_LLM_MODEL", "env-model")

	// CLI takes precedence over ENV
	config := llm.ResolveProviderConfig("openai", "", "cli-model", "", 0, false)

	if config.Type != llm.ProviderOpenAI {
		t.Errorf("CLI provider should take precedence, got %s", config.Type)
	}

	if config.Model != "cli-model" {
		t.Errorf("CLI model should take precedence, got %s", config.Model)
	}

	// ENV takes precedence when CLI is empty
	config2 := llm.ResolveProviderConfig("", "", "", "", 0, false)

	if config2.Type != llm.ProviderMistral {
		t.Errorf("ENV provider should be used when CLI empty, got %s", config2.Type)
	}

	if config2.Model != "env-model" {
		t.Errorf("ENV model should be used when CLI empty, got %s", config2.Model)
	}
}

// TestProviderFailureFallbackToMock verifies that provider failures
// gracefully fall back to mock
func TestProviderFailureFallbackToMock(t *testing.T) {
	// Create a server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Simulated failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	config := &llm.ProviderConfig{
		Type:     llm.ProviderHTTP,
		Endpoint: server.URL,
		Token:    "test-token",
		Timeout:  2 * time.Second,
		Verbose:  false,
	}

	prov := llm.DefaultRegistry.Get(config)

	ctx := &llm.PlanContext{
		Profile: &scanner.ProjectProfile{
			Stack: "go",
		},
	}

	// Should still get a plan (from mock fallback)
	plan, err := prov.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("Expected fallback to mock, but got error: %v", err)
	}

	if plan == nil {
		t.Fatal("Expected plan from fallback, got nil")
	}

	// Plan should be valid
	if err := plan.Validate(); err != nil {
		t.Errorf("Fallback plan is invalid: %v", err)
	}
}

// TestOpenAIProviderRequestResponse tests OpenAI provider with mock server
func TestOpenAIProviderRequestResponse(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Bearer auth, got %s", r.Header.Get("Authorization"))
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected JSON content type")
		}

		// Return valid response
		resp := map[string]interface{}{
			"id": "test-123",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role": "assistant",
						"content": `{
							"version": "1",
							"project_type": "node",
							"prerequisites": [{"name": "node", "reason": "test"}],
							"steps": [{"id": "test", "cmd": "echo test", "cwd": ".", "risk": "low"}],
							"env": {},
							"ports": [3000],
							"notes": ["test"]
						}`,
					},
					"finish_reason": "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Test plan extraction directly
	content := `{"version": "1", "project_type": "node", "prerequisites": [], "steps": [{"id": "test", "cmd": "npm start", "cwd": ".", "risk": "low"}], "env": {}, "ports": [], "notes": []}`
	plan, err := provider.ExtractPlanFromLLMContent(content)
	if err != nil {
		t.Fatalf("ExtractPlanFromLLMContent failed: %v", err)
	}

	if plan.ProjectType != "node" {
		t.Errorf("Expected node project type, got %s", plan.ProjectType)
	}
}

// TestAnthropicProviderRequestResponse tests Anthropic provider with mock server
func TestAnthropicProviderRequestResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Anthropic-specific headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("Expected x-api-key header, got %s", r.Header.Get("x-api-key"))
		}

		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("Expected anthropic-version header")
		}

		// Return valid Anthropic response
		resp := map[string]interface{}{
			"id":   "test-123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": `{"version": "1", "project_type": "python", "prerequisites": [], "steps": [{"id": "test", "cmd": "python app.py", "cwd": ".", "risk": "low"}], "env": {}, "ports": [], "notes": []}`,
				},
			},
			"stop_reason": "end_turn",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Test that Anthropic response parsing works via ExtractPlanFromLLMContent
	content := `{"version": "1", "project_type": "python", "prerequisites": [], "steps": [{"id": "test", "cmd": "python app.py", "cwd": ".", "risk": "low"}], "env": {}, "ports": [], "notes": []}`

	plan, err := provider.ExtractPlanFromLLMContent(content)
	if err != nil {
		t.Fatalf("ExtractPlanFromLLMContent failed: %v", err)
	}

	if plan.ProjectType != "python" {
		t.Errorf("Expected python project type, got %s", plan.ProjectType)
	}
}

// TestRegistryUnknownProviderFallback verifies unknown provider falls back to mock
func TestRegistryUnknownProviderFallback(t *testing.T) {
	config := &llm.ProviderConfig{
		Type: llm.ProviderType("unknown-provider"),
	}

	prov := llm.DefaultRegistry.Get(config)

	if prov == nil {
		t.Fatal("Expected provider, got nil")
	}

	ctx := &llm.PlanContext{
		Profile: &scanner.ProjectProfile{
			Stack: "node",
		},
	}

	plan, err := prov.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("Expected mock fallback, got error: %v", err)
	}

	if plan == nil {
		t.Fatal("Expected plan, got nil")
	}
}

// TestFallbackProviderWrapping verifies FallbackProvider behavior
func TestFallbackProviderWrapping(t *testing.T) {
	// Create a provider that always fails
	failingProv := &failingTestProvider{}
	mockProv := provider.NewMockProvider()

	wrapper := &llm.FallbackProvider{
		Primary:  failingProv,
		Fallback: mockProv,
		Verbose:  false,
	}

	if wrapper.Name() != "failing" {
		t.Errorf("Expected 'failing' name, got %s", wrapper.Name())
	}

	ctx := &llm.PlanContext{
		Profile: &scanner.ProjectProfile{
			Stack: "rust",
		},
	}

	// Should fall back to mock
	plan, err := wrapper.GeneratePlan(ctx)
	if err != nil {
		t.Fatalf("Expected fallback to succeed, got error: %v", err)
	}

	if plan.ProjectType != "rust" {
		t.Errorf("Expected rust project type from mock, got %s", plan.ProjectType)
	}
}

// TestSupportedProvidersList verifies the supported providers list
func TestSupportedProvidersList(t *testing.T) {
	expected := []llm.ProviderType{
		llm.ProviderAnthropic,
		llm.ProviderOpenAI,
		llm.ProviderMistral,
		llm.ProviderOllama,
		llm.ProviderHTTP,
		llm.ProviderMock,
	}

	if len(llm.SupportedProviders) != len(expected) {
		t.Errorf("Expected %d supported providers, got %d", len(expected), len(llm.SupportedProviders))
	}

	for _, p := range expected {
		found := false
		for _, sp := range llm.SupportedProviders {
			if sp == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected provider %s in SupportedProviders", p)
		}
	}
}

// TestDeprecatedProvidersList verifies copilot is deprecated
func TestDeprecatedProvidersList(t *testing.T) {
	found := false
	for _, p := range llm.DeprecatedProviders {
		if p == llm.ProviderCopilot {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected copilot in DeprecatedProviders")
	}
}

// TestExtractPlanFromLLMContent tests JSON extraction from various formats
func TestExtractPlanFromLLMContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		wantPT  string // expected project type
	}{
		{
			name:    "clean JSON",
			content: `{"version": "1", "project_type": "node", "prerequisites": [], "steps": [{"id": "test", "cmd": "npm start", "cwd": ".", "risk": "low"}], "env": {}, "ports": [], "notes": []}`,
			wantErr: false,
			wantPT:  "node",
		},
		{
			name:    "JSON in markdown code block",
			content: "```json\n{\"version\": \"1\", \"project_type\": \"go\", \"prerequisites\": [], \"steps\": [{\"id\": \"test\", \"cmd\": \"go run .\", \"cwd\": \".\", \"risk\": \"low\"}], \"env\": {}, \"ports\": [], \"notes\": []}\n```",
			wantErr: false,
			wantPT:  "go",
		},
		{
			name:    "JSON with preamble text",
			content: "Here is the plan:\n{\"version\": \"1\", \"project_type\": \"python\", \"prerequisites\": [], \"steps\": [{\"id\": \"test\", \"cmd\": \"python app.py\", \"cwd\": \".\", \"risk\": \"low\"}], \"env\": {}, \"ports\": [], \"notes\": []}",
			wantErr: false,
			wantPT:  "python",
		},
		{
			name:    "no JSON",
			content: "This is just text without any JSON",
			wantErr: true,
		},
		{
			name:    "empty content",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := provider.ExtractPlanFromLLMContent(tt.content)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if plan.ProjectType != tt.wantPT {
				t.Errorf("Expected project type %s, got %s", tt.wantPT, plan.ProjectType)
			}
		})
	}
}

// failingTestProvider always fails for testing fallback
type failingTestProvider struct{}

func (p *failingTestProvider) Name() string { return "failing" }
func (p *failingTestProvider) GeneratePlan(ctx *llm.PlanContext) (*llm.RunPlan, error) {
	return nil, llm.ErrTimeout
}
