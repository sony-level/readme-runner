// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Node.js stack detector

package stacks

import "github.com/sony-level/readme-runner/internal/scanner"

// NodeDetector detects Node.js projects
type NodeDetector struct {
	BaseDetector
}

// NewNodeDetector creates a new Node.js detector
func NewNodeDetector() *NodeDetector {
	return &NodeDetector{
		BaseDetector: NewBaseDetector(StackNode, PriorityNode),
	}
}

// Detect checks if the project uses Node.js
func (d *NodeDetector) Detect(profile *scanner.ProjectProfile) (StackMatch, bool) {
	var signals []string
	var reasons []string

	// Check for package.json (required for Node.js detection)
	if !hasPackage(profile, "package.json") && !hasSignal(profile, "package.json") {
		return StackMatch{}, false
	}

	signals = append(signals, "package.json")
	reasons = append(reasons, "Node.js project detected (package.json)")

	// Check for lock files to determine package manager
	lockFileDetected := false

	if hasSignal(profile, "package-lock.json") || hasPackage(profile, "package-lock.json") {
		signals = append(signals, "package-lock.json")
		reasons = append(reasons, "npm package manager in use")
		lockFileDetected = true
	}

	if hasSignal(profile, "yarn.lock") || hasPackage(profile, "yarn.lock") {
		signals = append(signals, "yarn.lock")
		reasons = append(reasons, "Yarn package manager in use")
		lockFileDetected = true
	}

	if hasSignal(profile, "pnpm-lock.yaml") || hasPackage(profile, "pnpm-lock.yaml") {
		signals = append(signals, "pnpm-lock.yaml")
		reasons = append(reasons, "pnpm package manager in use")
		lockFileDetected = true
	}

	if hasSignal(profile, "bun.lockb") || hasPackage(profile, "bun.lockb") {
		signals = append(signals, "bun.lockb")
		reasons = append(reasons, "Bun runtime in use")
		lockFileDetected = true
	}

	// If no lock file, assume npm
	if !lockFileDetected {
		reasons = append(reasons, "npm assumed (no lock file)")
	}

	// Check for Node.js tools
	nodeTools := []string{"npm", "yarn", "pnpm", "bun", "npx"}
	for _, tool := range nodeTools {
		if hasTool(profile, tool) {
			signals = append(signals, tool)
		}
	}

	// Check for TypeScript
	if hasSignal(profile, "tsconfig.json") {
		signals = append(signals, "tsconfig.json")
		reasons = append(reasons, "TypeScript configuration detected")
	}

	// Check for JavaScript config files
	jsConfigs := []string{"jsconfig.json", ".nvmrc", ".node-version", ".npmrc", ".yarnrc", ".yarnrc.yml"}
	for _, config := range jsConfigs {
		if hasSignal(profile, config) {
			signals = append(signals, config)
		}
	}

	// Check for common framework configs
	frameworkConfigs := []string{
		"next.config.js", "next.config.mjs", "next.config.ts",
		"nuxt.config.js", "nuxt.config.ts",
		"vite.config.js", "vite.config.ts",
		"webpack.config.js",
		"rollup.config.js",
		"babel.config.js", ".babelrc",
		"eslint.config.js", ".eslintrc", ".eslintrc.js", ".eslintrc.json",
		"prettier.config.js", ".prettierrc",
	}
	for _, config := range frameworkConfigs {
		if hasSignal(profile, config) {
			signals = append(signals, config)
		}
	}

	// Check for language detection
	if hasLanguage(profile, "javascript") {
		reasons = append(reasons, "JavaScript source files present")
	}
	if hasLanguage(profile, "typescript") {
		reasons = append(reasons, "TypeScript source files present")
	}

	return createMatch(StackNode, d.Priority(), signals, reasons), true
}
