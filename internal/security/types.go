// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Security types and configuration

package security

import "github.com/sony-level/readme-runner/internal/llm"

// PolicyConfig holds security policy settings
type PolicyConfig struct {
	AllowSudo       bool     // Allow sudo without extra confirmation
	BlockedCommands []string // Commands that are always blocked
	WhitelistedURLs []string // Allowed remote script URLs
	MaxStepTimeout  int      // Maximum timeout per step (seconds)
}

// DefaultPolicy returns the default security policy
func DefaultPolicy() *PolicyConfig {
	return &PolicyConfig{
		AllowSudo: false,
		BlockedCommands: []string{
			"rm -rf /",
			"rm -rf /*",
			"rm -rf ~",
			"rm -rf $HOME",
			"mkfs",
			"dd if=/dev/zero",
			"dd if=/dev/random",
			"shutdown",
			"reboot",
			"halt",
			"poweroff",
			"chmod -R 777 /",
			"chmod 777 /",
			"chown -R",
			"useradd",
			"userdel",
			"passwd",
			"visudo",
			":(){:|:&};:",     // fork bomb
			"> /dev/sda",      // disk wipe
			"mv /* /dev/null", // move everything to null
		},
		WhitelistedURLs: []string{
			// Official installers that are generally safe
			"https://sh.rustup.rs",
			"https://get.docker.com",
			"https://raw.githubusercontent.com/nvm-sh/nvm",
			"https://pyenv.run",
			"https://install.python-poetry.org",
		},
		MaxStepTimeout: 600, // 10 minutes
	}
}

// ValidationResult contains plan validation outcome
type ValidationResult struct {
	Valid       bool
	Errors      []string
	Warnings    []string
	RiskSummary map[llm.RiskLevel]int
}

// NewValidationResult creates a new validation result
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid:       true,
		Errors:      []string{},
		Warnings:    []string{},
		RiskSummary: make(map[llm.RiskLevel]int),
	}
}

// AddError adds an error and marks result as invalid
func (r *ValidationResult) AddError(msg string) {
	r.Valid = false
	r.Errors = append(r.Errors, msg)
}

// AddWarning adds a warning
func (r *ValidationResult) AddWarning(msg string) {
	r.Warnings = append(r.Warnings, msg)
}

// CommandAnalysis contains risk analysis for a single command
type CommandAnalysis struct {
	Command      string
	Risk         llm.RiskLevel
	RequiresSudo bool
	IsBlocked    bool
	BlockReason  string
	Warnings     []string
}

// NewCommandAnalysis creates a new command analysis
func NewCommandAnalysis(cmd string) *CommandAnalysis {
	return &CommandAnalysis{
		Command:  cmd,
		Risk:     llm.RiskLow,
		Warnings: []string{},
	}
}

// SudoApproval represents the result of a sudo approval request
type SudoApproval int

const (
	SudoApproved      SudoApproval = iota // Approved for this step
	SudoApprovedAll                       // Approved for all steps
	SudoShowManual                        // Show manual instructions only
	SudoDenied                            // User denied
	SudoAborted                           // User aborted entire operation
)
