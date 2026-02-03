// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Step executor with streaming output and sudo handling

package exec

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sony-level/readme-runner/internal/llm"
)

// Runner executes plan steps
type Runner struct {
	config        *RunnerConfig
	sudoPrompt    SudoPromptFunc
	failurePrompt FailurePromptFunc
	sudoApproveAll bool
	mu            sync.Mutex
}

// NewRunner creates a new step runner
func NewRunner(config *RunnerConfig) *Runner {
	if config == nil {
		config = &RunnerConfig{
			Mode:        ModeDryRun,
			StepTimeout: DefaultStepTimeout,
		}
	}

	return &Runner{
		config:        config,
		sudoPrompt:    DefaultSudoPrompt(),
		failurePrompt: DefaultFailurePrompt(),
	}
}

// SetSudoPrompt sets the sudo confirmation prompt function
func (r *Runner) SetSudoPrompt(fn SudoPromptFunc) {
	r.sudoPrompt = fn
}

// SetFailurePrompt sets the failure handling prompt function
func (r *Runner) SetFailurePrompt(fn FailurePromptFunc) {
	r.failurePrompt = fn
}

// Execute runs all steps in the plan
func (r *Runner) Execute(plan *llm.RunPlan) *ExecutionResult {
	result := NewExecutionResult()
	startTime := time.Now()

	for i := range plan.Steps {
		step := &plan.Steps[i]

		// Callback: step starting
		if r.config.OnStepStart != nil {
			r.config.OnStepStart(step)
		}

		// Execute the step
		stepResult := r.executeStep(step)
		result.AddStepResult(stepResult)

		// Callback: step complete
		if r.config.OnStepComplete != nil {
			r.config.OnStepComplete(step, stepResult)
		}

		// Handle failure
		if !stepResult.Success && !stepResult.Skipped {
			if stepResult.Error != nil && stepResult.Error.Error() == "aborted by user" {
				result.AbortedByUser = true
				break
			}

			// Ask user how to proceed (unless auto-yes)
			if !r.config.AutoYes {
				choice := r.failurePrompt(step, stepResult)
				switch choice {
				case FailureChoiceRetry:
					// Retry the step
					stepResult = r.executeStep(step)
					result.StepResults[len(result.StepResults)-1] = stepResult
					if stepResult.Success {
						result.Failed--
						result.Completed++
						result.Success = len(result.StepResults) == result.Completed+result.Skipped
					}
				case FailureChoiceContinue:
					// Continue to next step
					continue
				case FailureChoiceAbort:
					result.AbortedByUser = true
					break
				}
			}

			if result.AbortedByUser {
				break
			}
		}
	}

	result.TotalTime = time.Since(startTime)
	return result
}

// executeStep runs a single step
func (r *Runner) executeStep(step *llm.Step) *StepResult {
	result := &StepResult{
		StepID: step.ID,
	}

	startTime := time.Now()

	// Dry-run mode: just display what would happen
	if r.config.Mode == ModeDryRun {
		result.Success = true
		result.Duration = time.Since(startTime)
		return result
	}

	// Check sudo requirement
	if step.RequiresSudo && !r.config.AllowSudo {
		r.mu.Lock()
		approveAll := r.sudoApproveAll
		r.mu.Unlock()

		if !approveAll {
			choice := r.sudoPrompt(step)
			switch choice {
			case SudoChoiceAllow:
				// Continue with this step
			case SudoChoiceAllowAll:
				r.mu.Lock()
				r.sudoApproveAll = true
				r.mu.Unlock()
			case SudoChoiceManual:
				result.Skipped = true
				result.SkipReason = "User chose manual execution"
				result.Success = true
				result.Duration = time.Since(startTime)
				return result
			case SudoChoiceAbort:
				result.Success = false
				result.Error = fmt.Errorf("aborted by user")
				result.Duration = time.Since(startTime)
				return result
			}
		}
	}

	// Execute the command
	cmdResult := r.runCommand(step)
	result.Success = cmdResult.Success
	result.ExitCode = cmdResult.ExitCode
	result.Stdout = cmdResult.Stdout
	result.Stderr = cmdResult.Stderr
	result.Error = cmdResult.Error
	result.Duration = time.Since(startTime)

	return result
}

// CommandResult contains the raw result of running a command
type CommandResult struct {
	Success  bool
	ExitCode int
	Stdout   string
	Stderr   string
	Error    error
}

// runCommand executes a shell command
func (r *Runner) runCommand(step *llm.Step) *CommandResult {
	result := &CommandResult{}

	// Determine working directory
	workDir := r.config.WorkingDir
	if step.Cwd != "" && step.Cwd != "." {
		workDir = filepath.Join(r.config.WorkingDir, step.Cwd)
	}

	// Get timeout
	timeout := GetStepTimeout(step, r.config.StepTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctx, "sh", "-c", step.Cmd)
	cmd.Dir = workDir

	// Set up pipes for stdout/stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Error = fmt.Errorf("failed to create stdout pipe: %w", err)
		return result
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Error = fmt.Errorf("failed to create stderr pipe: %w", err)
		return result
	}

	// Start command
	if err := cmd.Start(); err != nil {
		result.Error = fmt.Errorf("failed to start command: %w", err)
		return result
	}

	// Read output concurrently
	var stdoutBuf, stderrBuf strings.Builder
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		r.streamOutput(stdout, &stdoutBuf, os.Stdout)
	}()

	go func() {
		defer wg.Done()
		r.streamOutput(stderr, &stderrBuf, os.Stderr)
	}()

	// Wait for output readers
	wg.Wait()

	// Wait for command to complete
	err = cmd.Wait()

	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("command timed out after %v", timeout)
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = fmt.Errorf("command exited with code %d", result.ExitCode)
		} else {
			result.Error = err
		}
		return result
	}

	result.Success = true
	result.ExitCode = 0
	return result
}

// streamOutput reads from a pipe and writes to both a buffer and output
func (r *Runner) streamOutput(pipe io.ReadCloser, buf *strings.Builder, out io.Writer) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line)
		buf.WriteString("\n")

		// Only write to output if verbose or in execute mode
		if r.config.Verbose || r.config.Mode == ModeExecute {
			fmt.Fprintln(out, line)
		}
	}
}

// FormatStepResult returns a human-readable step result
func FormatStepResult(result *StepResult) string {
	var sb strings.Builder

	if result.Skipped {
		sb.WriteString(fmt.Sprintf("⊘ %s: Skipped", result.StepID))
		if result.SkipReason != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", result.SkipReason))
		}
	} else if result.Success {
		sb.WriteString(fmt.Sprintf("✓ %s: Success", result.StepID))
	} else {
		sb.WriteString(fmt.Sprintf("✗ %s: Failed", result.StepID))
		if result.Error != nil {
			sb.WriteString(fmt.Sprintf(" - %s", result.Error.Error()))
		}
	}

	sb.WriteString(fmt.Sprintf(" (%v)", result.Duration.Round(time.Millisecond)))
	return sb.String()
}

// FormatExecutionResult returns a human-readable execution summary
func FormatExecutionResult(result *ExecutionResult) string {
	var sb strings.Builder

	sb.WriteString("\n─────────────────────────────────────\n")
	sb.WriteString("Execution Summary\n")
	sb.WriteString("─────────────────────────────────────\n")

	if result.Success {
		sb.WriteString("✓ All steps completed successfully\n")
	} else if result.AbortedByUser {
		sb.WriteString("⊘ Execution aborted by user\n")
	} else {
		sb.WriteString("✗ Execution failed\n")
	}

	sb.WriteString(fmt.Sprintf("\nTotal steps: %d\n", result.TotalSteps))
	sb.WriteString(fmt.Sprintf("  Completed: %d\n", result.Completed))
	sb.WriteString(fmt.Sprintf("  Failed:    %d\n", result.Failed))
	sb.WriteString(fmt.Sprintf("  Skipped:   %d\n", result.Skipped))
	sb.WriteString(fmt.Sprintf("\nTotal time: %v\n", result.TotalTime.Round(time.Millisecond)))

	if result.FailedStep != nil {
		sb.WriteString(fmt.Sprintf("\nFailed at step: %s\n", result.FailedStep.StepID))
		if result.FailedStep.Stderr != "" {
			sb.WriteString("\nError output:\n")
			// Show last 10 lines of stderr
			lines := strings.Split(strings.TrimSpace(result.FailedStep.Stderr), "\n")
			if len(lines) > 10 {
				lines = lines[len(lines)-10:]
			}
			for _, line := range lines {
				sb.WriteString("  " + line + "\n")
			}
		}
	}

	return sb.String()
}

// DryRunDisplay shows what would be executed in dry-run mode
func DryRunDisplay(plan *llm.RunPlan, workDir string) string {
	var sb strings.Builder

	sb.WriteString("\n╔══════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    DRY-RUN MODE                              ║\n")
	sb.WriteString("║              No commands will be executed                    ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════╝\n\n")

	sb.WriteString(fmt.Sprintf("Project type: %s\n", plan.ProjectType))
	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))

	// Prerequisites
	if len(plan.Prerequisites) > 0 {
		sb.WriteString("Prerequisites:\n")
		for _, prereq := range plan.Prerequisites {
			sb.WriteString(fmt.Sprintf("  • %s", prereq.Name))
			if prereq.MinVersion != "" {
				sb.WriteString(fmt.Sprintf(" (>= %s)", prereq.MinVersion))
			}
			sb.WriteString(fmt.Sprintf(" - %s\n", prereq.Reason))
		}
		sb.WriteString("\n")
	}

	// Steps
	sb.WriteString("Steps to execute:\n")
	for i, step := range plan.Steps {
		sb.WriteString(fmt.Sprintf("\n  [%d] %s\n", i+1, step.ID))
		sb.WriteString(fmt.Sprintf("      Command: %s\n", step.Cmd))
		if step.Cwd != "" && step.Cwd != "." {
			sb.WriteString(fmt.Sprintf("      Directory: %s\n", step.Cwd))
		}
		sb.WriteString(fmt.Sprintf("      Risk: %s\n", step.Risk))
		if step.RequiresSudo {
			sb.WriteString("      ⚠ Requires sudo\n")
		}
		if step.Description != "" {
			sb.WriteString(fmt.Sprintf("      Description: %s\n", step.Description))
		}
	}

	// Environment variables
	if len(plan.Env) > 0 {
		sb.WriteString("\nEnvironment variables:\n")
		for key, value := range plan.Env {
			// Mask potentially sensitive values
			displayValue := value
			if isSensitiveKey(key) {
				displayValue = "[REDACTED]"
			}
			sb.WriteString(fmt.Sprintf("  %s=%s\n", key, displayValue))
		}
	}

	// Ports
	if len(plan.Ports) > 0 {
		sb.WriteString("\nExposed ports:\n")
		for _, port := range plan.Ports {
			sb.WriteString(fmt.Sprintf("  • %d\n", port))
		}
	}

	// Notes
	if len(plan.Notes) > 0 {
		sb.WriteString("\nNotes:\n")
		for _, note := range plan.Notes {
			sb.WriteString(fmt.Sprintf("  • %s\n", note))
		}
	}

	return sb.String()
}

// isSensitiveKey checks if an env var key might contain sensitive data
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	sensitivePatterns := []string{
		"password", "secret", "token", "key", "api_key",
		"apikey", "private", "credential", "auth",
	}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
