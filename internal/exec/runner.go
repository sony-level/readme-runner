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

// Compile-time interface checks
var _ StepRunner = (*Runner)(nil)
var _ PlanExecutor = (*Runner)(nil)

// Runner executes plan steps
type Runner struct {
	config         *RunnerConfig
	sudoPrompt     SudoPromptFunc
	failurePrompt  FailurePromptFunc
	sudoApproveAll bool
	mu             sync.Mutex
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

// RunStep executes a single step with the given options.
// This method satisfies the StepRunner interface.
func (r *Runner) RunStep(ctx context.Context, step *llm.Step, opts *RunOptions) (*StepResult, error) {
	// Build environment from options
	var env []string
	if opts != nil && len(opts.Environment) > 0 {
		env = os.Environ()
		for key, value := range opts.Environment {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Override working directory if specified in opts
	originalWorkDir := r.config.WorkingDir
	if opts != nil && opts.WorkingDir != "" {
		r.config.WorkingDir = opts.WorkingDir
	}
	defer func() {
		r.config.WorkingDir = originalWorkDir
	}()

	// Override verbose if specified
	originalVerbose := r.config.Verbose
	if opts != nil {
		r.config.Verbose = opts.Verbose
	}
	defer func() {
		r.config.Verbose = originalVerbose
	}()

	// Execute the step
	result := r.executeStepWithContext(ctx, step, env)

	// Return error if step failed (but only if it's a real error, not just non-zero exit)
	if !result.Success && result.Error != nil && result.Error.Error() == "aborted by user" {
		return result, result.Error
	}

	return result, nil
}

// Execute runs all steps in the plan
func (r *Runner) Execute(plan *llm.RunPlan) *ExecutionResult {
	return r.ExecuteWithContext(context.Background(), plan)
}

// ExecuteWithContext runs all steps with cancellation support
func (r *Runner) ExecuteWithContext(ctx context.Context, plan *llm.RunPlan) *ExecutionResult {
	result := NewExecutionResult()
	startTime := time.Now()

	// Store plan ports and notes for post-execution report
	result.Ports = plan.Ports
	result.Notes = plan.Notes

	// Apply global timeout if configured
	var cancel context.CancelFunc
	if r.config.GlobalTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, r.config.GlobalTimeout)
		defer cancel()
	}

	// Merge environment: process env + config env + plan env
	mergedEnv := r.buildMergedEnv(plan.Env)

	for i := range plan.Steps {
		// Check for cancellation before starting step
		select {
		case <-ctx.Done():
			result.AbortedByUser = true
			if ctx.Err() == context.DeadlineExceeded {
				result.TimeoutReached = true
			}
			break
		default:
		}

		if result.AbortedByUser {
			break
		}

		step := &plan.Steps[i]

		// Callback: step starting
		if r.config.OnStepStart != nil {
			r.config.OnStepStart(step)
		}

		// Execute the step with context and merged env
		stepResult := r.executeStepWithContext(ctx, step, mergedEnv)
		result.AddStepResult(stepResult)

		// Callback: step complete
		if r.config.OnStepComplete != nil {
			r.config.OnStepComplete(step, stepResult)
		}

		// Handle cancellation
		if stepResult.Cancelled {
			result.AbortedByUser = true
			if stepResult.Error != nil && strings.Contains(stepResult.Error.Error(), "timeout") {
				result.TimeoutReached = true
			}
			break
		}

		// Handle failure
		if !stepResult.Success && !stepResult.Skipped {
			if stepResult.Error != nil && stepResult.Error.Error() == "aborted by user" {
				result.AbortedByUser = true
				break
			}

			// Try deterministic auto-recovery for common startup failures before prompting.
			if recoveredResult, recovered := r.tryAutoRecoverStep(ctx, step, stepResult, mergedEnv); recovered {
				stepResult = recoveredResult
				result.StepResults[len(result.StepResults)-1] = stepResult
				if result.FailedStep != nil && result.FailedStep.StepID == step.ID {
					result.FailedStep = stepResult
				}

				if stepResult.Success {
					result.Failed--
					result.Completed++
					result.Success = result.Failed == 0
					if result.FailedStep != nil && result.FailedStep.StepID == step.ID {
						result.FailedStep = nil
					}
					continue
				}
			}

			// Ask user how to proceed (unless auto-yes)
			// Loop to allow multiple retries
		retryLoop:
			for !r.config.AutoYes {
				choice := r.failurePrompt(step, stepResult)
				switch choice {
				case FailureChoiceRetry:
					// Retry the step
					stepResult = r.executeStepWithContext(ctx, step, mergedEnv)
					result.StepResults[len(result.StepResults)-1] = stepResult
					if stepResult.Success {
						result.Failed--
						result.Completed++
						result.Success = len(result.StepResults) == result.Completed+result.Skipped
						break retryLoop // Success, exit retry loop
					}
					// Still failed, continue retry loop to prompt again
					continue
				case FailureChoiceSkip:
					// Convert failed step to skipped step
					stepResult.Skipped = true
					stepResult.SkipReason = "Skipped by user after failure"
					result.StepResults[len(result.StepResults)-1] = stepResult
					result.Failed--
					result.Skipped++
					// Clear FailedStep if this was the first failure
					if result.FailedStep == stepResult {
						result.FailedStep = nil
					}
					// Recalculate success: success if all non-skipped steps completed
					result.Success = result.Failed == 0
					break retryLoop
				case FailureChoiceContinue:
					// Continue to next step (step remains marked as failed)
					break retryLoop
				case FailureChoiceAbort:
					result.AbortedByUser = true
					break retryLoop
				}
			}

			if result.AbortedByUser {
				break
			}
		}
	}

	result.TotalTime = time.Since(startTime)

	// Mark as failed if aborted or timed out
	if result.AbortedByUser || result.TimeoutReached {
		result.Success = false
	}

	return result
}

// buildMergedEnv creates a merged environment from process env, config env, and plan env
func (r *Runner) buildMergedEnv(planEnv map[string]string) []string {
	// Start with current process environment
	env := os.Environ()

	// Add config-level environment variables
	for key, value := range r.config.Environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Add plan-level environment variables (highest priority)
	for key, value := range planEnv {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}

// executeStep runs a single step (backward compatibility wrapper)
func (r *Runner) executeStep(step *llm.Step) *StepResult {
	return r.executeStepWithContext(context.Background(), step, nil)
}

// executeStepWithContext runs a single step with context and merged environment
func (r *Runner) executeStepWithContext(ctx context.Context, step *llm.Step, mergedEnv []string) *StepResult {
	result := &StepResult{
		StepID: step.ID,
	}

	startTime := time.Now()

	// Check for cancellation
	select {
	case <-ctx.Done():
		result.Cancelled = true
		result.Success = false
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("global timeout reached")
		} else {
			result.Error = fmt.Errorf("execution cancelled")
		}
		result.Duration = time.Since(startTime)
		return result
	default:
	}

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

	// Execute the command with context and environment
	cmdResult := r.runCommandWithContext(ctx, step, mergedEnv)
	result.Success = cmdResult.Success
	result.ExitCode = cmdResult.ExitCode
	result.Stdout = cmdResult.Stdout
	result.Stderr = cmdResult.Stderr
	result.Error = cmdResult.Error
	result.Cancelled = cmdResult.Cancelled
	result.Duration = time.Since(startTime)

	return result
}

// CommandResult contains the raw result of running a command
type CommandResult struct {
	Success   bool
	ExitCode  int
	Stdout    string
	Stderr    string
	Error     error
	Cancelled bool
}

// runCommand executes a shell command (backward compatibility wrapper)
func (r *Runner) runCommand(step *llm.Step) *CommandResult {
	return r.runCommandWithContext(context.Background(), step, nil)
}

// runCommandWithContext executes a shell command with context and environment.
// This implementation uses process groups to ensure all child processes are
// properly terminated on timeout or cancellation.
func (r *Runner) runCommandWithContext(ctx context.Context, step *llm.Step, mergedEnv []string) *CommandResult {
	result := &CommandResult{}

	// Determine working directory
	workDir := r.config.WorkingDir
	if step.Cwd != "" && step.Cwd != "." {
		workDir = filepath.Join(r.config.WorkingDir, step.Cwd)
	}

	// Get timeout and create timeout context
	timeout := GetStepTimeout(step, r.config.StepTimeout)
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command with OS-appropriate shell
	// Note: We don't use exec.CommandContext because it only kills the direct
	// child process, not the entire process group. Instead, we manage
	// cancellation manually using process groups.
	var cmd *exec.Cmd
	if isWindows() {
		cmd = exec.Command("cmd", "/C", step.Cmd)
	} else {
		cmd = exec.Command("sh", "-c", step.Cmd)
	}
	cmd.Dir = workDir

	// Set up process group for proper child process termination
	setPlatformProcessGroup(cmd)

	// Set merged environment if provided
	if mergedEnv != nil {
		cmd.Env = mergedEnv
	}

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

	// Channel to signal when command completes
	done := make(chan error, 1)
	ready := make(chan struct{}, 1)
	autoStopOnReady := shouldAutoStopOnReady(step)

	// Read output concurrently
	var stdoutBuf, stderrBuf strings.Builder
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		r.streamOutput(stdout, &stdoutBuf, os.Stdout, func(line string) {
			if autoStopOnReady && isNextReadyLine(line) {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		r.streamOutput(stderr, &stderrBuf, os.Stderr, func(line string) {
			if autoStopOnReady && isNextReadyLine(line) {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
		})
	}()

	// Wait for command completion in a goroutine
	go func() {
		wg.Wait()          // Wait for output readers first
		done <- cmd.Wait() // Then wait for command
	}()

	// Wait for either command completion or context cancellation
	var waitErr error
	select {
	case waitErr = <-done:
		// Command completed normally (success or failure)
	case <-ready:
		// The command appears to have successfully started a server and is now
		// blocking (e.g. `npm start` for Next.js). Stop it and treat as success.
		if killErr := killProcessGroup(cmd); killErr != nil {
			_ = killErr
		}
		<-done
		result.Stdout = stdoutBuf.String()
		result.Stderr = stderrBuf.String()
		result.Success = true
		result.ExitCode = 0
		return result
	case <-stepCtx.Done():
		// Context was cancelled (timeout or manual cancellation)
		// Kill the entire process group
		if killErr := killProcessGroup(cmd); killErr != nil {
			// Log but don't fail - the process might have already exited
			_ = killErr
		}
		// Wait for the command to actually terminate
		<-done
		// Determine if this was a timeout or cancellation
		if ctx.Err() != nil {
			result.Cancelled = true
			if ctx.Err() == context.DeadlineExceeded {
				result.Error = fmt.Errorf("global timeout reached")
			} else {
				result.Error = fmt.Errorf("execution cancelled")
			}
		} else {
			result.Error = fmt.Errorf("command timed out after %v", timeout)
		}
		result.Stdout = stdoutBuf.String()
		result.Stderr = stderrBuf.String()
		return result
	}

	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()

	if waitErr != nil {
		// Check if this was due to context cancellation during wait
		if ctx.Err() != nil {
			result.Cancelled = true
			if ctx.Err() == context.DeadlineExceeded {
				result.Error = fmt.Errorf("global timeout reached")
			} else {
				result.Error = fmt.Errorf("execution cancelled")
			}
			return result
		}
		// Normal command failure
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = fmt.Errorf("command exited with code %d", result.ExitCode)
		} else {
			result.Error = waitErr
		}
		return result
	}

	result.Success = true
	result.ExitCode = 0
	return result
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return strings.Contains(strings.ToLower(os.Getenv("OS")), "windows")
}

// streamOutput reads from a pipe and writes to both a buffer and output
func (r *Runner) streamOutput(pipe io.ReadCloser, buf *strings.Builder, out io.Writer, onLine func(line string)) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line)
		buf.WriteString("\n")

		if onLine != nil {
			onLine(line)
		}

		// Only write to output if verbose or in execute mode
		if r.config.Verbose || r.config.Mode == ModeExecute {
			fmt.Fprintln(out, line)
		}
	}
}

func shouldAutoStopOnReady(step *llm.Step) bool {
	if step == nil {
		return false
	}
	// The "run" step is typically expected to start the app and may not exit.
	// For common frameworks (e.g. Next.js), we treat readiness output as success.
	if strings.EqualFold(step.ID, "run") {
		return true
	}
	return false
}

func isNextReadyLine(line string) bool {
	l := strings.ToLower(line)
	// Next.js prints: "ready started server on ..." or "ready - started server on ..."
	return strings.Contains(l, "ready") && strings.Contains(l, "started server")
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
	} else if result.TimeoutReached {
		sb.WriteString("⏱ Execution stopped: global timeout reached\n")
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

	// Post-execution report: ports and next actions
	if result.Success {
		sb.WriteString("\n─────────────────────────────────────\n")
		sb.WriteString("Next Steps\n")
		sb.WriteString("─────────────────────────────────────\n")

		// Show exposed ports
		if len(result.Ports) > 0 {
			sb.WriteString("\nApplication may be available at:\n")
			for _, port := range result.Ports {
				sb.WriteString(fmt.Sprintf("  → http://localhost:%d\n", port))
			}
		}

		// Show notes/next actions
		if len(result.Notes) > 0 {
			sb.WriteString("\nNotes:\n")
			for _, note := range result.Notes {
				sb.WriteString(fmt.Sprintf("  • %s\n", note))
			}
		}

		if len(result.Ports) == 0 && len(result.Notes) == 0 {
			sb.WriteString("\n  Check application output for next steps.\n")
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
		// Determine step type marker
		stepMarker := getStepTypeMarker(&step)

		sb.WriteString(fmt.Sprintf("\n  [%d] %s %s\n", i+1, step.ID, stepMarker))
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

// getStepTypeMarker returns a visual marker for the step type
func getStepTypeMarker(step *llm.Step) string {
	id := strings.ToLower(step.ID)
	cmd := strings.ToLower(step.Cmd)

	// Install-like steps
	if strings.Contains(id, "install") || strings.Contains(id, "deps") ||
		strings.Contains(cmd, "npm install") || strings.Contains(cmd, "npm ci") ||
		strings.Contains(cmd, "yarn install") || strings.Contains(cmd, "pnpm install") ||
		strings.Contains(cmd, "pip install") || strings.Contains(cmd, "poetry install") ||
		strings.Contains(cmd, "go mod download") || strings.Contains(cmd, "cargo fetch") ||
		strings.Contains(cmd, "bundle install") || strings.Contains(cmd, "composer install") {
		return "[📦 INSTALL]"
	}

	// Build steps
	if strings.Contains(id, "build") || strings.Contains(id, "compile") ||
		strings.Contains(cmd, "go build") || strings.Contains(cmd, "cargo build") ||
		strings.Contains(cmd, "npm run build") || strings.Contains(cmd, "yarn build") ||
		strings.Contains(cmd, "make build") || strings.Contains(cmd, "docker build") {
		return "[🔨 BUILD]"
	}

	// Test steps
	if strings.Contains(id, "test") ||
		strings.Contains(cmd, "go test") || strings.Contains(cmd, "npm test") ||
		strings.Contains(cmd, "pytest") || strings.Contains(cmd, "cargo test") {
		return "[🧪 TEST]"
	}

	// Run/start steps
	if strings.Contains(id, "run") || strings.Contains(id, "start") || strings.Contains(id, "serve") ||
		strings.Contains(cmd, "npm start") || strings.Contains(cmd, "npm run dev") ||
		strings.Contains(cmd, "docker compose up") || strings.Contains(cmd, "docker-compose up") ||
		strings.Contains(cmd, "docker run") || strings.Contains(cmd, "./") {
		return "[🚀 RUN]"
	}

	// Setup/config steps
	if strings.Contains(id, "setup") || strings.Contains(id, "config") || strings.Contains(id, "init") ||
		strings.Contains(id, "venv") || strings.Contains(cmd, "venv") {
		return "[⚙️ SETUP]"
	}

	return ""
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

// tryAutoRecoverStep attempts deterministic recovery for known command failures.
// Returns (result, true) when a recovery attempt was made.
func (r *Runner) tryAutoRecoverStep(ctx context.Context, step *llm.Step, failed *StepResult, mergedEnv []string) (*StepResult, bool) {
	buildCmd, ok := inferRecoveryBuildCommand(step, failed)
	if !ok {
		return failed, false
	}

	fmt.Printf("    ↻ Auto-recovery: Next.js production start requires a build\n")
	fmt.Printf("    ↻ Running: %s\n", buildCmd)

	buildStep := &llm.Step{
		ID:   step.ID + "_autobuild",
		Cmd:  buildCmd,
		Cwd:  step.Cwd,
		Risk: llm.RiskMedium,
	}

	buildResult := r.executeStepWithContext(ctx, buildStep, mergedEnv)
	if !buildResult.Success {
		fmt.Printf("    ↻ Auto-recovery failed: %s\n", buildResult.Error)
		combined := *failed
		combined.Stderr = strings.TrimSpace(strings.Join([]string{
			failed.Stderr,
			"[auto-recovery] " + buildCmd + " failed",
			buildResult.Stderr,
		}, "\n"))
		return &combined, true
	}

	fmt.Printf("    ↻ Build succeeded, retrying original step\n")
	retried := r.executeStepWithContext(ctx, step, mergedEnv)
	if retried.Success {
		fmt.Printf("    ↻ Auto-recovery succeeded\n")
	}

	return retried, true
}

func inferRecoveryBuildCommand(step *llm.Step, failed *StepResult) (string, bool) {
	if step == nil || failed == nil {
		return "", false
	}

	if !isNextMissingBuildError(failed) {
		return "", false
	}

	cmd := strings.ToLower(step.Cmd)

	switch {
	case strings.Contains(cmd, "pnpm run start") || strings.Contains(cmd, "pnpm start"):
		return "pnpm run build", true
	case strings.Contains(cmd, "yarn run start") || strings.Contains(cmd, "yarn start"):
		return "yarn build", true
	case strings.Contains(cmd, "bun run start") || strings.Contains(cmd, "bun start"):
		return "bun run build", true
	case strings.Contains(cmd, "next start"):
		return "next build", true
	case strings.Contains(cmd, "npm run start") || strings.Contains(cmd, "npm start"):
		return "npm run build", true
	default:
		return "", false
	}
}

func isNextMissingBuildError(failed *StepResult) bool {
	output := strings.ToLower(failed.Stderr + "\n" + failed.Stdout)

	return strings.Contains(output, "production-start-no-build-id") ||
		(strings.Contains(output, "could not find a production build") &&
			strings.Contains(output, ".next"))
}
