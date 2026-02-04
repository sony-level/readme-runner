// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Ollama provider for local LLM inference (no API key required)

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
	DefaultOllamaEndpoint = "http://localhost:11434/api/chat"
	DefaultOllamaModel    = "llama3.2"
)

// OllamaProvider uses local Ollama instance to generate plans
type OllamaProvider struct {
	config  *llm.ProviderConfig
	client  *http.Client
	builder *llm.PromptBuilder
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(config *llm.ProviderConfig) (*OllamaProvider, error) {
	endpoint := getOllamaEndpoint(config)
	config.Endpoint = endpoint

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = llm.DefaultTimeout
	}

	return &OllamaProvider{
		config: config,
		client: &http.Client{
			Timeout: timeout,
		},
		builder: llm.NewPromptBuilder(),
	}, nil
}

func getOllamaEndpoint(config *llm.ProviderConfig) string {
	if config.Endpoint != "" {
		return config.Endpoint
	}
	if endpoint := os.Getenv("OLLAMA_HOST"); endpoint != "" {
		if !strings.HasPrefix(endpoint, "http") {
			endpoint = "http://" + endpoint
		}
		return endpoint + "/api/chat"
	}
	return DefaultOllamaEndpoint
}

// Name returns the provider name
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// GeneratePlan generates a RunPlan using local Ollama
func (p *OllamaProvider) GeneratePlan(ctx *llm.PlanContext) (*llm.RunPlan, error) {
	prompt := p.builder.BuildPlanPrompt(ctx)

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		plan, err := p.callAPI(prompt)
		if err == nil {
			return plan, nil
		}
		lastErr = err

		if p.config.Verbose {
			fmt.Printf("  [Ollama] Attempt %d failed: %v\n", attempt, err)
		}

		if err == llm.ErrTimeout {
			break
		}
		if strings.Contains(err.Error(), "connection refused") {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("Ollama failed: %w", lastErr)
}

// OllamaRequest is the request body for Ollama API
type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *OllamaOptions  `json:"options,omitempty"`
}

// OllamaMessage represents a chat message
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaOptions contains model options
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// OllamaResponse is the response from Ollama API
type OllamaResponse struct {
	Model   string `json:"model"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done       bool   `json:"done"`
	DoneReason string `json:"done_reason,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (p *OllamaProvider) callAPI(prompt string) (*llm.RunPlan, error) {
	model := p.config.Model
	if model == "" {
		model = DefaultOllamaModel
	}

	reqBody := OllamaRequest{
		Model: model,
		Messages: []OllamaMessage{
			{
				Role:    "system",
				Content: "You are an expert at analyzing software projects and generating installation/run plans. IMPORTANT: Respond with ONLY valid JSON, no markdown code blocks, no explanation text. Follow the exact schema provided.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: false,
		Options: &OllamaOptions{
			Temperature: 0.1,
			NumPredict:  2048,
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

	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, llm.ErrTimeout
		}
		return nil, fmt.Errorf("request failed (is Ollama running?): %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, TruncateForError(string(body), 200))
	}

	return p.parseResponse(body)
}

func (p *OllamaProvider) parseResponse(body []byte) (*llm.RunPlan, error) {
	var resp OllamaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("Ollama error: %s", resp.Error)
	}

	content := resp.Message.Content
	if content == "" {
		return nil, llm.ErrEmptyResponse
	}

	return ExtractPlanFromLLMContent(content)
}

// IsOllamaAvailable checks if Ollama is running locally
func IsOllamaAvailable() bool {
	endpoint := DefaultOllamaEndpoint
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		if !strings.HasPrefix(host, "http") {
			host = "http://" + host
		}
		endpoint = host + "/api/chat"
	}

	client := &http.Client{Timeout: 2 * time.Second}
	tagsEndpoint := strings.Replace(endpoint, "/api/chat", "/api/tags", 1)
	resp, err := client.Get(tagsEndpoint)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
