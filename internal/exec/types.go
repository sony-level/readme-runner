// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Execution types and interfaces

package exec

import (
	"time"

	"github.com/sony-level/readme-runner/internal/llm"
)

// DefaultStepTimeout is the default timeout for a single step
const DefaultStepTimeout = 5 * time.Minute

// MaxStepTimeout is the maximum allowed timeout for a single step
const MaxStepTimeout = 30 * time.Minute

// ExecutionMode determines how the runner behaves
type ExecutionMode int

const (
	// ModeDryRun displays what would happen without executing
	ModeDryRun ExecutionMode = iota
	// ModeExecute actually runs the commands
	ModeExecute
)

// RunnerConfig configures the executor
type RunnerConfig struct {
	Mode           ExecutionMode
	WorkingDir     string         // Base working directory (usually workspace repo path)
	AutoYes        bool           // Auto-accept non-sudo prompts
	AllowSudo      bool           // Skip sudo confirmation prompts
	Verbose        bool           // Enable verbose output
	StepTimeout    time.Duration  // Default timeout per step
	OnStepStart    func(step *llm.Step)
	OnStepComplete func(step *llm.Step, result *StepResult)
}

// StepResult contains the result of executing a single step
type StepResult struct {
	StepID     string
	Success    bool
	Skipped    bool
	SkipReason string
	ExitCode   int
	Duration   time.Duration
	Stdout     string
	Stderr     string
	Error      error
}

// ExecutionResult contains the complete execution result
type ExecutionResult struct {
	Success      bool
	TotalSteps   int
	Completed    int
	Failed       int
	Skipped      int
	TotalTime    time.Duration
	StepResults  []*StepResult
	FailedStep   *StepResult
	AbortedByUser bool
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
	// FailureChoiceContinue continues to the next step
	FailureChoiceContinue FailureChoice = iota
	// FailureChoiceRetry retries the failed step
	FailureChoiceRetry
	// FailureChoiceAbort aborts the entire operation
	FailureChoiceAbort
)

// SudoPromptFunc is called when sudo confirmation is needed
type SudoPromptFunc func(step *llm.Step) SudoChoice

// FailurePromptFunc is called when a step fails
type FailurePromptFunc func(step *llm.Step, result *StepResult) FailureChoice

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
