```
     _            _           __            _
  __| | __ _ _ __| | __      / _| __ _  ___| |_ ___  _ __ _   _
 / _` |/ _` | '__| |/ /_____| |_ / _` |/ __| __/ _ \| '__| | | |
| (_| | (_| | |  |   <______|  _| (_| | (__| || (_) | |  | |_| |
 \__,_|\__,_|_|  |_|\_\     |_|  \__,_|\___|\__\___/|_|   \__, |
                                                           |___/
```

A Go CLI that orchestrates autonomous AI agents to implement GitHub issues,
review their own work, and merge — without human intervention.

### Philosophy

The hard part of software engineering isn't typing code — it's deciding what to
build and how it fits. Dark Factory keeps those decisions with humans. Engineers
write the roadmap, define architecture layers, design conventions, and author
issue specs. Agents operate within those constraints. The harness *is* the
design.

This is a collaborative architecture tool, not a "throw a ticket at an AI and
hope for the best" system. The adversarial review model reinforces this: a
separate reviewer agent checks whether the code respects the architecture a
human defined, follows conventions a human wrote, and meets acceptance criteria
a human specified. Every judgment call that shapes a codebase stays with the
humans who understand it.

### Features

- **Three-agent pipeline** — implementer, quality reviewer, and functional reviewer are independent agents with isolated permissions; reviewers literally cannot edit files
- **Specification-driven quality gates** — human-authored scenario specs define "done"; the functional reviewer generates ephemeral integration tests from specs, not just rubber-stamping the diff
- **Architecture-as-code enforcement** — machine-readable layer definitions validated by `godark vet`; reviewers check architectural compliance, not just correctness
- **Structured agent dialogue** — implementer posts reasoning as PR comments, reviewers challenge it; the PR thread is an auditable record of adversarial design review
- **Full run observability** — local web dashboard with review chain timelines, quality flags, tool traces, and agent dialogue history for every issue
- **Harness engineering lifecycle** — scaffold, validate, and enforce project constraints with `godark new`, `godark init`, `godark vet`, and six harness types
- **Auto-detected multi-language support** — detects project type from marker files and configures the sandbox, build, and test commands automatically
- **Single binary, runs on a laptop** — no infrastructure fleet, no MCP server farm; one Go binary, Docker, and a GitHub token

### Project type support

| Runtime  | Marker file       | Default build | Default test |
|----------|-------------------|---------------|--------------|
| Go       | `go.mod`          | `go build ./...` | `go test ./...` |
| Flutter  | `pubspec.yaml`    | _(none)_      | `flutter test` |
| Node.js  | `package.json`    | `npm run build` | `npm test` |
| Rust     | `Cargo.toml`      | `cargo build` | `cargo test` |
| Elixir   | `mix.exs`         | `mix compile` | `mix test` |
| Python   | `pyproject.toml` / `requirements.txt` | _(none)_ | `pytest` |

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
9. **Punchlist** — for each merged PR, a tool-less punchlist agent generates 3-5 concrete manual acceptance tests (specific config values, commands, expected outcomes) rendered as checkboxes alongside the existing punchlist output
10. **Repeat** — move to the next unblocked issue

## Setting up a new project

```
godark new <project-name> --repo owner/repo
```

Creates the directory, writes CLAUDE.md template and `.gitignore`, runs
`git init`, then scaffolds skills, `godark.yaml`, and empty harness doc
templates. Follow the harness setup steps below to populate the docs.

## Migrating an existing project

```
godark init --repo owner/repo
```

Installs skills, creates `godark.yaml` (if missing), and scaffolds empty harness
doc templates without overwriting existing files. Safe to re-run — skills are
always updated, everything else is skip-if-exists.

Harness documentation is optional — `godark run` works without it. But agents
produce better results when they have clear architecture definitions, coding
conventions, and a concise CLAUDE.md to orient from.

**Notes:**
- Use `--reset-claude-md` to replace an existing CLAUDE.md with the harness template before running `/godark-harness-claude-md`.
- If your project already has conventions in `CONTRIBUTING.md` or architecture in `docs/ADR/`, the harness skill will point to those instead of forcing a migration.

## Harness setup

After `godark new` or `godark init`, run these skills inside Claude Code to
populate the harness docs:

| Step | Command / Skill | What it does |
|------|-----------------|--------------|
| 1 | `/godark-define-architecture` | Analyze the codebase (or discuss plans for new projects) and write `docs/architecture.md` + `docs/architecture.json` |
| 2 | `/godark-define-conventions` | Analyze existing code patterns (or recommend idioms) and write `docs/conventions.md` |
| 3 | `/godark-harness-claude-md` | Compress CLAUDE.md into a minimal directory of pointers to subordinate docs |
| 4 | `godark vet architecture` | Validate the layer definitions have no cycles |

## Planning a phase

Once harness docs are in place, use the planning skills to create a roadmap and
prepare issues for agent execution:

| Step | Command / Skill | What it does |
|------|-----------------|--------------|
| 1 | `/godark-create-roadmap <project-goal>` | Create a phased roadmap and GitHub milestones |
| 2 | `/godark-create-planning-doc <phase-number>` | Write a detailed planning doc for a roadmap phase |
| 3 | `/godark-create-issues <phase-number>` | Create GitHub issues from the planning doc |
| 4 | `/godark-create-scenarios <phase-number>` | Generate scenario specs for a phase |
| 5 | `godark vet issues --repo owner/repo --tag phase-N` | Validate issues are agent-ready |
| 6 | `godark vet scenarios --repo owner/repo --tag phase-N` | Validate scenario specs |

Once vetting passes, kick off the loop:

```
godark run --milestone "Phase N" --repo owner/repo
```

## Supported runtimes

Project type is auto-detected by scanning the repo root for marker files (see
[table above](#project-type-support)). Detection applies only when `runtime`,
`build_command`, and `test_command` are all absent from the config. If no marker
file is found, `godark` proceeds without installing a language toolchain. Use a
custom `docker.dockerfile` or set `runtime:` explicitly for unsupported
languages.

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
  new         Create a new harness-ready project directory
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
      --config string      Path to configuration file (default "godark.yaml")
      --dry-run            Print issue details and exit
      --max-retries int    Maximum review/fix retry cycles (default 3)
      --auto-merge string  Merge strategy after approval: none (human merges), low_risk (auto-merge small/safe PRs), all (auto-merge everything) (default "none")
      --no-sandbox         Run agents on host instead of in Docker
      --punchlist string   Write manual testing punchlist to this file (always printed to stdout)
      --repo string        GitHub repository (owner/repo)
```

### godark vet

```
Check that issues have clear acceptance criteria, correct blocker
notations, and are fully actionable by agents.

Usage:
  godark vet [command]

Available Commands:
  architecture  Validate architecture layer definitions for DAG correctness
  issues        Validate GitHub issue structure for agent consumption
  scenarios     Validate scenario spec files
  roadmap       Validate planning docs against milestone issues

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

#### godark vet architecture

```
Validate architecture layer definitions for DAG correctness.

Reads docs/architecture.json and checks for cycles in the dependency
graph. Exits 0 with an info message if no architecture file exists
(harnesses are opt-in).

Flags:
      --architecture-file string   Path to architecture JSON file (default "docs/architecture.json")
      --json                       Output findings as JSON
```

### godark new

```
Create a new directory with all harness files for a greenfield project.

Creates the directory, writes CLAUDE.md and .gitignore, runs git init,
then scaffolds skills, godark.yaml, and harness documentation files.

Usage:
  godark new <project-name> [flags]

Flags:
      --repo string   GitHub repository (owner/repo) — pre-populates repo: in godark.yaml
```

### godark init

```
Write Claude Code skill files and a default godark.yaml to the current
directory. Skills are always overwritten (they are managed by godark).
The config file is only created if it does not already exist.

Also scaffolds harness documentation templates (docs/architecture.md,
docs/architecture.json, docs/conventions.md, docs/ROADMAP.md, and
prompt templates) using skip-if-exists semantics.

Usage:
  godark init [flags]

Flags:
      --repo string       GitHub repository (owner/repo) — used to create the godark-in-progress label
      --reset-claude-md   Replace existing CLAUDE.md with the harness template
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
  punchlist: ""
```

## Phase overviews

Each completed phase has a practical overview with real-world examples showing
what was built and how users experience it. These live in
[`docs/phase-overviews/`](docs/phase-overviews/):

| Phase | Overview |
|-------|----------|
| 1 | [Skeleton & Orchestration](docs/phase-overviews/phase-01-skeleton-and-orchestration.md) — CLI scaffold, config, deps, dry-run |
| 2 | [Quality & Vetting](docs/phase-overviews/phase-02-quality-and-vetting.md) — `godark vet` validation framework |
| 3 | [Docker Sandbox](docs/phase-overviews/phase-03-docker-sandbox.md) — container isolation, auth, cloning |
| 4 | [Agent Execution](docs/phase-overviews/phase-04-agent-execution.md) — implementer, reviewer, guard rails, retry loop |
| 5 | [Agent SDK Migration](docs/phase-overviews/phase-05-agent-sdk-migration.md) — SDK wrapper, role permissions, session resumption |
| 6 | [Multi-Language Support](docs/phase-overviews/phase-06-multi-language-support.md) — auto-detect, runtime config, pluggable Dockerfiles |
| 7 | [Review Quality & Dashboard](docs/phase-overviews/phase-07-review-quality-and-dashboard.md) — run data, quality flags, web dashboard |
| 8 | [Harness Engineering](docs/phase-overviews/phase-08-harness-engineering.md) — harness templates, `godark new`, vet architecture |
| 9 | [Harness-Aware Agent Execution](docs/phase-overviews/phase-09-harness-aware-agent-execution.md) — harness injection, dialogue, enforcement |
| 10 | [Deterministic Verification Pipeline](docs/phase-overviews/phase-10-deterministic-verification-pipeline.md) — verify step, auto-fix, bash deny-list |
| 11 | [Run Analysis & Prompt Feedback](docs/phase-overviews/phase-11-run-analysis-and-prompt-feedback.md) — `godark analyze`, trends, prompt gaps |
| 12 | [Complex Project Support](docs/phase-overviews/phase-12-complex-project-support.md) — multi-module, codegen, secrets, CI checks |

To generate an overview for a newly completed phase, use `/godark-create-phase-overview <phase-number>`.

## Building

```bash
go build -o bin/godark ./cmd/godark
go test ./...
```

## Status

See [docs/ROADMAP.md](docs/ROADMAP.md) for the full development roadmap.
