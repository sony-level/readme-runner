// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Prompt building and README clarity scoring

package llm

import (
	"fmt"
	"strings"

	"github.com/sony-level/readme-runner/internal/scanner"
)

// ClarityThreshold is the minimum score for README-first approach
const ClarityThreshold = 0.6

// PromptBuilder constructs LLM prompts
type PromptBuilder struct{}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// CalculateClarityScore computes README clarity (0.0-1.0)
func CalculateClarityScore(readme *scanner.ReadmeInfo) float64 {
	if readme == nil {
		return 0.0
	}

	score := 0.0
	maxScore := 5.0

	// HasInstall section (+1)
	if readme.HasInstall {
		score += 1.0
	}

	// HasUsage section (+1)
	if readme.HasUsage {
		score += 1.0
	}

	// HasQuickStart (+0.5)
	if readme.HasQuickStart {
		score += 0.5
	}

	// HasBuild (+0.5)
	if readme.HasBuild {
		score += 0.5
	}

	// CodeBlocks (up to +1 for 3+ blocks)
	if readme.CodeBlocks >= 3 {
		score += 1.0
	} else if readme.CodeBlocks >= 1 {
		score += 0.5
	}

	// ShellCommands (up to +1 for 2+ shell blocks)
	if readme.ShellCommands >= 2 {
		score += 1.0
	} else if readme.ShellCommands >= 1 {
		score += 0.5
	}

	return score / maxScore
}

// ShouldUseReadme returns true if README should be the primary source
func ShouldUseReadme(readme *scanner.ReadmeInfo) bool {
	return CalculateClarityScore(readme) >= ClarityThreshold
}

// BuildPlanPrompt creates the prompt for plan generation
func (b *PromptBuilder) BuildPlanPrompt(ctx *PlanContext) string {
	var sb strings.Builder

	// System instruction
	sb.WriteString(b.getSystemInstruction())

	// README-First or Profile-based approach
	if ctx.UseReadme && ctx.ReadmeInfo != nil && ctx.ReadmeInfo.Content != "" {
		sb.WriteString("## README Content (PRIMARY SOURCE)\n")
		sb.WriteString("The README is clear and should be your primary guide for installation steps.\n\n")
		sb.WriteString("```markdown\n")
		sb.WriteString(b.truncateContent(ctx.ReadmeInfo.Content, 8000))
		sb.WriteString("\n```\n\n")

		// Include sections info
		if len(ctx.ReadmeInfo.Sections) > 0 {
			sb.WriteString("### Detected Sections\n")
			for _, section := range ctx.ReadmeInfo.Sections {
				sb.WriteString(fmt.Sprintf("- %s\n", section))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("## README Status\n")
		if ctx.ReadmeInfo == nil {
			sb.WriteString("No README found. Use project files to determine setup steps.\n\n")
		} else {
			sb.WriteString("README is present but unclear or incomplete. Use project files as primary guide.\n\n")
		}
	}

	// Always include profile signals
	if ctx.Profile != nil {
		sb.WriteString("## Project Profile (File Signals)\n")
		sb.WriteString(b.formatProfile(ctx.Profile))
		sb.WriteString("\n")
	}

	// Target OS
	if ctx.OS != "" {
		sb.WriteString(fmt.Sprintf("## Target OS: %s\n\n", ctx.OS))
	}

	// Output schema
	sb.WriteString(b.getOutputSchema())

	return sb.String()
}

func (b *PromptBuilder) getSystemInstruction() string {
	return `You are an expert at analyzing software projects and generating installation/run plans.

IMPORTANT RULES:
1. Respond with valid JSON only. No text before or after the JSON.
2. Follow the README instructions when clear.
3. If README is unclear, use file signals to determine the best approach.
4. Prefer Docker/docker-compose if Dockerfile or compose files exist.
5. Use locked dependencies (npm ci, poetry install, etc.) when lock files exist.
6. Mark requires_sudo: true for any command that needs root privileges.
7. Never include secrets or tokens in the plan.

`
}

func (b *PromptBuilder) getOutputSchema() string {
	return `## Required Output Format
Return ONLY valid JSON matching this exact schema:
{
  "version": "1",
  "project_type": "docker|node|python|go|rust|mixed",
  "prerequisites": [
    {"name": "tool_name", "reason": "why needed", "min_version": "optional"}
  ],
  "steps": [
    {
      "id": "unique_step_id",
      "cmd": "command to run",
      "cwd": ".",
      "risk": "low|medium|high|critical",
      "requires_sudo": false
    }
  ],
  "env": {},
  "ports": [],
  "notes": ["any important notes"]
}

RISK LEVELS:
- low: Safe read-only or local operations
- medium: Modifies local files (npm install, pip install --user)
- high: Installs system packages or modifies system state
- critical: Requires sudo or affects system configuration

STEP ID CONVENTIONS:
- "install" for dependency installation
- "build" for compilation/build steps
- "run" for starting the application
- "test" for running tests
- "setup" for one-time configuration
`
}

func (b *PromptBuilder) truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	// Find a good break point (newline)
	truncated := content[:maxLen]
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > maxLen/2 {
		truncated = truncated[:lastNewline]
	}

	return truncated + "\n\n... (content truncated for brevity)"
}

func (b *PromptBuilder) formatProfile(p *scanner.ProjectProfile) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("- **Primary Stack**: %s\n", p.Stack))

	if len(p.Languages) > 0 {
		sb.WriteString(fmt.Sprintf("- **Languages**: %s\n", strings.Join(p.Languages, ", ")))
	}

	if len(p.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("- **Tools Detected**: %s\n", strings.Join(p.Tools, ", ")))
	}

	if len(p.Containers) > 0 {
		sb.WriteString(fmt.Sprintf("- **Container Files**: %s\n", strings.Join(p.Containers, ", ")))
	}

	if len(p.Packages) > 0 {
		sb.WriteString(fmt.Sprintf("- **Package Files**: %s\n", strings.Join(p.Packages, ", ")))
	}

	if len(p.Signals) > 0 {
		// Show first 10 signals
		signals := p.Signals
		if len(signals) > 10 {
			signals = signals[:10]
			sb.WriteString(fmt.Sprintf("- **Key Files**: %s, ... and %d more\n",
				strings.Join(signals, ", "), len(p.Signals)-10))
		} else {
			sb.WriteString(fmt.Sprintf("- **Key Files**: %s\n", strings.Join(signals, ", ")))
		}
	}

	return sb.String()
}

// BuildRepairPrompt creates a prompt for fixing a failed plan
func (b *PromptBuilder) BuildRepairPrompt(plan *RunPlan, failedStep *Step, errorOutput string) string {
	var sb strings.Builder

	sb.WriteString("A plan execution failed. Please provide a fixed plan.\n\n")

	sb.WriteString("## Failed Step\n")
	sb.WriteString(fmt.Sprintf("- ID: %s\n", failedStep.ID))
	sb.WriteString(fmt.Sprintf("- Command: %s\n", failedStep.Cmd))
	sb.WriteString(fmt.Sprintf("- Working Dir: %s\n", failedStep.Cwd))
	sb.WriteString("\n")

	sb.WriteString("## Error Output (last 500 chars)\n")
	sb.WriteString("```\n")
	if len(errorOutput) > 500 {
		sb.WriteString(errorOutput[len(errorOutput)-500:])
	} else {
		sb.WriteString(errorOutput)
	}
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Original Plan\n")
	sb.WriteString(fmt.Sprintf("Project Type: %s\n", plan.ProjectType))
	sb.WriteString("Steps:\n")
	for _, step := range plan.Steps {
		marker := "  "
		if step.ID == failedStep.ID {
			marker = "* "
		}
		sb.WriteString(fmt.Sprintf("%s%s: %s\n", marker, step.ID, step.Cmd))
	}
	sb.WriteString("\n")

	sb.WriteString(b.getOutputSchema())

	return sb.String()
}
