// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// workspace types/constants

package workspace

const (
	TempDirPrefix = ".rr-temp"
	RunIDPrefix   = "rr"
	RepoSubdir    = "repo"
	PlanSubdir    = "plan"
	LogsSubdir    = "logs"
)

// Workspace represents an isolated workspace for a single run
type Workspace struct {
	RunID   string
	Path    string
	BaseDir string
	keep    bool
}

// WorkspaceConfig holds configuration for workspace creation
type WorkspaceConfig struct {
	BaseDir string
	Keep    bool
}