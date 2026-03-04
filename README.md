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

## Prerequisites

### Authentication

| Variable | Required | Source |
|----------|----------|--------|
| `CLAUDE_CODE_OAUTH_TOKEN` or `ANTHROPIC_API_KEY` | Yes (one of) | Claude Code subscription token or Anthropic API key |
| `GH_TOKEN` | Yes | GitHub personal access token, or run `gh auth login` |

If both Anthropic tokens are set, `CLAUDE_CODE_OAUTH_TOKEN` takes priority.

`GH_TOKEN` can be set explicitly, or `godark` will attempt to retrieve it
automatically by running `gh auth token` (which requires a prior
`gh auth login`). If neither source provides a token, `godark` exits with
an error.

### Host tools

| Tool | Required | Notes |
|------|----------|-------|
| Docker | Yes (unless `--no-sandbox`) | Daemon must be running; used to build and run agent containers |
| `gh` | Yes | GitHub CLI — used for issue fetching, PR operations, and git auth |
| `git` | Yes | Repository operations and post-merge pulls |
| `python3` | Only in `--no-sandbox` mode | Runs the embedded `agent_runner.py` on the host; in sandbox mode, Python is pre-installed in the container |

### A note on `--no-sandbox`

> **Warning:** In `--no-sandbox` mode, agents run directly on your host machine
> with full filesystem and network access. There is no isolation — an agent can
> read, modify, or delete any file your user account can access, install
> packages, and execute arbitrary shell commands. Use this mode only for
> development and debugging on machines where you accept that risk. For
> production runs, always use the default Docker sandbox.

## How it works

Given a GitHub repo and a milestone, `godark` runs a three-agent development loop:

1. **Fetch** open issues from the milestone, sorted by priority (`p1` → `p2` → `p3` → unlabeled)
2. **Resolve dependencies** — issues declare `Blocked by: #N` or `Depends on: #N` in their body; skip any whose dependencies are still open
3. **Implementer** — Claude Code implements the issue, writes unit tests, and opens a PR
4. **Guard rails** — verify the PR exists, contains `Closes #N`, and didn't touch protected files
5. **Quality reviewer** — a separate Claude Code instance audits the PR for security, performance, and code quality issues; if it requests changes, the implementer retries before functional review begins
6. **Functional reviewer** — another Claude Code instance reviews the PR against human-authored scenario specs, generates ephemeral integration tests, and approves or requests changes
7. **Retry loop** — if either reviewer rejects, the implementer reads the review comments and pushes fixes (max N retries per gate)
8. **Merge or escalate** — approved PRs are squash-merged; failed PRs are labeled `needs-human-review`
9. **Repeat** — move to the next unblocked issue

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

## Supported runtimes

`godark` auto-detects the project language by scanning the repo root for well-known marker files. Detection applies only when `runtime`, `build_command`, and `test_command` are all absent from the config.

| Runtime  | Marker file       | Default build command | Default test command |
|----------|-------------------|-----------------------|----------------------|
| go       | `go.mod`          | `go build ./...`      | `go test ./...`      |
| flutter  | `pubspec.yaml`    | _(none)_              | `flutter test`       |
| node     | `package.json`    | `npm run build`       | `npm test`           |
| rust     | `Cargo.toml`      | `cargo build`         | `cargo test`         |
| elixir   | `mix.exs`         | `mix compile`         | `mix test`           |
| python   | `pyproject.toml` or `requirements.txt` | _(none)_ | `pytest` |

If no marker file is found, `godark` proceeds without installing a language toolchain. Use a custom `docker.dockerfile` or set `runtime:` explicitly for unsupported languages.

## CLI reference

```
godark orchestrates autonomous AI agents to implement GitHub issues,
review their own work, and merge — without human intervention.

It fetches issues from a milestone, resolves dependencies, and runs a
three-agent loop: one agent implements, a second audits code quality,
and a third reviews against scenario specs. Approved PRs are
squash-merged automatically.

Usage:
  godark [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
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
      --max-retries int    Maximum review/fix retry cycles per issue (default 3)
      --milestone string   GitHub milestone to process (exact title)
      --no-sandbox         Run agents on host instead of in Docker
      --repo string        GitHub repository (owner/repo)
      --tag string         Milestone tag (e.g., phase-3) — resolved to full milestone title
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
      --max-retries int   Maximum review/fix retry cycles (default 3)
      --no-sandbox        Run agents on host instead of in Docker
      --repo string       GitHub repository (owner/repo)
```

### godark vet

```
Check that issues have clear acceptance criteria, correct blocker
notations, and are fully actionable by agents.

Usage:
  godark vet [command]

Available Commands:
  issues      Validate GitHub issue structure for agent consumption
  scenarios   Validate scenario spec files
  roadmap     Validate planning docs against milestone issues

Flags:
      --config string   Path to configuration file (default "godark.yaml")
```

#### godark vet issues

```
Flags:
      --json               Output findings as JSON
      --milestone string   GitHub milestone to validate (exact title)
      --repo string        GitHub repository (owner/repo)
      --tag string         Milestone tag (e.g., phase-3) — resolved to full milestone title
```

#### godark vet scenarios

```
Flags:
      --json                  Output findings as JSON
      --milestone string      GitHub milestone to validate (exact title)
      --repo string           GitHub repository (owner/repo)
      --scenario-dir string   Path to scenario spec directory (default "tests/scenarios/")
      --tag string            Milestone tag (e.g., phase-3) — resolved to full milestone title
```

#### godark vet roadmap

```
Flags:
      --json                  Output findings as JSON
      --milestone string      GitHub milestone to validate (exact title)
      --planning-dir string   Path to planning docs directory (default "docs/planning/")
      --repo string           GitHub repository (owner/repo)
      --tag string            Milestone tag (e.g., phase-3) — resolved to full milestone title
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

### godark version

```
Print the version and build time.

Usage:
  godark version [flags]
```

## Configuration

`godark init` creates a default `godark.yaml` in the project root. All fields
can be overridden by CLI flags where applicable.

```yaml
# Required — GitHub repository (owner/repo)
repo: owner/repo

# Retry and execution
max_retries: 3            # review/fix cycles before escalating (default 3)
agent_timeout: "30m"      # max wall-clock time per agent run
no_sandbox: false         # run agents on host instead of Docker
quality_strictness_decay: true  # use diminishing strictness on quality review retries

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
  quality_reviewer: ""
  spec_generator: ""
```

## Building

```bash
go build -o bin/godark ./cmd/godark
go test ./...
```

## Status

See [docs/ROADMAP.md](docs/ROADMAP.md) for the full development roadmap.
