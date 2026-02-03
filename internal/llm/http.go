// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// HTTP LLM provider for custom endpoints

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPProvider calls a custom HTTP endpoint for plan generation
type HTTPProvider struct {
	config  *ProviderConfig
	client  *http.Client
	builder *PromptBuilder
}

// NewHTTPProvider creates a new HTTP LLM provider
func NewHTTPProvider(config *ProviderConfig) *HTTPProvider {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	return &HTTPProvider{
		config: config,
		client: &http.Client{
			Timeout: timeout,
		},
		builder: NewPromptBuilder(),
	}
}

// Name returns the provider name
func (p *HTTPProvider) Name() string {
	return "http"
}

// GeneratePlan generates a RunPlan using a custom HTTP endpoint
func (p *HTTPProvider) GeneratePlan(ctx *PlanContext) (*RunPlan, error) {
	// Build the prompt
	prompt := p.builder.BuildPlanPrompt(ctx)

	// Try up to 2 times (initial + 1 retry)
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		plan, err := p.callEndpoint(prompt)
		if err == nil {
			return plan, nil
		}
		lastErr = err

		if p.config.Verbose {
			fmt.Printf("  [HTTP] Attempt %d failed: %v\n", attempt, err)
		}

		// Don't retry on timeout
		if err == ErrTimeout {
			break
		}

		// Brief pause before retry
		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("HTTP LLM failed after retries: %w", lastErr)
}

// HTTPRequest is the request body sent to the LLM endpoint
type HTTPRequest struct {
	Model    string        `json:"model,omitempty"`
	Messages []HTTPMessage `json:"messages"`
	Options  HTTPOptions   `json:"options,omitempty"`
}

// HTTPMessage represents a chat message
type HTTPMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// HTTPOptions contains optional request parameters
type HTTPOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// HTTPResponse is the expected response from the LLM endpoint
type HTTPResponse struct {
	Content string `json:"content"`          // Direct content field
	Message struct {                         // OpenAI-style
		Content string `json:"content"`
	} `json:"message"`
	Choices []struct {                       // OpenAI-style array
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error string `json:"error,omitempty"`
}

// callEndpoint makes the HTTP request to the LLM
func (p *HTTPProvider) callEndpoint(prompt string) (*RunPlan, error) {
	// Build request body
	reqBody := HTTPRequest{
		Model: p.config.Model,
		Messages: []HTTPMessage{
			{
				Role:    "system",
				Content: "You are an expert at analyzing software projects. Respond ONLY with valid JSON, no other text.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Options: HTTPOptions{
			Temperature: 0.1, // Low temperature for deterministic output
			MaxTokens:   2000,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if p.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.Token)
	}

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
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	return p.parseResponse(body)
}

// parseResponse extracts the RunPlan from HTTP response
func (p *HTTPProvider) parseResponse(body []byte) (*RunPlan, error) {
	// First try to parse as our expected response format
	var httpResp HTTPResponse
	if err := json.Unmarshal(body, &httpResp); err == nil {
		// Check for error in response
		if httpResp.Error != "" {
			return nil, fmt.Errorf("LLM error: %s", httpResp.Error)
		}

		// Try to extract content from various response formats
		content := httpResp.Content
		if content == "" {
			content = httpResp.Message.Content
		}
		if content == "" && len(httpResp.Choices) > 0 {
			content = httpResp.Choices[0].Message.Content
		}

		if content != "" {
			return p.extractPlanFromContent(content)
		}
	}

	// If response is directly a RunPlan JSON
	var plan RunPlan
	if err := json.Unmarshal(body, &plan); err == nil {
		if plan.Version == ValidPlanVersion {
			if err := plan.Validate(); err != nil {
				return nil, fmt.Errorf("plan validation failed: %w", err)
			}
			return &plan, nil
		}
	}

	return nil, fmt.Errorf("%w: unable to extract plan from response", ErrInvalidJSON)
}

// extractPlanFromContent parses the RunPlan from LLM content
func (p *HTTPProvider) extractPlanFromContent(content string) (*RunPlan, error) {
	if content == "" {
		return nil, ErrEmptyResponse
	}

	// Try to extract JSON from content (may include markdown code blocks)
	jsonStr := extractJSONFromText(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("%w: no JSON found in response", ErrInvalidJSON)
	}

	var plan RunPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("plan validation failed: %w", err)
	}

	return &plan, nil
}

// extractJSONFromText finds JSON in potentially mixed content
func extractJSONFromText(text string) string {
	// Remove markdown code blocks if present
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	// Find JSON object
	start := strings.Index(text, "{")
	if start < 0 {
		return ""
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}

	return ""
}

// HTTPProviderHealthCheck checks if the endpoint is reachable
func HTTPProviderHealthCheck(endpoint string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}

	// Try a simple HEAD request first
	resp, err := client.Head(endpoint)
	if err != nil {
		// Fall back to GET if HEAD not supported
		resp, err = client.Get(endpoint)
	}
	if err != nil {
		return fmt.Errorf("endpoint unreachable: %w", err)
	}
	defer resp.Body.Close()

	// Accept any 2xx or 4xx (4xx means endpoint exists but request invalid)
	if resp.StatusCode >= 500 {
		return fmt.Errorf("endpoint error: HTTP %d", resp.StatusCode)
	}

	return nil
}
