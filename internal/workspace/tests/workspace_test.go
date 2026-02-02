// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Workspace tests

package workspace_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/sony-level/readme-runner/internal/workspace"
)

func TestGenerateRunID(t *testing.T) {
	// Reset global state
	workspace.ResetRunIDState()

	runID, err := workspace.GenerateRunID()
	if err != nil {
		t.Fatalf("GenerateRunID() error = %v", err)
	}

	// Check format: rr-YYYYMMDD-HHMM-xxx
	pattern := `^rr-\d{8}-\d{4}-[a-f0-9]{3}\d*$`
	matched, err := regexp.MatchString(pattern, runID)
	if err != nil {
		t.Fatalf("regexp.MatchString error = %v", err)
	}
	if !matched {
		t.Errorf("GenerateRunID() = %v, want format rr-YYYYMMDD-HHMM-xxx", runID)
	}

	// Check prefix
	if !strings.HasPrefix(runID, workspace.RunIDPrefix+"-") {
		t.Errorf("GenerateRunID() = %v, want prefix %s-", runID, workspace.RunIDPrefix)
	}
}

func TestGenerateRunID_Uniqueness(t *testing.T) {
	// Reset global state
	workspace.ResetRunIDState()

	const numIDs = 100
	ids := make(map[string]bool)

	for i := 0; i < numIDs; i++ {
		runID, err := workspace.GenerateRunID()
		if err != nil {
			t.Fatalf("GenerateRunID() error = %v", err)
		}
		if ids[runID] {
			t.Errorf("Duplicate run ID generated: %v", runID)
		}
		ids[runID] = true
	}
}

func TestGenerateRunID_ThreadSafety(t *testing.T) {
	// Reset global state
	workspace.ResetRunIDState()

	const numGoroutines = 10
	const idsPerGoroutine = 20

	var wg sync.WaitGroup
	idChan := make(chan string, numGoroutines*idsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				runID, err := workspace.GenerateRunID()
				if err != nil {
					t.Errorf("GenerateRunID() error = %v", err)
					return
				}
				idChan <- runID
			}
		}()
	}

	wg.Wait()
	close(idChan)

	ids := make(map[string]bool)
	for runID := range idChan {
		if ids[runID] {
			t.Errorf("Duplicate run ID in concurrent generation: %v", runID)
		}
		ids[runID] = true
	}

	if len(ids) != numGoroutines*idsPerGoroutine {
		t.Errorf("Expected %d unique IDs, got %d", numGoroutines*idsPerGoroutine, len(ids))
	}
}

func TestNew(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "workspace-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &workspace.WorkspaceConfig{
		BaseDir: tmpDir,
		Keep:    false,
	}

	ws, err := workspace.New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Check workspace exists
	if !ws.Exists() {
		t.Error("Workspace directory does not exist")
	}

	// Check subdirectories exist
	subdirs := []string{
		ws.RepoPath(),
		ws.PlanPath(),
		ws.LogsPath(),
	}

	for _, subdir := range subdirs {
		info, err := os.Stat(subdir)
		if err != nil {
			t.Errorf("Subdirectory %s does not exist: %v", subdir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", subdir)
		}
	}

	// Check paths are correct
	expectedRepoPath := filepath.Join(ws.Path, workspace.RepoSubdir)
	if ws.RepoPath() != expectedRepoPath {
		t.Errorf("RepoPath() = %v, want %v", ws.RepoPath(), expectedRepoPath)
	}

	expectedPlanPath := filepath.Join(ws.Path, workspace.PlanSubdir)
	if ws.PlanPath() != expectedPlanPath {
		t.Errorf("PlanPath() = %v, want %v", ws.PlanPath(), expectedPlanPath)
	}

	expectedLogsPath := filepath.Join(ws.Path, workspace.LogsSubdir)
	if ws.LogsPath() != expectedLogsPath {
		t.Errorf("LogsPath() = %v, want %v", ws.LogsPath(), expectedLogsPath)
	}

	// Cleanup
	err = ws.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}

	// Verify cleanup
	if ws.Exists() {
		t.Error("Workspace still exists after cleanup")
	}
}

func TestNew_NilConfig(t *testing.T) {
	// Reset global state
	workspace.ResetRunIDState()

	ws, err := workspace.New(nil)
	if err != nil {
		t.Fatalf("New(nil) error = %v", err)
	}
	defer ws.Cleanup()

	if ws.RunID == "" {
		t.Error("RunID should not be empty")
	}

	if ws.Path == "" {
		t.Error("Path should not be empty")
	}

	if !ws.Exists() {
		t.Error("Workspace should exist")
	}
}

func TestWorkspace_Keep(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workspace-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &workspace.WorkspaceConfig{
		BaseDir: tmpDir,
		Keep:    true,
	}

	ws, err := workspace.New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if !ws.ShouldKeep() {
		t.Error("ShouldKeep() should return true")
	}

	// Cleanup should not remove when keep=true
	err = ws.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}

	// Workspace should still exist
	if !ws.Exists() {
		t.Error("Workspace should still exist when keep=true")
	}

	// Test SetKeep
	ws.SetKeep(false)
	if ws.ShouldKeep() {
		t.Error("ShouldKeep() should return false after SetKeep(false)")
	}

	// Now cleanup should work
	err = ws.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}

	if ws.Exists() {
		t.Error("Workspace should not exist after cleanup with keep=false")
	}
}

func TestWorkspace_PlanFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workspace-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &workspace.WorkspaceConfig{
		BaseDir: tmpDir,
		Keep:    false,
	}

	ws, err := workspace.New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer ws.Cleanup()

	expectedPlanFile := filepath.Join(ws.PlanPath(), "run-plan.json")
	if ws.PlanFile() != expectedPlanFile {
		t.Errorf("PlanFile() = %v, want %v", ws.PlanFile(), expectedPlanFile)
	}

	expectedLogFile := filepath.Join(ws.LogsPath(), "execution.log")
	if ws.LogFile() != expectedLogFile {
		t.Errorf("LogFile() = %v, want %v", ws.LogFile(), expectedLogFile)
	}
}

func TestWorkspace_String(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workspace-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &workspace.WorkspaceConfig{
		BaseDir: tmpDir,
		Keep:    false,
	}

	ws, err := workspace.New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer ws.Cleanup()

	str := ws.String()
	if !strings.Contains(str, ws.RunID) {
		t.Errorf("String() should contain RunID, got %v", str)
	}
	if !strings.Contains(str, ws.Path) {
		t.Errorf("String() should contain Path, got %v", str)
	}
}

func TestCleanupAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workspace-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple workspaces
	for i := 0; i < 3; i++ {
		config := &workspace.WorkspaceConfig{
			BaseDir: tmpDir,
			Keep:    true,
		}
		_, err := workspace.New(config)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
	}

	// Verify temp dir exists with workspaces
	tempDir := filepath.Join(tmpDir, workspace.TempDirPrefix)
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Expected 3 workspaces, got %d", len(entries))
	}

	// Cleanup all
	err = workspace.CleanupAll(tmpDir)
	if err != nil {
		t.Errorf("CleanupAll() error = %v", err)
	}

	// Verify all workspaces removed
	_, err = os.Stat(tempDir)
	if !os.IsNotExist(err) {
		t.Error("Temp directory should not exist after CleanupAll")
	}
}

func TestCleanupAll_NonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workspace-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// CleanupAll on empty directory should not error
	err = workspace.CleanupAll(tmpDir)
	if err != nil {
		t.Errorf("CleanupAll() on non-existent should not error, got %v", err)
	}
}
