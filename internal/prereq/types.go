/// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Prerequisite types and tool definitions

package prereq

// Tool represents a prerequisite tool
type Tool struct {
	Name         string   // Tool name
	Command      string   // Command to check existence
	VersionCmd   string   // Command to get version
	Alternatives []string // Alternative command names
	InstallGuide string   // Installation instructions
	Category     string   // Category (runtime, build, container, etc.)
}

// DefaultTools returns the list of supported tools
func DefaultTools() map[string]*Tool {
	return map[string]*Tool{
		"git": {
			Name:       "git",
			Command:    "git",
			VersionCmd: "git --version",
			Category:   "vcs",
			InstallGuide: `Install git:
  macOS:   brew install git
  Ubuntu:  sudo apt install git
  Fedora:  sudo dnf install git
  Windows: https://git-scm.com/download/win`,
		},
		"docker": {
			Name:       "docker",
			Command:    "docker",
			VersionCmd: "docker --version",
			Category:   "container",
			InstallGuide: `Install Docker:
  macOS:   brew install --cask docker
  Ubuntu:  https://docs.docker.com/engine/install/ubuntu/
  Fedora:  https://docs.docker.com/engine/install/fedora/
  Windows: https://docs.docker.com/desktop/install/windows-install/`,
		},
		"docker-compose": {
			Name:         "docker-compose",
			Command:      "docker-compose",
			VersionCmd:   "docker-compose --version",
			Alternatives: []string{"docker compose"},
			Category:     "container",
			InstallGuide: `Docker Compose is included with Docker Desktop.
For standalone installation:
  Linux:   sudo apt install docker-compose-plugin
  Or use:  docker compose (v2 built into Docker)`,
		},
		"node": {
			Name:         "node",
			Command:      "node",
			VersionCmd:   "node --version",
			Alternatives: []string{"nodejs"},
			Category:     "runtime",
			InstallGuide: `Install Node.js:
  macOS:   brew install node
  Ubuntu:  sudo apt install nodejs npm
  Fedora:  sudo dnf install nodejs npm
  All:     https://nodejs.org/en/download/

  Recommended: Use nvm for version management
    curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash`,
		},
		"npm": {
			Name:       "npm",
			Command:    "npm",
			VersionCmd: "npm --version",
			Category:   "package",
			InstallGuide: `npm is included with Node.js.
Install Node.js to get npm.`,
		},
		"yarn": {
			Name:       "yarn",
			Command:    "yarn",
			VersionCmd: "yarn --version",
			Category:   "package",
			InstallGuide: `Install Yarn:
  npm:     npm install -g yarn
  macOS:   brew install yarn
  Ubuntu:  https://classic.yarnpkg.com/en/docs/install`,
		},
		"pnpm": {
			Name:       "pnpm",
			Command:    "pnpm",
			VersionCmd: "pnpm --version",
			Category:   "package",
			InstallGuide: `Install pnpm:
  npm:     npm install -g pnpm
  macOS:   brew install pnpm
  curl:    curl -fsSL https://get.pnpm.io/install.sh | sh -`,
		},
		"bun": {
			Name:       "bun",
			Command:    "bun",
			VersionCmd: "bun --version",
			Category:   "runtime",
			InstallGuide: `Install Bun:
  macOS/Linux: curl -fsSL https://bun.sh/install | bash
  Windows:     powershell -c "irm bun.sh/install.ps1 | iex"`,
		},
		"python": {
			Name:         "python",
			Command:      "python3",
			VersionCmd:   "python3 --version",
			Alternatives: []string{"python"},
			Category:     "runtime",
			InstallGuide: `Install Python:
  macOS:   brew install python
  Ubuntu:  sudo apt install python3 python3-pip python3-venv
  Fedora:  sudo dnf install python3 python3-pip
  Windows: https://www.python.org/downloads/

  Recommended: Use pyenv for version management`,
		},
		"pip": {
			Name:         "pip",
			Command:      "pip3",
			VersionCmd:   "pip3 --version",
			Alternatives: []string{"pip"},
			Category:     "package",
			InstallGuide: `pip is included with Python 3.4+.
If missing:
  python3 -m ensurepip --upgrade`,
		},
		"poetry": {
			Name:       "poetry",
			Command:    "poetry",
			VersionCmd: "poetry --version",
			Category:   "package",
			InstallGuide: `Install Poetry:
  curl:    curl -sSL https://install.python-poetry.org | python3 -
  pipx:    pipx install poetry`,
		},
		"pipenv": {
			Name:       "pipenv",
			Command:    "pipenv",
			VersionCmd: "pipenv --version",
			Category:   "package",
			InstallGuide: `Install Pipenv:
  pip:     pip install --user pipenv
  macOS:   brew install pipenv`,
		},
		"go": {
			Name:       "go",
			Command:    "go",
			VersionCmd: "go version",
			Category:   "runtime",
			InstallGuide: `Install Go:
  macOS:   brew install go
  Ubuntu:  sudo apt install golang-go
  Fedora:  sudo dnf install golang
  All:     https://go.dev/dl/`,
		},
		"cargo": {
			Name:       "cargo",
			Command:    "cargo",
			VersionCmd: "cargo --version",
			Category:   "build",
			InstallGuide: `Install Rust/Cargo:
  All:     curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh`,
		},
		"rustc": {
			Name:       "rustc",
			Command:    "rustc",
			VersionCmd: "rustc --version",
			Category:   "runtime",
			InstallGuide: `rustc is installed with Cargo via rustup.
  All:     curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh`,
		},
		"rustup": {
			Name:       "rustup",
			Command:    "rustup",
			VersionCmd: "rustup --version",
			Category:   "build",
			InstallGuide: `Install rustup:
  All:     curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh`,
		},
		"make": {
			Name:       "make",
			Command:    "make",
			VersionCmd: "make --version",
			Category:   "build",
			InstallGuide: `Install Make:
  macOS:   xcode-select --install
  Ubuntu:  sudo apt install build-essential
  Fedora:  sudo dnf install make`,
		},
		"java": {
			Name:         "java",
			Command:      "java",
			VersionCmd:   "java --version",
			Alternatives: []string{"java"},
			Category:     "runtime",
			InstallGuide: `Install Java:
  macOS:   brew install openjdk
  Ubuntu:  sudo apt install default-jdk
  Fedora:  sudo dnf install java-latest-openjdk`,
		},
		"maven": {
			Name:       "maven",
			Command:    "mvn",
			VersionCmd: "mvn --version",
			Category:   "build",
			InstallGuide: `Install Maven:
  macOS:   brew install maven
  Ubuntu:  sudo apt install maven
  Fedora:  sudo dnf install maven`,
		},
		"gradle": {
			Name:       "gradle",
			Command:    "gradle",
			VersionCmd: "gradle --version",
			Category:   "build",
			InstallGuide: `Install Gradle:
  macOS:   brew install gradle
  Ubuntu:  sudo apt install gradle
  SDKMAN:  sdk install gradle`,
		},
	}
}

// CheckResult contains the result of checking a tool
type CheckResult struct {
	Name    string // Tool name
	Found   bool   // Whether tool was found
	Version string // Detected version (if found)
	Path    string // Path to tool (if found)
	Error   error  // Error during check (if any)
}

// CheckSummary contains results for all checks
type CheckSummary struct {
	Results      []CheckResult // Individual results
	AllFound     bool          // Whether all tools were found
	MissingTools []string      // List of missing tool names
}

// NewCheckSummary creates a new check summary
func NewCheckSummary() *CheckSummary {
	return &CheckSummary{
		Results:      []CheckResult{},
		AllFound:     true,
		MissingTools: []string{},
	}
}

// AddResult adds a check result to the summary
func (s *CheckSummary) AddResult(result CheckResult) {
	s.Results = append(s.Results, result)
	if !result.Found {
		s.AllFound = false
		s.MissingTools = append(s.MissingTools, result.Name)
	}
}
