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

## Phase 4: Agent Execution

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

**Issues**: #29 (closed); remaining issues not yet created

---

## Phase 5: Agent SDK Migration

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

## Phase 6: Multi-Language Support

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

## Phase 7: Run Review Dashboard

**Goal**: `godark status` serves a local web UI for reviewing dev-loop runs,
making it easy for humans to spot-check agent work.

**Milestone**: `Phase 7` | **Label**: `phase-7`

- Local web server serving a review dashboard
- Homepage: list of dev-loop runs with summary stats (issues processed, pass/fail counts)
- Run detail view: per-issue outcomes (implemented, skipped, failed, retry count)
- GitHub diff links for each PR (easy human spot-checking)
- Log viewer (parsed JSON structured logs with filtering)

### Manual testing punchlist
- At the end of each `godark run` or `godark implement` call, generate a
  human-readable punchlist of manual verification steps for each merged PR
- Derive punchlist items from the issue body, scenario specs, and the PR diff
  (e.g., "Verify the new button appears on the settings page")
- Output to stdout as a numbered checklist and optionally write to a file
  (`--punchlist <path>`)
- Include PR links and relevant file paths so the user can jump straight to
  what needs manual verification
- Focus on things automated tests can't catch: UI appearance, UX flows,
  integration with external services, deployment behavior

---

## Phase 8: Review Quality & Code Quality Gates

**Goal**: Ensure the reviewer agent is doing genuine, thorough work — not
rubberstamping. Add automated quality gates that catch shallow reviews and
improve overall code quality in the dev loop.

**Milestone**: `Phase 8` | **Label**: `phase-8`

### Review depth validation
- Tool trace floor: reject reviews where the reviewer didn't read the PR diff,
  run tests, or generate review tests in `tests/review/`
- Token/cost floor: flag suspiciously cheap reviews (below a configurable
  threshold) as likely rubberstamps
- Review body quality check: verify the reviewer's PR review contains specific
  file references and concrete observations (not generic "looks good")

### Mandatory review test execution
- Hard gate: reviewer must create and run ephemeral tests in `tests/review/`
  for the review to count — no tests means automatic `CHANGES_REQUESTED`
- Verify via PostToolUse audit trace: scan for Write to `tests/review/` and
  Bash with the configured `test_command`

### Canary defect injection
- Before review, inject a known trivial bug into the PR (e.g., a syntax error
  or failing test assertion) via a guard rail step
- If the reviewer approves despite the canary, the review is invalid
- Remove the canary before merge if the reviewer catches it
- Configurable: `canary_defects: true` in `godark.yaml` (default off)

### Dual-reviewer sampling
- Configurable percentage of PRs get a second independent review
- If the two reviewers disagree, escalate to `needs-human-review`
- Provides ground truth for calibrating other quality gates
- `dual_review_sample_rate: 0.1` in `godark.yaml` (default 0, disabled)

### SDK version check
- On startup, check PyPI for the latest `claude-agent-sdk` version and warn if
  it exceeds the pinned range (`>=0.1.0,<0.2.0`) in the generated Dockerfile
- Gives early notice that a new major/minor SDK version is available and may
  require migration work before the pin can be bumped

### `godark doctor` command
- Pre-flight check that verifies all system dependencies and environment
  variables are in place before running the dev loop
- Check for: Docker daemon running, `gh` CLI installed and authenticated,
  `ANTHROPIC_API_KEY` set, detected runtime toolchain available,
  Python 3 available
- Print a pass/fail checklist with actionable fix instructions for each failure
- Exit non-zero if any required check fails

### Configurable auth token preference
- Allow `godark.yaml` to specify which auth token to prefer when both
  `ANTHROPIC_API_KEY` and `CLAUDE_CODE_OAUTH_TOKEN` are set
- Currently hardcoded to prefer OAuth; make it configurable for teams that
  want to use API keys for cost tracking or rate-limit isolation
- `auth_preference: oauth | api_key` in `godark.yaml` (default `oauth`)

### Configurable lint gate
- Run a configurable lint command (e.g., `golangci-lint run`) as an additional
  quality gate after the implementer finishes, before review
- Lint failures trigger a retry without consuming a review cycle
- `lint_command` in `godark.yaml` (default empty, disabled)
