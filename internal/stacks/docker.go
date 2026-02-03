// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Docker stack detector

package stacks

import (
	"strings"

	"github.com/sony-level/readme-runner/internal/scanner"
)

// DockerDetector detects Docker-based projects
type DockerDetector struct {
	BaseDetector
}

// NewDockerDetector creates a new Docker detector
func NewDockerDetector() *DockerDetector {
	return &DockerDetector{
		BaseDetector: NewBaseDetector(StackDocker, PriorityDocker),
	}
}

// Detect checks if the project uses Docker
func (d *DockerDetector) Detect(profile *scanner.ProjectProfile) (StackMatch, bool) {
	var signals []string
	var reasons []string

	// Check for Docker Compose (highest priority indicator)
	composeFiles := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	hasCompose := false
	for _, compose := range composeFiles {
		if hasContainer(profile, compose) || hasSignal(profile, compose) {
			signals = append(signals, compose)
			hasCompose = true
		}
	}
	if hasCompose {
		reasons = append(reasons, "Docker Compose configuration found")
	}

	// Check for Dockerfile
	if hasContainer(profile, "Dockerfile") || hasSignal(profile, "Dockerfile") {
		signals = append(signals, "Dockerfile")
		reasons = append(reasons, "Dockerfile present")
	}

	// Check for .dockerignore
	if hasSignal(profile, ".dockerignore") {
		signals = append(signals, ".dockerignore")
		reasons = append(reasons, "Docker ignore file present")
	}

	// Check for docker tool detection
	if hasTool(profile, "docker") {
		signals = append(signals, "docker")
	}
	if hasTool(profile, "docker-compose") {
		signals = append(signals, "docker-compose")
	}

	// Check for Kubernetes manifests (related to containerization)
	if hasTool(profile, "kubernetes") {
		signals = append(signals, "kubernetes")
		reasons = append(reasons, "Kubernetes manifests detected")
	}

	// Must have at least Dockerfile or Compose to be considered Docker project
	hasDockerfile := hasContainer(profile, "Dockerfile") || hasSignal(profile, "Dockerfile")
	if !hasDockerfile && !hasCompose {
		return StackMatch{}, false
	}

	// Check README for Docker instructions
	if profile.Readme && hasDockerInstructions(profile) {
		reasons = append(reasons, "README contains Docker instructions")
	}

	return createMatch(StackDocker, d.Priority(), signals, reasons), true
}

// hasDockerInstructions checks if profile signals suggest Docker usage
func hasDockerInstructions(profile *scanner.ProjectProfile) bool {
	dockerKeywords := []string{"docker", "container", "compose"}
	for _, signal := range profile.Signals {
		lower := strings.ToLower(signal)
		for _, keyword := range dockerKeywords {
			if strings.Contains(lower, keyword) {
				return true
			}
		}
	}
	return false
}
