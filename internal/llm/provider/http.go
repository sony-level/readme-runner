// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// HTTP LLM provider for custom endpoints

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sony-level/readme-runner/internal/llm"
)

// HTTPProvider calls a custom HTTP endpoint for plan generation
type HTTPProvider struct {
	config  *llm.ProviderConfig
	client  *http.Client
	builder *llm.PromptBuilder
}

// NewHTTPProvider creates a new HTTP LLM provider
func NewHTTPProvider(config *llm.ProviderConfig) *HTTPProvider {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = llm.DefaultTimeout
	}

	return &HTTPProvider{
		config: config,
		client: &http.Client{
			Timeout: timeout,
		},
		builder: llm.NewPromptBuilder(),
	}
}

// Name returns the provider name
func (p *HTTPProvider) Name() string {
	return "http"
}

// GeneratePlan generates a RunPlan using a custom HTTP endpoint
func (p *HTTPProvider) GeneratePlan(ctx *llm.PlanContext) (*llm.RunPlan, error) {
	prompt := p.builder.BuildPlanPrompt(ctx)

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

		if err == llm.ErrTimeout {
			break
		}

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
	Content string `json:"content"`
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error string `json:"error,omitempty"`
}

func (p *HTTPProvider) callEndpoint(prompt string) (*llm.RunPlan, error) {
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
			Temperature: 0.1,
			MaxTokens:   2000,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.Token)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, llm.ErrTimeout
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return p.parseResponse(body)
}

func (p *HTTPProvider) parseResponse(body []byte) (*llm.RunPlan, error) {
	var httpResp HTTPResponse
	if err := json.Unmarshal(body, &httpResp); err == nil {
		if httpResp.Error != "" {
			return nil, fmt.Errorf("LLM error: %s", httpResp.Error)
		}

		content := httpResp.Content
		if content == "" {
			content = httpResp.Message.Content
		}
		if content == "" && len(httpResp.Choices) > 0 {
			content = httpResp.Choices[0].Message.Content
		}

		if content != "" {
			return ExtractPlanFromLLMContent(content)
		}
	}

	var plan llm.RunPlan
	if err := json.Unmarshal(body, &plan); err == nil {
		if plan.Version == llm.ValidPlanVersion {
			if err := plan.Validate(); err != nil {
				return nil, fmt.Errorf("plan validation failed: %w", err)
			}
			return &plan, nil
		}
	}

	return nil, fmt.Errorf("%w: unable to extract plan from response", llm.ErrInvalidJSON)
}

// extractJSONFromText finds JSON in potentially mixed content
func extractJSONFromText(text string) string {
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	start := strings.Index(text, "{")
	if start < 0 {
		return ""
	}

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

	resp, err := client.Head(endpoint)
	if err != nil {
		resp, err = client.Get(endpoint)
	}
	if err != nil {
		return fmt.Errorf("endpoint unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("endpoint error: HTTP %d", resp.StatusCode)
	}

	return nil
}
