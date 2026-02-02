// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Cleanup functionality

package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Cleanup removes the workspace directory unless keep is true
// Returns nil if keep is true or cleanup succeeds
func (w *Workspace) Cleanup() error {
	if w.keep {
		return nil
	}

	if !w.Exists() {
		return nil
	}

	if err := os.RemoveAll(w.Path); err != nil {
		return fmt.Errorf("failed to cleanup workspace %s: %w", w.Path, err)
	}

	// Try to remove the parent .rr-temp directory if it's empty
	tempDir := filepath.Join(w.BaseDir, TempDirPrefix)
	_ = os.Remove(tempDir) // Ignore error - directory might not be empty

	return nil
}

// CleanupAll removes all workspace directories in the base directory
// Use with caution - this removes all .rr-temp/* directories
func CleanupAll(baseDir string) error {
	tempDir := filepath.Join(baseDir, TempDirPrefix)

	info, err := os.Stat(tempDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to stat temp directory %s: %w", tempDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", tempDir)
	}

	if err := os.RemoveAll(tempDir); err != nil {
		return fmt.Errorf("failed to cleanup all workspaces: %w", err)
	}

	return nil
}

// CleanupStale removes workspaces older than the given number of hours
// Useful for cleaning up abandoned workspaces
func CleanupStale(baseDir string, maxAgeHours int) (int, error) {
	tempDir := filepath.Join(baseDir, TempDirPrefix)

	info, err := os.Stat(tempDir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to stat temp directory %s: %w", tempDir, err)
	}
	if !info.IsDir() {
		return 0, nil
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read temp directory: %w", err)
	}

	now := time.Now()
	cleaned := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		modTime := entryInfo.ModTime()
		ageInHours := int(now.Sub(modTime).Hours())

		if ageInHours >= maxAgeHours {
			wsPath := filepath.Join(tempDir, entry.Name())
			if err := os.RemoveAll(wsPath); err == nil {
				cleaned++
			}
		}
	}

	// Try to remove the parent directory if empty
	_ = os.Remove(tempDir)

	return cleaned, nil
}
