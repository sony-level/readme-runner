// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Rust stack detector

package stacks

import "github.com/sony-level/readme-runner/internal/scanner"

// RustDetector detects Rust projects
type RustDetector struct {
	BaseDetector
}

// NewRustDetector creates a new Rust detector
func NewRustDetector() *RustDetector {
	return &RustDetector{
		BaseDetector: NewBaseDetector(StackRust, PriorityRust),
	}
}

// Detect checks if the project uses Rust
func (d *RustDetector) Detect(profile *scanner.ProjectProfile) (StackMatch, bool) {
	var signals []string
	var reasons []string

	// Check for Cargo.toml (required for Rust)
	hasCargoToml := hasPackage(profile, "Cargo.toml") || hasSignal(profile, "Cargo.toml")
	if !hasCargoToml {
		return StackMatch{}, false
	}

	signals = append(signals, "Cargo.toml")
	reasons = append(reasons, "Rust project detected (Cargo.toml)")

	// Check for Cargo.lock
	if hasSignal(profile, "Cargo.lock") || hasPackage(profile, "Cargo.lock") {
		signals = append(signals, "Cargo.lock")
		reasons = append(reasons, "Cargo dependencies locked")
	}

	// Check for Rust tools
	rustTools := []string{"cargo", "rustc", "rustup", "rustfmt", "clippy"}
	for _, tool := range rustTools {
		if hasTool(profile, tool) {
			signals = append(signals, tool)
		}
	}

	// Check for Rust-specific config files
	rustConfigs := []string{
		"rustfmt.toml", ".rustfmt.toml",
		"clippy.toml", ".clippy.toml",
		"rust-toolchain", "rust-toolchain.toml",
		".cargo/config", ".cargo/config.toml",
	}
	for _, config := range rustConfigs {
		if hasSignal(profile, config) {
			signals = append(signals, config)
		}
	}

	// Check for workspace (multi-crate project)
	// This would be indicated by multiple Cargo.toml files
	cargoCount := countSignals(profile, "Cargo.toml")
	if cargoCount > 1 {
		reasons = append(reasons, "Cargo workspace detected (multiple crates)")
	}

	// Check for common Rust directories
	rustDirs := []string{"src", "benches", "examples", "tests"}
	for _, dir := range rustDirs {
		if hasSignal(profile, dir) {
			signals = append(signals, dir)
		}
	}

	// Check for language detection
	if hasLanguage(profile, "rust") {
		reasons = append(reasons, "Rust source files present")
	}

	return createMatch(StackRust, d.Priority(), signals, reasons), true
}
