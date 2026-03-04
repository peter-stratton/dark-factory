```
     _            _           __            _
  __| | __ _ _ __| | __      / _| __ _  ___| |_ ___  _ __ _   _
 / _` |/ _` | '__| |/ /_____| |_ / _` |/ __| __/ _ \| '__| | | |
| (_| | (_| | |  |   <______|  _| (_| | (__| || (_) | |  | |_| |
 \__,_|\__,_|_|  |_|\_\     |_|  \__,_|\___|\__\___/|_|   \__, |
                                                           |___/
```

A Go CLI that orchestrates autonomous AI agents to implement GitHub issues,
review their own work, and merge — without human intervention. Supports Go,
Flutter, Node.js, Rust, and Python projects.

## How it works

Given a GitHub repo and a milestone, `godark` runs a two-agent development loop:

1. **Fetch** open issues from the milestone, sorted by priority (`p1` → `p2` → `p3` → unlabeled)
2. **Resolve dependencies** — issues declare `Blocked by: #N` or `Depends on: #N` in their body; skip any whose dependencies are still open
3. **Agent 1 (Implementer)** — Claude Code implements the issue, writes unit tests, and opens a PR
4. **Agent 2 (Reviewer)** — a separate Claude Code instance reviews the PR against human-authored scenario specs, generates ephemeral integration tests, and approves or requests changes
5. **Retry loop** — if the reviewer rejects, Agent 1 reads the review comments and pushes fixes (max N retries)
6. **Merge or escalate** — approved PRs are squash-merged; failed PRs are labeled `needs-human-review`
7. **Repeat** — move to the next unblocked issue

## Pre-run checklist

Before running the dev loop, use the planning skills (installed by `godark init`)
inside Claude Code to prepare your project:

| Step | Skill | What it does |
|------|-------|--------------|
| 1 | `/godark-create-roadmap <project-goal>` | Create a phased roadmap and GitHub milestones |
| 2 | `/godark-create-planning-doc <phase-number>` | Write a detailed planning doc for a roadmap phase |
| 3 | `/godark-create-issues <phase-number>` | Create GitHub issues from the planning doc phase #|
| 4 | `/godark-create-scenario <issue-number>` | Generate a scenario spec for each issue |
| 5 | `godark vet issues --repo owner/repo --tag phase-N` | Validate issues are agent-ready |
| 6 | `godark vet scenarios --repo owner/repo --tag phase-N` | Validate scenario specs |

Once vetting passes, kick off the loop:

```
godark run --milestone "Phase N" --repo owner/repo
```

## CLI reference

```
godark orchestrates autonomous AI agents to implement GitHub issues,
review their own work, and merge — without human intervention.

Usage:
  godark [command]

Available Commands:
  implement   Implement a single GitHub issue
  init        Initialize a project with godark skills and default config
  run         Run the development loop for a milestone or single issue
  status      Show summary of the most recent run
  version     Print the version and build time
  vet         Validate roadmap docs and issue quality for agent consumption
```

### godark run

```
Fetch issues from a GitHub milestone, resolve dependencies, and process
each unblocked issue through the implement → review → merge loop.

Usage:
  godark run [flags]

Flags:
      --config string      Path to configuration file (default "godark.yaml")
      --dry-run            Print execution plan without taking action
      --issue int          Single issue number to process (instead of milestone)
      --max-retries int    Maximum review/fix retry cycles per issue (default 2)
      --milestone string   GitHub milestone to process
      --no-sandbox         Run agents on host instead of in Docker
      --repo string        GitHub repository (owner/repo)
```

### godark implement

```
Fetch a GitHub issue by number and run the implement → review → merge
loop directly, without milestone or dependency resolution.

Usage:
  godark implement <issue-number> [flags]

Flags:
      --config string     Path to configuration file (default "godark.yaml")
      --dry-run           Print issue details and exit
      --max-retries int   Maximum review/fix retry cycles (default 2)
      --no-sandbox        Run agents on host instead of in Docker
      --repo string       GitHub repository (owner/repo)
```

### godark vet

```
Usage:
  godark vet [command]

Available Commands:
  issues      Validate GitHub issue structure for agent consumption
  scenarios   Validate scenario spec files
  roadmap     Validate planning docs against milestone issues
```

#### godark vet issues

```
Flags:
      --json               Output findings as JSON
      --milestone string   GitHub milestone to validate (exact title)
      --tag string         Milestone tag (e.g., phase-3) — resolved to full milestone title
      --repo string        GitHub repository (owner/repo)
```

#### godark vet scenarios

```
Flags:
      --json                  Output findings as JSON
      --milestone string      GitHub milestone to validate (exact title)
      --tag string            Milestone tag (e.g., phase-3) — resolved to full milestone title
      --repo string           GitHub repository (owner/repo)
      --scenario-dir string   Path to scenario spec directory (default "tests/scenarios/")
```

#### godark vet roadmap

```
Flags:
      --json                  Output findings as JSON
      --milestone string      GitHub milestone to validate (exact title)
      --tag string            Milestone tag (e.g., phase-3) — resolved to full milestone title
      --planning-dir string   Path to planning docs directory (default "docs/planning/")
      --repo string           GitHub repository (owner/repo)
```

### godark init

```
Write Claude Code skill files and a default godark.yaml to the current
directory. Skills are always overwritten (they are managed by godark).
The config file is only created if it does not already exist.

Usage:
  godark init [flags]
```

### godark status

```
Parse the latest run log and display a summary of issues processed,
PRs opened, and outcomes.

Usage:
  godark status [flags]
```

## Configuration

`godark init` creates a default `godark.yaml` in the project root. All fields
can be overridden by CLI flags where applicable.

```yaml
# Required — GitHub repository (owner/repo)
repo: owner/repo

# Required (one of) — target milestone or single issue
milestone: "Phase 1"
issue: 0

# Retry and execution
max_retries: 2            # review/fix cycles before escalating (default 2)
agent_timeout: "30m"      # max wall-clock time per agent run
no_sandbox: false         # run agents on host instead of Docker

# Build and test commands run inside the container (auto-detected if not set)
build_command: ""
test_command: ""

# Environment variables injected into the sandbox container
sandbox_env: {}

# Project runtime — auto-detected from go.mod, pubspec.yaml, etc. if not set
runtime:
  name: ""     # go, flutter, node, rust, python
  version: ""  # optional — auto-detected from go.mod, package.json engines, etc.

# Paths (relative to repo root)
protected_paths: []                    # files agents must never modify
roadmap_path: "docs/ROADMAP.md"
planning_dir: "docs/planning/"
scenario_dir: "tests/scenarios/"
review_dir:   "tests/review/"
log_dir:      "logs/"

# Docker sandbox settings
docker:
  image: ""                # base image (default: ubuntu:22.04)
  dockerfile: ""           # custom Dockerfile path (overrides generated one)
  mount: ""                # host path to mount into the container
  user: ""                 # non-root user inside the container
  node_version: ""         # Node.js major version to install (default: 20)
  extra_packages: []       # additional apt packages to install

# Prompt template overrides (paths to custom prompt files)
prompts:
  implementer: ""
  implementer_retry: ""
  reviewer: ""
  spec_generator: ""
```

## Supported runtimes

`godark` auto-detects the project language by scanning the repo root for well-known marker files. Detection applies only when `runtime`, `build_command`, and `test_command` are all absent from the config.

| Runtime  | Marker file       | Default build command | Default test command |
|----------|-------------------|-----------------------|----------------------|
| go       | `go.mod`          | `go build ./...`      | `go test ./...`      |
| flutter  | `pubspec.yaml`    | _(none)_              | `flutter test`       |
| node     | `package.json`    | `npm run build`       | `npm test`           |
| rust     | `Cargo.toml`      | `cargo build`         | `cargo test`         |
| python   | `pyproject.toml` or `requirements.txt` | _(none)_ | `pytest` |

If no marker file is found, `godark` proceeds without installing a language toolchain. Use a custom `docker.dockerfile` or set `runtime:` explicitly for unsupported languages.

## Building

```bash
go build -o bin/godark ./cmd/godark
go test ./...
```

## Status

See [docs/ROADMAP.md](docs/ROADMAP.md) for the full development roadmap.
