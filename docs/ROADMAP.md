# Dark Factory — Roadmap

> A Go CLI that orchestrates autonomous AI agents to implement GitHub issues,
> review their own work, and merge — without human intervention.

---

## Phase 1: Skeleton + Orchestration ✅

**Goal**: `godark run --milestone "Phase 1" --repo owner/repo --dry-run` works
end-to-end. Fetches issues, resolves dependencies, sorts by priority, and
prints the execution plan. No agent execution in this phase.

**Milestone**: `Phase 1` | **Label**: `phase-1`

- Project scaffold and CLI skeleton (Cobra, subcommand stubs)
- YAML config parsing with CLI flag overrides
- GitHub issue fetching with priority sorting (p1 → p2 → p3 → unlabeled)
- Dependency resolution from issue bodies (`Blocked by`, `Depends on`)
- Structured logging (JSON file + human-readable stdout)
- Orchestration loop with dry-run mode
- CLAUDE.md and scenario specs for Phase 2 validation
- `godark init` command — installs skills and default config into a project
- Planning skills: `/godark-create-roadmap`, `/godark-create-planning-doc`, `/godark-create-issues`, `/godark-create-scenario`

**Issues**: #1–#7 (all closed), init + skills added post-milestone

---

## Phase 2: Quality & Vetting ✅

**Goal**: The `godark vet` subcommand validates that roadmap docs and GitHub
issues are clear, unambiguous, and fully actionable by agents. Built early so
it can be used to validate issues for all subsequent phases.

**Milestone**: `Phase 2` | **Label**: `phase-2`

- Vet command scaffold and validation framework (Finding types, report format)
- Issue structure validation (`godark vet issues`)
- Scenario spec validation (`godark vet scenarios`)
- Roadmap validation (`godark vet roadmap`)

**Issues**: #14–#17 (all closed)

---

## Phase 3: Docker Sandbox ✅

**Goal**: Run agents in isolated containers for safety. The user's working
directory is never touched — all agent work happens in a container. Required
before agent execution so that `--dangerously-skip-permissions` runs in a
confined environment.

**Milestone**: `Phase 3` | **Label**: `phase-3`

- Dockerfile generation and image management
- Container lifecycle: build image, run agent, capture output, cleanup
- Auth forwarding: `ANTHROPIC_API_KEY`, `GH_TOKEN`, `CLAUDE_CODE_OAUTH_TOKEN`
- Pre-configured `.claude.json` for headless operation (skip onboarding, trust dialogs)
- Non-root user (Claude Code refuses `--dangerously-skip-permissions` as root)
- Repo cloning or worktree inside container (not host volume mount)
- `--no-sandbox` flag to run agents directly on the host

**Issues**: #20–#24 (all closed)

---

## Phase 4: Agent Execution ✅

**Goal**: `godark run --milestone "M" --repo owner/repo` autonomously implements,
reviews, and merges GitHub issues using Claude Code agents inside Docker
containers. This is the first phase whose issues are vetted by `godark vet`
before implementation begins.

**Milestone**: `Phase 4` | **Label**: `phase-4`

### Agent launcher
- Invoke `claude -p --dangerously-skip-permissions` with prompt templates
- Load prompt templates from config-specified file paths
- Capture agent stdout/stderr for log parsing
- Execute agents inside Docker sandbox (from Phase 3)

### Implementer agent (Agent 1)
- **Fresh mode**: create feature branch, implement issue, write unit tests, open PR
- **Retry mode**: check out existing PR, read review comments, fix issues, push

### Reviewer agent (Agent 2)
- Check out PR, read matching scenario specs
- Generate ephemeral integration tests in `tests/review/`
- Run all tests, approve or request changes
- Clean up `tests/review/` before finishing
- Output `REVIEW_RESULT=APPROVED` or `REVIEW_RESULT=CHANGES_REQUESTED`

### Guard rails (script-level, not prompt-level)
- PR existence check after implementer finishes
- `Closes #N` auto-append if missing from PR body
- Protected path drift detection (reject PRs touching protected files)
- Scenario spec presence warning (comment on PR if no spec exists)
- `REVIEW_RESULT` sentinel parsing from reviewer output
- golangci-lint as additional quality gate (configurable lint command in `godark.yaml`)

### Orchestration
- Review/retry loop (max N retries from config)
- Merge approved PRs (squash + delete branch) or escalate (label `needs-human-review`)
- Baseline commit recording before each issue for rollback reference
- `git pull --rebase origin main` after each merge
- Single-issue mode (`--issue N`)
- `godark implement <issue-number>` — direct single-issue command (no milestone/deps)
- Summary stats at end (implemented, skipped, failed)

**Issues**: #29 (closed); remaining work completed without formal issue tracking

---

## Phase 5: Agent SDK Migration ✅

**Goal**: Replace the `claude -p` shell invocation layer with the Claude Agent
SDK (`claude-agent-sdk`), running inside the existing Docker container. The Go
CLI and container isolation are preserved; the SDK runs as a small Python script
inside the container image.

**Milestone**: `Phase 5` | **Label**: `phase-5`

### Invocation layer rewrite
- Add `agent_runner.py` to the Docker image — a thin SDK wrapper that reads
  the prompt from `$GODARK_PROMPT`, calls `query()`, and streams structured
  output to stdout
- Update `launcher.go` (`runSandbox`) to invoke the Python runner instead of
  building a shell entrypoint around `claude -p`
- Update `dockerfile.go` to install Python + `claude-agent-sdk` (replace
  Claude Code CLI install)

### Role-scoped permissions
- Implementer: `allowed_tools=["Read", "Write", "Edit", "Bash", "Glob", "Grep"]`
- Reviewer: `allowed_tools=["Read", "Glob", "Grep", "Bash"]`,
  `disallowed_tools=["Write", "Edit"]` — reviewer literally cannot modify files
- Spec generator: `allowed_tools=["Read", "Write", "Glob", "Grep"]` — no Bash

### Preventive guardrails via hooks
- `PreToolUse` hook on `Write|Edit` to deny modifications to protected paths
  (currently detected post-hoc by `CheckProtectedDrift`)
- Agent receives a `systemMessage` explaining why the write was blocked, so it
  can adjust rather than failing silently
- `PostToolUse` audit hook logging every tool call for structured telemetry

### Session resumption for retries
- Capture `session_id` from implementer's first run
- On `CHANGES_REQUESTED`, resume the implementer session with reviewer feedback
  instead of cold-starting a new agent invocation
- Eliminates context re-discovery on retries — agent remembers its reasoning,
  which files it changed, and why

### Structured output parsing
- Replace `REVIEW_RESULT=` sentinel grepping with typed SDK message parsing
- Capture implementer's tool-use trace and pass to reviewer for richer review
  context (reviewer sees what the agent explored, not just the final diff)

### Cleanup
- Delete `GenerateClaudeConfig()` from `auth.go` (SDK has no onboarding dialogs)
- Remove `ClaudeFlags` from config and `CLAUDE_CODE_OAUTH_TOKEN` from auth
- Simplify `auth.go` `CollectAuthEnv()` (SDK handles API key natively;
  only `GH_TOKEN` still needs forwarding)

**Issues**: #36–#41

---

## Phase 6: Multi-Language Support ✅

**Goal**: `godark` can orchestrate agents against non-Go projects. The
Dockerfile, config, and prompts are language-agnostic; project type is
auto-detected or configured explicitly.

**Milestone**: `Phase 6` | **Label**: `phase-6`

### Auto-detect project type
- Scan target repo for language markers: `go.mod` (Go), `pubspec.yaml`
  (Flutter/Dart), `package.json` (Node), `Cargo.toml` (Rust),
  `requirements.txt`/`pyproject.toml` (Python)
- Set sensible default `build_command` and `test_command` per detected type
- Allow explicit override in `godark.yaml` (config always wins over detection)
- Subsumes issue #33 (auto-detect Go version) into broader auto-detect

### Generic runtime config
- Replace `GoVersion` and `CrossCompile{GOOS, GOARCH}` with a `runtime:` block:
  ```yaml
  runtime:
    name: flutter    # go | flutter | node | rust | python
    version: "3.41"  # optional — auto-detected if omitted
  ```
- Preserve backwards compatibility: if `go_version` is set in existing configs,
  treat it as `runtime: {name: go, version: "<value>"}`
- Move `CrossCompile` env vars into a generic `sandbox_env:` map

### Pluggable Dockerfile generation
- Refactor `dockerfile.go` to select a toolchain install stanza based on
  `runtime.name` instead of hardcoding the Go tarball download
- Supported runtimes at launch: Go, Flutter/Dart, Node
- Each runtime provides: base packages, SDK install commands, PATH setup
- Runtime stanzas are Go templates, not external files (keep it simple)

### Language-aware reviewer prompt
- Replace hardcoded `go test ./{{.ReviewDir}} -v` in `reviewer.txt` with
  `{{.TestCommand}}` (already used by the implementer prompt)
- Make review test generation language-aware: reviewer prompt should reference
  the detected language so it generates tests in the right framework
  (Go tests, Dart tests, Jest, etc.)

### Cleanup
- Remove `GoVersion` field from `DockerConfig` and `Config` structs
- Remove `CrossCompile` struct; replace with `SandboxEnv map[string]string`
- Update `godark doctor` checklist references (Phase 8) to be runtime-aware
- Update docs (`CONTEXT.md`, `README.md`) to reflect multi-language support

**Issues**: #50–#54

---

## Phase 7: Review Quality & Dashboard ✅

**Goal**: Capture review telemetry, report on review quality metrics, and
surface it all in a local web dashboard for human spot-checking.

**Milestone**: `Phase 7` | **Label**: `phase-7`

### Run data
- Structured JSON files written to `~/.godark/runs/<owner>/<repo>/<timestamp>/`
- Per-run metadata (config, milestone, issue list, summary stats)
- Per-issue outcome files with per-step telemetry (implement, QA review
  cycles, retries, functional review)
- Debug log (`debug.log`) co-located in the run directory (replaces `logs/`)
- Both `godark run` and `godark implement` write the same format

### Telemetry
- Wall-clock duration per agent invocation (measured on Go side)
- Cost, tool trace, verdict, and session ID (already captured in `Result`)
- Quality review stores an array of results for multi-cycle reviews

### Quality reporting
- Flag reviews with low cost, short duration, missing diff reads, or missing
  test runs — report only, no enforcement
- Review test execution reporting: flag functional reviews that didn't create
  or run tests in `tests/review/`
- Configurable thresholds in `godark.yaml` (`quality:` block)

### Dashboard
- `godark status` serves a local web UI (Go templates + htmx + Alpine.js)
- Tech: embedded static assets, single binary, localhost only
- Run list: all runs across all repos, filterable, with summary stats and
  quality flag counts
- Run detail: per-issue outcomes with status, PR links, retry count, cost
- Issue detail: review chain timeline with expandable tool traces
- Log viewer: parsed `debug.log` with level filtering and search

**Issues**: #94–#103

**Planning doc**: `docs/planning/phase-7-review-quality-and-dashboard.md`

---

## Phase 8: Harness Engineering ✅

**Goal**: Any project — new or existing — can adopt structured harness files
that make agents dramatically more effective. `godark new` creates a
harness-ready project from scratch. `godark init` scaffolds harness docs
into existing projects. Agents communicate reasoning via structured PR
comments. Architecture constraints are validated before execution.

**Milestone**: `Phase 8` | **Label**: `phase-8`

- Harness document templates package (`internal/harness/`) with embedded
  Go templates for all harness docs (architecture.md, conventions.md,
  ROADMAP.md, CLAUDE.md)
- Expand `godark init` to scaffold harness docs (architecture.md,
  conventions.md, ROADMAP.md, default prompt templates) using skip-if-exists
  pattern; does not scaffold CLAUDE.md; prints guidance message
- `godark new` command — creates project directory, runs `git init`,
  scaffolds CLAUDE.md template, then runs `godark init` internally;
  `--repo` flag; errors if directory exists
- Architecture layer parser (`internal/harness/layers/`) — parses layer
  definitions from markdown tables and optional JSON, returns directed
  dependency graph
- `godark vet architecture` subcommand — cycle detection, directory mapping
  warnings, import smell detection, layer skip warnings; skips gracefully
  if no architecture doc exists
- Agent dialogue and architecture reference prompt templates — update
  implementer, implementer_retry, and reviewer prompts with structured PR
  comment instructions (Implementation Notes / Review Notes) and
  architecture/conventions doc references via template variables
- `/godark-define-architecture` skill — analyzes existing codebase or
  recommends idiomatic layers for new projects; suggests
  `/godark-create-roadmap` when discrepancies found between definition
  and codebase
- `/godark-define-conventions` skill — analyzes existing codebase or
  recommends idiomatic conventions with agent-friendliness filter; suggests
  `/godark-create-roadmap` for standardization phases
- Update planning skills and embed new skills — update
  `/godark-create-roadmap` and `/godark-create-planning-doc` to read
  architecture/conventions docs and prompt for updates; add new skills
  to `embed.go`

**Issues**: #121–#129

**Planning doc**: `docs/planning/phase-8-harness-engineering.md`

**Design docs**: `docs/design/harnesses.md`, `docs/design/godark-new.md`

---

## Phase 9: Harness-Aware Agent Execution

**Goal**: The harness files scaffolded and validated in Phase 8 are wired
into actual agent runs. Agents read architecture and conventions docs,
post structured dialogue on PRs, and the reviewer checks layer compliance
— all driven by the orchestrator, not just prompt template text.

**Milestone**: `Phase 9` | **Label**: `phase-9`

- Update architecture.json for dialogue package — add `internal/dialogue/`
  to domain layer paths
- Populate harness template variables in launcher — add `architecture_doc`
  and `conventions_doc` config fields with defaults; read file contents in
  `newPromptData()`; empty string for missing files (graceful degradation)
- Structured PR comment parser — new `internal/dialogue/` package; parse
  Implementation Notes and Review Notes from PR comment text into typed
  structs
- Wire agent dialogue into run data — `DialogueEntry` struct in rundata;
  orchestrator fetches PR comments after review cycles and writes
  `dialogue.json` per issue
- Surface agent dialogue in dashboard — dialogue timeline in issue detail
  view with expandable entries styled by role
- Architecture JSON context for reviewer — add `{{.ArchitectureJSON}}`
  template variable; reviewer gets structured layer definitions for
  compliance checking
- Configurable architecture enforcement — `enforce_architecture` config
  option; when enabled, reviewer must reject layer violations; when
  disabled (default), violations are informational only

**Issues**: #146–#152

**Planning doc**: `docs/planning/phase-9-harness-aware-agent-execution.md`

---

### Future considerations (not yet scoped)
- Linter config generation from `architecture.json` (per-language)
- Daemon mode (`godark watch`) — continuous polling
- Bounded concurrency — parallel agent execution for independent issues
