// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Git cloning implementation

package fetcher

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// fetchFromGit clones a repository from GitHub or GitLab
func fetchFromGit(config *FetchConfig, sourceType string) (*FetchResult, error) {
	// Parse and normalize the URL
	repoInfo, err := ParseGitURL(config.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse git URL: %w", err)
	}

	// Normalize to HTTPS URL for cloning
	cloneURL := NormalizeGitURL(config.Source)

	if config.Verbose {
		fmt.Fprintf(config.Progress, "Cloning %s/%s from %s...\n", repoInfo.Owner, repoInfo.Repo, repoInfo.Platform)
		fmt.Fprintf(config.Progress, "URL: %s\n", cloneURL)
		fmt.Fprintf(config.Progress, "Destination: %s\n", config.Destination)
	}

	// Prepare clone options
	cloneOpts := &git.CloneOptions{
		URL:      cloneURL,
		Progress: nil,
	}

	// Enable progress output if verbose
	if config.Verbose {
		cloneOpts.Progress = config.Progress
	}

	// Use shallow clone if requested
	if config.ShallowClone {
		cloneOpts.Depth = 1
		cloneOpts.SingleBranch = true
		cloneOpts.ReferenceName = plumbing.HEAD
	}

	// Clone the repository
	_, err = git.PlainClone(config.Destination, false, cloneOpts)
	if err != nil {
		// Clean up partial clone on failure
		_ = os.RemoveAll(config.Destination)
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Count files in the cloned repository
	fileCount, byteCount, err := countFiles(config.Destination)
	if err != nil {
		// Non-fatal error, just log it
		if config.Verbose {
			fmt.Fprintf(config.Progress, "Warning: could not count files: %v\n", err)
		}
	}

	if config.Verbose {
		fmt.Fprintf(config.Progress, "Cloned %d files (%d bytes)\n", fileCount, byteCount)
	}

	return &FetchResult{
		Source:      config.Source,
		Destination: config.Destination,
		SourceType:  sourceType,
		IsGitRepo:   true,
		FilesCopied: fileCount,
		BytesCopied: byteCount,
	}, nil
}

// countFiles counts files and total bytes in a directory
func countFiles(dir string) (int, int64, error) {
	var fileCount int
	var byteCount int64

	err := walkDir(dir, func(path string, info os.FileInfo) error {
		if !info.IsDir() {
			fileCount++
			byteCount += info.Size()
		}
		return nil
	})

	return fileCount, byteCount, err
}
