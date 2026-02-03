// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Plan validation with security policy integration

package plan

import (
	"fmt"
	"strings"

	"github.com/sony-level/readme-runner/internal/llm"
	"github.com/sony-level/readme-runner/internal/security"
)

// Validator validates and enhances RunPlan security
type Validator struct {
	policyChecker *security.PolicyChecker
}

// NewValidator creates a new plan validator
func NewValidator() *Validator {
	return &Validator{
		policyChecker: security.NewPolicyChecker(nil),
	}
}

// NewValidatorWithPolicy creates a validator with custom policy
func NewValidatorWithPolicy(policy *security.PolicyConfig) *Validator {
	return &Validator{
		policyChecker: security.NewPolicyChecker(policy),
	}
}

// ValidationResult contains the results of plan validation
type ValidationResult struct {
	Valid      bool
	Errors     []string
	Warnings   []string
	RiskReport RiskReport
}

// RiskReport summarizes risk levels in the plan
type RiskReport struct {
	Low      int
	Medium   int
	High     int
	Critical int
	HasSudo  bool
}

// Validate checks a RunPlan for validity and security issues
func (v *Validator) Validate(plan *llm.RunPlan) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Schema validation
	if err := plan.Validate(); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
	}

	// Security policy validation
	secResult := v.policyChecker.ValidatePlan(plan)
	result.Errors = append(result.Errors, secResult.Errors...)
	result.Warnings = append(result.Warnings, secResult.Warnings...)

	if len(secResult.Errors) > 0 {
		result.Valid = false
	}

	// Build risk report
	result.RiskReport = RiskReport{
		Low:      secResult.RiskSummary[llm.RiskLow],
		Medium:   secResult.RiskSummary[llm.RiskMedium],
		High:     secResult.RiskSummary[llm.RiskHigh],
		Critical: secResult.RiskSummary[llm.RiskCritical],
		HasSudo:  plan.HasSudoSteps(),
	}

	// Additional validation rules
	v.validateStepIDs(plan, result)
	v.validatePaths(plan, result)
	v.validateEnvVars(plan, result)

	return result
}

// validateStepIDs ensures step IDs are unique
func (v *Validator) validateStepIDs(plan *llm.RunPlan, result *ValidationResult) {
	seen := make(map[string]bool)
	for _, step := range plan.Steps {
		if seen[step.ID] {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Duplicate step ID: %s", step.ID))
		}
		seen[step.ID] = true
	}
}

// validatePaths checks for path traversal attempts
func (v *Validator) validatePaths(plan *llm.RunPlan, result *ValidationResult) {
	for _, step := range plan.Steps {
		// Check cwd for path traversal
		if strings.Contains(step.Cwd, "..") {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Step %s: cwd contains path traversal (..)", step.ID))
		}

		// Check command for absolute paths outside workspace
		if strings.Contains(step.Cmd, "/home/") ||
			strings.Contains(step.Cmd, "/root/") ||
			strings.Contains(step.Cmd, "/tmp/") {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Step %s: command references absolute paths", step.ID))
		}
	}
}

// validateEnvVars checks for potentially sensitive environment variables
func (v *Validator) validateEnvVars(plan *llm.RunPlan, result *ValidationResult) {
	sensitivePatterns := []string{
		"password", "secret", "token", "key", "api_key",
		"apikey", "private", "credential", "auth",
	}

	for key, value := range plan.Env {
		lowerKey := strings.ToLower(key)
		for _, pattern := range sensitivePatterns {
			if strings.Contains(lowerKey, pattern) {
				// Check if value looks like a placeholder vs actual secret
				if !isPlaceholder(value) {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("Environment variable '%s' may contain sensitive data", key))
				}
				break
			}
		}
	}
}

// isPlaceholder checks if a value looks like a placeholder
func isPlaceholder(value string) bool {
	placeholderPatterns := []string{
		"${", "$(",       // Variable expansion
		"<",             // HTML-style placeholder
		"your_", "YOUR_", // Common placeholder prefix
		"xxx", "XXX",     // Redacted pattern
		"changeme",       // Common placeholder
		"example",        // Example value
	}

	lower := strings.ToLower(value)
	for _, pattern := range placeholderPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}

	return value == "" || value == "***" || value == "..."
}

// EnhancePlan updates the plan with corrected risk levels and sudo flags
func (v *Validator) EnhancePlan(plan *llm.RunPlan) *llm.RunPlan {
	enhanced := *plan // Copy the plan
	enhanced.Steps = make([]llm.Step, len(plan.Steps))

	for i, step := range plan.Steps {
		enhanced.Steps[i] = step

		// Analyze command for accurate risk assessment
		analysis := v.policyChecker.AnalyzeCommand(step.Cmd)

		// Update risk if analysis shows higher risk
		if riskLevel(analysis.Risk) > riskLevel(step.Risk) {
			enhanced.Steps[i].Risk = analysis.Risk
		}

		// Update requires_sudo if detected
		if analysis.RequiresSudo && !step.RequiresSudo {
			enhanced.Steps[i].RequiresSudo = true
		}
	}

	return &enhanced
}

// riskLevel converts a RiskLevel to an integer for comparison
func riskLevel(r llm.RiskLevel) int {
	switch r {
	case llm.RiskLow:
		return 1
	case llm.RiskMedium:
		return 2
	case llm.RiskHigh:
		return 3
	case llm.RiskCritical:
		return 4
	default:
		return 0
	}
}

// FormatValidationResult returns a human-readable validation result
func FormatValidationResult(result *ValidationResult) string {
	var sb strings.Builder

	if result.Valid {
		sb.WriteString("✓ Plan is valid\n")
	} else {
		sb.WriteString("✗ Plan has validation errors\n")
	}

	if len(result.Errors) > 0 {
		sb.WriteString("\nErrors:\n")
		for _, err := range result.Errors {
			sb.WriteString(fmt.Sprintf("  • %s\n", err))
		}
	}

	if len(result.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, warn := range result.Warnings {
			sb.WriteString(fmt.Sprintf("  • %s\n", warn))
		}
	}

	// Risk summary
	sb.WriteString("\nRisk Summary:\n")
	sb.WriteString(fmt.Sprintf("  Low: %d, Medium: %d, High: %d, Critical: %d\n",
		result.RiskReport.Low,
		result.RiskReport.Medium,
		result.RiskReport.High,
		result.RiskReport.Critical))

	if result.RiskReport.HasSudo {
		sb.WriteString("  ⚠ Plan contains sudo commands\n")
	}

	return sb.String()
}
