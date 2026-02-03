// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Base detector functionality

package stacks

import (
	"strings"

	"github.com/sony-level/readme-runner/internal/scanner"
)

// BaseDetector provides common functionality for detectors
type BaseDetector struct {
	name     string
	priority int
}

// NewBaseDetector creates a new base detector
func NewBaseDetector(name string, priority int) BaseDetector {
	return BaseDetector{name: name, priority: priority}
}

// Name returns the detector name
func (d BaseDetector) Name() string {
	return d.name
}

// Priority returns the detector priority
func (d BaseDetector) Priority() int {
	return d.priority
}

// Helper methods for detectors

// hasSignal checks if a signal exists in the profile
func hasSignal(profile *scanner.ProjectProfile, signal string) bool {
	for _, s := range profile.Signals {
		if strings.EqualFold(s, signal) {
			return true
		}
	}
	return false
}

// hasAnySignal checks if any of the signals exist
func hasAnySignal(profile *scanner.ProjectProfile, signals ...string) bool {
	for _, signal := range signals {
		if hasSignal(profile, signal) {
			return true
		}
	}
	return false
}

// countSignals counts how many of the specified signals exist
func countSignals(profile *scanner.ProjectProfile, signals ...string) int {
	count := 0
	for _, signal := range signals {
		if hasSignal(profile, signal) {
			count++
		}
	}
	return count
}

// hasTool checks if a tool is detected
func hasTool(profile *scanner.ProjectProfile, tool string) bool {
	for _, t := range profile.Tools {
		if strings.EqualFold(t, tool) {
			return true
		}
	}
	return false
}

// hasAnyTool checks if any of the tools exist
func hasAnyTool(profile *scanner.ProjectProfile, tools ...string) bool {
	for _, tool := range tools {
		if hasTool(profile, tool) {
			return true
		}
	}
	return false
}

// hasPackage checks if a package file exists
func hasPackage(profile *scanner.ProjectProfile, pkg string) bool {
	for _, p := range profile.Packages {
		if strings.EqualFold(p, pkg) {
			return true
		}
	}
	return false
}

// hasContainer checks if a container file exists
func hasContainer(profile *scanner.ProjectProfile, container string) bool {
	for _, c := range profile.Containers {
		if strings.EqualFold(c, container) {
			return true
		}
	}
	return false
}

// hasAnyContainer checks if any container file exists
func hasAnyContainer(profile *scanner.ProjectProfile, containers ...string) bool {
	for _, container := range containers {
		if hasContainer(profile, container) {
			return true
		}
	}
	return false
}

// hasLanguage checks if a language is detected
func hasLanguage(profile *scanner.ProjectProfile, lang string) bool {
	for _, l := range profile.Languages {
		if strings.EqualFold(l, lang) {
			return true
		}
	}
	return false
}

// createMatch creates a StackMatch with calculated confidence
func createMatch(name string, priority int, signals []string, reasons []string) StackMatch {
	confidence := calculateConfidence(signals, reasons)

	return StackMatch{
		Name:       name,
		Confidence: confidence,
		Reasons:    reasons,
		Signals:    uniqueStrings(signals),
		Priority:   priority,
	}
}

// calculateConfidence computes confidence based on signal strength
func calculateConfidence(signals []string, reasons []string) float64 {
	// Base confidence from number of signals
	signalStrength := float64(len(signals)) / 10.0 // Normalize to 0.0-1.0

	// Boost from quality of reasons
	reasonStrength := float64(len(reasons)) / 5.0

	confidence := (signalStrength + reasonStrength) / 2.0

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Minimum confidence if we have any matches
	if confidence < 0.1 && (len(signals) > 0 || len(reasons) > 0) {
		confidence = 0.1
	}

	return confidence
}

// uniqueStrings removes duplicates from a slice
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, s := range slice {
		lower := strings.ToLower(s)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, s)
		}
	}

	return result
}
