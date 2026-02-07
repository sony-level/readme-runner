// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// LLM provider factory and configuration

package llm

import (
	"errors"
	"time"
)

// Default timeouts
const (
	DefaultTimeout = 60 * time.Second
	MaxTimeout     = 300 * time.Second
)

// Provider errors
var (
	ErrUnknownProvider = errors.New("unknown provider type")
	ErrMissingEndpoint = errors.New("HTTP provider requires endpoint")
	ErrMissingToken    = errors.New("provider requires API token")
	ErrInvalidJSON     = errors.New("LLM returned invalid JSON")
	ErrTimeout         = errors.New("LLM request timed out")
	ErrEmptyResponse   = errors.New("LLM returned empty response")

	// Deprecated
	ErrCopilotNotFound = errors.New("GitHub Copilot is deprecated - use anthropic, openai, or mock")
)

// ProviderConfig holds configuration for LLM providers
type ProviderConfig struct {
	Type     ProviderType  // Provider type: anthropic, openai, mistral, ollama, http, mock
	Endpoint string        // HTTP endpoint URL (for HTTP/Ollama provider)
	Model    string        // Model name (optional)
	Token    string        // Authentication token
	Timeout  time.Duration // Request timeout
	Verbose  bool          // Enable verbose output
}

// Validate checks if the provider config is valid
func (c *ProviderConfig) Validate() error {
	switch c.Type {
	case ProviderHTTP:
		if c.Endpoint == "" {
			return ErrMissingEndpoint
		}
	case ProviderOpenAI, ProviderAnthropic, ProviderMistral:
		// Token validation happens in provider constructor
	case ProviderOllama, ProviderMock:
		// No validation required
	case ProviderCopilot:
		// Deprecated but still valid (will fallback to mock)
	default:
		return ErrUnknownProvider
	}
	return nil
}

// WithDefaults applies default values to the config
func (c *ProviderConfig) WithDefaults() *ProviderConfig {
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.Timeout > MaxTimeout {
		c.Timeout = MaxTimeout
	}
	return c
}

// NewProvider creates an LLM provider based on configuration.
// Uses the registry with automatic fallback to mock on failure.
func NewProvider(config *ProviderConfig) (Provider, error) {
	DefaultRegistry.SetVerbose(config != nil && config.Verbose)
	return DefaultRegistry.Get(config), nil
}

// NewProviderWithFallback creates a provider that gracefully falls back to mock.
// This is the recommended way to create providers for production use.
func NewProviderWithFallback(config *ProviderConfig) Provider {
	DefaultRegistry.SetVerbose(config != nil && config.Verbose)
	return DefaultRegistry.Get(config)
}

// MaskToken returns a masked version of the token for logging
func MaskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "[REDACTED]"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
