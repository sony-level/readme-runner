// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Prerequisite checker for tool existence and versions

package prereq

import (
	"os/exec"
	"strings"

	"github.com/sony-level/readme-runner/internal/llm"
)

// Checker verifies tool existence
type Checker struct {
	tools map[string]*Tool
}

// NewChecker creates a new prerequisite checker
func NewChecker() *Checker {
	return &Checker{
		tools: DefaultTools(),
	}
}

// NewCheckerWithTools creates a checker with custom tools
func NewCheckerWithTools(tools map[string]*Tool) *Checker {
	return &Checker{
		tools: tools,
	}
}

// CheckPrerequisites verifies all required tools from a plan
func (c *Checker) CheckPrerequisites(prereqs []llm.Prerequisite) *CheckSummary {
	summary := NewCheckSummary()

	for _, prereq := range prereqs {
		result := c.CheckTool(prereq.Name)
		summary.AddResult(result)
	}

	return summary
}

// CheckTool checks if a specific tool exists
func (c *Checker) CheckTool(name string) CheckResult {
	result := CheckResult{Name: name}

	// Look up tool definition
	tool, ok := c.tools[strings.ToLower(name)]
	if !ok {
		// Unknown tool - try direct command check
		result.Found = c.commandExists(name)
		if result.Found {
			result.Path = c.whichCommand(name)
		}
		return result
	}

	// Check primary command
	if c.commandExists(tool.Command) {
		result.Found = true
		result.Path = c.whichCommand(tool.Command)
		result.Version = c.getVersion(tool.VersionCmd)
		return result
	}

	// Check alternatives
	for _, alt := range tool.Alternatives {
		// Handle multi-word alternatives like "docker compose"
		parts := strings.Fields(alt)
		if len(parts) > 0 && c.commandExists(parts[0]) {
			// For "docker compose", check if subcommand works
			if len(parts) > 1 {
				if c.subcommandWorks(parts[0], parts[1:]) {
					result.Found = true
					result.Path = c.whichCommand(parts[0])
					return result
				}
			} else {
				result.Found = true
				result.Path = c.whichCommand(alt)
				return result
			}
		}
	}

	return result
}

// GetTool returns a tool definition by name
func (c *Checker) GetTool(name string) *Tool {
	return c.tools[strings.ToLower(name)]
}

// GetInstallGuide returns installation instructions for a tool
func (c *Checker) GetInstallGuide(name string) string {
	tool := c.GetTool(name)
	if tool == nil {
		return "No installation guide available for " + name
	}
	return tool.InstallGuide
}

// GetAllTools returns all known tools
func (c *Checker) GetAllTools() map[string]*Tool {
	return c.tools
}

// commandExists checks if a command exists in PATH
func (c *Checker) commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// whichCommand returns the full path to a command
func (c *Checker) whichCommand(cmd string) string {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return ""
	}
	return path
}

// getVersion executes a version command and returns the output
func (c *Checker) getVersion(versionCmd string) string {
	if versionCmd == "" {
		return ""
	}

	parts := strings.Fields(versionCmd)
	if len(parts) == 0 {
		return ""
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Return first line of output, trimmed
	output := strings.TrimSpace(string(out))
	if idx := strings.Index(output, "\n"); idx > 0 {
		output = output[:idx]
	}

	return output
}

// subcommandWorks checks if a subcommand works (e.g., "docker compose")
func (c *Checker) subcommandWorks(mainCmd string, args []string) bool {
	// Add --version or --help to check if subcommand exists
	testArgs := append(args, "--version")
	cmd := exec.Command(mainCmd, testArgs...)
	err := cmd.Run()
	if err == nil {
		return true
	}

	// Try with --help
	testArgs = append(args, "--help")
	cmd = exec.Command(mainCmd, testArgs...)
	err = cmd.Run()
	return err == nil
}

// CheckMultiple checks multiple tools and returns a summary
func (c *Checker) CheckMultiple(names []string) *CheckSummary {
	summary := NewCheckSummary()

	for _, name := range names {
		result := c.CheckTool(name)
		summary.AddResult(result)
	}

	return summary
}

// FormatMissing returns a formatted string of missing tools with install guides
func (c *Checker) FormatMissing(summary *CheckSummary) string {
	if summary.AllFound {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Missing prerequisites:\n\n")

	for _, name := range summary.MissingTools {
		sb.WriteString("─────────────────────────────────\n")
		sb.WriteString(name + "\n")
		sb.WriteString("─────────────────────────────────\n")
		sb.WriteString(c.GetInstallGuide(name))
		sb.WriteString("\n\n")
	}

	return sb.String()
}
