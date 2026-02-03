// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Mock LLM provider for testing

package llm

// MockProvider returns fixed plans for testing
type MockProvider struct {
	fixedPlan *RunPlan
}

// NewMockProvider creates a new mock provider
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// NewMockProviderWithPlan creates a mock with a specific plan
func NewMockProviderWithPlan(plan *RunPlan) *MockProvider {
	return &MockProvider{fixedPlan: plan}
}

// Name returns the provider name
func (p *MockProvider) Name() string {
	return "mock"
}

// GeneratePlan generates a plan based on the context
func (p *MockProvider) GeneratePlan(ctx *PlanContext) (*RunPlan, error) {
	if p.fixedPlan != nil {
		return p.fixedPlan, nil
	}

	// Return appropriate default based on detected stack
	stack := "unknown"
	if ctx.Profile != nil {
		stack = ctx.Profile.Stack
	}

	return p.defaultPlanForStack(stack, ctx), nil
}

func (p *MockProvider) defaultPlanForStack(stack string, ctx *PlanContext) *RunPlan {
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

func (p *MockProvider) dockerPlan(ctx *PlanContext) *RunPlan {
	// Check if docker-compose exists
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

	plan := &RunPlan{
		Version:     "1",
		ProjectType: "docker",
		Prerequisites: []Prerequisite{
			{Name: "docker", Reason: "Docker required for containerized execution"},
		},
		Env:   make(map[string]string),
		Ports: []int{8080},
		Notes: []string{"Using Docker for isolated environment"},
	}

	if hasCompose {
		plan.Prerequisites = append(plan.Prerequisites,
			Prerequisite{Name: "docker-compose", Reason: "Docker Compose configuration detected"})
		plan.Steps = []Step{
			{ID: "build", Cmd: "docker compose build", Cwd: ".", Risk: RiskMedium},
			{ID: "run", Cmd: "docker compose up", Cwd: ".", Risk: RiskLow},
		}
	} else {
		plan.Steps = []Step{
			{ID: "build", Cmd: "docker build -t app .", Cwd: ".", Risk: RiskMedium},
			{ID: "run", Cmd: "docker run -p 8080:8080 app", Cwd: ".", Risk: RiskLow},
		}
	}

	return plan
}

func (p *MockProvider) nodePlan(ctx *PlanContext) *RunPlan {
	// Detect package manager
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

		// Prefer ci for locked dependencies
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

	return &RunPlan{
		Version:     "1",
		ProjectType: "node",
		Prerequisites: []Prerequisite{
			{Name: "node", Reason: "Node.js runtime required"},
			{Name: pkgManager, Reason: "Package manager for dependencies"},
		},
		Steps: []Step{
			{ID: "install", Cmd: installCmd, Cwd: ".", Risk: RiskMedium},
			{ID: "run", Cmd: pkgManager + " start", Cwd: ".", Risk: RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{3000},
		Notes: []string{"Using " + pkgManager + " package manager"},
	}
}

func (p *MockProvider) pythonPlan(ctx *PlanContext) *RunPlan {
	// Detect Python tool
	tool := "pip"
	installCmd := "pip install -r requirements.txt"

	if ctx.Profile != nil {
		for _, t := range ctx.Profile.Tools {
			switch t {
			case "poetry":
				tool = "poetry"
				installCmd = "poetry install"
			case "pipenv":
				tool = "pipenv"
				installCmd = "pipenv install"
			}
		}

		// Check for pyproject.toml
		for _, pkg := range ctx.Profile.Packages {
			if pkg == "pyproject.toml" && tool == "pip" {
				installCmd = "pip install -e ."
			}
		}
	}

	runCmd := "python -m app"
	if tool == "poetry" {
		runCmd = "poetry run python -m app"
	} else if tool == "pipenv" {
		runCmd = "pipenv run python -m app"
	}

	return &RunPlan{
		Version:     "1",
		ProjectType: "python",
		Prerequisites: []Prerequisite{
			{Name: "python", Reason: "Python runtime required", MinVersion: "3.8"},
			{Name: tool, Reason: "Package manager for dependencies"},
		},
		Steps: []Step{
			{ID: "install", Cmd: installCmd, Cwd: ".", Risk: RiskMedium},
			{ID: "run", Cmd: runCmd, Cwd: ".", Risk: RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{8000},
		Notes: []string{"Using " + tool + " for dependency management"},
	}
}

func (p *MockProvider) goPlan(ctx *PlanContext) *RunPlan {
	return &RunPlan{
		Version:     "1",
		ProjectType: "go",
		Prerequisites: []Prerequisite{
			{Name: "go", Reason: "Go compiler required", MinVersion: "1.21"},
		},
		Steps: []Step{
			{ID: "build", Cmd: "go build -o app .", Cwd: ".", Risk: RiskLow},
			{ID: "run", Cmd: "./app", Cwd: ".", Risk: RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{8080},
		Notes: []string{"Go project with modules"},
	}
}

func (p *MockProvider) rustPlan(ctx *PlanContext) *RunPlan {
	return &RunPlan{
		Version:     "1",
		ProjectType: "rust",
		Prerequisites: []Prerequisite{
			{Name: "cargo", Reason: "Rust build tool required"},
			{Name: "rustc", Reason: "Rust compiler required"},
		},
		Steps: []Step{
			{ID: "build", Cmd: "cargo build --release", Cwd: ".", Risk: RiskLow},
			{ID: "run", Cmd: "cargo run --release", Cwd: ".", Risk: RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{8080},
		Notes: []string{"Rust project using Cargo"},
	}
}

func (p *MockProvider) unknownPlan(ctx *PlanContext) *RunPlan {
	notes := []string{"Unknown project type - manual setup may be required"}

	// Try to provide something useful based on profile
	if ctx.Profile != nil && len(ctx.Profile.Tools) > 0 {
		notes = append(notes, "Detected tools: "+ctx.Profile.Tools[0])
	}

	return &RunPlan{
		Version:       "1",
		ProjectType:   "mixed",
		Prerequisites: []Prerequisite{},
		Steps: []Step{
			{ID: "info", Cmd: "echo 'Project type not determined. Please check README for instructions.'", Cwd: ".", Risk: RiskLow},
		},
		Env:   make(map[string]string),
		Ports: []int{},
		Notes: notes,
	}
}
