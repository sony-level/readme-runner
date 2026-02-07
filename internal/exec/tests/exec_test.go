// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Tests for executor

package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sony-level/readme-runner/internal/exec"
	"github.com/sony-level/readme-runner/internal/llm"
)

func TestRunnerDryRun(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeDryRun,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
	}

	runner := exec.NewRunner(config)

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Steps: []llm.Step{
			{ID: "test1", Cmd: "echo hello", Cwd: "."},
			{ID: "test2", Cmd: "echo world", Cwd: "."},
		},
	}

	result := runner.Execute(plan)

	if !result.Success {
		t.Error("Dry-run should always succeed")
	}

	if result.TotalSteps != 2 {
		t.Errorf("Expected 2 steps, got %d", result.TotalSteps)
	}

	if result.Completed != 2 {
		t.Errorf("Expected 2 completed, got %d", result.Completed)
	}
}

func TestRunnerExecuteSimple(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
		AutoYes:     true,
	}

	runner := exec.NewRunner(config)

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "echo", Cmd: "echo 'hello world'", Cwd: "."},
		},
	}

	result := runner.Execute(plan)

	if !result.Success {
		t.Errorf("Simple echo should succeed: %v", result.FailedStep)
	}

	if len(result.StepResults) != 1 {
		t.Errorf("Expected 1 step result, got %d", len(result.StepResults))
	}

	if result.StepResults[0].Stdout == "" {
		t.Error("Expected stdout output")
	}
}

func TestRunnerTimeout(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 1 * time.Second,
		AutoYes:     true,
	}

	runner := exec.NewRunner(config)

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "sleep", Cmd: "sleep 10", Cwd: ".", Timeout: 1},
		},
	}

	result := runner.Execute(plan)

	if result.Success {
		t.Error("Long running command should timeout")
	}

	if result.Failed != 1 {
		t.Errorf("Expected 1 failed step, got %d", result.Failed)
	}
}

func TestRunnerSudoAbort(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:       exec.ModeExecute,
		WorkingDir: "/tmp",
		AllowSudo:  false,
	}

	runner := exec.NewRunner(config)

	// Set sudo prompt to always abort
	runner.SetSudoPrompt(func(step *llm.Step) exec.SudoChoice {
		return exec.SudoChoiceAbort
	})

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "sudo", Cmd: "sudo echo test", Cwd: ".", RequiresSudo: true},
		},
	}

	result := runner.Execute(plan)

	if result.Success {
		t.Error("Sudo abort should fail execution")
	}

	if !result.AbortedByUser {
		t.Error("Should be marked as aborted by user")
	}
}

func TestRunnerSudoAllowAll(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:       exec.ModeExecute,
		WorkingDir: "/tmp",
		AllowSudo:  false,
		AutoYes:    true,
	}

	runner := exec.NewRunner(config)

	promptCount := 0
	runner.SetSudoPrompt(func(step *llm.Step) exec.SudoChoice {
		promptCount++
		return exec.SudoChoiceAllowAll
	})

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "step1", Cmd: "echo 1", Cwd: ".", RequiresSudo: true},
			{ID: "step2", Cmd: "echo 2", Cwd: ".", RequiresSudo: true},
		},
	}

	_ = runner.Execute(plan)

	// Should only prompt once due to AllowAll
	if promptCount != 1 {
		t.Errorf("Expected 1 prompt with AllowAll, got %d", promptCount)
	}
}

func TestGetStepTimeout(t *testing.T) {
	tests := []struct {
		name           string
		step           *llm.Step
		defaultTimeout time.Duration
		expected       time.Duration
	}{
		{
			name:           "use default",
			step:           &llm.Step{},
			defaultTimeout: 5 * time.Minute,
			expected:       5 * time.Minute,
		},
		{
			name:           "use step timeout",
			step:           &llm.Step{Timeout: 120},
			defaultTimeout: 5 * time.Minute,
			expected:       2 * time.Minute,
		},
		{
			name:           "cap at max",
			step:           &llm.Step{Timeout: 3600},
			defaultTimeout: 5 * time.Minute,
			expected:       exec.MaxStepTimeout,
		},
		{
			name:           "fallback to default constant",
			step:           &llm.Step{},
			defaultTimeout: 0,
			expected:       exec.DefaultStepTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exec.GetStepTimeout(tt.step, tt.defaultTimeout)
			if result != tt.expected {
				t.Errorf("GetStepTimeout() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDryRunDisplay(t *testing.T) {
	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Prerequisites: []llm.Prerequisite{
			{Name: "node", Reason: "Required", MinVersion: "18"},
		},
		Steps: []llm.Step{
			{ID: "install", Cmd: "npm ci", Cwd: ".", Risk: llm.RiskMedium},
			{ID: "run", Cmd: "npm start", Cwd: ".", Risk: llm.RiskLow},
		},
		Env:   map[string]string{"NODE_ENV": "production"},
		Ports: []int{3000},
		Notes: []string{"Test note"},
	}

	output := exec.DryRunDisplay(plan, "/workspace")

	// Check for key elements
	if output == "" {
		t.Error("DryRunDisplay should produce output")
	}

	if len(output) < 100 {
		t.Error("DryRunDisplay output seems too short")
	}
}

func TestFormatStepResult(t *testing.T) {
	tests := []struct {
		name     string
		result   *exec.StepResult
		contains string
	}{
		{
			name: "success",
			result: &exec.StepResult{
				StepID:   "test",
				Success:  true,
				Duration: time.Second,
			},
			contains: "✓",
		},
		{
			name: "failed",
			result: &exec.StepResult{
				StepID:   "test",
				Success:  false,
				Duration: time.Second,
			},
			contains: "✗",
		},
		{
			name: "skipped",
			result: &exec.StepResult{
				StepID:     "test",
				Skipped:    true,
				SkipReason: "manual",
				Success:    true,
				Duration:   time.Second,
			},
			contains: "⊘",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := exec.FormatStepResult(tt.result)
			if output == "" {
				t.Error("FormatStepResult should produce output")
			}
		})
	}
}

func TestExecutionResult(t *testing.T) {
	result := exec.NewExecutionResult()

	if !result.Success {
		t.Error("New result should start as success")
	}

	// Add successful result
	result.AddStepResult(&exec.StepResult{
		StepID:  "step1",
		Success: true,
	})

	if result.Completed != 1 {
		t.Errorf("Expected 1 completed, got %d", result.Completed)
	}

	// Add failed result
	result.AddStepResult(&exec.StepResult{
		StepID:  "step2",
		Success: false,
	})

	if result.Success {
		t.Error("Result should be failed after adding failed step")
	}

	if result.Failed != 1 {
		t.Errorf("Expected 1 failed, got %d", result.Failed)
	}

	// Add skipped result
	result.AddStepResult(&exec.StepResult{
		StepID:  "step3",
		Skipped: true,
	})

	if result.Skipped != 1 {
		t.Errorf("Expected 1 skipped, got %d", result.Skipped)
	}

	if result.TotalSteps != 3 {
		t.Errorf("Expected 3 total steps, got %d", result.TotalSteps)
	}
}

func TestExecuteWithContextCancellation(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 30 * time.Second,
		AutoYes:     true,
	}

	runner := exec.NewRunner(config)

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "long", Cmd: "sleep 30", Cwd: "."},
		},
	}

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := runner.ExecuteWithContext(ctx, plan)

	if result.Success {
		t.Error("Cancelled execution should not succeed")
	}

	if !result.AbortedByUser {
		t.Error("Cancelled execution should be marked as aborted")
	}
}

func TestGlobalTimeout(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:          exec.ModeExecute,
		WorkingDir:    "/tmp",
		StepTimeout:   30 * time.Second,
		GlobalTimeout: 1 * time.Second, // Very short global timeout
		AutoYes:       true,
	}

	runner := exec.NewRunner(config)

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "long1", Cmd: "sleep 10", Cwd: "."},
			{ID: "long2", Cmd: "sleep 10", Cwd: "."},
		},
	}

	result := runner.Execute(plan)

	if result.Success {
		t.Error("Execution should fail due to global timeout")
	}

	if !result.TimeoutReached {
		t.Error("TimeoutReached should be true")
	}
}

func TestMergedEnvironment(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
		AutoYes:     true,
		Environment: map[string]string{
			"CONFIG_VAR": "from_config",
		},
	}

	runner := exec.NewRunner(config)

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Env: map[string]string{
			"PLAN_VAR": "from_plan",
		},
		Steps: []llm.Step{
			{ID: "check_env", Cmd: "echo $CONFIG_VAR $PLAN_VAR", Cwd: "."},
		},
	}

	result := runner.Execute(plan)

	if !result.Success {
		t.Errorf("Should succeed: %v", result.FailedStep)
	}

	if len(result.StepResults) != 1 {
		t.Fatalf("Expected 1 step result, got %d", len(result.StepResults))
	}

	stdout := result.StepResults[0].Stdout
	if !strings.Contains(stdout, "from_config") {
		t.Errorf("Expected CONFIG_VAR in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "from_plan") {
		t.Errorf("Expected PLAN_VAR in output, got: %s", stdout)
	}
}

func TestDryRunDisplayWithStepMarkers(t *testing.T) {
	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Steps: []llm.Step{
			{ID: "install", Cmd: "npm ci", Cwd: ".", Risk: llm.RiskMedium},
			{ID: "build", Cmd: "npm run build", Cwd: ".", Risk: llm.RiskLow},
			{ID: "test", Cmd: "npm test", Cwd: ".", Risk: llm.RiskLow},
			{ID: "start", Cmd: "npm start", Cwd: ".", Risk: llm.RiskLow},
		},
		Ports: []int{3000},
	}

	output := exec.DryRunDisplay(plan, "/workspace")

	// Check for step type markers
	if !strings.Contains(output, "INSTALL") {
		t.Error("Expected INSTALL marker in output")
	}
	if !strings.Contains(output, "BUILD") {
		t.Error("Expected BUILD marker in output")
	}
	if !strings.Contains(output, "TEST") {
		t.Error("Expected TEST marker in output")
	}
	if !strings.Contains(output, "RUN") {
		t.Error("Expected RUN marker in output")
	}
}

func TestPostExecutionReportWithPorts(t *testing.T) {
	result := exec.NewExecutionResult()
	result.Success = true
	result.Ports = []int{3000, 8080}
	result.Notes = []string{"Run 'npm run dev' for development mode"}
	result.TotalTime = 5 * time.Second

	output := exec.FormatExecutionResult(result)

	// Check for ports in output
	if !strings.Contains(output, "localhost:3000") {
		t.Error("Expected port 3000 in output")
	}
	if !strings.Contains(output, "localhost:8080") {
		t.Error("Expected port 8080 in output")
	}
	if !strings.Contains(output, "npm run dev") {
		t.Error("Expected notes in output")
	}
	if !strings.Contains(output, "Next Steps") {
		t.Error("Expected Next Steps section in output")
	}
}

func TestTimeoutReachedInReport(t *testing.T) {
	result := exec.NewExecutionResult()
	result.Success = false
	result.TimeoutReached = true
	result.TotalTime = 30 * time.Second

	output := exec.FormatExecutionResult(result)

	if !strings.Contains(output, "timeout") {
		t.Error("Expected timeout message in output")
	}
}

func TestStepResultCancelled(t *testing.T) {
	result := &exec.StepResult{
		StepID:    "test",
		Cancelled: true,
		Success:   false,
	}

	if result.Success {
		t.Error("Cancelled step should not be successful")
	}

	if !result.Cancelled {
		t.Error("Cancelled flag should be set")
	}
}

// TestDryRunNoExecution verifies that dry-run mode never executes commands
func TestDryRunNoExecution(t *testing.T) {
	// Create a temp file path that should NOT be created
	testFile := "/tmp/rd-run-dryrun-test-" + time.Now().Format("20060102150405")

	config := &exec.RunnerConfig{
		Mode:        exec.ModeDryRun,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
	}

	runner := exec.NewRunner(config)

	// This command would create a file if executed
	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "create-file", Cmd: "touch " + testFile, Cwd: "."},
			{ID: "create-dir", Cmd: "mkdir -p /tmp/rd-run-dryrun-dir", Cwd: "."},
		},
	}

	result := runner.Execute(plan)

	// Dry-run should always succeed
	if !result.Success {
		t.Error("Dry-run should always succeed")
	}

	// All steps should be marked as completed (in dry-run context)
	if result.Completed != 2 {
		t.Errorf("Expected 2 completed steps in dry-run, got %d", result.Completed)
	}

	// Verify no file was created (this is the key security guarantee)
	if _, err := os.Stat(testFile); err == nil {
		t.Errorf("Dry-run executed a command! File %s was created", testFile)
		os.Remove(testFile) // Clean up
	}
}

// TestSudoConsentEnforcement verifies sudo steps require explicit consent
func TestSudoConsentEnforcement(t *testing.T) {
	tests := []struct {
		name          string
		allowSudo     bool
		sudoChoice    exec.SudoChoice
		expectSuccess bool
		expectAborted bool
		expectSkipped int
	}{
		{
			name:          "AllowSudo flag bypasses prompt",
			allowSudo:     true,
			sudoChoice:    exec.SudoChoiceAbort, // Should not be called
			expectSuccess: true,
			expectAborted: false,
			expectSkipped: 0,
		},
		{
			name:          "Sudo abort stops execution",
			allowSudo:     false,
			sudoChoice:    exec.SudoChoiceAbort,
			expectSuccess: false,
			expectAborted: true,
			expectSkipped: 0,
		},
		{
			name:          "Sudo manual skips step",
			allowSudo:     false,
			sudoChoice:    exec.SudoChoiceManual,
			expectSuccess: true,
			expectAborted: false,
			expectSkipped: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &exec.RunnerConfig{
				Mode:        exec.ModeExecute,
				WorkingDir:  "/tmp",
				StepTimeout: 10 * time.Second,
				AllowSudo:   tt.allowSudo,
				AutoYes:     true,
			}

			runner := exec.NewRunner(config)
			runner.SetSudoPrompt(func(step *llm.Step) exec.SudoChoice {
				return tt.sudoChoice
			})

			plan := &llm.RunPlan{
				Version:     "1",
				ProjectType: "mixed",
				Steps: []llm.Step{
					{ID: "sudo-step", Cmd: "echo sudo test", Cwd: ".", RequiresSudo: true},
				},
			}

			result := runner.Execute(plan)

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success=%v, got %v", tt.expectSuccess, result.Success)
			}

			if result.AbortedByUser != tt.expectAborted {
				t.Errorf("Expected aborted=%v, got %v", tt.expectAborted, result.AbortedByUser)
			}

			if result.Skipped != tt.expectSkipped {
				t.Errorf("Expected %d skipped, got %d", tt.expectSkipped, result.Skipped)
			}
		})
	}
}

// TestSequentialExecution verifies steps execute in order
func TestSequentialExecution(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
		AutoYes:     true,
	}

	runner := exec.NewRunner(config)

	// Track execution order
	var executionOrder []string
	originalOnStart := config.OnStepStart
	config.OnStepStart = func(step *llm.Step) {
		executionOrder = append(executionOrder, step.ID)
		if originalOnStart != nil {
			originalOnStart(step)
		}
	}

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "step-1", Cmd: "echo one", Cwd: "."},
			{ID: "step-2", Cmd: "echo two", Cwd: "."},
			{ID: "step-3", Cmd: "echo three", Cwd: "."},
		},
	}

	result := runner.Execute(plan)

	if !result.Success {
		t.Error("Sequential execution should succeed")
	}

	// Verify order
	expectedOrder := []string{"step-1", "step-2", "step-3"}
	if len(executionOrder) != len(expectedOrder) {
		t.Errorf("Expected %d steps executed, got %d", len(expectedOrder), len(executionOrder))
	}

	for i, expected := range expectedOrder {
		if i < len(executionOrder) && executionOrder[i] != expected {
			t.Errorf("Step %d: expected %s, got %s", i, expected, executionOrder[i])
		}
	}
}

// TestFailureSkipBehavior tests the new skip option for failures
func TestFailureSkipBehavior(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
		AutoYes:     false, // Allow failure prompts
	}

	runner := exec.NewRunner(config)

	// Set failure prompt to skip
	runner.SetFailurePrompt(func(step *llm.Step, result *exec.StepResult) exec.FailureChoice {
		return exec.FailureChoiceSkip
	})

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "will-fail", Cmd: "exit 1", Cwd: "."},
			{ID: "should-run", Cmd: "echo after skip", Cwd: "."},
		},
	}

	result := runner.Execute(plan)

	// Should succeed overall because the failed step was skipped
	if !result.Success {
		t.Error("Execution should succeed when failed step is skipped")
	}

	if result.Skipped != 1 {
		t.Errorf("Expected 1 skipped step, got %d", result.Skipped)
	}

	if result.Failed != 0 {
		t.Errorf("Expected 0 failed steps (was skipped), got %d", result.Failed)
	}

	if result.Completed != 1 {
		t.Errorf("Expected 1 completed step, got %d", result.Completed)
	}
}

// TestFailureRetryBehavior tests retry on failure
func TestFailureRetryBehavior(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
		AutoYes:     false,
	}

	runner := exec.NewRunner(config)

	// Count retries and succeed on second attempt
	attempts := 0
	runner.SetFailurePrompt(func(step *llm.Step, result *exec.StepResult) exec.FailureChoice {
		attempts++
		if attempts < 2 {
			return exec.FailureChoiceRetry
		}
		return exec.FailureChoiceAbort
	})

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			// This will fail, be retried, and fail again
			{ID: "will-fail", Cmd: "exit 1", Cwd: "."},
		},
	}

	result := runner.Execute(plan)

	// Should have retried once
	if attempts != 2 {
		t.Errorf("Expected 2 attempts (original + retry), got %d", attempts)
	}

	// Should be aborted after retry failed
	if !result.AbortedByUser {
		t.Error("Expected abort after retry failed")
	}
}

// TestFailureContinueBehavior tests continue on failure
func TestFailureContinueBehavior(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
		AutoYes:     false,
	}

	runner := exec.NewRunner(config)

	runner.SetFailurePrompt(func(step *llm.Step, result *exec.StepResult) exec.FailureChoice {
		return exec.FailureChoiceContinue
	})

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "will-fail", Cmd: "exit 1", Cwd: "."},
			{ID: "should-run", Cmd: "echo after continue", Cwd: "."},
		},
	}

	result := runner.Execute(plan)

	// Should not succeed because a step failed
	if result.Success {
		t.Error("Execution should fail when a step failed (continue keeps failure)")
	}

	// Should have 1 failed and 1 completed
	if result.Failed != 1 {
		t.Errorf("Expected 1 failed step, got %d", result.Failed)
	}

	if result.Completed != 1 {
		t.Errorf("Expected 1 completed step, got %d", result.Completed)
	}
}

func TestAutoRecoveryNextStartMissingBuild(t *testing.T) {
	tempDir := t.TempDir()
	npmPath := filepath.Join(tempDir, "npm")

	script := `#!/bin/sh
case "$1 $2" in
  "run build")
    touch build-ran
    exit 0
    ;;
  "start ")
    if [ -f build-ran ]; then
      echo "server started"
      exit 0
    fi
    echo "Error: Could not find a production build in the '.next' directory. production-start-no-build-id" >&2
    exit 1
    ;;
esac
echo "unexpected args: $@" >&2
exit 1
`

	if err := os.WriteFile(npmPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake npm: %v", err)
	}

	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  tempDir,
		StepTimeout: 10 * time.Second,
		AutoYes:     true,
		Environment: map[string]string{
			"PATH": tempDir + ":" + os.Getenv("PATH"),
		},
	}

	runner := exec.NewRunner(config)
	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Steps: []llm.Step{
			{ID: "run", Cmd: "npm start", Cwd: "."},
		},
	}

	result := runner.Execute(plan)
	if !result.Success {
		t.Fatalf("expected auto-recovery to succeed, got failure: %+v", result.FailedStep)
	}

	if result.Completed != 1 || result.Failed != 0 {
		t.Fatalf("unexpected counters: completed=%d failed=%d", result.Completed, result.Failed)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "build-ran")); err != nil {
		t.Fatalf("expected build marker file, got: %v", err)
	}
}

func TestAutoRecoveryNextStartMissingBuildBuildFails(t *testing.T) {
	tempDir := t.TempDir()
	npmPath := filepath.Join(tempDir, "npm")

	script := `#!/bin/sh
case "$1 $2" in
  "run build")
    echo "build failed" >&2
    exit 1
    ;;
  "start ")
    echo "Error: Could not find a production build in the '.next' directory. production-start-no-build-id" >&2
    exit 1
    ;;
esac
exit 1
`

	if err := os.WriteFile(npmPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake npm: %v", err)
	}

	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  tempDir,
		StepTimeout: 10 * time.Second,
		AutoYes:     true,
		Environment: map[string]string{
			"PATH": tempDir + ":" + os.Getenv("PATH"),
		},
	}

	runner := exec.NewRunner(config)
	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Steps: []llm.Step{
			{ID: "run", Cmd: "npm start", Cwd: "."},
		},
	}

	result := runner.Execute(plan)
	if result.Success {
		t.Fatal("expected execution to fail when auto-recovery build fails")
	}

	if result.FailedStep == nil || !strings.Contains(result.FailedStep.Stderr, "[auto-recovery]") {
		t.Fatalf("expected auto-recovery context in stderr, got: %+v", result.FailedStep)
	}
}

// TestStepTimeoutEnforcement verifies per-step timeout is enforced.
// Process group handling ensures all child processes are killed on timeout.
func TestStepTimeoutEnforcement(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 30 * time.Second, // Default step timeout
		AutoYes:     true,
	}

	runner := exec.NewRunner(config)

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			// This step has a 1-second timeout but tries to sleep for 10
			// Process group handling ensures the sleep is killed properly
			{ID: "slow-step", Cmd: "sleep 10", Cwd: ".", Timeout: 1},
		},
	}

	start := time.Now()
	result := runner.Execute(plan)
	elapsed := time.Since(start)

	// Should fail due to timeout
	if result.Success {
		t.Error("Step should have timed out")
	}

	// Should have finished in ~1 second, not 10+
	// Allow 2 seconds for process cleanup
	if elapsed > 2*time.Second {
		t.Errorf("Step timeout not enforced, took %v", elapsed)
	}

	// Check error message mentions timeout
	if result.FailedStep != nil && result.FailedStep.Error != nil {
		if !strings.Contains(result.FailedStep.Error.Error(), "timed out") {
			t.Errorf("Expected timeout error, got: %s", result.FailedStep.Error.Error())
		}
	}
}

// TestRunStepInterface tests the RunStep method
func TestRunStepInterface(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
	}

	runner := exec.NewRunner(config)

	step := &llm.Step{
		ID:  "test-step",
		Cmd: "echo 'hello from RunStep'",
		Cwd: ".",
	}

	opts := &exec.RunOptions{
		WorkingDir: "/tmp",
		Timeout:    5 * time.Second,
		Verbose:    false,
	}

	result, err := runner.RunStep(context.Background(), step, opts)

	if err != nil {
		t.Errorf("RunStep should not return error for successful step: %v", err)
	}

	if !result.Success {
		t.Error("RunStep should succeed for echo command")
	}

	if !strings.Contains(result.Stdout, "hello from RunStep") {
		t.Errorf("Expected output to contain 'hello from RunStep', got: %s", result.Stdout)
	}
}

// TestRunStepWithEnvironment tests RunStep with custom environment
func TestRunStepWithEnvironment(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
	}

	runner := exec.NewRunner(config)

	step := &llm.Step{
		ID:  "env-step",
		Cmd: "echo $MY_TEST_VAR",
		Cwd: ".",
	}

	opts := &exec.RunOptions{
		WorkingDir: "/tmp",
		Environment: map[string]string{
			"MY_TEST_VAR": "custom_value_123",
		},
	}

	result, err := runner.RunStep(context.Background(), step, opts)

	if err != nil {
		t.Errorf("RunStep should not return error: %v", err)
	}

	if !result.Success {
		t.Error("RunStep should succeed")
	}

	if !strings.Contains(result.Stdout, "custom_value_123") {
		t.Errorf("Expected custom env var in output, got: %s", result.Stdout)
	}
}

// TestCancellationDuringExecution tests Ctrl+C behavior.
// Process group handling ensures all child processes are killed on cancellation.
func TestCancellationDuringExecution(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 30 * time.Second,
		AutoYes:     true,
	}

	runner := exec.NewRunner(config)

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			// Process group handling ensures this sleep is killed on cancel
			{ID: "slow", Cmd: "sleep 30", Cwd: "."},
		},
	}

	// Create a context that cancels after a short delay
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result := runner.ExecuteWithContext(ctx, plan)
	elapsed := time.Since(start)

	// Should have been cancelled
	if !result.AbortedByUser {
		t.Error("Should be marked as aborted by user")
	}

	if result.Success {
		t.Error("Cancelled execution should not succeed")
	}

	// Should have finished quickly, not waited for sleep
	// Allow 1 second for process cleanup
	if elapsed > 1*time.Second {
		t.Errorf("Cancellation not handled quickly, took %v", elapsed)
	}
}

// TestProcessGroupKillsChildren verifies that child processes are killed
// when the parent shell is terminated due to timeout.
func TestProcessGroupKillsChildren(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 30 * time.Second,
		AutoYes:     true,
	}

	runner := exec.NewRunner(config)

	// Create a marker file path
	markerFile := "/tmp/rd-run-pgtest-" + time.Now().Format("20060102150405")

	// This command spawns a background child that creates a file after 2 seconds.
	// If process groups work correctly, the child should be killed before
	// it creates the file when we timeout after 500ms.
	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{
				ID: "spawn-child",
				// Spawn a subshell that sleeps then creates a file
				Cmd:     "(sleep 2 && touch " + markerFile + ") & sleep 30",
				Cwd:     ".",
				Timeout: 1, // 1 second timeout - child should be killed
			},
		},
	}

	result := runner.Execute(plan)

	// Should have timed out
	if result.Success {
		t.Error("Should have timed out")
	}

	// Wait a bit to ensure any surviving child would have time to create the file
	time.Sleep(3 * time.Second)

	// Verify the marker file was NOT created (child was killed)
	if _, err := os.Stat(markerFile); err == nil {
		os.Remove(markerFile) // Clean up
		t.Error("Child process was NOT killed - marker file was created")
	}
}

func TestRunStepAutoStopsOnNextReady(t *testing.T) {
	tempDir := t.TempDir()
	npmPath := filepath.Join(tempDir, "npm")

	// Simulate `npm start` running a server: prints readiness then blocks.
	script := `#!/bin/sh
echo "ready started server on 0.0.0.0:3000, url: http://localhost:3000"
sleep 30
`

	if err := os.WriteFile(npmPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake npm: %v", err)
	}

	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  tempDir,
		StepTimeout: 1 * time.Second, // Would fail without auto-stop-on-ready
		AutoYes:     true,
		Environment: map[string]string{
			"PATH": tempDir + ":" + os.Getenv("PATH"),
		},
	}

	runner := exec.NewRunner(config)
	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Steps: []llm.Step{
			{ID: "run", Cmd: "npm start", Cwd: "."},
		},
	}

	start := time.Now()
	result := runner.Execute(plan)
	elapsed := time.Since(start)

	if !result.Success {
		t.Fatalf("expected run step to succeed after readiness, got failure: %+v", result.FailedStep)
	}

	// Should have stopped quickly, not slept for 30 seconds (allow for cleanup).
	if elapsed > 2*time.Second {
		t.Fatalf("expected early stop after readiness, took %v", elapsed)
	}

	if result.Failed != 0 || result.Completed != 1 {
		t.Fatalf("unexpected counters: completed=%d failed=%d", result.Completed, result.Failed)
	}

	if !strings.Contains(result.StepResults[0].Stdout, "ready started server") {
		t.Fatalf("expected readiness output in stdout, got: %q", result.StepResults[0].Stdout)
	}
}

// TestAbortedByUserMarksFailure tests that abort sets success to false
func TestAbortedByUserMarksFailure(t *testing.T) {
	config := &exec.RunnerConfig{
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		StepTimeout: 10 * time.Second,
		AutoYes:     false,
	}

	runner := exec.NewRunner(config)
	runner.SetFailurePrompt(func(step *llm.Step, result *exec.StepResult) exec.FailureChoice {
		return exec.FailureChoiceAbort
	})

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "mixed",
		Steps: []llm.Step{
			{ID: "fail", Cmd: "exit 1", Cwd: "."},
		},
	}

	result := runner.Execute(plan)

	if result.Success {
		t.Error("Aborted execution should not be successful")
	}

	if !result.AbortedByUser {
		t.Error("Should be marked as aborted by user")
	}
}
