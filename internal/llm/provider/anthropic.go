// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Anthropic API provider for plan generation (recommended default)

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sony-level/readme-runner/internal/llm"
)

const (
	AnthropicEndpoint     = "https://api.anthropic.com/v1/messages"
	DefaultAnthropicModel = "claude-sonnet-4-20250514"
	AnthropicAPIVersion   = "2023-06-01"
)

// AnthropicProvider uses Anthropic API to generate plans
type AnthropicProvider struct {
	config  *llm.ProviderConfig
	client  *http.Client
	builder *llm.PromptBuilder
}

// NewAnthropicProvider creates a new Anthropic API provider
func NewAnthropicProvider(config *llm.ProviderConfig) (*AnthropicProvider, error) {
	token := getAnthropicToken(config)
	if token == "" {
		return nil, fmt.Errorf("Anthropic API key not found (set ANTHROPIC_API_KEY or use --llm-token)")
	}

	config.Token = token

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = llm.DefaultTimeout
	}

	return &AnthropicProvider{
		config: config,
		client: &http.Client{
			Timeout: timeout,
		},
		builder: llm.NewPromptBuilder(),
	}, nil
}

func getAnthropicToken(config *llm.ProviderConfig) string {
	if config.Token != "" {
		return config.Token
	}
	if token := os.Getenv("ANTHROPIC_API_KEY"); token != "" {
		return token
	}
	return os.Getenv("RD_LLM_TOKEN")
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// GeneratePlan generates a RunPlan using Anthropic API
func (p *AnthropicProvider) GeneratePlan(ctx *llm.PlanContext) (*llm.RunPlan, error) {
	prompt := p.builder.BuildPlanPrompt(ctx)

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		plan, err := p.callAPI(prompt)
		if err == nil {
			return plan, nil
		}
		lastErr = err

		if p.config.Verbose {
			fmt.Printf("  [Anthropic] Attempt %d failed: %v\n", attempt, err)
		}

		if err == llm.ErrTimeout {
			break
		}
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("Anthropic API failed: %w", lastErr)
}

// AnthropicRequest is the request body for Anthropic API
type AnthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []AnthropicMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
}

// AnthropicMessage represents a chat message
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicResponse is the response from Anthropic API
type AnthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *AnthropicProvider) callAPI(prompt string) (*llm.RunPlan, error) {
	model := p.config.Model
	if model == "" {
		model = DefaultAnthropicModel
	}

	reqBody := AnthropicRequest{
		Model:     model,
		MaxTokens: 2048,
		System:    "You are an expert at analyzing software projects and generating installation/run plans. IMPORTANT: Respond with ONLY valid JSON, no markdown code blocks, no explanation text. Follow the exact schema provided.",
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.1,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", AnthropicEndpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.Token)
	req.Header.Set("anthropic-version", AnthropicAPIVersion)

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
		return nil, parseAnthropicError(resp.StatusCode, body)
	}

	return p.parseResponse(body)
}

func parseAnthropicError(status int, body []byte) error {
	var errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		switch status {
		case 401:
			return fmt.Errorf("HTTP 401: invalid API key - check ANTHROPIC_API_KEY")
		case 403:
			return fmt.Errorf("HTTP 403: access forbidden - %s", errResp.Error.Message)
		case 429:
			return fmt.Errorf("HTTP 429: rate limited - %s", errResp.Error.Message)
		default:
			return fmt.Errorf("HTTP %d: %s", status, errResp.Error.Message)
		}
	}
	return fmt.Errorf("HTTP %d: %s", status, TruncateForError(string(body), 200))
}

func (p *AnthropicProvider) parseResponse(body []byte) (*llm.RunPlan, error) {
	var resp AnthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("API error: %s", resp.Error.Message)
	}

	if len(resp.Content) == 0 {
		return nil, llm.ErrEmptyResponse
	}

	var content string
	for _, c := range resp.Content {
		if c.Type == "text" {
			content = c.Text
			break
		}
	}

	if content == "" {
		return nil, llm.ErrEmptyResponse
	}

	return ExtractPlanFromLLMContent(content)
}

// IsAnthropicKeyAvailable checks if Anthropic API key is configured
func IsAnthropicKeyAvailable() bool {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return true
	}
	return os.Getenv("RD_LLM_TOKEN") != ""
}
