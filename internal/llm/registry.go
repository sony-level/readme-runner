// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Provider registry with factory pattern and graceful fallback

package llm

import (
	"fmt"
	"sync"
)

// ProviderFactory creates a provider from config
type ProviderFactory func(config *ProviderConfig) (Provider, error)

// Registry manages provider factories and handles fallback
type Registry struct {
	mu          sync.RWMutex
	factories   map[ProviderType]ProviderFactory
	mockFactory ProviderFactory
	verbose     bool
}

// DefaultRegistry is the global provider registry
var DefaultRegistry = NewRegistry()

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[ProviderType]ProviderFactory),
	}
}

// SetMockFactory sets the mock provider factory for fallback
func (r *Registry) SetMockFactory(factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mockFactory = factory
	// Also register it as the mock provider type
	r.factories[ProviderMock] = factory
}

// Register adds a provider factory to the registry
func (r *Registry) Register(providerType ProviderType, factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[providerType] = factory
}

// SetVerbose enables verbose output for registry operations
func (r *Registry) SetVerbose(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verbose = v
}

// Get creates a provider with graceful fallback to mock
func (r *Registry) Get(config *ProviderConfig) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if config == nil {
		config = &ProviderConfig{Type: ProviderMock}
	}
	config = config.WithDefaults()

	// Check if deprecated
	if isDeprecated(config.Type) {
		if r.verbose {
			fmt.Printf("  [Registry] Provider '%s' is deprecated, using mock\n", config.Type)
		}
		return r.getMockProvider(config)
	}

	// Try to create the requested provider
	factory, ok := r.factories[config.Type]
	if !ok {
		if r.verbose {
			fmt.Printf("  [Registry] Unknown provider '%s', using mock\n", config.Type)
		}
		return r.getMockProvider(config)
	}

	prov, err := factory(config)
	if err != nil {
		if r.verbose {
			fmt.Printf("  [Registry] Failed to create provider '%s': %v, using mock\n", config.Type, err)
		}
		return r.getMockProvider(config)
	}

	// Wrap with fallback
	mockProv := r.getMockProvider(config)
	return &FallbackProvider{
		Primary:  prov,
		Fallback: mockProv,
		Verbose:  config.Verbose,
	}
}

// getMockProvider returns a mock provider
func (r *Registry) getMockProvider(config *ProviderConfig) Provider {
	if r.mockFactory == nil {
		// Return a minimal fallback if no mock factory is set
		return &minimalMockProvider{}
	}
	prov, err := r.mockFactory(config)
	if err != nil {
		return &minimalMockProvider{}
	}
	return prov
}

// isDeprecated checks if a provider type is deprecated
func isDeprecated(pt ProviderType) bool {
	for _, dep := range DeprecatedProviders {
		if pt == dep {
			return true
		}
	}
	return false
}

// minimalMockProvider is a fallback when no mock factory is set
type minimalMockProvider struct{}

func (p *minimalMockProvider) Name() string { return "minimal-mock" }

func (p *minimalMockProvider) GeneratePlan(ctx *PlanContext) (*RunPlan, error) {
	return &RunPlan{
		Version:       "1",
		ProjectType:   "unknown",
		Prerequisites: []Prerequisite{},
		Steps: []Step{
			{ID: "info", Cmd: "echo 'No LLM provider configured'", Cwd: ".", Risk: RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{},
		Notes: []string{"Mock provider - no LLM configured"},
	}, nil
}

// FallbackProvider wraps a primary provider with mock fallback
type FallbackProvider struct {
	Primary  Provider
	Fallback Provider
	Verbose  bool
}

// Name returns the primary provider name
func (p *FallbackProvider) Name() string {
	return p.Primary.Name()
}

// GeneratePlan tries primary, falls back to mock on error
func (p *FallbackProvider) GeneratePlan(ctx *PlanContext) (*RunPlan, error) {
	plan, err := p.Primary.GeneratePlan(ctx)
	if err == nil {
		return plan, nil
	}

	if p.Verbose {
		fmt.Printf("  [%s] Failed: %v, falling back to mock\n", p.Primary.Name(), err)
	}

	// Fallback to mock - never abort
	return p.Fallback.GeneratePlan(ctx)
}

// PrintCopilotDeprecationWarning prints a deprecation warning for copilot provider
func PrintCopilotDeprecationWarning() {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║  WARNING: 'copilot' provider is deprecated and unsupported   ║")
	fmt.Println("  ║                                                              ║")
	fmt.Println("  ║  GitHub Copilot API is not available for custom tools.       ║")
	fmt.Println("  ║  Please migrate to one of these providers:                   ║")
	fmt.Println("  ║    • anthropic (recommended) - set ANTHROPIC_API_KEY         ║")
	fmt.Println("  ║    • openai                  - set OPENAI_API_KEY            ║")
	fmt.Println("  ║    • mistral                 - set MISTRAL_API_KEY           ║")
	fmt.Println("  ║    • ollama                  - local, no key needed          ║")
	fmt.Println("  ║    • mock                    - offline mode                  ║")
	fmt.Println("  ║                                                              ║")
	fmt.Println("  ║  Falling back to mock provider (offline mode)...             ║")
	fmt.Println("  ╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
}
