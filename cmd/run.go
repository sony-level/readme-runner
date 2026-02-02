/*
Copyright © 2026 ソニーレベル <c7kali3@gmail.com>

*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sony-level/readme-runner/internal/fetcher"
	"github.com/sony-level/readme-runner/internal/scanner"
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

	// Display detected stacks
	stacks := scanResult.DetectedStacks()
	if len(stacks) > 0 {
		fmt.Printf("  → Primary stack: %s\n", scanResult.PrimaryStack())
		fmt.Printf("  → All stacks: %s\n", strings.Join(stacks, ", "))
	}

	// Display project files in verbose mode
	if verbose && len(scanResult.ProjectFiles) > 0 {
		fmt.Printf("  → Project files:\n")
		for fileType, paths := range scanResult.ProjectFiles {
			fmt.Printf("    %s: %s\n", fileType, strings.Join(paths, ", "))
		}
	}

	// Phase 3: Plan (AI)
	fmt.Println("\n[3/7] Plan (AI)")
	fmt.Println("  → (not implemented)")

	// Phase 4: Validate / Normalize
	fmt.Println("\n[4/7] Validate / Normalize")
	fmt.Println("  → (not implemented)")

	// Phase 5: Prerequisites
	fmt.Println("\n[5/7] Prerequisites")
	fmt.Println("  → (not implemented)")

	// Phase 6: Execute (or Dry-run)
	fmt.Println("\n[6/7] Execute")
	if dryRun {
		fmt.Println("  → Skipped (dry-run mode)")
	} else {
		fmt.Println("  → (not implemented)")
	}

	// Phase 7: Post-run / Cleanup
	fmt.Println("\n[7/7] Post-run / Cleanup")
	if keepWorkspace {
		fmt.Printf("  → Workspace preserved: %s\n", ws.Path)
	} else {
		fmt.Println("  → Workspace will be cleaned up")
	}

	return nil
}
