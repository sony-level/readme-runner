// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// GitHub Copilot API provider for plan generation

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	// CopilotAPIEndpoint is the legacy GitHub Copilot API endpoint.
	CopilotAPIEndpoint = "https://api.githubcopilot.com/chat/completions"
	// CopilotModelsEndpoint is the GitHub Models inference endpoint.
	CopilotModelsEndpoint = "https://models.github.ai/inference/chat/completions"
	// DefaultCopilotModel is the default model to use
	DefaultCopilotModel = "openai/gpt-4.1"
	// GitHubAPIVersion is the recommended REST API version header value.
	GitHubAPIVersion = "2022-11-28"
)

// CopilotProvider uses GitHub Copilot API to generate plans
type CopilotProvider struct {
	config  *ProviderConfig
	client  *http.Client
	builder *PromptBuilder
}

// NewCopilotProvider creates a new Copilot API provider
func NewCopilotProvider(config *ProviderConfig) *CopilotProvider {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	return &CopilotProvider{
		config: config,
		client: &http.Client{
			Timeout: timeout,
		},
		builder: NewPromptBuilder(),
	}
}

// Name returns the provider name
func (p *CopilotProvider) Name() string {
	return "copilot"
}

// GeneratePlan generates a RunPlan using GitHub Copilot API
func (p *CopilotProvider) GeneratePlan(ctx *PlanContext) (*RunPlan, error) {
	// Get token from config or environment
	token := p.getToken()
	if token == "" {
		return nil, fmt.Errorf("copilot token not found (set GITHUB_TOKEN, GH_TOKEN, RD_LLM_TOKEN, or use --llm-token)")
	}

	// Build the prompt
	prompt := p.builder.BuildPlanPrompt(ctx)

	// Try up to 2 times (initial + 1 retry)
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		plan, err := p.callAPI(prompt, token)
		if err == nil {
			return plan, nil
		}
		lastErr = err

		if p.config.Verbose {
			fmt.Printf("  [Copilot] Attempt %d failed: %v\n", attempt, err)
		}

		// Don't retry on timeout or auth errors
		if err == ErrTimeout {
			break
		}
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			break
		}

		// Brief pause before retry
		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("copilot API failed: %w", lastErr)
}

// getToken retrieves the API token from config or environment
func (p *CopilotProvider) getToken() string {
	if p.config.Token != "" {
		return p.config.Token
	}
	// Try GITHUB_TOKEN first (most common)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	// Try GH_TOKEN (GitHub CLI convention)
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token
	}
	// Try RD_LLM_TOKEN (our custom variable)
	return os.Getenv("RD_LLM_TOKEN")
}

// CopilotRequest is the request body for Copilot API
type CopilotRequest struct {
	Model       string            `json:"model"`
	Messages    []CopilotMessage  `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
}

// CopilotMessage represents a chat message
type CopilotMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CopilotResponse is the response from Copilot API
type CopilotResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// callAPI makes the HTTP request to Copilot API
func (p *CopilotProvider) callAPI(prompt, token string) (*RunPlan, error) {
	// Use configured endpoint or default
	endpoint := p.config.Endpoint
	if endpoint == "" {
		endpoint = CopilotModelsEndpoint
	}

	// Use configured model or default
	model := p.config.Model
	if model == "" {
		model = DefaultCopilotModel
	}

	// Build request body
	reqBody := CopilotRequest{
		Model: model,
		Messages: []CopilotMessage{
			{
				Role: "system",
				Content: `You are an expert at analyzing software projects and generating installation/run plans.
IMPORTANT: Respond with ONLY valid JSON, no markdown code blocks, no explanation text.
Follow the exact schema provided in the user's message.`,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.1, // Low temperature for deterministic output
		MaxTokens:   2048,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", GitHubAPIVersion)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "readme-runner/1.0")

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		bodyMsg := truncateBody(string(body), 200)
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("HTTP 401: unauthorized (invalid or expired token): %s", bodyMsg)
		case http.StatusForbidden:
			return nil, fmt.Errorf("HTTP 403: access forbidden (token may be missing GitHub Models access or `models:read` permission): %s", bodyMsg)
		default:
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, bodyMsg)
		}
	}

	// Parse response
	return p.parseResponse(body)
}

// parseResponse extracts the RunPlan from Copilot API response
func (p *CopilotProvider) parseResponse(body []byte) (*RunPlan, error) {
	var resp CopilotResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API error
	if resp.Error != nil {
		return nil, fmt.Errorf("API error: %s", resp.Error.Message)
	}

	// Extract content from first choice
	if len(resp.Choices) == 0 {
		return nil, ErrEmptyResponse
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		return nil, ErrEmptyResponse
	}

	// Extract and parse the JSON plan
	return p.extractPlanFromContent(content)
}

// extractPlanFromContent parses the RunPlan from LLM content
func (p *CopilotProvider) extractPlanFromContent(content string) (*RunPlan, error) {
	// Clean up the content - remove markdown code blocks if present
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// Find JSON object boundaries
	start := strings.Index(content, "{")
	if start < 0 {
		return nil, fmt.Errorf("%w: no JSON object found", ErrInvalidJSON)
	}

	// Find matching closing brace
	depth := 0
	end := -1
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end > 0 {
			break
		}
	}

	if end <= start {
		return nil, fmt.Errorf("%w: malformed JSON", ErrInvalidJSON)
	}

	jsonStr := content[start:end]

	// Parse the JSON
	var plan RunPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	// Validate the plan
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("plan validation failed: %w", err)
	}

	return &plan, nil
}

// BuildCopilotPrompt creates a prompt optimized for Copilot API
func (p *CopilotProvider) BuildCopilotPrompt(ctx *PlanContext) string {
	var sb strings.Builder

	sb.WriteString("Generate a JSON installation plan for this project.\n\n")

	// Add OS context
	os := ctx.OS
	if os == "" {
		os = runtime.GOOS
	}
	sb.WriteString(fmt.Sprintf("Target OS: %s\n\n", os))

	// README-first approach
	if ctx.UseReadme && ctx.ReadmeInfo != nil && ctx.ReadmeInfo.Content != "" {
		sb.WriteString("README (PRIMARY SOURCE):\n")
		sb.WriteString("```\n")
		content := ctx.ReadmeInfo.Content
		if len(content) > 6000 {
			content = content[:6000] + "\n... (truncated)"
		}
		sb.WriteString(content)
		sb.WriteString("\n```\n\n")
	}

	// Profile signals
	if ctx.Profile != nil {
		sb.WriteString("Project files detected:\n")
		if ctx.Profile.Stack != "" {
			sb.WriteString(fmt.Sprintf("- Primary stack: %s\n", ctx.Profile.Stack))
		}
		if len(ctx.Profile.Languages) > 0 {
			sb.WriteString(fmt.Sprintf("- Languages: %s\n", strings.Join(ctx.Profile.Languages, ", ")))
		}
		if len(ctx.Profile.Tools) > 0 {
			sb.WriteString(fmt.Sprintf("- Tools: %s\n", strings.Join(ctx.Profile.Tools, ", ")))
		}
		if len(ctx.Profile.Containers) > 0 {
			sb.WriteString(fmt.Sprintf("- Containers: %s\n", strings.Join(ctx.Profile.Containers, ", ")))
		}
		if len(ctx.Profile.Packages) > 0 {
			sb.WriteString(fmt.Sprintf("- Package files: %s\n", strings.Join(ctx.Profile.Packages, ", ")))
		}
		sb.WriteString("\n")
	}

	// Output format instruction
	sb.WriteString(`Output ONLY valid JSON:
{
  "version": "1",
  "project_type": "docker|node|python|go|rust|mixed",
  "prerequisites": [{"name": "tool", "reason": "why", "min_version": ""}],
  "steps": [{"id": "step_id", "cmd": "command", "cwd": ".", "risk": "low|medium|high|critical", "requires_sudo": false}],
  "env": {},
  "ports": [],
  "notes": []
}
`)

	return sb.String()
}

// CopilotAvailable checks if Copilot API is accessible with the given token
func CopilotAvailable(token string) bool {
	if token == "" {
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// truncateBody truncates a string for error messages
func truncateBody(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
