// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// LLM types and interfaces for plan generation

package llm

import "github.com/sony-level/readme-runner/internal/scanner"

// RiskLevel represents command execution risk
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// ProviderType identifies the LLM provider
type ProviderType string

const (
	// Active providers
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderMistral   ProviderType = "mistral"
	ProviderOllama    ProviderType = "ollama"
	ProviderHTTP      ProviderType = "http"
	ProviderMock      ProviderType = "mock"

	// Deprecated providers (will fallback to mock)
	ProviderCopilot ProviderType = "copilot" // DEPRECATED: Use openai, anthropic, or mock instead
)

// SupportedProviders lists all active provider types
var SupportedProviders = []ProviderType{
	ProviderAnthropic,
	ProviderOpenAI,
	ProviderMistral,
	ProviderOllama,
	ProviderHTTP,
	ProviderMock,
}

// DeprecatedProviders lists providers that are no longer supported
var DeprecatedProviders = []ProviderType{
	ProviderCopilot,
}

// ValidPlanVersion is the current RunPlan schema version
const ValidPlanVersion = "1"

// ValidProjectTypes are the allowed project types
var ValidProjectTypes = []string{"docker", "node", "python", "go", "rust", "mixed"}

// Provider interface for LLM providers
type Provider interface {
	// Name returns the provider name
	Name() string
	// GeneratePlan generates a RunPlan from the given context
	GeneratePlan(ctx *PlanContext) (*RunPlan, error)
}

// PlanContext contains input for plan generation
type PlanContext struct {
	ReadmeInfo   *scanner.ReadmeInfo     // README metadata and content
	Profile      *scanner.ProjectProfile // Detected project profile
	ClarityScore float64                 // README clarity score (0.0-1.0)
	UseReadme    bool                    // Whether to primarily use README
	OS           string                  // Target OS (linux, darwin, windows)
	Verbose      bool                    // Enable verbose output
}

// RunPlan is the JSON v1 schema for execution plans
type RunPlan struct {
	Version       string            `json:"version"`
	ProjectType   string            `json:"project_type"`
	Prerequisites []Prerequisite    `json:"prerequisites"`
	Steps         []Step            `json:"steps"`
	Env           map[string]string `json:"env"`
	Ports         []int             `json:"ports"`
	Notes         []string          `json:"notes"`
}

// Prerequisite defines a required tool
type Prerequisite struct {
	Name       string `json:"name"`
	Reason     string `json:"reason"`
	MinVersion string `json:"min_version,omitempty"`
}

// Step defines an execution step
type Step struct {
	ID           string    `json:"id"`
	Cmd          string    `json:"cmd"`
	Cwd          string    `json:"cwd"`
	Risk         RiskLevel `json:"risk"`
	RequiresSudo bool      `json:"requires_sudo"`
	Timeout      int       `json:"timeout,omitempty"`      // seconds, 0 = default
	Description  string    `json:"description,omitempty"` // optional description
}

// Validate checks if the RunPlan is valid
func (p *RunPlan) Validate() error {
	if p.Version != ValidPlanVersion {
		return &ValidationError{Field: "version", Message: "invalid version: " + p.Version}
	}

	if !isValidProjectType(p.ProjectType) {
		return &ValidationError{Field: "project_type", Message: "invalid project type: " + p.ProjectType}
	}

	for i, step := range p.Steps {
		if step.ID == "" {
			return &ValidationError{Field: "steps", Message: "step " + string(rune(i+1)) + " has empty id"}
		}
		if step.Cmd == "" {
			return &ValidationError{Field: "steps", Message: "step " + step.ID + " has empty cmd"}
		}
		if step.Cwd == "" {
			// Default to current directory
			p.Steps[i].Cwd = "."
		}
		if step.Risk == "" {
			// Default to low risk
			p.Steps[i].Risk = RiskLow
		}
	}

	return nil
}

// ValidationError represents a plan validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "validation error in " + e.Field + ": " + e.Message
}

// isValidProjectType checks if the project type is valid
func isValidProjectType(pt string) bool {
	for _, valid := range ValidProjectTypes {
		if pt == valid {
			return true
		}
	}
	return false
}

// GetStepByID returns a step by its ID
func (p *RunPlan) GetStepByID(id string) *Step {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			return &p.Steps[i]
		}
	}
	return nil
}

// HasSudoSteps returns true if any step requires sudo
func (p *RunPlan) HasSudoSteps() bool {
	for _, step := range p.Steps {
		if step.RequiresSudo {
			return true
		}
	}
	return false
}

// GetHighRiskSteps returns steps with high or critical risk
func (p *RunPlan) GetHighRiskSteps() []Step {
	var highRisk []Step
	for _, step := range p.Steps {
		if step.Risk == RiskHigh || step.Risk == RiskCritical {
			highRisk = append(highRisk, step)
		}
	}
	return highRisk
}
