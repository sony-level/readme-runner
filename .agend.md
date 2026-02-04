Here is a complete **AGEND.md** to guide the creation of **readme-runner** until the deadline. It integrates **structure**, **operation**, **implementation order**, **DoD**, **AI prompts**, **testing**, **security**, and a **day-by-day roadmap**.

```md
# AGEND.md — readme-runner (readme-run / rd-run)
> “An order. Any README. Installation + automatic launch. »

## 0) Objective & constraints
### Objective
Create a standalone CLI Go tool that:
- takes a local repo or GitHub URL,
- README analysis + key files,
- generates an installation/execution plan (strict JSON) via IA (Copilot CLI by default),
- checks/installs the prerequisites (with security),
- executes (or simulates) the plan.

### Deadline
- Current date: **2026-02-01**
- Deadline: **2026-02-15**
- Priority: **Stable MVP + solid demo + DEV.to article**

### Non-negotiable (MVP)
- `--dry-run` by default
- strict and validated JSON plan
- confirmations for risky actions (sudo, remote scripts, destructive)
- support: Docker/Compose + Node + Python + Go + Rust (MVP)
- targeted success rate >80% on a test rest set

---

## 1) CLI & UX commands (user contract)
### Name and alias
- main command: `readme-run`
- alias: `rd-run`

### Usage
- `rd-run <path|github_url>`
- Examples:
- `rd-run.`
- `rd-run https://github.com/sony-level/readme-runner`
- `rd-run https://github.com/sony-level/readme-runner --yes --verbose`
- `rd-run . --dry-run`
- `rd-run . --llm-provider http --llm-endpoint ... --llm-token ...`

### Flags (MVP)
- `--dry-run`: does not run anything, displays the plan + commands
- `--yes`: auto-accept (except reinforced security rules)
- `--verbose`: detailed logs (without exposing tokens)
- `--keep`: keeps `.rr-temp/<run-id>`
- `--docker-preferred` (default true)
- `--podman-preferred` (phase 2)
- `--minikube-preferred` (phase 2)
- AI:
- `--llm-provider copilot|http|mock`
- `--llm-model <name>`
- `--llm-endpoint <url>`
- `--llm-token <token>`

### Expected terminal output (UX)
- clear phases:
1) Fetch / Workspace
2) Scan
3) Plan (AI)
4) Validate / Normalize
5) Prerequisites
6) Execute (or Dry-run)
7) Post-run / Cleanup
- colorful + readable display, errors explained

---

## 2) Architecture (repo structure)
### Recommended tree
```

readme-runner/
cmd/
root.go
run.go
internal/
config/ # env + flags
workspace/ # .rr-temp + run-id + cleanup
scanner/ # signals + README chunking
stacks/ # detectors (docker/node/python/go/rust/...)
plan/ # RunPlan schema + validation + normalize
llm/ # CopilotProvider + HTTPProvider + MockProvider
prereq/ # check/install tools (policy)
exec/ # runner (timeouts, logs stream)
security/ # policy commands + sudo + remote scripts
ui/ # format output (lipgloss)
docs/
ARCHITECTURE.md
SECURITY.md
scripts/
dev-setup.sh
.github/workflows/
ci.yml
.gitignore
.golangci.yml
Makefile
go.mod
README.md

````

---

## 3) Internal functioning (pipeline)
### Pipeline (fixed order)
1. **InputResolver**
- determine if `path` or `url`
2. **Workspace**
- create `.rr-temp/<run-id>`
3. **Fetcher**
- git clone (url) or copy (local)
4. **Scan**
- collect signals (key files + README)
- produce `ProjectProfile`
5. **Stack Detection**
- detectors (docker/python/node/go/rust…) → priorities
6. **Plan Builder**
- IA structured prompt → `RunPlan JSON`
7. **Validate + Normalize**
- JSON schema, security policy, OS/lockfiles standardization
8. **Prereq Manager**
- check tools & suggest install
9. **Executor**
- execute steps (or simulation)
10. **Error Repair Loop (option)**
- if fail → prompt fix plan → patch → revalidate
11. **Post-run**
- print ports / next steps
12. **Cleanup**
- delete workspace except `--keep`

---

## 4) Data Contract (RunPlan JSON)
### Objective
The LLM should produce strict JSON, not free text.

### Model (MVP)
```json
{
"version": "1",
"project_type": "docker|node|python|go|rust|mixed",
"prerequisites": [
{ "name": "docker", "reason": "docker-compose.yml detected" }
],
"steps": [
{ "id": "install", "cmd": "npm ci", "cwd": ".", "risk": "low" },
{ "id": "run", "cmd": "npm run dev", "cwd": ".", "risk": "low" }
],
"env": { "EXAMPLE": "value" },
"ports": [3000],
"notes": ["If error: ..."]
}
````

### Rules (validation)

* `version` required
* `steps[].cmd` not empty
* refuse clearly destructive commands
* `curl|sh` = prohibited by default except whitelist + confirmation
* `sudo` = confirmation
* no token in `env` logs

---

## 5) Security (policies)

### Defaults

* `dry-run` by default: no execution without agreement
* confirmations:

* `sudo`
* remote scripts
* system modification (installers)
* risky orders

### Minimal blocklist (examples)

* `rm -rf /`
* `mkfs`
* `dd if=`
* `shutdown`, `reboot`
* `chmod -R 777 /`
* `useradd`, `passwd` (out of scope)
* writing outside of workspace not authorized (except controlled exceptions)

### Whitelist “official installers” (phase 2)

* rustup (official)
* go install (modules)
* uv/poetry (official)
* minikube (official)

> Each URL displayed + confirm if executed

---

## 6) Step by step roadmap (implementation)

> Each step has a DoD (Definition of Done). Do not proceed until DoD is reached.

### Step A — Bootstrap repo (J1)

**Tasks**

* init repo Go + cobra
* Makefile + lint + minimal CI
* `rd-run --help` works

**DoD**

* `make test` OK
* `golangci-lint run` OK
* binary compiles on your OS

---

### Step B — Workspace + Fetcher (J1-J2)

**Tasks**

* `workspace.New(runID)` creates `.rr-temp/<run-id>`
*Fetcher:

* GitHub URL → clone
* local path → copy
* option `--keep` keeps

**DoD**

* `rd-run https://github.com/... --dry-run` clone without errors
* cleanup OK if not `--keep`

---

### Step C — Scanner + ProjectProfile (J2)

**Tasks**

* detect key files:

* Docker: Dockerfile, compose
*Node:package.json(+lock)
* Python: pyproject.toml, requirements.txt
* Go:go.mod
* Rust: Cargo.toml
* K8s: yaml manifests (phase 2)
* load README (limited size + chunking)

**DoD**

* displays a `profile` summary in verbose
* walks over 5 test rests

---

### Step D — Stack Detectors (J2)

**Tasks**

* implement Detector interface
* MVP detectors:

* docker/compose
*node
*python
*go
*rust
* calculate a “dominant stack”

**DoD**

* on a node repo → `node` detected
* on docker-compose repo → `docker` dominant

---

### Step E — Plan (MockProvider first) (J3)

**Tasks**

* create Go types `RunPlan`, `Step`, `Prerequisite`
* implement `MockProvider` (fixed plane)
* `--dry-run` displays plan + commands
* `plan.Validate()` minimal

**DoD**

* `rd-run . --llm-provider mock --dry-run` shows a valid plan

---

### Step F — IA Copilot CLI (J3-J4)

**Tasks**

* `CopilotProvider` runs `copilot` via `os/exec`
* structured prompt → strict JSON
* parse JSON + validation
* hide tokens, logs safe

**DoD**

* `rd-run <repo-test> --dry-run` produces a realistic plan via Copilot
* 10 consecutive runs without crash analysis

---

### Step G — HTTPProvider (LLM custom) (J4)

**Tasks**

* HTTP provider (endpoint + token)
* identical contract (return JSON plan)
* timeout management

**DoD**

* test with a local mock endpoint (simple server)
* tokens never logged

---

### Step H — Prereq checker (MVP “detect + guide”) (J5)

**Tasks**

* `prereq.Check()`:

* docker, git, node, python, go, rustup, cargo
* MVP mode:

* if missing: suggest instructions (not necessarily auto-install at the beginning)
* phase 2: controlled self-install

**DoD**

* on machine without docker: clear message + abort option
* on machine with docker: OK

---

### Stage I — Executor (J5-J6)

**Tasks**

* runner with streaming logs
* timeouts
* `--yes` skips prompts
* `--dry-run` does not run anything

**DoD**

* real execution on 3 simple repos (node/python/go)
* readable logs

---

### Stage J — Enhanced Security (J6)

**Tasks**

* policy blocklist + sudo guard + remote scripts guard
* plan validation before execution
* refuse orders outside of policy

**DoD**

* a malicious plan is refused
* a normal plan passes

---

### Step K — Repair Loop (optional, if time) (J7)

**Tasks**

* on error: capture stderr tail + step
* request minimal patch from the LLM
* revalidate then suggest apply

**DoD**

* on a “broken” repo, offers a useful fix at least 1 out of 3 times

---

## 7) Test plan (quality + challenge score)

### Dataset rest tests (minimum 20)

Distribute:

* 5 Nodes
*5 Python
* 3 GB
*3 Rust
* 4 Docker/Compose

### Types of tests

* unit:

* stack detection
* plan validation
* security policy
* integration:

* dry-run run on 20 rests
* real run on 5 “safe” rests

### Metrics

* % plan success (parse + validation)
* % complete dry-run success
* % real run success (5 rests)
* average plan time < 5s (target)

---

## 8) Documentation & deliverables (to win)

### Essential documents

*README:

* gif/terminal demo
* binary installation
*examples
* security (dry-run default)
* docs/ARCHITECTURE.md:

* pipeline + modules
* docs/SECURITY.md:

* policies, confirmations, blocklist

### Item DEV.to (structure)

* problem → solution
* demo
* how Copilot CLI is used (prompt + JSON)
* security + design decisions
* results on rest tests

---

## 9) Prompts IA (Copilot CLI) — strict formats

> Always require unique JSON, with no text around it.

### Prompt 1 — Generate plan

**Input**

* chunked README
* ProjectProfile (files detected, OS, dominant stack)
**Instruction**
* produce RunPlan JSON v1
* never include secrets

### Prompt 2 — Standardize / improve plan

* adapt commands to lockfiles
* prefer docker compose if compose present
* avoid sudo

### Prompt 3 — Repair patch

* provide minimal patch:

* replace command
* add step
* add non-sensitive env variable

---

## 10) Final checklist (before release)

* [ ] `--dry-run` default OK
* [ ] validated JSON plan
* [ ] no token leak in logs
* [ ] dataset 20 rest + results
* [ ] Green IC
* [ ] binary builds (Linux/macOS at least)
* [ ] docs + ready article

---

## 11) Recommended schedule (from February 1 to 15, 2026)

* 01-02: Bootstrap + Workspace + Scanner
* 03-04: Plan + Mock + CopilotProvider
* 05-06: Executor + Prereq checker + Security
* 07-10: dataset rest tests + fixed
* 11-12: UX polish + docs + CI + packaging
* 13-14: DEV.to article + demo + final tests
* 15: release + submission

```
