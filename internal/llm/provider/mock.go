// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Mock LLM provider for testing and offline mode

package provider

import (
	"github.com/sony-level/readme-runner/internal/llm"
)

// MockProvider returns fixed plans for testing
type MockProvider struct {
	fixedPlan *llm.RunPlan
}

// NewMockProvider creates a new mock provider
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// NewMockProviderWithPlan creates a mock with a specific plan
func NewMockProviderWithPlan(plan *llm.RunPlan) *MockProvider {
	return &MockProvider{fixedPlan: plan}
}

// Name returns the provider name
func (p *MockProvider) Name() string {
	return "mock"
}

// GeneratePlan generates a plan based on the context
func (p *MockProvider) GeneratePlan(ctx *llm.PlanContext) (*llm.RunPlan, error) {
	if p.fixedPlan != nil {
		return p.fixedPlan, nil
	}

	// Handle nil context gracefully
	if ctx == nil {
		ctx = &llm.PlanContext{}
	}

	stack := "unknown"
	if ctx.Profile != nil {
		stack = ctx.Profile.Stack
	}

	return p.defaultPlanForStack(stack, ctx), nil
}

func (p *MockProvider) defaultPlanForStack(stack string, ctx *llm.PlanContext) *llm.RunPlan {
	switch stack {
	case "docker":
		return p.dockerPlan(ctx)
	case "node":
		return p.nodePlan(ctx)
	case "python":
		return p.pythonPlan(ctx)
	case "go":
		return p.goPlan(ctx)
	case "rust":
		return p.rustPlan(ctx)
	default:
		return p.unknownPlan(ctx)
	}
}

func (p *MockProvider) dockerPlan(ctx *llm.PlanContext) *llm.RunPlan {
	hasCompose := false
	if ctx.Profile != nil {
		for _, container := range ctx.Profile.Containers {
			if container == "docker-compose.yml" || container == "docker-compose.yaml" ||
				container == "compose.yml" || container == "compose.yaml" {
				hasCompose = true
				break
			}
		}
	}

	plan := &llm.RunPlan{
		Version:     "1",
		ProjectType: "docker",
		Prerequisites: []llm.Prerequisite{
			{Name: "docker", Reason: "Docker required for containerized execution"},
		},
		Env:   make(map[string]string),
		Ports: []int{8080},
		Notes: []string{"Using Docker for isolated environment"},
	}

	if hasCompose {
		plan.Prerequisites = append(plan.Prerequisites,
			llm.Prerequisite{Name: "docker-compose", Reason: "Docker Compose configuration detected"})
		plan.Steps = []llm.Step{
			{ID: "build", Cmd: "docker compose build", Cwd: ".", Risk: llm.RiskMedium},
			{ID: "run", Cmd: "docker compose up", Cwd: ".", Risk: llm.RiskLow},
		}
	} else {
		plan.Steps = []llm.Step{
			{ID: "build", Cmd: "docker build -t app .", Cwd: ".", Risk: llm.RiskMedium},
			{ID: "run", Cmd: "docker run -p 8080:8080 app", Cwd: ".", Risk: llm.RiskLow},
		}
	}

	return plan
}

func (p *MockProvider) nodePlan(ctx *llm.PlanContext) *llm.RunPlan {
	pkgManager := "npm"
	installCmd := "npm install"

	if ctx.Profile != nil {
		for _, tool := range ctx.Profile.Tools {
			switch tool {
			case "yarn":
				pkgManager = "yarn"
				installCmd = "yarn install"
			case "pnpm":
				pkgManager = "pnpm"
				installCmd = "pnpm install"
			case "bun":
				pkgManager = "bun"
				installCmd = "bun install"
			}
		}

		for _, pkg := range ctx.Profile.Packages {
			switch pkg {
			case "package-lock.json":
				if pkgManager == "npm" {
					installCmd = "npm ci"
				}
			case "yarn.lock":
				if pkgManager == "yarn" {
					installCmd = "yarn install --frozen-lockfile"
				}
			case "pnpm-lock.yaml":
				if pkgManager == "pnpm" {
					installCmd = "pnpm install --frozen-lockfile"
				}
			}
		}
	}

	return &llm.RunPlan{
		Version:     "1",
		ProjectType: "node",
		Prerequisites: []llm.Prerequisite{
			{Name: "node", Reason: "Node.js runtime required"},
			{Name: pkgManager, Reason: "Package manager for dependencies"},
		},
		Steps: []llm.Step{
			{ID: "install", Cmd: installCmd, Cwd: ".", Risk: llm.RiskMedium},
			{ID: "run", Cmd: pkgManager + " start", Cwd: ".", Risk: llm.RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{3000},
		Notes: []string{"Using " + pkgManager + " package manager"},
	}
}

func (p *MockProvider) pythonPlan(ctx *llm.PlanContext) *llm.RunPlan {
	tool := "pip"
	hasRequirements := true
	hasPyproject := false
	entryPoint := "" // Detected entry point file

	if ctx.Profile != nil {
		for _, t := range ctx.Profile.Tools {
			switch t {
			case "poetry":
				tool = "poetry"
			case "pipenv":
				tool = "pipenv"
			case "uv":
				tool = "uv"
			}
		}

		for _, pkg := range ctx.Profile.Packages {
			switch pkg {
			case "pyproject.toml":
				hasPyproject = true
			case "requirements.txt":
				hasRequirements = true
			}
		}

		// Detect common Python entry points from signals
		for _, signal := range ctx.Profile.Signals {
			switch signal {
			case "main.py":
				if entryPoint == "" {
					entryPoint = "main.py"
				}
			case "app.py":
				if entryPoint == "" {
					entryPoint = "app.py"
				}
			case "run.py":
				if entryPoint == "" {
					entryPoint = "run.py"
				}
			case "__main__.py":
				// This indicates a package with __main__.py
				entryPoint = "__main__"
			case "manage.py":
				// Django project
				entryPoint = "manage.py runserver"
			case "wsgi.py":
				// WSGI application (Flask/Django)
				if entryPoint == "" {
					entryPoint = "wsgi"
				}
			}
		}
	}

	var steps []llm.Step
	var notes []string
	var prereqs []llm.Prerequisite

	prereqs = append(prereqs, llm.Prerequisite{
		Name: "python", Reason: "Python runtime required", MinVersion: "3.8",
	})

	// Determine run command based on entry point
	getRunCmd := func(pythonBin string) (string, bool) {
		switch {
		case entryPoint == "manage.py runserver":
			return pythonBin + " manage.py runserver", true
		case entryPoint == "main.py" || entryPoint == "app.py" || entryPoint == "run.py":
			return pythonBin + " " + entryPoint, true
		case entryPoint == "__main__":
			return pythonBin + " .", true // Run as package
		case entryPoint == "wsgi":
			return pythonBin + " -m flask run", true
		default:
			return "", false // No entry point detected
		}
	}

	switch tool {
	case "poetry":
		prereqs = append(prereqs, llm.Prerequisite{Name: "poetry", Reason: "Poetry for dependency management"})
		steps = []llm.Step{
			{ID: "install", Cmd: "poetry install", Cwd: ".", Risk: llm.RiskMedium, Description: "Install dependencies with Poetry"},
		}
		if runCmd, hasEntry := getRunCmd("poetry run python"); hasEntry {
			steps = append(steps, llm.Step{
				ID: "run", Cmd: runCmd, Cwd: ".", Risk: llm.RiskLow, Description: "Run application",
			})
		}
		notes = append(notes, "Using Poetry for dependency management")

	case "pipenv":
		prereqs = append(prereqs, llm.Prerequisite{Name: "pipenv", Reason: "Pipenv for dependency management"})
		steps = []llm.Step{
			{ID: "install", Cmd: "pipenv install", Cwd: ".", Risk: llm.RiskMedium, Description: "Install dependencies with Pipenv"},
		}
		if runCmd, hasEntry := getRunCmd("pipenv run python"); hasEntry {
			steps = append(steps, llm.Step{
				ID: "run", Cmd: runCmd, Cwd: ".", Risk: llm.RiskLow, Description: "Run application",
			})
		}
		notes = append(notes, "Using Pipenv for dependency management")

	case "uv":
		prereqs = append(prereqs, llm.Prerequisite{Name: "uv", Reason: "uv for fast dependency management"})
		steps = []llm.Step{
			{ID: "venv", Cmd: "uv venv", Cwd: ".", Risk: llm.RiskLow, Description: "Create virtual environment with uv"},
			{ID: "install", Cmd: "uv pip install -r requirements.txt", Cwd: ".", Risk: llm.RiskMedium, Description: "Install dependencies with uv"},
		}
		if runCmd, hasEntry := getRunCmd(".venv/bin/python"); hasEntry {
			steps = append(steps, llm.Step{
				ID: "run", Cmd: runCmd, Cwd: ".", Risk: llm.RiskLow, Description: "Run application",
			})
		}
		notes = append(notes, "Using uv for fast dependency management")

	default:
		// Standard pip with virtual environment (PEP 668 compliant)
		installCmd := ".venv/bin/pip install -r requirements.txt"
		if hasPyproject && !hasRequirements {
			installCmd = ".venv/bin/pip install -e ."
		}

		steps = []llm.Step{
			{ID: "venv", Cmd: "python3 -m venv .venv", Cwd: ".", Risk: llm.RiskLow, Description: "Create virtual environment"},
			{ID: "install", Cmd: installCmd, Cwd: ".", Risk: llm.RiskMedium, Description: "Install dependencies in venv"},
		}
		if runCmd, hasEntry := getRunCmd(".venv/bin/python"); hasEntry {
			steps = append(steps, llm.Step{
				ID: "run", Cmd: runCmd, Cwd: ".", Risk: llm.RiskLow, Description: "Run application from venv",
			})
		}
		notes = append(notes, "Using virtual environment (PEP 668 compliant)")
		notes = append(notes, "Activate manually with: source .venv/bin/activate")
	}

	// If no entry point detected, add a helpful note
	if entryPoint == "" {
		notes = append(notes, "No entry point detected - check README for run instructions")
	}

	return &llm.RunPlan{
		Version:       "1",
		ProjectType:   "python",
		Prerequisites: prereqs,
		Steps:         steps,
		Env:           make(map[string]string),
		Ports:         []int{8000},
		Notes:         notes,
	}
}

func (p *MockProvider) goPlan(ctx *llm.PlanContext) *llm.RunPlan {
	return &llm.RunPlan{
		Version:     "1",
		ProjectType: "go",
		Prerequisites: []llm.Prerequisite{
			{Name: "go", Reason: "Go compiler required", MinVersion: "1.21"},
		},
		Steps: []llm.Step{
			{ID: "build", Cmd: "go build -o app .", Cwd: ".", Risk: llm.RiskLow},
			{ID: "run", Cmd: "./app", Cwd: ".", Risk: llm.RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{},
		Notes: []string{"Go project with modules"},
	}
}

func (p *MockProvider) rustPlan(ctx *llm.PlanContext) *llm.RunPlan {
	return &llm.RunPlan{
		Version:     "1",
		ProjectType: "rust",
		Prerequisites: []llm.Prerequisite{
			{Name: "cargo", Reason: "Rust build tool required"},
			{Name: "rustc", Reason: "Rust compiler required"},
		},
		Steps: []llm.Step{
			{ID: "build", Cmd: "cargo build --release", Cwd: ".", Risk: llm.RiskLow},
			{ID: "run", Cmd: "cargo run --release", Cwd: ".", Risk: llm.RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{},
		Notes: []string{"Rust project using Cargo"},
	}
}

func (p *MockProvider) unknownPlan(ctx *llm.PlanContext) *llm.RunPlan {
	notes := []string{"Unknown project type - manual setup may be required"}

	if ctx.Profile != nil && len(ctx.Profile.Tools) > 0 {
		notes = append(notes, "Detected tools: "+ctx.Profile.Tools[0])
	}

	return &llm.RunPlan{
		Version:       "1",
		ProjectType:   "mixed",
		Prerequisites: []llm.Prerequisite{},
		Steps: []llm.Step{
			{ID: "info", Cmd: "echo 'Project type not determined. Please check README for instructions.'", Cwd: ".", Risk: llm.RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{},
		Notes: notes,
	}
}
