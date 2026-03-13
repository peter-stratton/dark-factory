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
- Planning skills: `/godark-create-roadmap`, `/godark-create-planning-doc`, `/godark-create-issues`, `/godark-create-scenarios`

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

- Worker pool with configurable max concurrency (default 1)
- Dependency-aware scheduling from existing topological sort
- Per-worker sandbox containers and isolated git worktrees
- Single-goroutine merge serializer (squash-merge, rebase, signal next)
- Rebase conflict re-queue for fix cycle
- Thread-safe run data writer (mutex or per-issue writers)
- Per-issue log files for concurrent debuggability
- Active workers indicator and concurrent status badges in dashboard
- GitHub API rate limit backpressure
- Anthropic API concurrency limit awareness

**Issues**: TBD

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

### Future considerations (not yet scoped)
- Linter config generation from `architecture.json` (per-language)
- Multi-cluster deployment and geographic distribution
- Cost allocation and chargeback per team/repo
- Per-module change detection — diff PR changed files against module paths
  and only run build/test for affected modules and their dependents (currently
  all modules are built/tested unconditionally)
- Compose-based test infrastructure — `test_infra` config block for managing
  docker-compose lifecycle (setup/teardown) around the verify pipeline;
  deferred because `wait_for_checks` covers integration testing via CI
- Docker-in-Docker (DinD) support for running compose-based tests inside
  the sandbox container without requiring `no_sandbox: true`
- Landing page and docs site
- Demo / example repo that people can point godark at to try it out
- Homebrew core inclusion (`brew install godark` without tap prefix)
- README badges — license, latest release, CI build status, test coverage, Go Report Card
- Quality review ROI evaluation — instrument overlap between quality reviewer
  and functional reviewer catches; consider merging into a single review pass
  if overlap is high
- Judge agent for stuck retry loops — read-only agent that evaluates whether
  an implementation approach is working or going in circles; decides to retry
  with a different strategy, restart fresh, or escalate; implement only if
  retry data from recon agent runs shows a persistent pattern of stuck loops
  that strictness decay doesn't resolve
