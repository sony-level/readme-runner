// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Provider registration - registers all providers with the LLM registry

package provider

import (
	"github.com/sony-level/readme-runner/internal/llm"
)

func init() {
	RegisterProviders()
}

// RegisterProviders registers all built-in providers with the default registry
func RegisterProviders() {
	reg := llm.DefaultRegistry

	// Set mock factory first (needed for fallback)
	reg.SetMockFactory(func(config *llm.ProviderConfig) (llm.Provider, error) {
		return NewMockProvider(), nil
	})

	// Register active providers
	reg.Register(llm.ProviderOpenAI, func(config *llm.ProviderConfig) (llm.Provider, error) {
		return NewOpenAIProvider(config)
	})

	reg.Register(llm.ProviderAnthropic, func(config *llm.ProviderConfig) (llm.Provider, error) {
		return NewAnthropicProvider(config)
	})

	reg.Register(llm.ProviderMistral, func(config *llm.ProviderConfig) (llm.Provider, error) {
		return NewMistralProvider(config)
	})

	reg.Register(llm.ProviderOllama, func(config *llm.ProviderConfig) (llm.Provider, error) {
		return NewOllamaProvider(config)
	})

	reg.Register(llm.ProviderHTTP, func(config *llm.ProviderConfig) (llm.Provider, error) {
		if config.Endpoint == "" {
			return nil, llm.ErrMissingEndpoint
		}
		return NewHTTPProvider(config), nil
	})

	// Register deprecated copilot provider (prints warning, returns mock)
	reg.Register(llm.ProviderCopilot, func(config *llm.ProviderConfig) (llm.Provider, error) {
		llm.PrintCopilotDeprecationWarning()
		return NewMockProvider(), nil
	})
}
