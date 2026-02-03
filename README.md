# readme-runner

> **One command. Any README. Automatic installation + launch.**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**readme-runner** (`rd-run`) is a fast, secure CLI tool that automatically installs and runs any software project by intelligently analyzing its README and project files.

```bash
# Clone and run any project with one command
rd-run https://github.com/user/awesome-project
```

---

## ✨ Features

- **🧠 README-first Intelligence** — Analyzes README.md to understand how to build and run your project
- **📁 Smart Fallback** — Uses project files (Dockerfile, package.json, go.mod, etc.) when README is unclear
- **🔒 Security-first** — Dry-run by default, sudo confirmation, command blocklist
- **🤖 AI-Powered Plans** — Uses GitHub Copilot API or custom LLM to generate installation plans
- **🐳 Docker Preferred** — Automatically uses Docker/Compose when available for isolation
- **📦 Multi-Stack Support** — Node.js, Python, Go, Rust, Docker, and mixed projects
- **✅ Prerequisite Checking** — Verifies tools are installed before running
- **🔄 Error Recovery** — Retry, continue, or abort on failures

---

## 📥 Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/sony-level/readme-runner.git
cd readme-runner

# Build the binary
make build

# Or install to $GOPATH/bin
go install .
```

### Binary Location

After building, binaries are available at:
- `./rd-run` — Main binary
- `./readme-run` — Alias

Add to your PATH for global access:

```bash
# Add to ~/.bashrc or ~/.zshrc
export PATH="$PATH:/path/to/readme-runner"
```

---

## 🚀 Quick Start

### Basic Usage

```bash
# Analyze current directory (dry-run by default)
rd-run .

# Analyze a GitHub repository
rd-run https://github.com/expressjs/express

# Actually execute the plan
rd-run . --dry-run=false

# Execute with auto-confirm (except sudo)
rd-run . --dry-run=false --yes
```

### Example Output

```
Run ID: rr-20260203-1542-abc
Input: https://github.com/user/project
Source type: github

[DRY-RUN MODE] No commands will be executed.

[1/7] Fetch / Workspace
  → Workspace ready at .rr-temp/rr-20260203-1542-abc
  → Fetched 142 files (1.2 MB)

[2/7] Scan
  → README found: README.md (4.2 KB)
  → Primary stack: node
  → Stack Detection: node (confidence: 0.85)

[3/7] Plan (AI)
  → README clarity score: 0.80
  → Using README as primary source
  → Using LLM provider: copilot
  → Plan generated: node project with 2 steps

[4/7] Validate / Normalize
  → ✓ Plan is valid
  → Risk summary: Low=1, Medium=1, High=0, Critical=0

[5/7] Prerequisites
  → ✓ All 2 prerequisites available
  → ✓ node: v20.10.0
  → ✓ npm: 10.2.3

[6/7] Execute

╔══════════════════════════════════════════════════════════════╗
║                    DRY-RUN MODE                              ║
║              No commands will be executed                    ║
╚══════════════════════════════════════════════════════════════╝

Project type: node
Working directory: .rr-temp/rr-20260203-1542-abc/repo

Prerequisites:
  • node (>= 18) - Node.js runtime required
  • npm - Package manager for dependencies

Steps to execute:

  [1] install
      Command: npm ci
      Risk: medium

  [2] run
      Command: npm start
      Risk: low

Exposed ports:
  • 3000

[7/7] Post-run / Cleanup
  → Workspace will be cleaned up

  To execute this plan, run again without --dry-run:
    rd-run https://github.com/user/project --dry-run=false
```

---

## 📋 Command Reference

### Syntax

```bash
rd-run [command] [path|url] [flags]
```

### Commands

| Command | Description |
|---------|-------------|
| `run` | Run installation from README (default) |
| `help` | Help about any command |
| `completion` | Generate shell autocompletion |

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `true` | Show plan without executing |
| `--yes`, `-y` | `false` | Auto-accept prompts (except sudo) |
| `--verbose`, `-v` | `false` | Enable verbose output |
| `--keep` | `false` | Keep workspace after execution |
| `--allow-sudo` | `false` | Allow sudo without confirmation |

### LLM Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--llm-provider` | `copilot` | LLM provider: `copilot`, `http`, `mock` |
| `--llm-endpoint` | — | HTTP endpoint for custom LLM |
| `--llm-model` | — | Model name for LLM provider |
| `--llm-token` | — | Auth token (prefer `GITHUB_TOKEN` env) |

---

## 🔒 Security

### Design Principles

1. **Dry-run by default** — Nothing executes without explicit `--dry-run=false`
2. **Sudo requires consent** — Even with `--yes`, sudo commands prompt for approval
3. **Command blocklist** — Dangerous commands are blocked (see below)
4. **LLM output validation** — AI-generated plans are validated against security policy
5. **Workspace isolation** — All operations happen in `.rr-temp/<run-id>/`

### Blocked Commands

The following patterns are **always blocked**:

```
rm -rf /          # Root deletion
rm -rf ~          # Home deletion
mkfs              # Filesystem creation
dd if=/dev/zero   # Disk wiping
shutdown/reboot   # System control
chmod -R 777 /    # Dangerous permissions
:(){:|:&};:       # Fork bomb
```

### Sudo Handling

When a command requires sudo, you'll see:

```
╔══════════════════════════════════════════════════════════════╗
║                    SUDO REQUIRED                             ║
╚══════════════════════════════════════════════════════════════╝

  Step:    install-docker
  Command: sudo apt install docker.io

  This command requires elevated (sudo) privileges.

  Choose an option:
    1) Allow for this step only
    2) Allow for all sudo steps in this run
    3) Show manual instructions (skip this step)
    4) Abort entire operation

  Enter choice [1-4]:
```

### Risk Levels

| Level | Description | Examples |
|-------|-------------|----------|
| `low` | Safe, read-only operations | `echo`, `cat`, `ls` |
| `medium` | Modifies local files | `npm install`, `pip install --user` |
| `high` | System package managers | `apt install`, `brew install` |
| `critical` | Requires sudo or system changes | `sudo ...`, remote scripts |

---

## 🔧 How It Works

### Pipeline Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         readme-runner                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  [1] Input          Path or GitHub URL                          │
│       ↓                                                         │
│  [2] Fetch          Clone repo / Copy local files               │
│       ↓                                                         │
│  [3] Scan           Detect files + Parse README                 │
│       ↓                                                         │
│  [4] Stack Detect   Identify: docker/node/python/go/rust        │
│       ↓                                                         │
│  [5] Plan (AI)      Generate RunPlan JSON via LLM               │
│       ↓                                                         │
│  [6] Validate       Security policy + Risk assessment           │
│       ↓                                                         │
│  [7] Normalize      Adapt commands for OS/lockfiles             │
│       ↓                                                         │
│  [8] Prerequisites  Check required tools are installed          │
│       ↓                                                         │
│  [9] Execute        Run steps (or display in dry-run)           │
│       ↓                                                         │
│  [10] Cleanup       Remove workspace (unless --keep)            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### README-first Strategy

The tool calculates a **clarity score** (0.0-1.0) for the README:

| Factor | Points |
|--------|--------|
| Has "Installation" section | +1.0 |
| Has "Usage" section | +1.0 |
| Has "Quick Start" section | +0.5 |
| Has "Build" section | +0.5 |
| Has 3+ code blocks | +1.0 |
| Has 2+ shell commands | +1.0 |

- **Score ≥ 0.6**: README is primary source
- **Score < 0.6**: Project files are primary source

### Supported Stacks

| Stack | Detection Files |
|-------|-----------------|
| **Docker** | `Dockerfile`, `docker-compose.yml`, `compose.yaml` |
| **Node.js** | `package.json`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml` |
| **Python** | `pyproject.toml`, `requirements.txt`, `Pipfile`, `setup.py` |
| **Go** | `go.mod`, `go.sum` |
| **Rust** | `Cargo.toml`, `Cargo.lock` |
| **Java** | `pom.xml`, `build.gradle` |

---

## 🤖 LLM Providers

### GitHub Copilot (Default)

Uses GitHub Copilot API with your GitHub token:

```bash
# Set token via environment variable (recommended)
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"

# Or via flag
rd-run . --llm-token "ghp_xxxxxxxxxxxx"
```

### Custom HTTP Provider

Connect to any OpenAI-compatible API:

```bash
rd-run . --llm-provider http \
         --llm-endpoint "http://localhost:11434/v1/chat/completions" \
         --llm-token "your-token" \
         --llm-model "llama3"
```

### Mock Provider (Testing)

Returns predefined plans based on detected stack:

```bash
rd-run . --llm-provider mock
```

---

## 📄 RunPlan JSON Schema

The LLM generates plans in this format:

```json
{
  "version": "1",
  "project_type": "node",
  "prerequisites": [
    {
      "name": "node",
      "reason": "Node.js runtime required",
      "min_version": "18"
    }
  ],
  "steps": [
    {
      "id": "install",
      "cmd": "npm ci",
      "cwd": ".",
      "risk": "medium",
      "requires_sudo": false,
      "timeout": 300,
      "description": "Install dependencies"
    },
    {
      "id": "run",
      "cmd": "npm start",
      "cwd": ".",
      "risk": "low",
      "requires_sudo": false
    }
  ],
  "env": {
    "NODE_ENV": "production"
  },
  "ports": [3000],
  "notes": [
    "Application will be available at http://localhost:3000"
  ]
}
```

### Field Reference

| Field | Required | Description |
|-------|----------|-------------|
| `version` | ✅ | Schema version (always `"1"`) |
| `project_type` | ✅ | `docker`, `node`, `python`, `go`, `rust`, `mixed` |
| `prerequisites` | ✅ | Required tools with reasons |
| `steps` | ✅ | Ordered execution steps |
| `env` | ❌ | Environment variables |
| `ports` | ❌ | Exposed ports |
| `notes` | ❌ | Additional information |

---

## 🛠️ Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | GitHub token for Copilot API |
| `GH_TOKEN` | Alternative GitHub token variable |
| `RD_LLM_TOKEN` | Custom LLM token |

### Workspace Structure

```
.rr-temp/
└── rr-20260203-1542-abc/     # Run ID
    ├── repo/                  # Cloned/copied project
    ├── plan/                  # Generated plan files
    └── logs/                  # Execution logs
```

---

## 📊 Examples

### Run a Node.js Project

```bash
rd-run https://github.com/expressjs/express --dry-run=false --yes
```

### Run a Python Project with Poetry

```bash
rd-run https://github.com/python-poetry/poetry --verbose
```

### Run a Docker Compose Project

```bash
rd-run . --dry-run=false
# Docker Compose projects auto-detect and use: docker compose up
```

### Use Local LLM (Ollama)

```bash
rd-run . --llm-provider http \
         --llm-endpoint "http://localhost:11434/api/chat" \
         --llm-model "codellama"
```

### Keep Workspace for Debugging

```bash
rd-run https://github.com/user/project --keep --verbose
# Workspace preserved at .rr-temp/rr-xxxxx/
```

---

## 🧪 Development

### Build

```bash
make build          # Build binaries
make test           # Run tests
make lint           # Run linter (requires golangci-lint)
make clean          # Clean build artifacts
```

### Project Structure

```
readme-runner/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command + flags
│   └── run.go             # Main execution pipeline
├── internal/
│   ├── config/            # Configuration management
│   ├── workspace/         # Temp workspace handling
│   ├── fetcher/           # Git clone / local copy
│   ├── scanner/           # File detection + README parsing
│   ├── stacks/            # Stack detectors (docker/node/etc.)
│   ├── llm/               # LLM providers (copilot/http/mock)
│   ├── plan/              # Plan validation + normalization
│   ├── prereq/            # Prerequisite checking
│   ├── exec/              # Step execution
│   ├── security/          # Security policies + sudo guard
│   └── ui/                # Terminal output formatting
├── docs/                   # Documentation
├── test/                   # Test fixtures
└── scripts/               # Development scripts
```

### Running Tests

```bash
# All tests
go test ./... -v

# Specific package
go test ./internal/llm/tests/... -v

# With coverage
go test ./... -cover
```

---

## 🤝 Contributing

Contributions are welcome! Please see our contributing guidelines.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI
- Powered by [GitHub Copilot](https://github.com/features/copilot) for AI planning

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/sony-level">ソニーレベル</a>
</p>
