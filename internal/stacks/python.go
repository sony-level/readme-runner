// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Python stack detector

package stacks

import "github.com/sony-level/readme-runner/internal/scanner"

// PythonDetector detects Python projects
type PythonDetector struct {
	BaseDetector
}

// NewPythonDetector creates a new Python detector
func NewPythonDetector() *PythonDetector {
	return &PythonDetector{
		BaseDetector: NewBaseDetector(StackPython, PriorityPython),
	}
}

// Detect checks if the project uses Python
func (d *PythonDetector) Detect(profile *scanner.ProjectProfile) (StackMatch, bool) {
	var signals []string
	var reasons []string

	// Check for Python package files
	hasPyProject := hasPackage(profile, "pyproject.toml") || hasSignal(profile, "pyproject.toml")
	hasRequirements := hasAnySignal(profile, "requirements.txt", "requirements-dev.txt", "requirements.in")
	hasPipfile := hasSignal(profile, "Pipfile")
	hasSetupPy := hasSignal(profile, "setup.py")
	hasSetupCfg := hasSignal(profile, "setup.cfg")

	// Must have at least one Python indicator
	if !hasPyProject && !hasRequirements && !hasPipfile && !hasSetupPy && !hasSetupCfg {
		return StackMatch{}, false
	}

	// pyproject.toml (modern Python)
	if hasPyProject {
		signals = append(signals, "pyproject.toml")
		reasons = append(reasons, "Modern Python project (pyproject.toml)")

		// Check for Poetry
		if hasTool(profile, "poetry") {
			signals = append(signals, "poetry")
			reasons = append(reasons, "Poetry dependency manager")
		}

		// Check for poetry.lock
		if hasSignal(profile, "poetry.lock") || hasPackage(profile, "poetry.lock") {
			signals = append(signals, "poetry.lock")
		}
	}

	// requirements.txt (pip)
	if hasRequirements {
		signals = append(signals, "requirements.txt")
		reasons = append(reasons, "pip requirements file")
	}

	// Pipfile (Pipenv)
	if hasPipfile {
		signals = append(signals, "Pipfile")
		reasons = append(reasons, "Pipenv dependency manager")

		if hasSignal(profile, "Pipfile.lock") {
			signals = append(signals, "Pipfile.lock")
		}
	}

	// setup.py (legacy)
	if hasSetupPy {
		signals = append(signals, "setup.py")
		reasons = append(reasons, "Python setup script (legacy)")
	}

	// setup.cfg
	if hasSetupCfg {
		signals = append(signals, "setup.cfg")
	}

	// Check for Python tools
	pythonTools := []string{"pip", "poetry", "pipenv", "conda", "uv", "python", "python3", "pytest", "mypy", "ruff", "black", "flake8"}
	for _, tool := range pythonTools {
		if hasTool(profile, tool) {
			signals = append(signals, tool)
		}
	}

	// Check for Python config files
	pythonConfigs := []string{
		"tox.ini", "pytest.ini", ".pytest.ini",
		"mypy.ini", ".mypy.ini",
		".flake8", ".pylintrc",
		".python-version",
		"ruff.toml", ".ruff.toml",
		"pyproject.toml",
	}
	for _, config := range pythonConfigs {
		if hasSignal(profile, config) && config != "pyproject.toml" { // Already handled above
			signals = append(signals, config)
		}
	}

	// Check for virtual environment indicators
	venvIndicators := []string{".venv", "venv", ".python-version"}
	for _, indicator := range venvIndicators {
		if hasSignal(profile, indicator) {
			signals = append(signals, indicator)
		}
	}

	// Check for language detection
	if hasLanguage(profile, "python") {
		reasons = append(reasons, "Python source files present")
	}

	return createMatch(StackPython, d.Priority(), signals, reasons), true
}
