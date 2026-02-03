// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Stack detection aggregation logic

package stacks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sony-level/readme-runner/internal/scanner"
)

// Aggregator runs all detectors and determines the dominant stack
type Aggregator struct {
	detectors []Detector
}

// NewAggregator creates a new aggregator with all detectors
func NewAggregator() *Aggregator {
	return &Aggregator{
		detectors: []Detector{
			NewDockerDetector(),
			NewNodeDetector(),
			NewPythonDetector(),
			NewGoDetector(),
			NewRustDetector(),
		},
	}
}

// NewAggregatorWithDetectors creates an aggregator with custom detectors
func NewAggregatorWithDetectors(detectors ...Detector) *Aggregator {
	return &Aggregator{
		detectors: detectors,
	}
}

// AddDetector adds a custom detector to the aggregator
func (a *Aggregator) AddDetector(detector Detector) {
	a.detectors = append(a.detectors, detector)
}

// Detect runs all detectors and returns the analysis
func (a *Aggregator) Detect(profile *scanner.ProjectProfile) DetectionResult {
	if profile == nil {
		return DetectionResult{
			Matches:     []StackMatch{},
			Explanation: "No profile provided",
		}
	}

	var matches []StackMatch

	// Run all detectors
	for _, detector := range a.detectors {
		if match, found := detector.Detect(profile); found {
			matches = append(matches, match)
		}
	}

	// Sort by priority (descending) then by confidence (descending)
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Priority != matches[j].Priority {
			return matches[i].Priority > matches[j].Priority
		}
		return matches[i].Confidence > matches[j].Confidence
	})

	// Determine dominant stack
	dominant, isMixed, explanation := a.determineDominant(matches)

	// Extract all unique stack names
	allStacks := make([]string, 0, len(matches))
	for _, match := range matches {
		allStacks = append(allStacks, match.Name)
	}

	return DetectionResult{
		Matches:     matches,
		Dominant:    dominant,
		IsMixed:     isMixed,
		AllStacks:   allStacks,
		Explanation: explanation,
	}
}

// determineDominant selects the primary stack based on rules
func (a *Aggregator) determineDominant(matches []StackMatch) (StackMatch, bool, string) {
	if len(matches) == 0 {
		return StackMatch{Name: "unknown"}, false, "No stacks detected"
	}

	if len(matches) == 1 {
		return matches[0], false, fmt.Sprintf("Single stack detected: %s", matches[0].Name)
	}

	// Rule 1: If Docker Compose exists, Docker is always dominant
	for _, match := range matches {
		if match.Name == StackDocker {
			for _, signal := range match.Signals {
				lower := strings.ToLower(signal)
				if strings.Contains(lower, "compose") {
					return match, true,
						fmt.Sprintf("Docker Compose detected, Docker is dominant (also found: %s)",
							a.otherStackNames(matches, StackDocker))
				}
			}
		}
	}

	// Rule 2: If Dockerfile exists and has highest priority, Docker is dominant
	if matches[0].Name == StackDocker {
		return matches[0], len(matches) > 1,
			fmt.Sprintf("Dockerfile present, Docker is dominant (also found: %s)",
				a.otherStackNames(matches, StackDocker))
	}

	// Rule 3: Check if multiple stacks have same priority (mixed project)
	isMixed := false
	if len(matches) > 1 && matches[0].Priority == matches[1].Priority {
		isMixed = true
	}

	// Rule 4: Highest priority/confidence wins
	dominant := matches[0]

	var explanation strings.Builder
	explanation.WriteString(fmt.Sprintf("Selected %s as dominant stack", dominant.Name))

	if isMixed {
		// List all stacks with same priority
		samePriority := []string{dominant.Name}
		for i := 1; i < len(matches); i++ {
			if matches[i].Priority == dominant.Priority {
				samePriority = append(samePriority, matches[i].Name)
			}
		}
		explanation.WriteString(fmt.Sprintf(" (mixed project with: %s)", strings.Join(samePriority, ", ")))
	} else if len(matches) > 1 {
		explanation.WriteString(fmt.Sprintf(" (priority over: %s)", a.otherStackNames(matches, dominant.Name)))
	}

	return dominant, isMixed, explanation.String()
}

// otherStackNames returns names of all stacks except the specified one
func (a *Aggregator) otherStackNames(matches []StackMatch, exclude string) string {
	var names []string
	for _, match := range matches {
		if match.Name != exclude {
			names = append(names, match.Name)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// GetDetectors returns the list of detectors
func (a *Aggregator) GetDetectors() []Detector {
	return a.detectors
}
