// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// OpenAI API provider for plan generation

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
	OpenAIEndpoint     = "https://api.openai.com/v1/chat/completions"
	DefaultOpenAIModel = "gpt-4o-mini"
)

// OpenAIProvider uses OpenAI API to generate plans
type OpenAIProvider struct {
	config  *llm.ProviderConfig
	client  *http.Client
	builder *llm.PromptBuilder
}

// NewOpenAIProvider creates a new OpenAI API provider
func NewOpenAIProvider(config *llm.ProviderConfig) (*OpenAIProvider, error) {
	token := getOpenAIToken(config)
	if token == "" {
		return nil, fmt.Errorf("OpenAI API key not found (set OPENAI_API_KEY or use --llm-token)")
	}

	config.Token = token

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = llm.DefaultTimeout
	}

	return &OpenAIProvider{
		config: config,
		client: &http.Client{
			Timeout: timeout,
		},
		builder: llm.NewPromptBuilder(),
	}, nil
}

func getOpenAIToken(config *llm.ProviderConfig) string {
	if config.Token != "" {
		return config.Token
	}
	if token := os.Getenv("OPENAI_API_KEY"); token != "" {
		return token
	}
	return os.Getenv("RD_LLM_TOKEN")
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// GeneratePlan generates a RunPlan using OpenAI API
func (p *OpenAIProvider) GeneratePlan(ctx *llm.PlanContext) (*llm.RunPlan, error) {
	prompt := p.builder.BuildPlanPrompt(ctx)

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		plan, err := p.callAPI(prompt)
		if err == nil {
			return plan, nil
		}
		lastErr = err

		if p.config.Verbose {
			fmt.Printf("  [OpenAI] Attempt %d failed: %v\n", attempt, err)
		}

		if err == llm.ErrTimeout {
			break
		}
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("OpenAI API failed: %w", lastErr)
}

// OpenAIRequest is the request body for OpenAI API
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

// OpenAIMessage represents a chat message
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse is the response from OpenAI API
type OpenAIResponse struct {
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

func (p *OpenAIProvider) callAPI(prompt string) (*llm.RunPlan, error) {
	model := p.config.Model
	if model == "" {
		model = DefaultOpenAIModel
	}

	reqBody := OpenAIRequest{
		Model: model,
		Messages: []OpenAIMessage{
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

	req, err := http.NewRequestWithContext(ctx, "POST", OpenAIEndpoint, bytes.NewReader(jsonBody))
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
		return nil, parseOpenAIError(resp.StatusCode, body)
	}

	return p.parseResponse(body)
}

func parseOpenAIError(status int, body []byte) error {
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		switch status {
		case 401:
			return fmt.Errorf("HTTP 401: invalid API key - check OPENAI_API_KEY")
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

func (p *OpenAIProvider) parseResponse(body []byte) (*llm.RunPlan, error) {
	var resp OpenAIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("API error: %s", resp.Error.Message)
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
