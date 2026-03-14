```
     _            _           __            _
  __| | __ _ _ __| | __      / _| __ _  ___| |_ ___  _ __ _   _
 / _` |/ _` | '__| |/ /_____| |_ / _` |/ __| __/ _ \| '__| | | |
| (_| | (_| | |  |   <______|  _| (_| | (__| || (_) | |  | |_| |
 \__,_|\__,_|_|  |_|\_\     |_|  \__,_|\___|\__\___/|_|   \__, |
                                                           |___/
```

A Go CLI built for [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
that orchestrates autonomous AI agents to implement GitHub issues, review their
own work, and merge — without human intervention.

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

Dark Factory has been built entirely by its own agent pipeline — every feature
was implemented, reviewed, and merged by `godark run`. The humans write specs
and design harnesses; the agents write code.

## Install

**Homebrew** (macOS):

```bash
brew install peter-stratton/dark-factory/godark
```

**Go install**:

```bash
go install github.com/peter-stratton/dark-factory/cmd/godark@latest
```

**Binary download**: grab a pre-built binary from
[GitHub Releases](https://github.com/peter-stratton/dark-factory/releases).

### Platform support

Dark Factory is built for Claude Code and GitHub. The architecture is designed
around Claude Code's specific capabilities — session resumption, CLAUDE.md as a
control surface, slash command skills, and sandboxed execution.

| Layer | Supported |
|-------|-----------|
| AI agent | Claude Code (Anthropic) |
| Version control | GitHub |

### Features

- **Three-agent pipeline** — implementer, quality reviewer, and functional reviewer are independent agents with isolated permissions; reviewers literally cannot edit files
- **Specification-driven quality gates** — human-authored scenario specs define "done"; the functional reviewer generates ephemeral integration tests from specs, not just rubber-stamping the diff
- **Architecture-as-code enforcement** — machine-readable layer definitions validated by `godark vet`; reviewers check architectural compliance, not just correctness
- **Structured agent dialogue** — implementer posts reasoning as PR comments, reviewers challenge it; the PR thread is an auditable record of adversarial design review
- **Full run observability** — local web dashboard with review chain timelines, quality flags, tool traces, and agent dialogue history for every issue
- **Harness engineering lifecycle** — scaffold, validate, and enforce project constraints with `godark new`, `godark init`, `godark vet`, and six harness types
- **Auto-detected multi-language support** — detects project type from marker files and configures the sandbox, build, and test commands automatically
- **Fully sandboxed agent runs by default** — agents execute inside ephemeral Docker containers with no access to the host filesystem or network beyond what's explicitly configured
- **Single binary, runs on a laptop** — no infrastructure fleet, no MCP server farm; just a Go binary, and Docker 

### Project type support

**Supported** — tested in production runs:

| Runtime  | Marker file       | Default build | Default test |
|----------|-------------------|---------------|--------------|
| Go       | `go.mod`          | `go build ./...` | `go test ./...` |
| Flutter  | `pubspec.yaml`    | _(none)_      | `flutter test` |

**Coming soon** — auto-detected but not yet production-tested:

| Runtime  | Marker file       | Default build | Default test |
|----------|-------------------|---------------|--------------|
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
| `python3` + `anthropic[agent]` | Only in `--no-sandbox` mode | Runs the embedded `agent_runner.py` on the host using the Anthropic Agent SDK; in sandbox mode, both are pre-installed in the container |

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
templates.

Then open the project in Claude Code and run these skills in order:

| Step | Skill | What it does |
|------|-------|--------------|
| 1 | `/godark-define-architecture` | Discuss your planned architecture and write `docs/architecture.md` + `docs/architecture.json` with layer definitions |
| 2 | `/godark-define-conventions` | Discuss language idioms and patterns, write `docs/conventions.md` |
| 3 | `/godark-harness-claude-md` | Compress CLAUDE.md into a minimal directory of pointers to the docs above |
| 4 | `godark vet architecture` | Validate the layer definitions have no dependency cycles |

Steps 1-3 are conversational — the skill asks questions about your project and
writes the docs based on your answers. For a greenfield project there's no code
to analyze, so the skills focus on what you're planning to build.

## Migrating an existing project

```
godark init --repo owner/repo
```

Installs skills, creates `godark.yaml` (if missing), and scaffolds empty harness
doc templates without overwriting existing files. Safe to re-run — skills are
always updated, everything else is skip-if-exists.

Then open the project in Claude Code and run these skills in order:

| Step | Skill | What it does |
|------|-------|--------------|
| 1 | `/godark-configure-project` | Scan the codebase and populate `godark.yaml` with detected modules, codegen, CI checks, and environment config |
| 2 | `/godark-define-architecture` | Analyze existing code and write `docs/architecture.md` + `docs/architecture.json` with layer definitions |
| 3 | `/godark-define-conventions` | Analyze existing patterns and write `docs/conventions.md` |
| 4 | `/godark-harness-claude-md` | Compress CLAUDE.md into a minimal directory of pointers to the docs above |
| 5 | `godark vet architecture` | Validate the layer definitions have no dependency cycles |

For existing projects, step 1 (`/godark-configure-project`) detects your build
system, test runner, code generation, CI workflows, and environment variables
automatically. Steps 2-3 analyze your actual code to extract architecture and
conventions rather than starting from scratch.

**Notes:**
- Use `--reset-claude-md` to replace an existing CLAUDE.md with the harness template before running `/godark-harness-claude-md`.
- If your project already has conventions in `CONTRIBUTING.md` or architecture in `docs/ADR/`, the harness skills will reference those instead of forcing a migration.
- Harness documentation is optional — `godark run` works without it. But agents produce better results when they have clear architecture definitions, coding conventions, and a concise CLAUDE.md to orient from.

## Planning and running a phase

Once harness docs are in place, use the planning skills to create a roadmap and
prepare issues for agent execution:

| Step | Skill | What it does |
|------|-------|--------------|
| 1 | `/godark-create-roadmap <goal>` | Discuss project goals, create a phased roadmap, and set up GitHub milestones |
| 2 | `/godark-create-planning-doc <phase>` | Flesh out each issue in a phase with specs, constraints, acceptance criteria, and test cases |
| 3 | `/godark-create-issues <phase>` | Create GitHub issues from the planning doc |
| 4 | `/godark-create-scenarios <phase>` | Generate scenario spec files the functional reviewer uses to write integration tests |
| 5 | `godark vet issues --tag phase-N` | Validate issues are agent-ready |
| 6 | `godark vet scenarios --tag phase-N` | Validate scenario specs match their issues |

Steps 1-2 are conversational — they ask clarifying questions and let you shape
the specs before anything is created in GitHub.

Once vetting passes, kick off the loop:

```
godark run --tag phase-N --repo owner/repo
```

After the phase completes, generate a practical overview documenting what was
built:

```
/godark-create-phase-overview <phase-number>
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
  analyze     Analyze run data and print aggregate report
  completion  Generate the autocompletion script for the specified shell
  doctor      Verify system dependencies and environment before running godark
  implement   Implement one or more GitHub issues
  init        Initialize a project with godark skills and default config
  new         Create a new project with all harness files
  run         Run the development loop for a milestone or single issue
  status      Start the dashboard web server
  unlock      Clear a stale run lock left by a crashed godark instance
  version     Print the version and build time
  vet         Validate roadmap docs and issue quality for agent consumption
  watch       Poll for PRs awaiting human review and detect CHANGES_REQUESTED reviews
```

### godark run

```
Fetch issues from a GitHub milestone, resolve dependencies, and process
each unblocked issue through the implement → review → merge loop.

Usage:
  godark run [flags]

Flags:
      --auto-merge string   Merge strategy: none, low_risk, all (default "none")
      --config string       Path to configuration file (default "godark.yaml")
      --dry-run             Print execution plan without taking action
      --force               Clear any existing run lock before starting
      --issue int           Single issue number to process (instead of milestone)
      --max-retries int     Maximum review/fix retry cycles per issue (default 3)
      --milestone string    GitHub milestone to process (exact title)
      --no-sandbox          Run agents on host instead of in Docker
      --punchlist string    Write manual testing punchlist to this file
      --repo string         GitHub repository (owner/repo)
      --tag string          Milestone tag (e.g., phase-3) — resolved to full milestone title
```

### godark implement

```
Fetch one or more GitHub issues by number and run the implement → review → merge
loop directly, without milestone or dependency resolution.

Issue numbers may be provided as positional arguments, via --issues, or both.

Usage:
  godark implement [issue-number...] [--issues 160,161,162] [flags]

Flags:
      --auto-merge string   Merge strategy: none, low_risk, all (default "none")
      --config string       Path to configuration file (default "godark.yaml")
      --dry-run             Print issue details and exit
      --force               Clear any existing run lock before starting
      --issues string       Comma-separated list of issue numbers (e.g. 160,161,162)
      --max-retries int     Maximum review/fix retry cycles (default 3)
      --no-sandbox          Run agents on host instead of in Docker
      --punchlist string    Write manual testing punchlist to this file
      --repo string         GitHub repository (owner/repo)
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
Start a local web server that serves a dashboard UI.
The homepage lists all runs from ~/.godark/runs/, most recent first.

Press Ctrl-C to stop the server.

Usage:
  godark status [flags]

Flags:
      --port int   port to listen on (default 8374)
```

### godark analyze

```
Read run data from ~/.godark/runs/, apply filters, and print an
aggregate report including outcome distribution, flag frequencies,
retry statistics, cost statistics, and detected prompt gaps.

Usage:
  godark analyze [flags]

Flags:
      --json               Output as JSON instead of human-readable table
      --milestone string   Filter to runs for this milestone
      --repo string        Filter to runs for this repository (owner/repo)
      --since string       Only include runs started at or after this date (YYYY-MM-DD)
      --until string       Only include runs started at or before this date (YYYY-MM-DD)
```

### godark doctor

```
Run pre-flight checks to confirm that all required tools and environment
variables are in place. Prints a pass/fail checklist and exits non-zero
if any check fails.

Checks performed:
  • Docker daemon running
  • gh CLI installed and authenticated
  • ANTHROPIC_API_KEY environment variable set
  • Detected runtime toolchain available
  • Python 3 available

Usage:
  godark doctor [flags]
```

### godark watch

```
Poll GitHub for pull requests labeled godark:awaiting-human-review and
detect when a human submits a CHANGES_REQUESTED review. When detected,
the implementer agent is invoked to address the feedback using session
resumption. After the fix is pushed, the PR is re-labeled
godark:awaiting-human-review.

Runs as a long-lived foreground process. Press Ctrl+C to stop.

Usage:
  godark watch [flags]

Flags:
      --config string   Path to configuration file (default "godark.yaml")
      --repo string     GitHub repository (owner/repo)
```

### godark unlock

```
Remove the godark-in-progress label from all open issues in the repo
and delete the local .godark/lock.json file.

Use this command when a previous godark run crashed mid-execution and
left the lock label on issues, preventing new runs from starting.
Alternatively, pass --force to godark run to clear the lock automatically.

Usage:
  godark unlock [flags]

Flags:
      --repo string   GitHub repository (owner/repo)
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
auto_merge:
  feature: "none"         # feature PR merge strategy: none, low_risk, all
  rollup: "none"          # rollup PR handling: none, manual, auto
quality_strictness_decay: true  # use diminishing strictness on quality review retries
auth_preference: ""       # force "oauth" or "api_key" (auto-detected if empty)

# Build, test, lint, and codegen commands (auto-detected if not set)
build_command: ""
test_command: ""
lint_command: ""
generate_command: ""

# Environment variables injected into the sandbox container
sandbox_env: {}

# Required environment variables — godark doctor checks these are set
required_env: []

# Project runtime — auto-detected from go.mod, pubspec.yaml, etc. if not set
runtime:
  name: ""     # go, flutter, node, rust, python
  version: ""  # optional — auto-detected from go.mod, package.json engines, etc.

# Paths (relative to repo root)
protected_paths: []                    # files agents must never modify
denied_commands: []                    # shell commands agents must not run
generated_paths: []                    # paths containing generated code (not reviewed)
roadmap_path: "docs/ROADMAP.md"
planning_dir: "docs/planning/"
scenario_dir: "tests/scenarios/"
review_dir:   "tests/review/"
architecture_doc: "docs/architecture.md"
architecture_json: "docs/architecture.json"
conventions_doc: "docs/conventions.md"
enforce_architecture: false            # inject architecture rules into agent prompts

# Docker sandbox settings
docker:
  image: ""                # base image (default: ubuntu:22.04)
  dockerfile: ""           # custom Dockerfile path (overrides generated one)
  mount: ""                # host path to mount into the container
  user: ""                 # non-root user inside the container
  node_version: ""         # Node.js major version to install (default: 20)
  extra_packages: []       # additional apt packages to install
  install_commands: []     # shell commands to run during image build (after runtime setup)

# Prompt template overrides (paths to custom prompt files)
prompts:
  implementer: ""
  implementer_retry: ""
  reviewer: ""
  quality_reviewer: ""
  spec_generator: ""
  punchlist: ""
  verify_fix: ""

# Quality review thresholds
quality:
  min_review_cost_usd: 0       # flag reviews cheaper than this
  min_review_duration_seconds: 0  # flag reviews shorter than this

# Deterministic verify step (build/test/lint before review)
verify:
  max_fix_attempts: 3      # auto-fix attempts before failing
  blocking: true           # fail the issue if verify doesn't pass

# CI check requirements (wait for GitHub Actions / status checks)
wait_for_checks:
  timeout: "10m"           # max time to wait for checks to complete
  required: []             # check names that must pass before merge

# Default branch of the repository (auto-detected from GitHub if omitted)
# default_branch: main

# Rollup PR behavior (when base_branch differs from default_branch)
# none   — godark merges feature PRs; human opens and merges the rollup PR
# manual — godark merges feature PRs and opens the rollup PR; human merges it
# auto   — godark merges feature PRs, opens the rollup PR, and merges it

# | Mode   | Feature PRs → base branch | Base branch → default branch           |
# |--------|---------------------------|----------------------------------------|
# | none   | godark merges             | human inspects branch, opens PR, merges|
# | manual | godark merges             | godark opens PR, human reviews/merges  |
# | auto   | godark merges             | godark opens PR and merges             |

# Risk thresholds for low_risk auto-merge
risk_thresholds:
  max_lines: 500           # PRs changing more lines are not low-risk
  max_files: 10            # PRs changing more files are not low-risk

# Multi-module support — per-module build/test overrides and dependency ordering
modules: {}
  # module_name:
  #   build_command: ""
  #   test_command: ""
  #   lint_command: ""
  #   generate_command: ""
  #   depends_on: []

# Watch mode — poll for human review feedback
watch:
  poll_interval: "5m"      # how often to check for new reviews

# Notifications
notify: []
  # - provider: telegram
  #   events: [run_complete, abort]
  #   settings:
  #     bot_token: "${TELEGRAM_BOT_TOKEN}"
  #     chat_id: "${TELEGRAM_CHAT_ID}"
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
| 13 | [Human-in-the-Loop Review](docs/phase-overviews/phase-13-human-in-the-loop-review.md) — graduated auto-merge, watch command, risk classifier, notifications |
| 14 | *Deferred* — Bounded Concurrency |
| 15 | *Deferred* — Server Mode & Centralized Operation |
| 16 | [Public Release](docs/phase-overviews/phase-16-public-release.md) — ELv2 license, GoReleaser, Homebrew tap, release workflow, CONTRIBUTING.md |
| 17 | [Configurable Base Branch](docs/phase-overviews/phase-17-configurable-base-branch.md) — base branch config, PR targeting, prompt safety, run data tracking |

To generate an overview for a newly completed phase, use `/godark-create-phase-overview <phase-number>`.

## Building

```bash
go build -o bin/godark ./cmd/godark
go test ./...
```

## Status

See [docs/ROADMAP.md](docs/ROADMAP.md) for the full development roadmap.

## License

Dark Factory is licensed under the [Elastic License 2.0](LICENSE).
