# Phase 5: Agent SDK Migration

> **Goal:** Replace the `claude -p` shell invocation layer with the Claude Agent
> SDK (`claude-agent-sdk`), running inside the existing Docker container. The Go
> CLI and container isolation are preserved; the SDK runs as a small Python script
> inside the container image.

## Milestone

`Phase 5`

---

## Issue 36: Invocation layer rewrite

### Description

Replace the current `claude -p --dangerously-skip-permissions` subprocess
invocation with a Python script (`agent_runner.py`) that uses the Claude Agent
SDK's `query()` function. The Go CLI remains the single shipped binary — the
Python script is embedded via `//go:embed` and written to the Docker image at
build time (or to a temp file in no-sandbox mode).

The script reads its configuration from environment variables set by the Go CLI:
`GODARK_PROMPT`, `GODARK_ROLE` (implementer / reviewer / spec_generator),
`GODARK_SESSION_ID` (for resume), and `GODARK_PROTECTED_PATHS`. It calls
`query()` with the appropriate `ClaudeAgentOptions` and streams all messages to
stdout as newline-delimited JSON, so the Go side can parse structured results
instead of grepping raw text.

### Key constraints

- New file: `internal/agent/runner/agent_runner.py` (embedded via `//go:embed`
  in a new `internal/agent/runner/embed.go`)
- `agent_runner.py` uses `claude_agent_sdk.query()` with:
  - `permission_mode="bypassPermissions"`
  - `setting_sources=["project"]` (loads CLAUDE.md from the repo)
  - `system_prompt={"type": "preset", "preset": "claude_code"}` (Claude Code
    system prompt so agents get full tool definitions)
  - `cwd` set to the working directory (default `/workspace`)
  - `env` passes through `GH_TOKEN` for GitHub access
- The script's stdout is structured: it prints a final JSON line with
  `{"session_id": "...", "result": "...", "cost_usd": ..., "is_error": bool}`
  extracted from the SDK's `ResultMessage`
- Update `launcher.go`:
  - `runHost()`: write embedded `agent_runner.py` to a temp file, invoke
    `python3 <temp>/agent_runner.py` with env vars instead of `claude -p`
  - `runSandbox()`: change the agent command in `EntrypointScript` from
    `claude -p "$GODARK_PROMPT"` to `python3 /usr/local/bin/agent_runner.py`
- Update `dockerfile.go` template:
  - Add `python3` and `pip` to apt-get install
  - Add `RUN pip install claude-agent-sdk` after Node.js/Claude Code install
  - Add `COPY agent_runner.py /usr/local/bin/agent_runner.py`
- Update `build.go` `BuildImage()`:
  - Write the embedded `agent_runner.py` to the temp build dir before
    `docker build` so the `COPY` directive can find it
- Update `Result` struct to include `SessionID string` and
  `CostUSD float64` fields parsed from the runner's structured output
- Remove `ClaudeFlags` from `RunOpts` — no longer needed (SDK options replace
  CLI flags)

### Acceptance criteria

- [ ] `agent_runner.py` is embedded in the Go binary and extractable at runtime
- [ ] `runHost()` invokes Python runner instead of `claude` CLI directly
- [ ] `runSandbox()` entrypoint calls Python runner instead of `claude -p`
- [ ] Generated Dockerfile installs Python 3, pip, and `claude-agent-sdk`
- [ ] `agent_runner.py` uses `query()` with `bypassPermissions` and project settings
- [ ] Runner outputs a structured JSON result line on stdout
- [ ] `Result` struct captures `SessionID` and `CostUSD` from runner output
- [ ] Existing prompt template rendering still works (prompt passed via env var)
- [ ] `go test ./internal/agent/...` passes
- [ ] `go test ./internal/sandbox/...` passes

### Test cases

- **Embedded file exists**: `runner.FS.ReadFile("agent_runner.py")` returns valid Python source
- **Host mode invokes python3**: In no-sandbox mode, `Runner` is called with `python3` and the temp script path (verified via stubbed `Runner`)
- **Sandbox entrypoint**: `EntrypointScript` output contains `python3 /usr/local/bin/agent_runner.py` instead of `claude -p`
- **Dockerfile includes Python**: `GenerateDockerfile` output contains `python3`, `pip install claude-agent-sdk`
- **Dockerfile copies runner**: `GenerateDockerfile` output contains `COPY agent_runner.py /usr/local/bin/agent_runner.py`
- **BuildImage writes runner**: `BuildImage` writes `agent_runner.py` to the temp build dir before invoking `docker build`
- **Structured result parsing**: Given runner stdout with `{"session_id": "abc", "result": "done", "cost_usd": 0.42, "is_error": false}`, `Result` fields are populated correctly
- **GODARK_PROMPT env var**: Sandbox mode sets `GODARK_PROMPT` in container env (existing behavior preserved)
- **GODARK_ROLE env var**: Role is passed via `GODARK_ROLE` environment variable

---

## Issue 37: Role-scoped permissions

**Blocked by**: #36

### Description

Configure different tool permissions for each agent role via the SDK's
`allowed_tools` and `disallowed_tools` options. The implementer needs full
read/write/execute access. The reviewer must be read-only — it literally cannot
modify files. The spec generator can write files but cannot run arbitrary
commands.

The role is passed to `agent_runner.py` via the `GODARK_ROLE` environment
variable. The script maps each role to a specific set of tool permissions.

### Key constraints

- `agent_runner.py` role → permissions mapping:
  - `implementer` / `implementer_retry`:
    `allowed_tools=["Read", "Write", "Edit", "Bash", "Glob", "Grep"]`
  - `reviewer`:
    `allowed_tools=["Read", "Glob", "Grep", "Bash"]`,
    `disallowed_tools=["Write", "Edit"]`
    (Bash is needed for `go test`, `gh pr diff`, etc. — but Write/Edit are
    hard-denied so the reviewer literally cannot modify files)
  - `spec_generator`:
    `allowed_tools=["Read", "Write", "Glob", "Grep"]`,
    `disallowed_tools=["Bash"]`
    (can create spec files but cannot run arbitrary commands)
- Update `launcher.go` to set `GODARK_ROLE` in the env map based on which
  agent function is calling `Run()`:
  - `Implement()` → `GODARK_ROLE=implementer`
  - `Retry()` → `GODARK_ROLE=implementer_retry`
  - `Review()` → `GODARK_ROLE=reviewer`
  - `GenerateSpec()` → `GODARK_ROLE=spec_generator`
- The role string is validated in `agent_runner.py` — unknown roles cause a
  non-zero exit with a clear error message

### Acceptance criteria

- [ ] Implementer agent has Read, Write, Edit, Bash, Glob, Grep tools available
- [ ] Reviewer agent cannot use Write or Edit tools (hard-denied via `disallowed_tools`)
- [ ] Reviewer agent can use Read, Glob, Grep, and Bash
- [ ] Spec generator cannot use Bash (hard-denied via `disallowed_tools`)
- [ ] Spec generator can use Read, Write, Glob, Grep
- [ ] `GODARK_ROLE` is set in the environment for each agent invocation
- [ ] Unknown role value causes non-zero exit with descriptive error
- [ ] `go test ./internal/agent/...` passes

### Test cases

- **Implementer role env**: `Implement()` sets `GODARK_ROLE=implementer` in RunOpts.Env
- **Retry role env**: `Retry()` sets `GODARK_ROLE=implementer_retry` in RunOpts.Env
- **Reviewer role env**: `Review()` sets `GODARK_ROLE=reviewer` in RunOpts.Env
- **Spec generator role env**: `GenerateSpec()` sets `GODARK_ROLE=spec_generator` in RunOpts.Env
- **Reviewer cannot write (unit test of agent_runner.py)**: Python test that verifies reviewer role config has `Write` and `Edit` in `disallowed_tools`
- **Unknown role exits**: `GODARK_ROLE=bogus` causes `agent_runner.py` to exit with code 1 and error message

---

## Issue 38: Preventive guardrails via hooks

**Blocked by**: #36

### Description

Use the SDK's in-process hook system to block protected path modifications
at the tool level (before they happen) and log every tool call for telemetry.

Currently, protected path drift is detected post-hoc by `CheckProtectedDrift`
— the agent modifies the file, then the Go CLI detects it and closes the PR.
With SDK hooks, we can block the write before it happens and inject a system
message telling the agent why, so it can adjust its approach.

### Key constraints

- `agent_runner.py` registers two hooks:
  1. **`PreToolUse` hook on `Write|Edit`**: checks if
     `tool_input.file_path` matches any path in `GODARK_PROTECTED_PATHS`
     (comma-separated env var). If matched, returns
     `{"decision": "block", "systemMessage": "Cannot modify protected path: <path>. ..."}`
  2. **`PostToolUse` audit hook** (all tools): logs tool name, input summary,
     and duration to stderr as structured JSON. The Go side captures stderr
     for the structured log.
- `GODARK_PROTECTED_PATHS` is set by the Go CLI from `cfg.ProtectedPaths`
  (already available as a comma-separated string in `PromptData.ProtectedPaths`)
- Protected path matching must handle both exact paths (`CLAUDE.md`) and
  directory prefixes (`tests/scenarios/` matches `tests/scenarios/foo.md`)
- The `CheckProtectedDrift` guard rail in `guardrails.go` remains as a
  belt-and-suspenders check — hooks are the primary defense, drift detection
  is the fallback
- Hook timeout: use the default 60s timeout (sufficient for path checking)

### Acceptance criteria

- [ ] `PreToolUse` hook blocks Write/Edit to paths listed in `GODARK_PROTECTED_PATHS`
- [ ] Blocked tool calls inject a system message explaining what was blocked and why
- [ ] Agent receives the system message and can adjust (does not silently fail)
- [ ] Directory-prefix matching works (e.g., `tests/scenarios/` blocks `tests/scenarios/foo.md`)
- [ ] `PostToolUse` audit hook logs tool name and input summary to stderr as JSON
- [ ] `GODARK_PROTECTED_PATHS` is set in the container env from config
- [ ] `CheckProtectedDrift` still runs as a fallback after agent finishes
- [ ] Python unit tests for hook logic pass

### Test cases

- **Exact path blocked**: Hook blocks `Write` to `CLAUDE.md` when `GODARK_PROTECTED_PATHS=CLAUDE.md,tests/scenarios/`
- **Directory prefix blocked**: Hook blocks `Edit` of `tests/scenarios/foo.md` when `GODARK_PROTECTED_PATHS=tests/scenarios/`
- **Non-protected path allowed**: Hook allows `Write` to `internal/foo/bar.go` (not in protected list)
- **System message injected**: Blocked hook returns dict with `decision=block` and non-empty `systemMessage`
- **Audit log format**: `PostToolUse` hook writes JSON to stderr with keys `tool`, `input_summary`, `timestamp`
- **Empty protected paths**: When `GODARK_PROTECTED_PATHS` is empty, no paths are blocked
- **Go side sets env**: `RunOpts.Env` includes `GODARK_PROTECTED_PATHS` from config

---

## Issue 39: Session resumption for retries

**Blocked by**: #36

### Description

Capture the `session_id` from the implementer's first run and pass it back on
retry, so the agent resumes with full context from its previous session. This
eliminates the cold-start problem on retries — the agent remembers which files
it read, what it changed, and why.

Currently, retry mode starts a completely new agent invocation. The retry
prompt includes the PR number and tells the agent to read review comments, but
the agent has no memory of its previous work. With session resumption, the
agent picks up exactly where it left off, plus the reviewer's feedback.

### Key constraints

- `Result` struct gains a `SessionID string` field (from Issue 1)
- `agent_runner.py` captures `session_id` from the `ResultMessage` and includes
  it in the structured output JSON
- When `GODARK_SESSION_ID` env var is set, `agent_runner.py` passes
  `resume=session_id` in `ClaudeAgentOptions`
- Update `Retry()` in `implementer.go`:
  - Accept the previous `Result` (or just the session ID) as a parameter
  - Set `GODARK_SESSION_ID` in `RunOpts.Env` from the implementer's result
- Update `ProcessIssue()` in `loop.go`:
  - Store the implementer's `Result.SessionID` after `Implement()`
  - Pass it to `Retry()` calls
- The resume feature is best-effort: if the session cannot be resumed (e.g.,
  expired or unavailable), the runner falls back to a fresh session with the
  retry prompt. Log a warning but don't fail.
- Session resumption only applies to the implementer retry path, not to the
  reviewer (reviewer always starts fresh)

### Acceptance criteria

- [ ] `Result.SessionID` is populated from the runner's structured output
- [ ] `Retry()` passes `GODARK_SESSION_ID` from the previous implementer result
- [ ] `agent_runner.py` uses `resume=session_id` when `GODARK_SESSION_ID` is set
- [ ] Resumed session receives the retry prompt as a continuation, not a cold start
- [ ] If resume fails, runner falls back to a fresh session (logs warning, doesn't crash)
- [ ] `ProcessIssue` stores and passes session ID through the retry loop
- [ ] Reviewer does not use session resumption (always fresh)
- [ ] `go test ./internal/agent/...` passes

### Test cases

- **Session ID captured**: Runner stdout with `{"session_id": "sess-abc123", ...}` → `Result.SessionID == "sess-abc123"`
- **Retry passes session**: `Retry()` with previous session ID sets `GODARK_SESSION_ID=sess-abc123` in env
- **No session ID on first run**: `Implement()` does not set `GODARK_SESSION_ID` (env var absent or empty)
- **Resume option set**: When `GODARK_SESSION_ID` is set, `agent_runner.py` passes `resume="sess-abc123"` to `query()`
- **Resume fallback**: When `GODARK_SESSION_ID` refers to an invalid session, runner starts fresh and logs warning
- **Reviewer has no session**: `Review()` never sets `GODARK_SESSION_ID`
- **Loop threading**: `ProcessIssue` captures session ID from `Implement()` result and passes to first `Retry()`, then captures from retry result for subsequent retries

---

## Issue 40: Structured output parsing

**Blocked by**: #36

### Description

Replace the fragile `REVIEW_RESULT=APPROVED` sentinel grepping with structured
output from the SDK. The reviewer's `agent_runner.py` output already includes a
structured `result` field in its JSON output (from Issue 1). This issue extends
that to include a typed verdict for the reviewer and richer context passing
between agents.

### Key constraints

- `agent_runner.py` structured output includes a `verdict` field when
  `GODARK_ROLE=reviewer`:
  ```json
  {
    "session_id": "...",
    "result": "...",
    "cost_usd": 0.42,
    "is_error": false,
    "verdict": "APPROVED"
  }
  ```
  The verdict is extracted from the agent's final text output — `agent_runner.py`
  scans `ResultMessage.result` for the `REVIEW_RESULT=` pattern and maps it to
  the `verdict` field. This keeps the reviewer prompt unchanged (it still prints
  `REVIEW_RESULT=APPROVED`) but moves the parsing into Python where it's more
  reliable.
- Update `Result` struct to include `Verdict string` field
- Update `ParseReviewResult` / `Review()` in `reviewer.go`:
  - Parse verdict from the structured JSON output instead of scanning raw stdout
  - Fall back to the old stdout scanning if structured output doesn't contain
    a verdict (backward compatibility during migration)
- Capture the implementer's tool-use trace summary and pass it to the reviewer
  via `GODARK_IMPL_SUMMARY` env var. `agent_runner.py` collects a summary of
  tool calls (file reads, edits, bash commands) from `AssistantMessage` content
  blocks and writes it to a summary file or env var. The reviewer prompt can
  reference this to understand what the agent explored, not just the final diff.
- Update `Result` struct to include `ToolTrace []string` — a list of tool
  call summaries (e.g., `"Edit internal/foo/bar.go"`, `"Bash: go test ./..."`)

### Acceptance criteria

- [ ] Reviewer output includes `verdict` field in structured JSON
- [ ] `Review()` reads verdict from structured output instead of grepping stdout
- [ ] Fallback: if no structured verdict, scan stdout for `REVIEW_RESULT=` (backward compat)
- [ ] `Result` struct includes `Verdict` and `ToolTrace` fields
- [ ] Implementer's tool trace is captured and available for passing to reviewer
- [ ] `ParseReviewResult` updated to try structured output first
- [ ] `go test ./internal/agent/...` passes

### Test cases

- **Structured verdict parsing**: Runner JSON with `"verdict": "APPROVED"` → `Result.Verdict == "APPROVED"`
- **Structured verdict CHANGES_REQUESTED**: Runner JSON with `"verdict": "CHANGES_REQUESTED"` → `Result.Verdict == "CHANGES_REQUESTED"`
- **No verdict field**: Runner JSON without `verdict` → falls back to stdout scanning
- **Fallback works**: No structured verdict + stdout contains `REVIEW_RESULT=APPROVED` → verdict is `"APPROVED"`
- **Neither source**: No structured verdict + no stdout sentinel → verdict is `""`
- **Tool trace captured**: Runner JSON with `"tool_trace": ["Read foo.go", "Edit bar.go"]` → `Result.ToolTrace` populated
- **Non-reviewer has no verdict**: Implementer runner output has no `verdict` field

---

## Issue 41: Migration cleanup

**Blocked by**: #37, #38, #39, #40

### Description

Remove code that is no longer needed after the SDK migration. The SDK handles
onboarding configuration, and permissions are now managed via SDK options
instead of CLI flags.

### Key constraints

- **Delete `GenerateClaudeConfig()`** and related types (`claudeConfig`,
  `projectConfig`) from `internal/sandbox/auth.go`:
  - The SDK's `permission_mode="bypassPermissions"` and `setting_sources`
    options eliminate the need for pre-configured `.claude.json`
  - The Dockerfile template currently doesn't reference this (it's called
    from `clone.go`'s entrypoint), but verify and remove any references
- **Remove `ClaudeFlags` from config**:
  - Delete `ClaudeFlags []string` from `Config` struct in `config.go`
  - Delete `claude_flags` YAML field handling
  - Remove `ClaudeFlags` from `RunOpts` in `launcher.go` (already done in
    Issue 1, verify here)
  - Remove any references in prompt code or tests
- **Simplify `CollectAuthEnv()`**:
  - The SDK handles `ANTHROPIC_API_KEY` natively — it reads it from the
    environment automatically. However, since we're running inside a Docker
    container, we still need to forward it from the host. Keep `CollectAuthEnv`
    but remove the `CLAUDE_CODE_OAUTH_TOKEN` path — the SDK uses API keys,
    not OAuth tokens.
  - Keep `GH_TOKEN` forwarding (still needed for `gh` CLI inside container)
- **Update Dockerfile template**:
  - Remove the `.claude.json` generation from the entrypoint if present
  - The `npm install -g @anthropic-ai/claude-code` line stays (SDK depends on
    the CLI under the hood)
- **Remove `maskToken` if unused** after `CollectAuthEnv` simplification

### Acceptance criteria

- [ ] `GenerateClaudeConfig()` and related types deleted from `auth.go`
- [ ] `ClaudeFlags` removed from `Config`, `RunOpts`, and all references
- [ ] `CLAUDE_CODE_OAUTH_TOKEN` handling removed from `CollectAuthEnv()`
- [ ] `CollectAuthEnv()` requires only `ANTHROPIC_API_KEY` (not "one of two")
- [ ] `GH_TOKEN` forwarding still works
- [ ] No dead code remains (unused functions, types, imports)
- [ ] All tests pass: `go test ./...`

### Test cases

- **No ClaudeFlags in config**: YAML with `claude_flags: ["-v"]` is either
  ignored or causes a warning (not an error, for backward compat)
- **Auth requires API key**: Missing `ANTHROPIC_API_KEY` returns error
  (no fallback to OAuth token)
- **GH_TOKEN still collected**: `CollectAuthEnv` returns `GH_TOKEN` from env
  or `gh auth token` fallback
- **GenerateClaudeConfig gone**: `sandbox.GenerateClaudeConfig` is no longer
  exported or callable
- **Build succeeds**: `go build ./cmd/godark` with no compilation errors
- **All tests pass**: `go test ./...` passes cleanly
