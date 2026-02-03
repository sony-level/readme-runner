// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Go stack detector

package stacks

import "github.com/sony-level/readme-runner/internal/scanner"

// GoDetector detects Go projects
type GoDetector struct {
	BaseDetector
}

// NewGoDetector creates a new Go detector
func NewGoDetector() *GoDetector {
	return &GoDetector{
		BaseDetector: NewBaseDetector(StackGo, PriorityGo),
	}
}

// Detect checks if the project uses Go
func (d *GoDetector) Detect(profile *scanner.ProjectProfile) (StackMatch, bool) {
	var signals []string
	var reasons []string

	// Check for go.mod (required for Go modules)
	hasGoMod := hasPackage(profile, "go.mod") || hasSignal(profile, "go.mod")
	if !hasGoMod {
		return StackMatch{}, false
	}

	signals = append(signals, "go.mod")
	reasons = append(reasons, "Go module detected")

	// Check for go.sum
	if hasSignal(profile, "go.sum") || hasPackage(profile, "go.sum") {
		signals = append(signals, "go.sum")
		reasons = append(reasons, "Go dependencies locked (go.sum)")
	}

	// Check for go.work (Go workspaces)
	if hasSignal(profile, "go.work") {
		signals = append(signals, "go.work")
		reasons = append(reasons, "Go workspace detected")
	}

	// Check for Go tools
	goTools := []string{"go", "gofmt", "goimports", "golangci-lint", "gopls"}
	for _, tool := range goTools {
		if hasTool(profile, tool) {
			signals = append(signals, tool)
		}
	}

	// Check for Make (commonly used with Go)
	if hasTool(profile, "make") && hasSignal(profile, "Makefile") {
		signals = append(signals, "Makefile")
		reasons = append(reasons, "Makefile present (common in Go projects)")
	}

	// Check for Go-specific config files
	goConfigs := []string{
		".golangci.yml", ".golangci.yaml", ".golangci.json",
		".goreleaser.yml", ".goreleaser.yaml",
		"tools.go",
	}
	for _, config := range goConfigs {
		if hasSignal(profile, config) {
			signals = append(signals, config)
		}
	}

	// Check for standard Go project layout directories
	goDirs := []string{"cmd", "pkg", "internal", "api"}
	dirCount := 0
	for _, dir := range goDirs {
		if hasSignal(profile, dir) {
			signals = append(signals, dir)
			dirCount++
		}
	}
	if dirCount >= 2 {
		reasons = append(reasons, "Standard Go project layout detected")
	}

	// Check for language detection
	if hasLanguage(profile, "go") {
		reasons = append(reasons, "Go source files present")
	}

	return createMatch(StackGo, d.Priority(), signals, reasons), true
}
