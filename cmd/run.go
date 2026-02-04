/*
Copyright © 2026 ソニーレベル <c7kali3@gmail.com>

*/
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/sony-level/readme-runner/internal/exec"
	"github.com/sony-level/readme-runner/internal/fetcher"
	"github.com/sony-level/readme-runner/internal/llm"
	llmprovider "github.com/sony-level/readme-runner/internal/llm/provider"
	"github.com/sony-level/readme-runner/internal/plan"
	"github.com/sony-level/readme-runner/internal/prereq"
	"github.com/sony-level/readme-runner/internal/scanner"
	"github.com/sony-level/readme-runner/internal/security"
	"github.com/sony-level/readme-runner/internal/stacks"
	"github.com/sony-level/readme-runner/internal/workspace"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [path|url]",
	Short: "Run installation from README",
	Long: `Analyze a repository's README and key files, generate an installation
plan, and execute it (or simulate with --dry-run).

Arguments:
  path    Local directory path to analyze
  url     GitHub/GitLab repository URL to clone and analyze

Examples:
  rd-run run .
  rd-run run https://github.com/user/repo
  rd-run run https://gitlab.com/user/repo
  rd-run run . --dry-run --verbose
  rd-run run . --keep`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine input path (default to current directory)
		inputPath := "."
		if len(args) > 0 {
			inputPath = args[0]
		}

		return executeRun(inputPath)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func executeRun(inputPath string) error {
	// Get current working directory for workspace base
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create workspace configuration
	wsConfig := &workspace.WorkspaceConfig{
		BaseDir: cwd,
		Keep:    keepWorkspace,
	}

	// Create new workspace
	ws, err := workspace.New(wsConfig)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Ensure cleanup happens at the end
	defer func() {
		if cleanupErr := ws.Cleanup(); cleanupErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: cleanup failed: %v\n", cleanupErr)
		}
	}()

	// Display workspace info
	if verbose {
		fmt.Printf("Workspace created:\n")
		fmt.Printf("  Run ID:    %s\n", ws.RunID)
		fmt.Printf("  Path:      %s\n", ws.Path)
		fmt.Printf("  Repo:      %s\n", ws.RepoPath())
		fmt.Printf("  Plan:      %s\n", ws.PlanPath())
		fmt.Printf("  Logs:      %s\n", ws.LogsPath())
		fmt.Printf("  Keep:      %v\n", ws.ShouldKeep())
		fmt.Println()
	}

	fmt.Printf("Run ID: %s\n", ws.RunID)
	fmt.Printf("Input: %s\n", inputPath)

	// Detect source type for display
	sourceType := fetcher.DetectSourceType(inputPath)
	fmt.Printf("Source type: %s\n", sourceType)

	if dryRun {
		fmt.Println("\n[DRY-RUN MODE] No commands will be executed.")
	}

	// Phase 1: Fetch / Workspace
	fmt.Println("\n[1/7] Fetch / Workspace")
	fmt.Printf("  → Workspace ready at %s\n", ws.Path)

	// Configure fetcher
	fetchConfig := &fetcher.FetchConfig{
		Source:       inputPath,
		Destination:  ws.RepoPath(),
		Verbose:      verbose,
		Progress:     os.Stdout,
		ShallowClone: true, // Use shallow clone for efficiency
	}

	// Fetch the project
	fmt.Printf("  → Fetching project...\n")
	fetchResult, err := fetcher.Fetch(fetchConfig)
	if err != nil {
		return fmt.Errorf("failed to fetch project: %w", err)
	}

	fmt.Printf("  → Fetched %d files (%d bytes) to %s\n",
		fetchResult.FilesCopied, fetchResult.BytesCopied, fetchResult.Destination)
	if fetchResult.IsGitRepo {
		fmt.Printf("  → Source is a git repository\n")
	}

	// Phase 2: Scan
	fmt.Println("\n[2/7] Scan")
	fmt.Printf("  → Scanning workspace for project files...\n")

	scanConfig := &scanner.ScanConfig{
		RootPath: ws.RepoPath(),
		MaxDepth: 3,
		Verbose:  verbose,
	}

	scanResult, err := scanner.Scan(scanConfig)
	if err != nil {
		return fmt.Errorf("failed to scan workspace: %w", err)
	}

	fmt.Printf("  → Scanned %d files in %d directories (%v)\n",
		scanResult.TotalFiles, scanResult.TotalDirs, scanResult.ScanDuration)

	// Display README info
	if scanResult.ReadmeFile != nil {
		fmt.Printf("  → README found: %s (%d bytes)\n",
			scanResult.ReadmeFile.RelPath, scanResult.ReadmeFile.Size)

		// Show README preview in verbose mode
		if verbose && scanResult.ReadmeFile.Content != "" {
			lines := strings.Split(scanResult.ReadmeFile.Content, "\n")
			fmt.Printf("    Preview:\n")
			previewLines := 0
			for _, line := range lines {
				if previewLines >= 5 { // Show first 5 non-empty lines
					fmt.Printf("      ...\n")
					break
				}
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					// Truncate long lines
					if len(trimmed) > 60 {
						trimmed = trimmed[:57] + "..."
					}
					fmt.Printf("      %s\n", trimmed)
					previewLines++
				}
			}

			// Show truncation warning
			if scanResult.ReadmeFile.Truncated {
				fmt.Printf("    (Content truncated: was %d bytes)\n",
					scanResult.ReadmeFile.OriginalSize)
			}
		}

		if verbose {
			fmt.Printf("    Sections: %d\n", len(scanResult.ReadmeFile.Sections))
			fmt.Printf("    Code blocks: %d\n", scanResult.ReadmeFile.CodeBlocks)
			fmt.Printf("    Shell commands: %d\n", scanResult.ReadmeFile.ShellCommands)
			if scanResult.ReadmeFile.HasInstall {
				fmt.Printf("    ✓ Has installation section\n")
			}
			if scanResult.ReadmeFile.HasUsage {
				fmt.Printf("    ✓ Has usage section\n")
			}
			if scanResult.ReadmeFile.HasBuild {
				fmt.Printf("    ✓ Has build section\n")
			}
			if scanResult.ReadmeFile.HasQuickStart {
				fmt.Printf("    ✓ Has quick start section\n")
			}
		}
	} else {
		fmt.Printf("  → ⚠ No README found\n")
	}

	// Display detected stacks (legacy method)
	detectedStacks := scanResult.DetectedStacks()
	if len(detectedStacks) > 0 {
		fmt.Printf("  → Primary stack: %s\n", scanResult.PrimaryStack())
		fmt.Printf("  → All stacks: %s\n", strings.Join(detectedStacks, ", "))
	}

	// Display ProjectProfile in verbose mode
	if verbose && scanResult.Profile != nil {
		profile := scanResult.Profile

		fmt.Printf("  → Project Profile:\n")
		fmt.Printf("    Root: %s\n", profile.Root)
		fmt.Printf("    Primary stack: %s\n", profile.Stack)

		if len(profile.Languages) > 0 {
			fmt.Printf("    Languages: %s\n", strings.Join(profile.Languages, ", "))
		}

		if len(profile.Tools) > 0 {
			fmt.Printf("    Tools: %s\n", strings.Join(profile.Tools, ", "))
		}

		if len(profile.Containers) > 0 {
			fmt.Printf("    Containers: %s\n", strings.Join(profile.Containers, ", "))
		}

		if len(profile.Packages) > 0 {
			fmt.Printf("    Package files: %s\n", strings.Join(profile.Packages, ", "))
		}

		if len(profile.Signals) > 0 {
			maxSignals := 5
			if len(profile.Signals) <= maxSignals {
				fmt.Printf("    Key signals: %s\n", strings.Join(profile.Signals, ", "))
			} else {
				fmt.Printf("    Key signals: %s\n", strings.Join(profile.Signals[:maxSignals], ", "))
				fmt.Printf("      ... and %d more\n", len(profile.Signals)-maxSignals)
			}
		}
	}

	// Run stack detection
	var stackDetection *stacks.DetectionResult
	if scanResult.Profile != nil {
		aggregator := stacks.NewAggregator()
		detection := aggregator.Detect(scanResult.Profile)
		stackDetection = &detection

		fmt.Printf("  → Stack Detection:\n")
		fmt.Printf("    Dominant: %s (confidence: %.2f)\n",
			detection.Dominant.Name, detection.Dominant.Confidence)

		if detection.IsMixed {
			fmt.Printf("    Type: Mixed project\n")
		}

		if verbose {
			fmt.Printf("    Explanation: %s\n", detection.Explanation)

			if len(detection.Matches) > 1 {
				fmt.Printf("    All detected stacks:\n")
				for _, match := range detection.Matches {
					fmt.Printf("      • %s (confidence: %.2f, priority: %d)\n",
						match.Name, match.Confidence, match.Priority)
					for _, reason := range match.Reasons {
						fmt.Printf("        - %s\n", reason)
					}
				}
			} else if len(detection.Matches) == 1 {
				fmt.Printf("    Reasons:\n")
				for _, reason := range detection.Dominant.Reasons {
					fmt.Printf("      - %s\n", reason)
				}
			}
		}
	}

	// Suppress unused variable warning
	_ = stackDetection

	// Display project files in verbose mode
	if verbose && len(scanResult.ProjectFiles) > 0 {
		fmt.Printf("  → Project files:\n")
		for fileType, paths := range scanResult.ProjectFiles {
			fmt.Printf("    %s: %s\n", fileType, strings.Join(paths, ", "))
		}
	}

	// Phase 3: Plan (AI)
	fmt.Println("\n[3/7] Plan (AI)")

	// Build LLM context
	clarityScore := llm.CalculateClarityScore(scanResult.ReadmeFile)
	useReadme := llm.ShouldUseReadme(scanResult.ReadmeFile)

	planCtx := &llm.PlanContext{
		ReadmeInfo:   scanResult.ReadmeFile,
		Profile:      scanResult.Profile,
		ClarityScore: clarityScore,
		UseReadme:    useReadme,
		OS:           runtime.GOOS,
		Verbose:      verbose,
	}

	if verbose {
		fmt.Printf("  → README clarity score: %.2f\n", clarityScore)
		if useReadme {
			fmt.Printf("  → Using README as primary source\n")
		} else {
			fmt.Printf("  → Using project files as primary source (README unclear or missing)\n")
		}
	}

	// Create LLM provider
	provider, err := createLLMProvider()
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}
	fmt.Printf("  → Using LLM provider: %s\n", provider.Name())

	// Generate plan
	fmt.Printf("  → Generating installation plan...\n")
	runPlan, err := provider.GeneratePlan(planCtx)
	if err != nil {
		// Fallback to mock provider on failure
		fmt.Printf("  → ⚠ LLM failed: %v\n", err)
		fmt.Printf("  → Falling back to mock provider...\n")
		mockProvider := llmprovider.NewMockProvider()
		runPlan, err = mockProvider.GeneratePlan(planCtx)
		if err != nil {
			return fmt.Errorf("failed to generate plan: %w", err)
		}
	}

	fmt.Printf("  → Plan generated: %s project with %d steps\n",
		runPlan.ProjectType, len(runPlan.Steps))

	// Phase 4: Validate / Normalize
	fmt.Println("\n[4/7] Validate / Normalize")

	// Validate plan
	validator := plan.NewValidator()
	validationResult := validator.Validate(runPlan)

	if !validationResult.Valid {
		fmt.Println("  → ✗ Plan validation failed:")
		for _, err := range validationResult.Errors {
			fmt.Printf("      • %s\n", err)
		}
		return fmt.Errorf("plan validation failed")
	}

	fmt.Println("  → ✓ Plan is valid")

	if len(validationResult.Warnings) > 0 && verbose {
		fmt.Println("  → Warnings:")
		for _, warn := range validationResult.Warnings {
			fmt.Printf("      • %s\n", warn)
		}
	}

	// Normalize plan
	normalizer := plan.NewNormalizer(scanResult.Profile)
	runPlan = normalizer.Normalize(runPlan)

	// Enhance plan with accurate risk levels
	runPlan = validator.EnhancePlan(runPlan)

	fmt.Printf("  → Plan normalized for %s\n", runtime.GOOS)

	// Show risk summary
	fmt.Printf("  → Risk summary: Low=%d, Medium=%d, High=%d, Critical=%d\n",
		validationResult.RiskReport.Low,
		validationResult.RiskReport.Medium,
		validationResult.RiskReport.High,
		validationResult.RiskReport.Critical)

	if runPlan.HasSudoSteps() {
		sudoCount := security.CountSudoSteps(runPlan)
		fmt.Printf("  → ⚠ Plan contains %d step(s) requiring sudo\n", sudoCount)
	}

	// Phase 5: Prerequisites
	fmt.Println("\n[5/7] Prerequisites")

	checker := prereq.NewChecker()
	checkSummary := checker.CheckPrerequisites(runPlan.Prerequisites)

	if checkSummary.AllFound {
		fmt.Printf("  → ✓ All %d prerequisites available\n", len(runPlan.Prerequisites))
	} else {
		fmt.Printf("  → ✗ Missing prerequisites:\n")
		for _, missing := range checkSummary.MissingTools {
			fmt.Printf("      • %s\n", missing)
			guide := checker.GetInstallGuide(missing)
			if guide != "" && verbose {
				lines := strings.Split(guide, "\n")
				for _, line := range lines[:min(3, len(lines))] {
					fmt.Printf("        %s\n", line)
				}
			}
		}

		if !dryRun && !yesFlag {
			fmt.Print("\n  Continue anyway? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				return fmt.Errorf("aborted: missing prerequisites")
			}
		}
	}

	// Show found tools in verbose mode
	if verbose {
		for _, result := range checkSummary.Results {
			if result.Found {
				version := result.Version
				if version == "" {
					version = "version unknown"
				}
				fmt.Printf("  → ✓ %s: %s\n", result.Name, version)
			}
		}
	}

	// Phase 6: Execute (or Dry-run)
	fmt.Println("\n[6/7] Execute")

	if dryRun {
		// Display dry-run output
		fmt.Print(exec.DryRunDisplay(runPlan, ws.RepoPath()))
	} else {
		// Create executor
		runnerConfig := &exec.RunnerConfig{
			Mode:        exec.ModeExecute,
			WorkingDir:  ws.RepoPath(),
			AutoYes:     yesFlag,
			AllowSudo:   allowSudo,
			Verbose:     verbose,
			StepTimeout: exec.DefaultStepTimeout,
			OnStepStart: func(step *llm.Step) {
				fmt.Printf("\n  → Executing: %s\n", step.ID)
				fmt.Printf("    Command: %s\n", step.Cmd)
			},
			OnStepComplete: func(step *llm.Step, result *exec.StepResult) {
				fmt.Printf("    %s\n", exec.FormatStepResult(result))
			},
		}

		runner := exec.NewRunner(runnerConfig)

		// Set up sudo prompt
		runner.SetSudoPrompt(createSudoPrompt())

		// Set up failure prompt
		runner.SetFailurePrompt(createFailurePrompt())

		// Execute the plan
		execResult := runner.Execute(runPlan)

		// Show execution summary
		fmt.Print(exec.FormatExecutionResult(execResult))

		if !execResult.Success {
			return fmt.Errorf("execution failed")
		}
	}

	// Phase 7: Post-run / Cleanup
	fmt.Println("\n[7/7] Post-run / Cleanup")

	// Show ports if any
	if len(runPlan.Ports) > 0 {
		fmt.Printf("  → Exposed ports: %v\n", runPlan.Ports)
	}

	// Show notes if any
	if len(runPlan.Notes) > 0 {
		fmt.Println("  → Notes:")
		for _, note := range runPlan.Notes {
			fmt.Printf("      • %s\n", note)
		}
	}

	if keepWorkspace {
		fmt.Printf("  → Workspace preserved: %s\n", ws.Path)
	} else {
		fmt.Println("  → Workspace will be cleaned up")
	}

	if dryRun {
		fmt.Println("\n  To execute this plan, run again without --dry-run:")
		fmt.Printf("    rd-run %s --dry-run=false\n", inputPath)
	}

	return nil
}

// createLLMProvider creates the appropriate LLM provider based on flags.
// Uses config resolution with precedence: CLI > ENV > config file > defaults.
// Gracefully falls back to mock provider on any failure.
func createLLMProvider() (llm.Provider, error) {
	// Resolve config with proper precedence
	config := llm.ResolveProviderConfig(
		llmProvider,  // CLI flag
		llmEndpoint,  // CLI flag
		llmModel,     // CLI flag
		GetLLMToken(), // CLI flag or env
		0,            // Use default timeout
		verbose,
	)

	// NewProvider now returns a FallbackProvider that never fails
	return llm.NewProvider(config)
}

// createSudoPrompt creates a sudo confirmation prompt function
func createSudoPrompt() exec.SudoPromptFunc {
	reader := bufio.NewReader(os.Stdin)

	return func(step *llm.Step) exec.SudoChoice {
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

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("  Error reading input: %v\n", err)
			return exec.SudoChoiceAbort
		}

		input = strings.TrimSpace(input)

		switch input {
		case "1", "y", "yes":
			fmt.Println("  → Approved for this step")
			return exec.SudoChoiceAllow
		case "2", "a", "all":
			fmt.Println("  → Approved for all sudo steps in this run")
			return exec.SudoChoiceAllowAll
		case "3", "m", "manual":
			fmt.Println()
			fmt.Println("  Manual execution instructions:")
			fmt.Println("  ─────────────────────────────────")
			fmt.Printf("  Run this command manually:\n")
			fmt.Printf("    %s\n", step.Cmd)
			fmt.Println()
			return exec.SudoChoiceManual
		case "4", "n", "no", "abort", "q", "quit":
			fmt.Println("  → Aborted by user")
			return exec.SudoChoiceAbort
		default:
			fmt.Println("  → Invalid choice, skipping step")
			return exec.SudoChoiceManual
		}
	}
}

// createFailurePrompt creates a failure handling prompt function
func createFailurePrompt() exec.FailurePromptFunc {
	reader := bufio.NewReader(os.Stdin)

	return func(step *llm.Step, result *exec.StepResult) exec.FailureChoice {
		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║                    STEP FAILED                               ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Printf("  Step:      %s\n", step.ID)
		fmt.Printf("  Command:   %s\n", step.Cmd)
		fmt.Printf("  Exit code: %d\n", result.ExitCode)

		if result.Error != nil {
			fmt.Printf("  Error:     %s\n", result.Error.Error())
		}

		if result.Stderr != "" {
			fmt.Println("\n  Last output:")
			lines := strings.Split(strings.TrimSpace(result.Stderr), "\n")
			if len(lines) > 5 {
				lines = lines[len(lines)-5:]
			}
			for _, line := range lines {
				fmt.Printf("    %s\n", line)
			}
		}

		fmt.Println()
		fmt.Println("  Choose an option:")
		fmt.Println("    1) Continue to next step")
		fmt.Println("    2) Retry this step")
		fmt.Println("    3) Abort entire operation")
		fmt.Println()
		fmt.Print("  Enter choice [1-3]: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			return exec.FailureChoiceAbort
		}

		input = strings.TrimSpace(input)

		switch input {
		case "1", "c", "continue":
			return exec.FailureChoiceContinue
		case "2", "r", "retry":
			return exec.FailureChoiceRetry
		case "3", "a", "abort", "q", "quit":
			return exec.FailureChoiceAbort
		default:
			return exec.FailureChoiceAbort
		}
	}
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
