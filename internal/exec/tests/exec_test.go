// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Tests for executor

package tests

import (
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
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		AllowSudo:   false,
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
		Mode:        exec.ModeExecute,
		WorkingDir:  "/tmp",
		AllowSudo:   false,
		AutoYes:     true,
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
