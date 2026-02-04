// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Mistral API provider for plan generation

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
	MistralEndpoint     = "https://api.mistral.ai/v1/chat/completions"
	DefaultMistralModel = "mistral-small-latest"
)

// MistralProvider uses Mistral API to generate plans
type MistralProvider struct {
	config  *llm.ProviderConfig
	client  *http.Client
	builder *llm.PromptBuilder
}

// NewMistralProvider creates a new Mistral API provider
func NewMistralProvider(config *llm.ProviderConfig) (*MistralProvider, error) {
	token := getMistralToken(config)
	if token == "" {
		return nil, fmt.Errorf("Mistral API key not found (set MISTRAL_API_KEY or use --llm-token)")
	}

	config.Token = token

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = llm.DefaultTimeout
	}

	return &MistralProvider{
		config: config,
		client: &http.Client{
			Timeout: timeout,
		},
		builder: llm.NewPromptBuilder(),
	}, nil
}

func getMistralToken(config *llm.ProviderConfig) string {
	if config.Token != "" {
		return config.Token
	}
	if token := os.Getenv("MISTRAL_API_KEY"); token != "" {
		return token
	}
	return os.Getenv("RD_LLM_TOKEN")
}

// Name returns the provider name
func (p *MistralProvider) Name() string {
	return "mistral"
}

// GeneratePlan generates a RunPlan using Mistral API
func (p *MistralProvider) GeneratePlan(ctx *llm.PlanContext) (*llm.RunPlan, error) {
	prompt := p.builder.BuildPlanPrompt(ctx)

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		plan, err := p.callAPI(prompt)
		if err == nil {
			return plan, nil
		}
		lastErr = err

		if p.config.Verbose {
			fmt.Printf("  [Mistral] Attempt %d failed: %v\n", attempt, err)
		}

		if err == llm.ErrTimeout {
			break
		}
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("Mistral API failed: %w", lastErr)
}

// MistralRequest is the request body for Mistral API
type MistralRequest struct {
	Model       string           `json:"model"`
	Messages    []MistralMessage `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
}

// MistralMessage represents a chat message
type MistralMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// MistralResponse is the response from Mistral API
type MistralResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func (p *MistralProvider) callAPI(prompt string) (*llm.RunPlan, error) {
	model := p.config.Model
	if model == "" {
		model = DefaultMistralModel
	}

	reqBody := MistralRequest{
		Model: model,
		Messages: []MistralMessage{
			{
				Role:    "system",
				Content: "You are an expert at analyzing software projects and generating installation/run plans. IMPORTANT: Respond with ONLY valid JSON, no markdown code blocks, no explanation text. Follow the exact schema provided.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.1,
		MaxTokens:   2048,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", MistralEndpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.Token)

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
		return nil, parseMistralError(resp.StatusCode, body)
	}

	return p.parseResponse(body)
}

func parseMistralError(status int, body []byte) error {
	var errResp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
		switch status {
		case 401:
			return fmt.Errorf("HTTP 401: invalid API key - check MISTRAL_API_KEY")
		case 403:
			return fmt.Errorf("HTTP 403: access forbidden - %s", errResp.Message)
		case 429:
			return fmt.Errorf("HTTP 429: rate limited - %s", errResp.Message)
		default:
			return fmt.Errorf("HTTP %d: %s", status, errResp.Message)
		}
	}
	return fmt.Errorf("HTTP %d: %s", status, TruncateForError(string(body), 200))
}

func (p *MistralProvider) parseResponse(body []byte) (*llm.RunPlan, error) {
	var resp MistralResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, llm.ErrEmptyResponse
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		return nil, llm.ErrEmptyResponse
	}

	return ExtractPlanFromLLMContent(content)
}
