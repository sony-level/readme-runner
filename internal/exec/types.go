// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Execution types and interfaces

package exec

import (
	"context"
	"time"

	"github.com/sony-level/readme-runner/internal/llm"
)

// DefaultStepTimeout is the default timeout for a single step
const DefaultStepTimeout = 5 * time.Minute

// MaxStepTimeout is the maximum allowed timeout for a single step
const MaxStepTimeout = 30 * time.Minute

// DefaultGlobalTimeout is the default global execution timeout (0 = no limit)
const DefaultGlobalTimeout = 0

// ExecutionMode determines how the runner behaves
type ExecutionMode int

const (
	// ModeDryRun displays what would happen without executing
	ModeDryRun ExecutionMode = iota
	// ModeExecute actually runs the commands
	ModeExecute
)

// StepRunner is the interface for executing individual steps
type StepRunner interface {
	// RunStep executes a single step with the given options
	RunStep(ctx context.Context, step *llm.Step, opts *RunOptions) (*StepResult, error)
}

// PlanExecutor is the interface for executing complete plans
type PlanExecutor interface {
	// Execute runs all steps in the plan
	Execute(plan *llm.RunPlan) *ExecutionResult
	// ExecuteWithContext runs all steps with cancellation support
	ExecuteWithContext(ctx context.Context, plan *llm.RunPlan) *ExecutionResult
}

// RunOptions contains options for running a single step
type RunOptions struct {
	WorkingDir  string            // Base working directory
	Environment map[string]string // Additional environment variables
	Timeout     time.Duration     // Step timeout
	Verbose     bool              // Verbose output
}

// RunnerConfig configures the executor
type RunnerConfig struct {
	Mode           ExecutionMode
	WorkingDir     string            // Base working directory (usually workspace repo path)
	Environment    map[string]string // Plan-level environment variables
	AutoYes        bool              // Auto-accept non-sudo prompts
	AllowSudo      bool              // Skip sudo confirmation prompts
	Verbose        bool              // Enable verbose output
	StepTimeout    time.Duration     // Default timeout per step
	GlobalTimeout  time.Duration     // Global execution timeout (0 = no limit)
	OnStepStart    func(step *llm.Step)
	OnStepComplete func(step *llm.Step, result *StepResult)
}

// StepResult contains the result of executing a single step
type StepResult struct {
	StepID     string
	Success    bool
	Skipped    bool
	SkipReason string
	Cancelled  bool
	ExitCode   int
	Duration   time.Duration
	Stdout     string
	Stderr     string
	Error      error
}

// ExecutionResult contains the complete execution result
type ExecutionResult struct {
	Success        bool
	TotalSteps     int
	Completed      int
	Failed         int
	Skipped        int
	TotalTime      time.Duration
	StepResults    []*StepResult
	FailedStep     *StepResult
	AbortedByUser  bool
	TimeoutReached bool
	Ports          []int    // Ports from the plan for post-execution report
	Notes          []string // Notes from the plan for post-execution report
}

// NewExecutionResult creates an empty execution result
func NewExecutionResult() *ExecutionResult {
	return &ExecutionResult{
		Success:     true,
		StepResults: make([]*StepResult, 0),
	}
}

// AddStepResult adds a step result to the execution
func (r *ExecutionResult) AddStepResult(result *StepResult) {
	r.StepResults = append(r.StepResults, result)
	r.TotalSteps++

	if result.Skipped {
		r.Skipped++
	} else if result.Success {
		r.Completed++
	} else {
		r.Failed++
		r.Success = false
		if r.FailedStep == nil {
			r.FailedStep = result
		}
	}
}

// SudoChoice represents the user's choice for sudo handling
type SudoChoice int

const (
	// SudoChoiceAllow allows the specific step
	SudoChoiceAllow SudoChoice = iota
	// SudoChoiceAllowAll allows all sudo steps in this run
	SudoChoiceAllowAll
	// SudoChoiceManual shows manual instructions
	SudoChoiceManual
	// SudoChoiceAbort aborts the entire operation
	SudoChoiceAbort
)

// FailureChoice represents the user's choice when a step fails
type FailureChoice int

const (
	// FailureChoiceContinue continues to the next step (step remains marked as failed)
	FailureChoiceContinue FailureChoice = iota
	// FailureChoiceRetry retries the failed step
	FailureChoiceRetry
	// FailureChoiceSkip marks the step as skipped and continues (step not counted as failed)
	FailureChoiceSkip
	// FailureChoiceAskAI asks an LLM to analyze the error and suggest a fix
	FailureChoiceAskAI
	// FailureChoiceAbort aborts the entire operation
	FailureChoiceAbort
)

// SudoPromptFunc is called when sudo confirmation is needed
type SudoPromptFunc func(step *llm.Step) SudoChoice

// FailurePromptFunc is called when a step fails
type FailurePromptFunc func(step *llm.Step, result *StepResult) FailureChoice

// AIFixContext contains context for AI-assisted error analysis
type AIFixContext struct {
	FailedStep    *llm.Step   // The step that failed
	StepResult    *StepResult // The result with error details
	ReadmeContent string      // README content for context
	ProjectType   string      // Detected project type
	AllSteps      []llm.Step  // All steps in the plan for context
	StepIndex     int         // Current step index
}

// AIFixSuggestion contains the LLM's suggested fix
type AIFixSuggestion struct {
	Analysis       string     // Explanation of what went wrong
	FixCommands    []string   // Commands to run to fix the issue
	ModifiedStep   *llm.Step  // Modified step to retry (if applicable)
	SkipStep       bool       // Whether to skip this step
	AdditionalNote string     // Any additional notes for the user
}

// AIFixFunc is called to get AI assistance for a failed step
type AIFixFunc func(ctx *AIFixContext) (*AIFixSuggestion, error)

// DefaultSudoPrompt returns a prompt function that always aborts
func DefaultSudoPrompt() SudoPromptFunc {
	return func(step *llm.Step) SudoChoice {
		return SudoChoiceAbort
	}
}

// DefaultFailurePrompt returns a prompt function that always aborts
func DefaultFailurePrompt() FailurePromptFunc {
	return func(step *llm.Step, result *StepResult) FailureChoice {
		return FailureChoiceAbort
	}
}

// GetStepTimeout returns the timeout for a step
func GetStepTimeout(step *llm.Step, defaultTimeout time.Duration) time.Duration {
	if step.Timeout > 0 {
		timeout := time.Duration(step.Timeout) * time.Second
		if timeout > MaxStepTimeout {
			return MaxStepTimeout
		}
		return timeout
	}
	if defaultTimeout > 0 {
		return defaultTimeout
	}
	return DefaultStepTimeout
}
