/*
Copyright © 2026 ソニーレベル <C7kali3@gmail.com>

*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	keepWorkspace bool
	dryRun        bool
	verbose       bool
	yesFlag       bool

	// LLM flags
	llmProvider string
	llmEndpoint string
	llmModel    string
	llmToken    string

	// Security flags
	allowSudo bool
)

// rootCmd represents the base command - runs directly without subcommand
var rootCmd = &cobra.Command{
	Use:   "rdr [path|url]",
	Short: "Automate installation and launch from README.md",
	Long: `rdr (readme-runner) is a fast and accurate standalone command line tool
designed to automate the installation and launch of software projects
from their README.md file.

It takes a local repository path or GitHub URL, analyzes the README
and key files, generates an installation plan, and executes it safely
with proper security checks and confirmations.

Examples:
  rdr .
  rdr https://github.com/user/repo
  rdr . --dry-run
  rdr . --keep --verbose
  rdr https://gitlab.com/user/repo -y`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to current directory if no argument provided
		inputPath := "."
		if len(args) > 0 {
			inputPath = args[0]
		}
		return executeRun(inputPath)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// GetLLMToken returns the LLM token from flag or environment variable
func GetLLMToken() string {
	if llmToken != "" {
		return llmToken
	}
	return os.Getenv("RD_LLM_TOKEN")
}

func init() {
	// Persistent flags - available to all subcommands
	rootCmd.PersistentFlags().BoolVar(&keepWorkspace, "keep", false, "Keep workspace directory after execution (.rr-temp/<run-id>)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", true, "Show plan without executing (default: true)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "Auto-accept prompts (except security-critical)")

	// LLM provider flags
	// Default is empty string to enable auto-selection: anthropic > openai > mistral > ollama > mock
	rootCmd.PersistentFlags().StringVar(&llmProvider, "llm-provider", "", "LLM provider: anthropic, openai, mistral, ollama, http, mock (default: auto-select)")
	rootCmd.PersistentFlags().StringVar(&llmProvider, "provider", "", "Alias for --llm-provider")
	rootCmd.PersistentFlags().StringVar(&llmEndpoint, "llm-endpoint", "", "HTTP endpoint for custom LLM provider")
	rootCmd.PersistentFlags().StringVar(&llmModel, "llm-model", "", "Model name for LLM provider")
	rootCmd.PersistentFlags().StringVar(&llmToken, "llm-token", "", "Authentication token for LLM (or env: ANTHROPIC_API_KEY, OPENAI_API_KEY, etc.)")

	// Security flags
	rootCmd.PersistentFlags().BoolVar(&allowSudo, "allow-sudo", false, "Allow sudo commands without confirmation prompts")
}
