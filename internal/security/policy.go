// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Security policy checker and command analysis

package security

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sony-level/readme-runner/internal/llm"
)

// PolicyChecker validates plans against security policy
type PolicyChecker struct {
	config *PolicyConfig
}

// NewPolicyChecker creates a new policy checker
func NewPolicyChecker(config *PolicyConfig) *PolicyChecker {
	if config == nil {
		config = DefaultPolicy()
	}
	return &PolicyChecker{config: config}
}

// ValidatePlan checks a RunPlan against security policy
func (c *PolicyChecker) ValidatePlan(plan *llm.RunPlan) *ValidationResult {
	result := NewValidationResult()

	// Validate version
	if plan.Version != llm.ValidPlanVersion {
		result.AddError(fmt.Sprintf("Invalid plan version: %s (expected %s)", plan.Version, llm.ValidPlanVersion))
	}

	// Validate project type
	validType := false
	for _, t := range llm.ValidProjectTypes {
		if plan.ProjectType == t {
			validType = true
			break
		}
	}
	if !validType {
		result.AddError(fmt.Sprintf("Invalid project type: %s", plan.ProjectType))
	}

	// Validate steps
	for i, step := range plan.Steps {
		stepNum := i + 1

		// Check for empty command
		if step.Cmd == "" {
			result.AddError(fmt.Sprintf("Step %d (%s): empty command", stepNum, step.ID))
			continue
		}

		// Analyze command
		analysis := c.AnalyzeCommand(step.Cmd)
		result.RiskSummary[analysis.Risk]++

		// Check if blocked
		if analysis.IsBlocked {
			result.AddError(fmt.Sprintf("Step %d (%s) blocked: %s", stepNum, step.ID, analysis.BlockReason))
		}

		// Add warnings
		for _, w := range analysis.Warnings {
			result.AddWarning(fmt.Sprintf("Step %d (%s): %s", stepNum, step.ID, w))
		}

		// Check sudo consistency
		if analysis.RequiresSudo && !step.RequiresSudo {
			result.AddWarning(fmt.Sprintf("Step %d (%s): command uses sudo but requires_sudo is false", stepNum, step.ID))
		}

		// Check risk consistency
		if analysis.Risk != step.Risk {
			result.AddWarning(fmt.Sprintf("Step %d (%s): detected risk '%s' differs from declared '%s'",
				stepNum, step.ID, analysis.Risk, step.Risk))
		}
	}

	// Check for env secrets (basic heuristic)
	for key := range plan.Env {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "password") ||
			strings.Contains(lowerKey, "secret") ||
			strings.Contains(lowerKey, "token") ||
			strings.Contains(lowerKey, "api_key") ||
			strings.Contains(lowerKey, "apikey") {
			result.AddWarning(fmt.Sprintf("Environment variable '%s' may contain sensitive data", key))
		}
	}

	return result
}

// AnalyzeCommand performs security analysis on a single command
func (c *PolicyChecker) AnalyzeCommand(cmd string) *CommandAnalysis {
	analysis := NewCommandAnalysis(cmd)
	cmd = strings.TrimSpace(cmd)
	lowerCmd := strings.ToLower(cmd)

	// Check blocklist
	for _, blocked := range c.config.BlockedCommands {
		if strings.Contains(lowerCmd, strings.ToLower(blocked)) {
			analysis.IsBlocked = true
			analysis.BlockReason = fmt.Sprintf("matches blocked pattern: %s", blocked)
			analysis.Risk = llm.RiskCritical
			return analysis
		}
	}

	// Detect sudo
	if c.detectsSudo(cmd) {
		analysis.RequiresSudo = true
		analysis.Risk = llm.RiskCritical
		analysis.Warnings = append(analysis.Warnings, "command requires sudo privileges")
	}

	// Detect package managers (high risk)
	if c.detectsPackageManager(cmd) {
		if analysis.Risk < llm.RiskHigh {
			analysis.Risk = llm.RiskHigh
		}
		analysis.Warnings = append(analysis.Warnings, "system package manager command")
	}

	// Detect remote scripts (critical risk)
	if c.detectsRemoteScript(cmd) {
		analysis.Risk = llm.RiskCritical
		analysis.Warnings = append(analysis.Warnings, "remote script execution detected")
		if !c.isWhitelistedURL(cmd) {
			analysis.IsBlocked = true
			analysis.BlockReason = "remote script from non-whitelisted URL"
		}
	}

	// Detect file modifications (medium risk)
	if c.detectsFileModification(cmd) {
		if analysis.Risk < llm.RiskMedium {
			analysis.Risk = llm.RiskMedium
		}
	}

	// Detect system directory access
	if c.detectsSystemDirectoryAccess(cmd) {
		if analysis.Risk < llm.RiskHigh {
			analysis.Risk = llm.RiskHigh
		}
		analysis.Warnings = append(analysis.Warnings, "accesses system directories")
	}

	return analysis
}

// detectsSudo checks if command requires sudo
func (c *PolicyChecker) detectsSudo(cmd string) bool {
	// Direct sudo usage
	if strings.HasPrefix(cmd, "sudo ") || strings.Contains(cmd, " sudo ") {
		return true
	}

	// Piped sudo
	if strings.Contains(cmd, "| sudo") || strings.Contains(cmd, "|sudo") {
		return true
	}

	return false
}

// detectsPackageManager checks for system package manager commands
func (c *PolicyChecker) detectsPackageManager(cmd string) bool {
	pkgManagers := []string{
		"apt ", "apt-get ", "aptitude ",
		"yum ", "dnf ", "zypper ",
		"pacman ", "emerge ",
		"brew install", "brew upgrade",
		"port install", "port upgrade",
		"choco install", "choco upgrade",
		"winget install", "winget upgrade",
		"snap install", "snap refresh",
		"flatpak install", "flatpak update",
	}

	lowerCmd := strings.ToLower(cmd)
	for _, pm := range pkgManagers {
		if strings.Contains(lowerCmd, pm) {
			return true
		}
	}

	return false
}

// detectsRemoteScript checks for remote script execution patterns
func (c *PolicyChecker) detectsRemoteScript(cmd string) bool {
	patterns := []string{
		`curl.*\|.*sh`,
		`curl.*\|.*bash`,
		`wget.*\|.*sh`,
		`wget.*\|.*bash`,
		`bash.*<\(curl`,
		`bash.*<\(wget`,
		`sh.*<\(curl`,
		`sh.*<\(wget`,
	}

	for _, pattern := range patterns {
		matched, _ := regexp.MatchString(pattern, cmd)
		if matched {
			return true
		}
	}

	return false
}

// isWhitelistedURL checks if the command contains a whitelisted URL
func (c *PolicyChecker) isWhitelistedURL(cmd string) bool {
	for _, url := range c.config.WhitelistedURLs {
		if strings.Contains(cmd, url) {
			return true
		}
	}
	return false
}

// detectsFileModification checks for file modification patterns
func (c *PolicyChecker) detectsFileModification(cmd string) bool {
	patterns := []string{
		` > `,  // redirect output
		` >> `, // append output
		`rm `,  // remove files
		`mv `,  // move files
		`cp `,  // copy files
		`mkdir `,
		`rmdir `,
		`touch `,
	}

	for _, pattern := range patterns {
		if strings.Contains(cmd, pattern) {
			return true
		}
	}

	return false
}

// detectsSystemDirectoryAccess checks for system directory access
func (c *PolicyChecker) detectsSystemDirectoryAccess(cmd string) bool {
	systemDirs := []string{
		"/usr/",
		"/etc/",
		"/var/",
		"/opt/",
		"/bin/",
		"/sbin/",
		"/lib/",
		"/boot/",
		"/root/",
		"C:\\Windows",
		"C:\\Program Files",
		"C:\\ProgramData",
	}

	for _, dir := range systemDirs {
		if strings.Contains(cmd, dir) {
			return true
		}
	}

	return false
}
