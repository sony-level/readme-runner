// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Sudo command confirmation and guard

package security

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sony-level/readme-runner/internal/llm"
)

// SudoGuard handles sudo command confirmation
type SudoGuard struct {
	allowSudo   bool // --allow-sudo flag
	autoYes     bool // --yes flag
	approveAll  bool // User chose "approve all" during session
	reader      *bufio.Reader
}

// NewSudoGuard creates a new sudo guard
func NewSudoGuard(allowSudo, autoYes bool) *SudoGuard {
	return &SudoGuard{
		allowSudo:  allowSudo,
		autoYes:    autoYes,
		approveAll: false,
		reader:     bufio.NewReader(os.Stdin),
	}
}

// CheckStep checks if a step requires sudo confirmation
// Returns (approved, abort) where abort means stop entire execution
func (g *SudoGuard) CheckStep(step *llm.Step) (approved bool, abort bool) {
	if !step.RequiresSudo {
		return true, false
	}

	// If --allow-sudo flag is set, approve automatically
	if g.allowSudo {
		return true, false
	}

	// If user already chose "approve all" in this session
	if g.approveAll {
		return true, false
	}

	// Even with --yes, sudo requires explicit confirmation
	// (This is the security-critical exception)
	return g.promptForApproval(step)
}

func (g *SudoGuard) promptForApproval(step *llm.Step) (approved bool, abort bool) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    SUDO REQUIRED                             ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  Step:    %s\n", step.ID)
	fmt.Printf("  Command: %s\n", step.Cmd)
	if step.Description != "" {
		fmt.Printf("  Purpose: %s\n", step.Description)
	}
	fmt.Println()
	fmt.Println("  This command requires elevated (sudo) privileges.")
	fmt.Println()
	fmt.Println("  Choose an option:")
	fmt.Println("    1) Allow for this step only")
	fmt.Println("    2) Allow for all sudo steps in this run")
	fmt.Println("    3) Show manual instructions (skip this step)")
	fmt.Println("    4) Abort entire operation")
	fmt.Println()
	fmt.Print("  Enter choice [1-4]: ")

	input, err := g.reader.ReadString('\n')
	if err != nil {
		fmt.Printf("  Error reading input: %v\n", err)
		return false, true
	}

	input = strings.TrimSpace(input)

	switch input {
	case "1", "y", "yes":
		fmt.Println("  -> Approved for this step")
		return true, false

	case "2", "a", "all":
		fmt.Println("  -> Approved for all sudo steps in this run")
		g.approveAll = true
		return true, false

	case "3", "m", "manual":
		fmt.Println()
		fmt.Println("  Manual execution instructions:")
		fmt.Println("  ─────────────────────────────────")
		fmt.Printf("  Run this command manually:\n")
		fmt.Printf("    %s\n", step.Cmd)
		fmt.Println()
		return false, false

	case "4", "n", "no", "abort", "q", "quit":
		fmt.Println("  -> Aborted by user")
		return false, true

	default:
		fmt.Println("  -> Invalid choice, skipping step")
		return false, false
	}
}

// FormatSudoWarning formats a warning message for sudo steps
func FormatSudoWarning(step *llm.Step) string {
	var sb strings.Builder
	sb.WriteString("This step requires sudo:\n")
	sb.WriteString(fmt.Sprintf("  Command: %s\n", step.Cmd))
	if step.Description != "" {
		sb.WriteString(fmt.Sprintf("  Purpose: %s\n", step.Description))
	}
	return sb.String()
}

// CountSudoSteps counts the number of steps requiring sudo
func CountSudoSteps(plan *llm.RunPlan) int {
	count := 0
	for _, step := range plan.Steps {
		if step.RequiresSudo {
			count++
		}
	}
	return count
}

// GetSudoSteps returns all steps that require sudo
func GetSudoSteps(plan *llm.RunPlan) []llm.Step {
	var sudoSteps []llm.Step
	for _, step := range plan.Steps {
		if step.RequiresSudo {
			sudoSteps = append(sudoSteps, step)
		}
	}
	return sudoSteps
}

// WarnAboutSudo prints a summary warning about sudo steps
func WarnAboutSudo(plan *llm.RunPlan) {
	sudoCount := CountSudoSteps(plan)
	if sudoCount == 0 {
		return
	}

	fmt.Println()
	fmt.Printf("  ⚠ This plan contains %d step(s) requiring sudo:\n", sudoCount)
	for _, step := range GetSudoSteps(plan) {
		fmt.Printf("    • %s: %s\n", step.ID, truncateCommand(step.Cmd, 50))
	}
	fmt.Println()
	fmt.Println("  You will be prompted before each sudo command.")
	fmt.Println("  Use --allow-sudo to skip these prompts.")
}

func truncateCommand(cmd string, maxLen int) string {
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen-3] + "..."
}
