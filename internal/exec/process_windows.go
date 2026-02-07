// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Windows-specific process handling

//go:build windows

package exec

import (
	"os/exec"
)

// setPlatformProcessGroup configures platform-specific process attributes.
// On Windows, we don't set up process groups the same way as Unix.
// The CommandContext will handle termination via TerminateProcess.
func setPlatformProcessGroup(cmd *exec.Cmd) {
	// Windows doesn't use Unix-style process groups
	// exec.CommandContext handles termination differently on Windows
}

// killProcessGroup kills the process and its children.
// On Windows, we rely on the default behavior of Process.Kill()
// which calls TerminateProcess.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// interruptProcessGroup attempts to gracefully stop the process.
// On Windows, there's no direct equivalent to SIGINT for console apps
// without a console, so we fall back to Kill.
func interruptProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	// Windows doesn't have a clean way to send Ctrl+C to a process
	// without a console, so we just kill it
	return cmd.Process.Kill()
}
