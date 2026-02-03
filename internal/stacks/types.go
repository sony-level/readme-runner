// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Stack detection types and interfaces

package stacks

import "github.com/sony-level/readme-runner/internal/scanner"

// Detector defines the interface for stack detectors
type Detector interface {
	Name() string
	Priority() int
	Detect(profile *scanner.ProjectProfile) (StackMatch, bool)
}

// StackMatch represents a detected stack with confidence and reasoning
type StackMatch struct {
	Name       string   `json:"name"`       // docker, node, python, go, rust, mixed
	Confidence float64  `json:"confidence"` // 0.0 to 1.0
	Reasons    []string `json:"reasons"`    // Why this stack was detected
	Signals    []string `json:"signals"`    // Files/keywords that triggered detection
	Priority   int      `json:"priority"`   // Detector priority (higher = more important)
}

// DetectionResult contains all detected stacks and the dominant choice
type DetectionResult struct {
	Matches     []StackMatch `json:"matches"`
	Dominant    StackMatch   `json:"dominant"`
	IsMixed     bool         `json:"is_mixed"`
	AllStacks   []string     `json:"all_stacks"`   // Unique stack names
	Explanation string       `json:"explanation"`  // Why dominant was chosen
}

// Stack priorities (higher = more dominant)
const (
	PriorityDocker = 100
	PriorityNode   = 80
	PriorityPython = 80
	PriorityGo     = 70
	PriorityRust   = 70
	PriorityJava   = 70
	PriorityDotNet = 60
	PriorityRuby   = 60
	PriorityPHP    = 60
	PriorityMixed  = 50
)

// Stack names
const (
	StackDocker = "docker"
	StackNode   = "node"
	StackPython = "python"
	StackGo     = "go"
	StackRust   = "rust"
	StackJava   = "java"
	StackDotNet = "dotnet"
	StackRuby   = "ruby"
	StackPHP    = "php"
	StackMixed  = "mixed"
)
