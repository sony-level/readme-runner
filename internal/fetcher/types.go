// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Fetcher types and constants

package fetcher

import (
	"io"
	"regexp"
)

// Source type constants
const (
	SourceTypeUnknown = "unknown"
	SourceTypeGitHub  = "github"
	SourceTypeGitLab  = "gitlab"
	SourceTypeLocal   = "local"
)

// Common patterns for GitHub URLs
var (
	// HTTPS: https://github.com/user/repo or https://github.com/user/repo.git
	githubHTTPSPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	// SSH: git@github.com:user/repo.git
	githubSSHPattern = regexp.MustCompile(`^git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)
	// HTTPS: https://gitlab.com/user/repo or https://gitlab.com/user/repo.git
	gitlabHTTPSPattern = regexp.MustCompile(`^https?://gitlab\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	// SSH: git@gitlab.com:user/repo.git
	gitlabSSHPattern = regexp.MustCompile(`^git@gitlab\.com:([^/]+)/([^/]+?)(?:\.git)?$`)
)

// FetchConfig holds configuration for fetching a project
type FetchConfig struct {
	Source      string    // GitHub URL or local path
	Destination string    // Target directory (workspace.RepoPath())
	Verbose     bool      // Enable verbose logging
	Progress    io.Writer // Progress output (optional, defaults to io.Discard)
	ShallowClone bool     // Use shallow clone for git (depth=1)
}

// FetchResult contains the result of a fetch operation
type FetchResult struct {
	Source      string // Original source
	Destination string // Where files were copied/cloned
	SourceType  string // Type of source (github, local)
	IsGitRepo   bool   // Whether source was a git repository
	FilesCopied int    // Number of files copied
	BytesCopied int64  // Total bytes copied
}

// GitRepoInfo contains parsed git repository information
type GitRepoInfo struct {
	Owner    string
	Repo     string
	URL      string
	Platform string // "github" or "gitlab"
}
