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
	ErrUnknownProvider   = errors.New("unknown provider type")
	ErrMissingEndpoint   = errors.New("HTTP provider requires endpoint")
	ErrMissingToken      = errors.New("HTTP provider requires token")
	ErrCopilotNotFound   = errors.New("GitHub Copilot CLI not found (gh copilot)")
	ErrInvalidJSON       = errors.New("LLM returned invalid JSON")
	ErrTimeout           = errors.New("LLM request timed out")
	ErrEmptyResponse     = errors.New("LLM returned empty response")
)

// ProviderConfig holds configuration for LLM providers
type ProviderConfig struct {
	Type     ProviderType  // Provider type: copilot, http, mock
	Endpoint string        // HTTP endpoint URL (for HTTP provider)
	Model    string        // Model name (optional)
	Token    string        // Authentication token (for HTTP provider)
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
		if c.Token == "" {
			return ErrMissingToken
		}
	case ProviderCopilot, ProviderMock:
		// No additional validation required
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

// NewProvider creates an LLM provider based on configuration
func NewProvider(config *ProviderConfig) (Provider, error) {
	if config == nil {
		config = &ProviderConfig{Type: ProviderMock}
	}

	// Apply defaults
	config = config.WithDefaults()

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, err
	}

	switch config.Type {
	case ProviderCopilot:
		return NewCopilotProvider(config), nil
	case ProviderHTTP:
		return NewHTTPProvider(config), nil
	case ProviderMock:
		return NewMockProvider(), nil
	default:
		return nil, ErrUnknownProvider
	}
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
