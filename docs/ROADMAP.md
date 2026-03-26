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
- Planning skills: `/godark-create-milestone`, `/godark-create-planning-doc`, `/godark-create-issues`, `/godark-create-scenarios`

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
  `/godark-create-milestone` when discrepancies found between definition
  and codebase
- `/godark-define-conventions` skill — analyzes existing codebase or
  recommends idiomatic conventions with agent-friendliness filter; suggests
  `/godark-create-milestone` for standardization phases
- Update planning skills and embed new skills — update
  `/godark-create-milestone` and `/godark-create-planning-doc` to read
  architecture/conventions docs and prompt for updates; add new skills
  to `embed.go`

**Issues**: #121–#129

**Planning doc**: `docs/planning/phase-8-harness-engineering.md`

**Design docs**: `docs/design/harnesses.md`, `docs/design/godark-new.md`

---

## Phase 9: Harness-Aware Agent Execution ✅

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

## Phase 10: Deterministic Verification Pipeline ✅

**Goal**: Agent implementation passes through a deterministic verify step
(build + lint + test) run by Go code — not by the agent — before review begins.
Failures are fed back to the implementer automatically, saving review cycles
and tokens. Agents are also restricted from running destructive shell commands.

**Milestone**: `Phase 10` | **Label**: `phase-10`

### Lint command config
- Add `lint_command` field to `godark.yaml` (empty string = skip)
- User provides any command or shell script — dark-factory runs it and checks
  the exit code, same pattern as `build_command` and `test_command`

### Go-side verify step
- New deterministic step in the agent loop between implementation and review
- Runs `build_command`, `lint_command`, and `test_command` in sequence
- Captures structured pass/fail result with summarized error output (not raw
  terminal dumps) — only the failing command's stderr/stdout, truncated to a
  reasonable length
- Runs inside the sandbox if sandboxing is enabled

### Auto-fix cycle
- On verify failure, feed the structured error summary back to the implementer
  for a fix attempt (reuses session for context continuity)
- Configurable max fix attempts before escalating to review or failing
- Verify step re-runs after each fix attempt

### Verify behavior config
- `verify:` config block controlling which checks run and failure behavior
- Option to treat verify failures as blocking (default) or warning-only
- Individual checks can be disabled (e.g. skip lint, keep build + test)

### Bash deny-list
- Deny-list for destructive commands in the `PreToolUse` hook (`rm -rf`,
  `git push --force`, `git reset --hard`, `curl | sh`, etc.)
- Configurable via `denied_commands` in `godark.yaml`
- Agent receives a system message explaining why the command was blocked

### Run data integration
- Verify step results written to run data (pass/fail per check, duration,
  fix attempt count)
- Quality flags for verify failures surfaced in dashboard

**Issues**: #176–#182

**Planning doc**: `docs/planning/phase-10-deterministic-verification-pipeline.md`

---

## Phase 11: Run Analysis & Prompt Feedback ✅

**Goal**: `godark analyze` reads run data across multiple runs to surface
failure patterns, common quality flags, and prompt gaps — closing the feedback
loop between agent execution and prompt engineering.

**Milestone**: `Phase 11` | **Label**: `phase-11`

### Analyze command
- `godark analyze` command — reads `~/.godark/runs/` across all repos and runs
- Filterable by repo, milestone, date range
- Outputs a structured report to stdout (human-readable, optionally JSON)

### Failure mode aggregation
- Common quality flag frequencies (e.g. "30% of reviews flagged
  `no_review_tests_written` on first pass")
- Retry reason distribution — why implementations needed retries
- Verdict distributions per phase/milestone
- Verify step failure rates by check type (build vs lint vs test)

### Prompt gap detection
- Correlate issue characteristics (title patterns, body length, label set)
  with failure rates
- Identify which template variables are empty on failing runs vs passing runs
- Surface issues that consistently exhaust retries

### Dashboard integration
- Analysis views in `godark status` alongside existing run/issue views
- Trend charts: success rate, average retries, cost per issue over time
- Drill-down from aggregate patterns to specific failing runs

**Issues**: #183–#190

**Planning doc**: `docs/planning/phase-11-run-analysis-and-prompt-feedback.md`

---

## Phase 12: Complex Project Support ✅

**Goal**: `godark` handles production repos with multiple modules, code
generation pipelines, compose-based test infrastructure, and build secrets.
Without this, godark is limited to simple single-module projects — blocking
adoption at orgs with real-world service complexity.

**Milestone**: `Phase 12` | **Label**: `phase-12`

### Multi-module monorepo support
- `modules:` section in `godark.yaml` maps subdirectories to their own
  build/test/lint commands:
  ```yaml
  modules:
    service:
      build_command: "go build ./..."
      test_command: "go test ./..."
      generate_command: "go generate ./..."
    admin-cli:
      build_command: "go build ./..."
      test_command: "go test ./..."
      depends_on: [service]
  ```
- If `modules:` is absent, current behavior is preserved (single root module)
- Change detection: when an issue touches files in `service/`, only
  `service` and its dependents (`admin-cli`) need build/test — skip unrelated
  modules
- The verify pipeline (Phase 10) runs per-module in dependency order
- Prompt templates receive module context so agents know which subdirectory
  they're working in

### Code generation pipeline
- `generate_command` field (string or list) runs before `build_command` in
  the verify pipeline — supports protoc, sqlc, gqlgen, mockery, etc.
- Per-module or root-level (if no `modules:` block)
- Generated file protection: `generated_paths` list marks directories or
  glob patterns that agents must not hand-edit (e.g., `service/api/grpc/gen/`,
  `service/repository/crdb/generated/`, `**/*.freezed.dart`)
  ```yaml
  generated_paths:
    - service/api/grpc/gen/
    - service/api/graph/generated/
    - service/repository/crdb/generated/
    - service/test/mocks/
    - "**/*.freezed.dart"    # glob patterns for co-located generated files
    - "**/*.g.dart"
  ```
- Enforced via the existing `PreToolUse` hook — same mechanism as
  `protected_paths` but with a distinct error message ("this file is
  generated — edit the source file instead")
- Prompt templates include a generated-files summary so agents know which
  source files drive which generated outputs

### Build secrets and environment
- `required_env` list in `godark.yaml` — fail fast at startup if any are
  missing:
  ```yaml
  required_env:
    - CLOUDSMITH_TOKEN
    - PUBSUB_EMULATOR_HOST
  ```
- Forwarded to sandbox automatically (extends current `CollectAuthEnv`)
- Secrets are never logged or written to run data

### CI status check awareness
- `wait_for_checks` config option — after PR creation, poll GitHub status
  checks before proceeding to review:
  ```yaml
  wait_for_checks:
    timeout: 10m
    required: [golangci-lint, apollo-check]
  ```
- If required checks fail, feed the failure output to the implementer for
  a fix cycle (same pattern as verify failures in Phase 10)
- If not configured, current behavior is preserved (proceed immediately)

### `/godark-configure-project` skill
- Embedded skill (installed by `godark init`) that analyzes an existing project
  and populates `godark.yaml` with the complex config fields added in this phase
- Detects: multiple `go.mod`/`pubspec.yaml`/`package.json` files → suggests
  `modules:` with per-module build/test commands and `depends_on` relationships
- Detects: `docker-compose*.yml` files → suggests `test_infra:` block with
  setup/teardown commands
- Detects: codegen config files (`sqlc.yml`, `gqlgen.yml`, `.mockery.yaml`,
  `build.yaml`, `protoc` invocations in Makefile/scripts) → suggests
  `generate_command` and `generated_paths`
- Detects: `.env.example`, `required_env` in CI configs, secret references →
  suggests `required_env` list
- Detects: CI workflow files → suggests `wait_for_checks` with required check
  names
- Interactive: presents findings and lets the user confirm/edit before writing
- Merges into existing `godark.yaml` (does not overwrite fields already set)
- Same pattern as `/godark-define-architecture` — analyze, recommend, confirm

### Config summary
```yaml
modules:
  service:
    build_command: "go build ./..."
    test_command: "go test ./..."
    generate_command: "make generate"
    depends_on: []
  admin-cli:
    build_command: "go build ./..."
    test_command: "go test ./..."
    depends_on: [service]
generated_paths:
  - service/api/grpc/gen/
  - service/api/graph/generated/
  - "**/*.freezed.dart"
  - "**/*.g.dart"
required_env:
  - CLOUDSMITH_TOKEN
wait_for_checks:
  timeout: 10m
  required: [golangci-lint]
```

**Issues**: #216–#226

**Planning doc**: `docs/planning/phase-12-complex-project-support.md`

---

## Phase 13: Human-in-the-Loop Review ✅

**Goal**: Humans can review godark-created PRs and request changes that the
agent automatically picks up and fixes. Teams adopt godark with full human
oversight and gradually increase autonomy as trust builds. This is the
critical path for org adoption — most teams will not start with auto-merge.

**Milestone**: `Phase 13` | **Label**: `phase-13`

### PR lifecycle state machine
- Each godark PR tracks state: `ai_review` → `awaiting_human` →
  `human_changes_requested` → `ai_fix` → `awaiting_human` (loop until
  approved or max cycles exceeded)
- State communicated via PR labels: `godark:awaiting-human-review`,
  `godark:fixing-review-feedback`, `godark:ready-to-merge`
- Labels are the source of truth — any external tooling or human can read
  the current state at a glance

### Feedback listener
- `godark watch` subcommand — polls for `CHANGES_REQUESTED` GitHub reviews
  and new review comments on godark-labeled PRs
- Configurable poll interval (default 60s)
- Filters to own PRs only (created by the configured GitHub user/app)
- Webhook mode as a future optimization (polling is simpler to deploy and
  sufficient for most orgs)

### Session resumption with human feedback
- When a human requests changes, feed their review comments into the
  implementer agent, resuming its prior session (`GODARK_SESSION_ID`)
- Agent has full context: original implementation reasoning, AI reviewer
  feedback from prior rounds, and now the human's feedback
- Human comments are treated the same as AI reviewer comments — the
  implementer sees a unified feedback stream
- After fixing, the agent pushes and re-labels the PR as
  `godark:awaiting-human-review`

### Graduated autonomy
- `auto_merge` config in `godark.yaml` controls merge behavior per-repo:
  ```yaml
  auto_merge: none       # default — stop at PR, human merges
  auto_merge: low_risk   # auto-merge small/safe PRs, stop for rest
  auto_merge: all        # human spot-checks only
  ```
- Risk classification for `low_risk` mode:
  - Lines changed threshold (configurable, e.g. < 200 lines)
  - No changes to protected paths, CI/CD configs, or dependency files
  - All verify checks passed on first attempt (no fix cycles)
  - No quality flags raised
- Risk assessment written to run data so humans can audit the classification

### Dashboard integration
- PRs awaiting human review surfaced prominently in run detail view
- Filter/sort by `awaiting_human` state across all runs
- Human feedback rounds visible in the issue detail dialogue timeline

### Notifications
- Pluggable notification provider model (`Notifier` interface) supporting
  multiple channels (Telegram at launch, extensible to Slack, email, etc.)
- Events: `run_complete`, `implementation_complete`, `abort`
- Provider-specific settings use `${VAR}` environment variable expansion
  for secrets
- Best-effort delivery — notification failures are logged, never block
  execution

### Config
```yaml
auto_merge: none  # none | low_risk | all
watch:
  poll_interval: 60s
risk_thresholds:
  max_lines: 200
  max_files: 10
notify:
  - provider: telegram
    events: [run_complete, abort]
    settings:
      bot_token: ${TELEGRAM_BOT_TOKEN}
      chat_id: "123456789"
```

**Issues**: #238–#249, #270–#272

**Planning doc**: `docs/planning/phase-13-human-in-the-loop-review.md`

---

## Phase 14: Bounded Concurrency

**Goal**: Independent issues within a run execute in parallel, bounded by a
configurable worker pool. Dependent issues still respect topological ordering.
Merge serialization ensures `main` stays linear.

**Milestone**: `Phase 14` | **Label**: `phase-14`

- Worker pool with configurable max concurrency (`concurrency.max_workers`, default 1)
- Wave barrier scheduling: process independent issues in parallel, wait for wave, merge all, re-resolve dependencies, next wave
- Dependency-aware scheduling from existing topological sort
- Per-worker sandbox containers with isolated git worktrees
- Concurrent/integration run modes: compose skipped when `max_workers > 1`; `--with-compose` forces single-worker integration mode
- Single-goroutine merge serializer (squash-merge, rebase, signal next)
- Merge coordinator agent (from Phase 26) used for post-wave conflict resolution
- Thread-safe run data writer (mutex or per-issue writers)
- Per-issue log files for concurrent debuggability
- Active workers indicator and concurrent status badges in dashboard

**Issues**: #593–#602

**Planning doc**: `docs/planning/phase-14-bounded-concurrency.md`

---

## Phase 15: Server Mode & Centralized Operation

**Goal**: `godark` can run as a centralized service orchestrating agent work
across many repos, while preserving the local CLI-first workflow for
individual developers. The same core engine powers both modes. Designed for
org-scale deployment where hundreds of developers across hundreds of
microservices need shared visibility, centralized scheduling, and
service-account auth.

**Milestone**: `Phase 15` | **Label**: `phase-15`

### Design principle: same engine, two frontends
- The orchestrator, agent loop, review cycle, verify pipeline, and sandbox
  execution are mode-agnostic — they don't know or care who invoked them
- `godark.yaml` stays in each repo (config travels with code, not with the
  server)
- Mode is determined by how the engine is invoked (CLI vs. server), not by
  a fork in the core logic

### Pluggable run data storage
- Introduce a `RunStore` interface behind `rundata.Writer` / `rundata.Reader`
- `LocalStore` — current filesystem implementation (default for CLI mode)
- `RemoteStore` — shared storage backend (S3-compatible, database, or
  shared filesystem) for server mode
- CLI mode can optionally push to a remote store for shared visibility
  (`run_store: s3://bucket/godark-runs` in config)
- Dashboard reads from whichever store is configured

### Server mode (`godark serve`)
- HTTP/gRPC API server that accepts run requests and reports status
- Endpoints: trigger run (repo + milestone/issue), query run status, list
  runs, stream logs
- Job queue for dispatching runs to worker nodes (initially in-process,
  later pluggable: Redis, SQS, NATS)
- Composes with Phase 13 concurrency — server manages a pool of workers
  across multiple repos simultaneously
- Health checks, graceful shutdown, and run recovery on restart

### Trigger mechanisms
- API call (CI/CD integration, chatops, internal tooling)
- GitHub webhook listener — trigger on issue label, milestone assignment,
  or scheduled event
- Cron/schedule — periodic sweeps of milestones across configured repos
- CLI remains the local trigger (`godark run` unchanged)

### Multi-repo configuration
- Server config file lists managed repos and their overrides:
  ```yaml
  # godark-server.yaml
  server:
    listen: ":8443"
    run_store: "s3://company-godark/runs"
    auth: github-app  # or personal-token
  repos:
    - org/service-a    # uses repo's own godark.yaml
    - org/service-b
    - org/service-c:
        auto_merge: none           # server-level override
        concurrency.max_workers: 2
  ```
- Per-repo `godark.yaml` is authoritative for project-specific config
  (prompts, architecture, conventions)
- Server config provides org-level defaults and overrides (merge policy,
  concurrency limits, risk thresholds)

### Auth model
- CLI mode: developer's personal tokens (current behavior, unchanged)
- Server mode: GitHub App installation (per-org, scoped permissions)
- API keys for external triggers (CI/CD, chatops)
- Per-repo permission scoping — the GitHub App's installation permissions
  limit which repos the server can touch

### Shared dashboard
- Same dashboard code, served by `godark serve` instead of `godark status`
- Aggregates runs across all repos and teams
- Team/repo filtering, org-wide quality metrics
- Role-based views: developer sees their repos, platform team sees
  everything
- Composes with Phase 11 analysis — cross-repo trend data becomes
  meaningful at org scale

### CLI ↔ server interop
- `godark run` can optionally delegate to a running server instead of
  executing locally (`server: https://godark.internal` in config)
- `godark status` can point at the shared dashboard
- Developers can still run fully local for experimentation and testing
- Local runs can push results to the shared store for visibility

**Issues**: TBD

**Planning doc**: `docs/planning/phase-15-server-mode.md`

---

## Phase 16: Public Release ✅

**Goal**: The repo is public and source-available under ELv2, installable via
`brew` or `go install`, with automated releases on tag push. v0.1.0 is
published.

**Milestone**: `Phase 16` | **Label**: `phase-16`

- ELv2 license — add `LICENSE` file with Elastic License 2.0 text
- `godark version` command — version embedding via ldflags at build time
- GoReleaser setup — `.goreleaser.yaml` for macOS (arm64 + amd64) and Linux
  (amd64) binaries, GitHub Release with changelog generation
- GitHub Actions release workflow — build and publish on version tag push
  (`v*`), triggers GoReleaser
- Homebrew tap — `peter-stratton/homebrew-dark-factory` repo with formula,
  GoReleaser auto-updates formula on release
- README polish — what it does, install instructions (brew, go install,
  clone), prerequisites (Docker, GitHub token, Anthropic API key), link to
  roadmap
- Hardcoded value scrub — audit Makefile and codebase for machine-specific
  paths or assumptions, make install target portable
- Platform smoke test — verify Docker sandbox works on Linux, document Mac +
  Linux as supported platforms
- CONTRIBUTING.md — explains project is built by its own automation, issues
  welcome, PRs by invitation, how the godark pipeline works
- Repo visibility flip — make the repo public, verify nothing sensitive in
  git history

**Issues**: #285–#293

**Planning doc**: `docs/planning/phase-16-public-release.md`

---

## Phase 17: Configurable Base Branch ✅

**Goal**: godark supports branching off and merging into a configurable base
branch instead of always targeting the repo's default branch. Teams that require
peer review on merges to main can run godark autonomously on sub-branches under
a parent feature branch, then submit the parent for human review.

**Milestone**: `Phase 17` | **Label**: `phase-17`

- Add `base_branch` config field to `godark.yaml` and `--base-branch` CLI flag,
  defaulting to the repo's default branch when unset
- Pass `--base` flag to `gh pr create` so PRs target the configured base branch
- Replace hardcoded "main" references in prompt templates with a
  `{{.BaseBranch}}` template variable
- Update orchestrator post-merge pull to use the configured base branch instead
  of hardcoded `origin main`
- Track base branch in run data for audit trail
- Surface base branch name in the status dashboard on run detail pages
- Two-tier merge model via structured `auto_merge` config: `auto_merge.feature`
  controls how feature PRs are merged into the base branch (`none`, `low_risk`,
  `all`); `auto_merge.rollup` controls what godark does with the rollup PR that
  merges the base branch into main (`none` = human handles everything, `manual` =
  godark opens the PR and human merges, `auto` = godark opens and merges)

**Issues**: #311-#316

**Planning doc**: `docs/planning/phase-17-configurable-base-branch.md`

---

## Phase 18: Adaptive Agent Loop ✅

**Goal**: The agent loop adapts to codebase drift within a run, recovers
intelligently from stuck retries, and produces better-informed implementations.
Issues late in a milestone execute as reliably as early ones because the system
accounts for changes made by prior issues.

**Milestone**: `Phase 18` | **Label**: `phase-18`

### Recon agent
- Recon agent prompt template and role — `recon.txt` prompt template, `recon`
  role with read-only permissions (`Read, Glob, Grep`), structured output
  format for supplemental implementation brief
- Recon config and prompt data wiring — `prompts.recon` config field,
  `ReconBrief` template variable on `PromptData`, implementer prompt updated
  to include the brief when present
- Recon orchestrator integration — invoke recon agent before `Implement()` in
  `ProcessIssue()`, pass output as implementer context, skip if not configured
- Recon run data and dashboard — persist recon brief to
  `~/.godark/runs/<run>/recon/` alongside other run data, write recon result
  (duration, cost, session ID) to run data, surface brief in issue detail view

### Hybrid retry strategy
- Fresh agent with structured handoff — on retry 3+, start a fresh agent
  session instead of resuming, pass PR comment dialogue (Implementation Notes
  / Review Notes) as structured handoff context
- Hybrid retry config — `max_resume_retries` config field (default 2), beyond
  which retries use fresh sessions with handoff artifact

**Issues**: #367–#372

**Planning doc**: `docs/planning/phase-18-adaptive-agent-loop.md`

---

## Phase 19: Spring Cleaning ✅

**Goal**: The codebase has zero duplicated patterns, all agent output parsing
uses structured formats with unified parsers, magic strings are typed constants,
and shared helpers replace copy-pasted boilerplate — making every file a clean
example of the project's conventions.

**Milestone**: `Phase 19` | **Label**: `phase-19`

### Verdict parsing & prompt consolidation
- Unify prompt verdict format — replace `REVIEW_RESULT=` and
  `QUALITY_RESULT=` with single `AGENT_RESULT=` prefix across `reviewer.txt`
  and `quality_reviewer.txt`
- Unified verdict parser — extract `parseVerdictFromOutput(stdout, keyword)`
  replacing duplicate `ParseReviewResult()` / `ParseQualityResult()`
- Extract shared CRITICAL RULES template variable — `{{.CriticalRulesText}}`
  rendered from single source, replacing duplicated rules across 5 prompt files
- Single-source scenario spec format — deduplicate format definition between
  `spec_generator.txt` and `godark-create-scenarios/SKILL.md`

### Agent loop simplification
- Extract review cycle function — `processReviewCycle()` to flatten quality
  and functional review nesting in `loop.go`
- Extract non-blocking agent result handler — shared
  `handleNonBlockingResult()` for spec-gen, recon, and verify
  error/timeout/hook-write boilerplate
- Extract handoff policy function — `shouldUseHandoff()` and
  `buildRetryContext()` replacing scattered session/handoff conditionals
- Extract drift-check helper — consolidate 4 repeated
  `checkDriftAndClose()` + early-return blocks

### CLI and command helpers
- Extract CLI flag parser — shared `parseCLIFlagsToConfig()` replacing
  duplicate flag blocks in `run.go` and `implement.go`
- Consolidate config resolution — move inline tag/milestone resolution from
  `run.go` into `vet_helpers.go` resolve functions with early returns
- Extract vet data fetcher — shared `fetchVetData(repo, milestone)` replacing
  duplicate GitHub fetch patterns across vet commands
- Consolidate file scaffold functions — shared `scaffoldDocs()` /
  `scaffoldPrompts()` replacing duplicate loops in `init.go` and `new.go`

### Shared utilities
- Extract `writeFileWithDirs()` helper — replace repeated `os.MkdirAll` +
  `os.WriteFile` + error-wrap pattern
- Extract `WalkMarkdownFiles()` helper — replace 3 identical
  `filepath.WalkDir` + `.md` filter patterns
- Extract `extractJSONFromText()` helper — safe JSON extraction replacing
  brittle `strings.Index("[")` / `strings.LastIndex("]")` in punchlist parsing
- Extract checkbox/bullet markdown parser — `extractCheckboxItem()` and
  `extractBulletItem()` replacing duplicated `HasPrefix`/`TrimPrefix` chains

### Type safety and constants
- Define outcome status constants — typed `OutcomeStatus` replacing magic
  strings in `implement.go` switch
- Define merge strategy enum — typed `MergeStrategy` with `Valid()` method
  replacing inline string checks
- Extract `issueDir()` method on `rundata.Writer` — replace 15+
  `fmt.Sprintf("%d", issueNum)` path constructions
- Group truncation limits into config struct — consolidate `maxPRDiffLen`,
  `verifyOutputLimit`, and other scattered magic numbers

### Test and infrastructure cleanup
- Consolidate skills test helpers — shared `readSkill(t, name)` and
  `parseFrontmatter()` replacing 6 duplicate test functions
- Unify `CommandRunner` pattern — shared interface replacing 3 independent
  `var CommandRunner` definitions across packages

**Issues**: #384–#408

**Planning doc**: `docs/planning/phase-19-spring-cleaning.md`

---

## Phase 20: Terminal UI ✅

**Goal**: `godark run` renders a live, rich terminal interface using Bubble Tea
when running interactively. The TUI shows run metadata, per-issue progress with
live status updates, and a summary bar — replacing the current structured log
output in interactive mode while preserving JSON output for piped/non-interactive
contexts.

**Milestone**: `Phase 20` | **Label**: `phase-20`

- Bubble Tea + Lip Gloss dependencies — add `github.com/charmbracelet/bubbletea`,
  `lipgloss`, and `bubbles` to `go.mod`
- TUI package scaffold — `internal/tui/` at the presentation layer; update
  `architecture.json` to include it alongside `internal/dashboard/`
- Header component — renders repo, milestone, timestamp, base branch, and
  auto-merge settings (feature + rollup), mirroring the dashboard run detail
  header
- Issue table component — rows with status markers (■ complete, ○ queued,
  spinner in-progress), issue number, title, current stage (recon → implement →
  verify → review → merged)
- Summary bar component — footer showing aggregate counts (merged, in review,
  queued, failed) and running cost total
- Live update event model — Bubble Tea message types for issue state
  transitions; orchestrator sends events as issues progress through the agent
  loop
- Hybrid output mode — TUI when `term.IsTerminal()` is true, current structured
  JSON/text when piped; `--no-tui` flag to force plain output
- Wire into `godark run` — replace `fmt.Println`/`logger.Info` output in the
  run command with TUI rendering; `debug.log` continues writing regardless of
  display mode
**Issues**: #439–#445

**Planning doc**: `docs/planning/phase-20-terminal-ui.md`

---

## Phase 21: Analytics Persistence ✅

**Goal**: Run statistics are persisted to a SQLite database (`~/.godark/stats.db`)
at run finalization, surviving run directory deletion. `godark analyze` and the
dashboard read from the database instead of scanning run directories, enabling
improved metrics: retry recovery rate, cost breakdown by step, duration trends,
and success rate by repo.

**Milestone**: `Phase 21` | **Label**: `phase-21`

### SQLite stats store
- SQLite dependency — add `modernc.org/sqlite` (pure Go, no CGO) to `go.mod`
- Schema and migrations — `internal/stats/` package; tables for runs, issue
  outcomes, step results (per-step cost, duration, flags); migration framework
  for schema evolution
- Write on finalize — `FinalizeRun()` writes aggregate stats to the database;
  one row per issue outcome, one row per step result; idempotent (re-finalizing
  a run updates rather than duplicates)
- Backfill command — `godark analyze --backfill` scans existing
  `~/.godark/runs/` directories and populates the database from historical
  run data
- Update `architecture.json` — add `internal/stats/` to the appropriate layer

### Improved analytics
- Retry recovery rate — of issues that retried, what percentage eventually
  succeeded vs exhausted retries
- Cost breakdown by step — implement vs quality review vs functional review vs
  retries as percentages of total cost
- Duration trends — per-step duration over time; helps identify when
  `agent_timeout` (default 30m) needs adjusting for specific repos
- Success rate by repo — pass/fail breakdown when running against multiple repos
- Surface verify stats — expose the verify check failure data that's currently
  computed but never displayed
- Rework prompt gaps — replace confusing "with/without quality reviewer"
  comparison with flag-to-outcome correlation (e.g., "issues with `no_diff_read`
  fail at 75% vs 20% baseline")
- Update `godark analyze` — read from SQLite instead of scanning run directories
- Update dashboard analysis page — same data source switch, add new metric
  cards and trend charts

**Issues**: #458–#468

**Planning doc**: `docs/planning/phase-21-analytics-persistence.md`

---

## Phase 22: Analytics Overhaul ✅

**Goal**: The `godark analyze` command and dashboard analytics page surface
actionable metrics that answer five operator questions: is the system improving,
where is money going, where is time going, what's failing and why, and what did
we ship. First-pass success rate, wasted cost, failure reason breakdown, and
per-repo efficiency replace low-value metrics.

**Milestone**: `Phase 22` | **Label**: `phase-22`

### Overview cards
- First-pass success rate — percentage of issues that succeed without any
  retries
- Avg cost per successful issue — total cost / implemented count
- Overview card row in dashboard (total runs, total issues, success rate,
  first-pass rate, total cost, avg cost per success)

### Trends
- First-pass success rate trend over time (new trend line alongside existing
  success rate)

### Cost analysis
- Cost per successful issue vs cost per failed issue
- Wasted cost — total cost on issues that ultimately failed

### Duration analysis
- Avg time to merge — end-to-end cycle time from issue start to implemented
- Timeout rate — percentage of steps that hit the agent_timeout

### Quality and failure analysis
- Failure reason breakdown — categorize failures into verify failure, review
  exhaustion, timeout, error
- Drop scenario spec gap condition (always present now)
- Drop exhausted retries listing (redundant with failure breakdown)

### Per-repo enrichment
- Avg cost per issue by repo
- First-pass success rate by repo

### Output
- Update `godark analyze` CLI output with all new metrics
- Update dashboard analysis page with new cards, charts, and tables
- Update `godark analyze --json` output to include all new fields

### Sprint summary report
- `godark report` command — new Cobra subcommand with `--since` (duration like
  `2w`, `30d`) and `--until` flags for date range, `--repo` filter, and
  `--format` flag (terminal default, markdown, html)
- Report content — sprint-scoped metrics from SQLite: issues closed, PRs
  merged, success rate, first-pass rate, cost breakdown, failure reasons,
  notable failures (with issue numbers and error messages)
- Markdown/HTML output suitable for pasting into Slack, email, or a wiki page

**Issues**: #489–#497

**Planning doc**: `docs/planning/phase-22-analytics-overhaul.md`

---

## Phase 23: Watch & Daemon Mode ✅

**Goal**: `godark watch` is validated end-to-end and handles both
`CHANGES_REQUESTED` and `APPROVED` reviews reliably. A new `--watch` flag on
`godark run` keeps the run alive after its first pass, polling for human merges
and automatically processing newly unblocked issues — eliminating the need to
manually re-run after human approvals.

**Milestone**: `Phase 23` | **Label**: `phase-23`

### Watch command validation and fixes
- Smoke test `godark watch` against a real repo — verify polling, label
  swapping, and agent invocation work e2e
- Fix any bugs discovered during validation (likely: stale lock handling,
  branch checkout issues, error recovery)
- Validate APPROVED → merge flow (#456 already implemented but never tested
  live)
- Add `--no-tui` flag to `godark watch` for consistency with run/implement
- Watch TUI view — Bubble Tea model showing polling status, PRs being watched,
  recent activity log
- Watch dashboard view — surface watch-managed PRs and their review cycles in
  `godark status`

### Daemon mode (`godark run --watch`)
- `--watch` flag on `godark run` — after the first pass completes, enter a
  polling loop instead of exiting
- Poll for merged PRs that were left in `awaiting-human-review` during the run
- When a merge is detected, re-resolve dependencies and process newly unblocked
  issues
- Reuse the existing wave re-resolution logic from `processIssues()`
- TUI transitions from "run complete" to "watching for merges" state with
  appropriate hint text
- Graceful exit: `ctrl+c` during watch mode cancels and exits cleanly
- Stats DB: write stats for daemon-mode-initiated issue processing (same as
  normal runs)

### Shared infrastructure
- Extract polling logic into a shared package (`internal/watch/` or similar)
  used by both `godark watch` and `godark run --watch`
- Configurable poll interval from `watch.poll_interval` in `godark.yaml`
  (existing config, reused)

**Issues**: #515–#523

**Planning doc**: `docs/planning/phase-23-watch-and-daemon-mode.md`

---

## Phase 24: Container Resource Tracking ✅

**Goal**: Every agent container execution records peak memory and CPU usage.
These metrics flow through run data, the stats database, the analyze command,
the dashboard, and sprint reports — giving operators the data they need to plan
bounded concurrency.

**Milestone**: `Phase 24` | **Label**: `phase-24`

- Poll Docker Stats API during `RunContainer` and capture peak memory (RSS) and
  CPU-seconds
- Add resource fields to `StepResult` and write to per-step JSON files
- Add `peak_memory_bytes` and `cpu_seconds` columns to `step_results` table
- Include resource stats in `analysis.Report` (per-step and per-issue
  aggregates)
- Add resource metrics to `analyze` CLI output (table + JSON)
- Surface resource stats in dashboard issue detail view
- Include resource summary in `report` sprint output
- Add resource stats to `--no-sandbox` mode (capture from host process instead
  of container)

**Issues**: #543–#548

**Planning doc**: `docs/planning/phase-24-container-resource-tracking.md`

---

## Phase 25: Docker Socket Mount & Compose Lifecycle ✅

**Goal**: Projects with `docker-compose` test infrastructure can run integration
tests inside the sandbox. godark manages the compose lifecycle (up before agent,
down after) via host Docker socket mount. The agent runs tests against
already-running infrastructure without managing containers itself.

**Milestone**: `Phase 25` | **Label**: `phase-25`

- Add `docker_compose` config block to `godark.yaml` (`file`, `project_name`)
- Mount host Docker socket into sandbox container when `docker_compose` is
  configured
- Install Docker CLI in sandbox image when socket mount is enabled
- Run `docker-compose up -d` before agent execution starts
- Run `docker-compose down` in deferred cleanup (even on crash/timeout)
- Unique project names per run to avoid port/name collisions (prefix with run
  ID or issue number)
- Forward `required_env` to compose containers (database URLs, emulator hosts,
  etc.)
- Update `godark doctor` to check Docker socket accessibility

**Issues**: #556–#564

**Planning doc**: `docs/planning/phase-25-docker-socket-mount-and-compose-lifecycle.md`

---

## Phase 26: Merge Coordinator Agent

**Goal**: A dedicated merge coordinator agent resolves branch conflicts and
divergence anywhere in the pipeline — per-issue pre-merge, rollup merge, and
both `godark run` and `godark implement` modes. It appears as a visible step
in the review chain with full telemetry. Replaces the current fallback to the
implementer retry agent for conflict resolution, which uses the full
implementer context and is slower than necessary.

**Milestone**: `Phase 26: Merge Coordinator Agent` | **Label**: `phase-26`

### Prompt template and agent role
- `prompts/merge_coordinator.txt` with template variables for branch name,
  base branch, conflict description (git output), and PR context
- Agent permissions: `Read`, `Edit`, `Bash` (for git commands), `Glob`, `Grep`
- `merge_coordinator` path configurable in `godark.yaml` prompts block
- Prompt instructs agent to: check out the branch, rebase onto base, resolve
  conflicts preserving intent of both sides, run build/test to verify, push

### Agent function and config wiring
- `MergeCoordinate()` function in `internal/agent/merge_coordinator.go`
  following existing agent function pattern (`newRunOpts` + `Run()`)
- Role name: `"merge_coordinator"`
- Add `MergeCoordinator string` field to `Prompts` struct in config and
  agent prompt loader
- Bounded by `max_rebase_attempts` (existing config field, default 1)

### Per-issue pre-merge integration
- Replace `Retry()` fallback in `runPreMergeRebasePhase()` in
  `internal/agent/loop.go` with `MergeCoordinate()` when `gh pr update-branch`
  fails
- After successful conflict resolution, re-run verify pipeline (existing
  behavior preserved)
- Works in both `godark run` and `godark implement` (the agent loop is shared)

### Rollup merge conflict handling
- Add conflict detection in `handleRollupPR()` in
  `internal/orchestrator/orchestrator.go` before `mergeRollupPRFn`
- Check PR mergeable status; if CONFLICTING, invoke merge coordinator
- Bounded by `max_rebase_attempts`; if exhausted, leave PR open for human
  review (same pattern as per-issue)

### Review chain visibility
- "Merge Coordinate" appears as a dedicated step in the dashboard issue detail
  review chain timeline with duration, cost, peak memory, CPU time
- Surface in TUI as a stage transition (recon → implement → verify → review →
  merge coordinate → merged)

### Run data
- `merge_coordinate.json` per issue recording duration, cost, session ID,
  attempt count, outcome (resolved/exhausted/error)
- Rollup merge coordinate result written alongside rollup verify result
- Quality flags for merge coordinator failures surfaced in dashboard

**Issues**: #605–#611

**Planning doc**: `docs/planning/phase-26-merge-coordinator-agent.md`

---

## Phase 27: Agent Efficiency & Resilience ⏸️ DEFERRED

**Goal**: Every agent step completes within its time budget, produces useful
output even on timeout, and never wastes time on tools it can't access. Recon
adapts its depth to issue complexity and codebase size. Prompts are audited
for tool/permission alignment across all roles.

**Milestone**: `Phase 27: Agent Efficiency & Resilience` | **Label**: `phase-27`

- Multi-pass recon with progressive detail — restructure recon prompt into
  prioritized passes (file list + drift → key snippets → pattern examples)
  so partial output is always useful
- Partial recon brief on timeout — capture and pass partial stdout to
  implementer when recon times out instead of discarding all work
- Adaptive recon depth by issue type — lightweight recon for wiring/refactor
  issues, deep recon for feature issues; optionally skip recon for
  well-specified issues
- Per-step timeout configuration — allow `agent_timeout` overrides per role
  (e.g. recon: 5m, spec_generator: 3m, implementer: 30m, reviewer: 15m)
- Prompt/permission audit — systematic scan of all prompts vs role permissions,
  remove any remaining instructions for unavailable tools
- Recon prompt generalization — remove Flutter/UI-specific language in favor
  of universal patterns, or make project-type-aware via template variables

**Issues**: #631–#635

**Planning doc**: `docs/planning/phase-27-agent-efficiency-and-resilience.md`

---

## Phase 28: Container Health Judge

**Goal**: A Go-side health monitor watches container log streams in real-time
and intervenes when agents stall, thrash, or hit transport failures — cutting
losses in seconds instead of waiting for the 30-minute timeout. No LLM calls;
pure heuristic pattern matching on structured log output.

**Milestone**: `Phase 28: Container Health Judge` | **Label**: `phase-28`

### Real-time log streaming
- Change `RunContainer` to stream `docker logs --follow` via a goroutine
  instead of waiting for container exit then reading logs
- Feed lines to a callback as they arrive
- Preserve existing behavior: full stdout/stderr still captured in `RunResult`

### Pattern matcher and rules engine
- New `internal/agent/judge/` package with configurable rules:
  - **Idle timeout**: no tool call audit lines for N seconds (default 180s
    for recon/spec, 300s for implementer) — distinguishes "no output at all"
    from "streaming assistant text but no tool calls"
  - **Tool thrash**: 3+ ToolSearch calls for the same query pattern within
    60s — agent is searching for an unavailable tool
  - **Transport failure**: 10+ stream-closed errors with zero tool calls —
    SDK transport is dead
- Each rule produces a `Judgment` (kill, retry-container, warn, ignore)
- Rules are configurable via `judge:` config block in `godark.yaml`

### Structured diagnostics
- When the judge intervenes, produce a structured `JudgeIntervention` record:
  - What was detected (pattern, counts, timing)
  - Which rule triggered
  - What the operator should check (specific prompt file, config field, or
    env var when applicable)
- Intervention records written to run data for dashboard and `godark analyze`
- Surfaced in TUI and notifications

### Wire into agent loop
- Connect the judge to `RunContainer` via the log streaming callback
- Handle kill decisions: stop container, return partial result
- Integrate with retry logic: transport failures trigger container retry,
  tool thrash triggers step skip with diagnostic
- Judge interventions visible as a distinct event type in the review chain

### Container retry for transport failures
- When the judge detects a transport failure (zero tool calls + stream errors),
  automatically retry the container (not the whole step) up to N times
- Distinct from the existing agent retry logic which re-runs the full step
  with reviewer feedback

**Issues**: #640–#649

**Planning doc**: `docs/planning/phase-28-container-health-judge.md`

---

## Phase 29: Complete CLI Migration

**Goal**: Remove all remnants of the Python SDK runner path. The only way to
run agents is via the Claude CLI inside Docker containers. No `--no-sandbox`,
no `runHost`, no `agent_runner.py`. Simplifies the codebase and eliminates a
dead code path that would fail at runtime.

**Milestone**: `Phase 29: Complete CLI Migration` | **Label**: `phase-29`

- Remove `--no-sandbox` flag, `NoSandbox` config field, and all conditional
  branches across orchestrator, commands, agent loop, and doctor
- Delete `runHost()` function, `Runner` var, `writerFunc`, and `goosForRusage`
  from launcher.go
- Delete `internal/agent/runner/` package (agent_runner.py, embed.go,
  embed_test.go, test_hooks.py)
- Migrate test infrastructure from `Runner` stubs to `SandboxRunner` stubs
  (~40 test functions in loop_test.go, helpers_test.go, punchlist_test.go,
  rebase_test.go)
- Remove `PullAfterMerge` host-mode conditionals from orchestrator and
  implement command
- Remove `NoSandbox` from doctor opts and host toolchain checks
- Update config schema, godark.md template, and flag tests

**Issues**: #668–#671

**Planning doc**: `docs/planning/phase-29-complete-cli-migration.md`

---

### Future considerations (not yet scoped)
- Configurable retry on judge Kill — currently only transport_failure retries
  the container; idle_timeout/no_progress/tool_thrash kills fail the step with
  no automatic retry (falls through to the normal max_retries review loop)
- Linter config generation from `architecture.json` (per-language)
- Multi-cluster deployment and geographic distribution
- Cost allocation and chargeback per team/repo
- Per-module change detection — diff PR changed files against module paths
  and only run build/test for affected modules and their dependents (currently
  all modules are built/tested unconditionally)
- Compose-based test infrastructure — `test_infra` config block for managing
  docker-compose lifecycle (setup/teardown) around the verify pipeline;
  deferred because `wait_for_checks` covers integration testing via CI
- Landing page and docs site
- Demo / example repo that people can point godark at to try it out
- GitLab support — godark currently assumes GitHub for everything: issue
  fetching, PR creation, review detection, label management, merge operations,
  and the `gh` CLI. Adding GitLab would require abstracting the VCS provider
  behind an interface (`internal/vcs/` or similar), implementing a GitLab
  client (likely using `glab` CLI or the GitLab API directly), and updating
  prompt templates that reference `gh` commands. Config would add a `provider:`
  field (`github` default, `gitlab` opt-in). Scope is significant — touches
  infrastructure, orchestration, and prompt layers — but the architecture
  already isolates GitHub calls in `internal/github/`
- Expanded distribution — add Windows builds, remove Linux arm64 ignore,
  add Scoop (Windows package manager), publish Docker images to GHCR, and
  optionally add AUR/Snap/DEB/RPM for broader Linux reach. All supported
  natively by GoReleaser except Winget (manual PR to winget-pkgs repo)
- Homebrew core inclusion (`brew install godark` without tap prefix)
- README badges — license, latest release, CI build status, test coverage, Go Report Card
- Quality review ROI evaluation — instrument overlap between quality reviewer
  and functional reviewer catches; consider merging into a single review pass
  if overlap is high
- Strategy agent for stuck retry loops — read-only LLM agent (distinct from
  the Go-side judge in Phase 28) that evaluates whether an implementation
  approach is working or going in circles; decides to retry with a different
  strategy, restart fresh, or escalate; implement only if retry data shows a
  persistent pattern of stuck loops that strictness decay doesn't resolve
- Docs site: add macOS `caffeinate` guidance — recommend `caffeinate -s godark
  run ...` for long/overnight runs to prevent macOS sleep from suspending
  Docker containers, dropping network connections, and stalling agent processes
- Golden evaluation dataset for prompt regression testing — a curated set of
  canonical issues (real or synthetic) with known-good implementations, stored
  in `tests/eval/`. A `godark eval` command runs agents against these issues
  and scores the results against baseline expectations. Primary use case is
  catching prompt regressions: run `godark eval` in CI after any change to
  `prompts/` and block the PR if scores drop below thresholds. Requires a new
  `internal/eval/` package, a scoring rubric (acceptance criteria coverage,
  verify pass rate, cost/duration within bounds), and a baseline snapshot to
  compare against. Start small — 20–30 issues spanning simple wiring, moderate
  features, and complex cross-cutting changes — and grow the dataset over time.
  Inspired by offline evaluation frameworks for LLM agents (three pillars:
  routing evaluation, LLM-as-judge scoring, and context grounding verification)
- Specification quality gate — a pre-implementation gate that scores issue
  readiness before spending compute. An agent evaluates whether the issue has
  sufficient detail, explicit acceptance criteria, and testable requirements.
  Weak specs get rejected or enriched before entering the pipeline. Could
  enforce structured issue templates that require testable acceptance criteria.
  Complements the existing `godark vet` validation but operates at runtime on
  individual issues rather than in bulk
- Eval as first-class contract — define issue-specific acceptance tests
  upfront (before or alongside spec generation) that the system explicitly
  targets, rather than relying on emergent evals from the reviewer agent.
  The eval becomes a hard contract: implementation iterates until the
  pre-defined acceptance test passes. Requires design work around authoring
  format, storage, and how it integrates with the existing verify and review
  pipeline. Distinct from scenario specs (which describe behavior) in that
  evals are executable pass/fail gates defined before implementation begins
- Production monitoring feedback loop — post-merge observation of deployed
  code to detect regressions and feed outcomes back into the pipeline. Could
  integrate with external metrics systems (Prometheus, Grafana, Datadog) to
  track whether dark-factory-built features cause production incidents, error
  rate spikes, or performance degradation. Enables quantifiable proof that
  the system produces production-quality code. Implementation depends heavily
  on the target deployment environment and observability stack. Could start
  simple (poll a health endpoint or error rate after merge) and grow toward
  richer integrations. Valuable for building organizational confidence in
  autonomous code generation
- Self-optimization from historical data — use accumulated run analytics
  (stats.db) to tune pipeline behavior automatically. Examples: issues of a
  certain type or complexity that fail review frequently get more recon/spec
  effort; prompts that correlate with higher success rates get preferred;
  retry budgets adjust based on historical pass rates. Requires server mode
  (Phase 15) for persistent cross-run state and enough data volume to draw
  meaningful conclusions. Likely a full milestone given the breadth of
  tuning surfaces and the need for guardrails against over-fitting to
  historical patterns
