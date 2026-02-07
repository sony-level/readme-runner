// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Unix-specific process group handling for proper signal propagation

//go:build !windows

package exec

import (
	"os/exec"
	"syscall"
)

// setPlatformProcessGroup configures the command to run in its own process group.
// On Unix, this allows us to kill all child processes when the parent is terminated.
func setPlatformProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}
}

// killProcessGroup kills the entire process group associated with the command.
// On Unix, we use negative PID to signal the entire process group.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	// Get the process group ID (same as PID when Setpgid is true)
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		// Fallback to killing just the process
		return cmd.Process.Kill()
	}

	// Kill the entire process group using negative PGID
	// This sends SIGKILL to all processes in the group
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

// interruptProcessGroup sends an interrupt signal to the process group.
// This gives processes a chance to clean up gracefully before being killed.
func interruptProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		// Fallback to interrupting just the process
		return cmd.Process.Signal(syscall.SIGINT)
	}

	// Send SIGINT to the entire process group
	return syscall.Kill(-pgid, syscall.SIGINT)
}
