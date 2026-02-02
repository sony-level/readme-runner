// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Main workspace logic

package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	// mutex ensures thread-safe run ID generation
	idMutex sync.Mutex
	// lastTimestamp prevents duplicate IDs in the same second
	lastTimestamp string
	lastCounter   int
)

// ResetRunIDState resets the global run ID generation state (for testing)
func ResetRunIDState() {
	idMutex.Lock()
	defer idMutex.Unlock()
	lastTimestamp = ""
	lastCounter = 0
}

// GenerateRunID creates a unique run ID with format: rr-YYYYMMDD-HHMM-3hexchars
// or rr-YYYYMMDD-HHMM-NNN (counter format) for rapid successive calls
// Thread-safe and guaranteed unique even with rapid consecutive calls
func GenerateRunID() (string, error) {
	idMutex.Lock()
	defer idMutex.Unlock()

	now := time.Now()
	timestamp := now.Format("20060102-1504")

	// Handle potential collisions within the same minute
	if timestamp == lastTimestamp {
		lastCounter++
		// Use counter format to ensure uniqueness (no random component)
		return fmt.Sprintf("%s-%s-%03d", RunIDPrefix, timestamp, lastCounter), nil
	}

	// New minute - generate fresh random hex
	randomBytes := make([]byte, 2)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	randomHex := hex.EncodeToString(randomBytes)[:3]

	lastTimestamp = timestamp
	lastCounter = 0

	return fmt.Sprintf("%s-%s-%s", RunIDPrefix, timestamp, randomHex), nil
}

// New creates a new workspace with the given configuration
// If config is nil, uses current working directory as base
func New(config *WorkspaceConfig) (*Workspace, error) {
	if config == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		config = &WorkspaceConfig{
			BaseDir: cwd,
			Keep:    false,
		}
	}

	runID, err := GenerateRunID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate run ID: %w", err)
	}

	workspacePath := filepath.Join(config.BaseDir, TempDirPrefix, runID)

	// Create workspace directory with subdirectories
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory %s: %w", workspacePath, err)
	}

	ws := &Workspace{
		RunID:   runID,
		Path:    workspacePath,
		BaseDir: config.BaseDir,
		keep:    config.Keep,
	}

	// Create subdirectories
	subdirs := []string{RepoSubdir, PlanSubdir, LogsSubdir}
	for _, subdir := range subdirs {
		subdirPath := filepath.Join(workspacePath, subdir)
		if err := os.MkdirAll(subdirPath, 0755); err != nil {
			// Cleanup on failure
			_ = os.RemoveAll(workspacePath)
			return nil, fmt.Errorf("failed to create subdirectory %s: %w", subdirPath, err)
		}
	}

	return ws, nil
}

// RepoPath returns the path to the cloned/copied repository
func (w *Workspace) RepoPath() string {
	return filepath.Join(w.Path, RepoSubdir)
}

// PlanPath returns the path to store plan files
func (w *Workspace) PlanPath() string {
	return filepath.Join(w.Path, PlanSubdir)
}

// LogsPath returns the path to store log files
func (w *Workspace) LogsPath() string {
	return filepath.Join(w.Path, LogsSubdir)
}

// PlanFile returns the path to the main plan JSON file
func (w *Workspace) PlanFile() string {
	return filepath.Join(w.PlanPath(), "run-plan.json")
}

// LogFile returns the path to the main execution log file
func (w *Workspace) LogFile() string {
	return filepath.Join(w.LogsPath(), "execution.log")
}

// Exists checks if the workspace directory exists
func (w *Workspace) Exists() bool {
	info, err := os.Stat(w.Path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// SetKeep sets whether to preserve the workspace on cleanup
func (w *Workspace) SetKeep(keep bool) {
	w.keep = keep
}

// ShouldKeep returns whether the workspace should be preserved
func (w *Workspace) ShouldKeep() bool {
	return w.keep
}

// String returns a string representation of the workspace
func (w *Workspace) String() string {
	return fmt.Sprintf("Workspace{RunID: %s, Path: %s, Keep: %v}", w.RunID, w.Path, w.keep)
}
