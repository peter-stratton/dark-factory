# Phase 5: Agent SDK Migration

Phase 5 replaced the `claude -p` shell invocation with the Claude Agent SDK,
a Python library that gives the Go orchestrator fine-grained control over what
each agent can do. The CLI still drives everything. Docker isolation is
unchanged. But the execution layer inside the container is now a small Python
script (`agent_runner.py`) that calls the SDK's `query()` function, enforces
role-scoped permissions, intercepts tool calls via hooks, and streams
structured JSON back to the Go side for parsing. The result: reviewers
physically cannot edit files, protected paths are blocked before the write
happens (not detected after the fact), retries resume the original session
instead of cold-starting, and review verdicts come from typed messages instead
of grepping stdout for sentinel strings.

---

## agent_runner.py -- the SDK wrapper

A single Python script, embedded in the Go binary via `//go:embed` and copied
into the Docker image at build time, serves as the bridge between the Go
orchestrator and the Claude Agent SDK.

It reads its configuration entirely from environment variables (`GODARK_PROMPT`,
`GODARK_ROLE`, `GODARK_SESSION_ID`, `GODARK_PROTECTED_PATHS`, etc.), calls
`claude_agent_sdk.query()`, streams every SDK message to stdout as
newline-delimited JSON, and prints a final structured result line containing
session ID, cost, verdict, and tool trace.

**How it works in practice:** When the Go orchestrator calls `Run()` in sandbox
mode, the container's entrypoint clones the repo and then runs:

```
cd /workspace && python3 /usr/local/bin/agent_runner.py
```

The prompt arrives via the `GODARK_PROMPT` environment variable -- no shell
quoting gymnastics required. In no-sandbox mode, the Go side writes the
embedded `agent_runner.py` to a temp file and invokes `python3` directly.

---

## Role-scoped tool permissions

Each agent role gets a fixed set of allowed and disallowed tools, enforced by
the SDK itself -- not by the prompt.

| Role                | Allowed tools                         | Disallowed tools |
|---------------------|---------------------------------------|------------------|
| `implementer`       | Read, Write, Edit, Bash, Glob, Grep   | --               |
| `implementer_retry` | Read, Write, Edit, Bash, Glob, Grep   | --               |
| `reviewer`          | Read, Glob, Grep, Bash                | Write, Edit      |
| `quality_reviewer`  | Read, Glob, Grep, Bash                | Write, Edit      |
| `spec_generator`    | Read, Write, Glob, Grep               | Bash             |
| `punchlist`         | Read, Glob, Grep                      | Write, Edit, Bash|

**What this means:** A reviewer agent that tries to call `Edit` gets a hard
rejection from the SDK runtime. It never reaches the model's tool execution.
Before Phase 5, the reviewer was trusted not to modify files because the prompt
said "don't modify files." Now it is structurally impossible.

---

## PreToolUse hooks -- protected path enforcement

Protected paths (like `CLAUDE.md` or `tests/scenarios/`) were previously
detected after the agent finished, by diffing the branch against the base SHA.
Phase 5 moves this check to a `PreToolUse` hook that fires before every
`Write`, `Edit`, or `Bash` call.

When the hook blocks a tool call, it returns a `systemMessage` explaining why,
so the agent can adjust its approach instead of failing silently.

**Example:** An implementer agent tries to modify `CLAUDE.md`:

```
Agent calls: Edit(file_path="CLAUDE.md", ...)
Hook response: {
  "decision": "block",
  "systemMessage": "Cannot modify protected path: CLAUDE.md. This path is
    listed in GODARK_PROTECTED_PATHS ('CLAUDE.md') and must not be modified
    by implementing agents. Please adjust your approach to avoid writing
    to this path."
}
```

The agent sees the system message and works around it. The old
`CheckProtectedDrift` function still exists as a belt-and-suspenders backstop
in `guardrails.go`, but the hook catches violations in real time.

---

## PostToolUse audit hook

Every tool call -- regardless of role -- is logged to stderr as a JSON record
with the tool name, a truncated input summary, and a UTC timestamp.

```json
{"tool": "Bash", "input_summary": "{\"command\": \"go test ./...\"}", "timestamp": "2025-09-12T14:32:01.123Z"}
```

The Go side captures stderr separately from stdout, so audit records never
interfere with the structured result stream. This gives you a complete trace
of what the agent explored, which files it read, and which commands it ran --
useful for debugging failed runs or auditing what a reviewer actually checked.

---

## Session resumption for retries

When the implementer's PR gets `CHANGES_REQUESTED`, the retry invocation
passes the original `session_id` via the `GODARK_SESSION_ID` environment
variable. The SDK's `resume` option picks up where the previous session left
off.

**Before Phase 5:** Each retry cold-started a new agent. The agent had to
re-read the codebase, re-discover the issue context, and figure out what it
had changed -- burning tokens and wall-clock time.

**After Phase 5:** The retry agent remembers its previous reasoning, the files
it modified, and the approach it took. It sees the reviewer's feedback and
makes targeted fixes.

```go
// In implementer.go Retry():
if prevSessionID != "" {
    opts.Env["GODARK_SESSION_ID"] = prevSessionID
}
```

If session resumption fails (e.g., the session expired), `agent_runner.py`
automatically falls back to a fresh session and retries with exponential
backoff, logging a warning to stderr.

---

## Structured output parsing

The final line of `agent_runner.py`'s stdout is a JSON object:

```json
{
  "session_id": "sess_abc123",
  "result": "Implementation complete. Created PR #42.",
  "cost_usd": 0.37,
  "is_error": false,
  "verdict": "APPROVED",
  "tool_trace": ["Read internal/agent/loop.go", "Edit internal/agent/loop.go", "Bash: go test ./..."]
}
```

The Go side parses this with `parseRunnerOutput()`, extracting the session ID
(for future retries), the cost (for dashboard reporting), the verdict (for
the review loop), and the tool trace (passed to the reviewer for context on
what the implementer explored). The old `ParseReviewResult` function that
grepped stdout for `REVIEW_RESULT=APPROVED` still exists as a fallback, but
the primary path uses the SDK's typed verdict field.

---

## Dockerfile changes

The generated Dockerfile installs Python 3 and the `claude-agent-sdk` pip
package alongside the existing toolchain (Go, Flutter, Node, etc.). The
embedded `agent_runner.py` is copied into the build context by
`sandbox.BuildImage()` and placed at `/usr/local/bin/agent_runner.py` inside
the image.

The relevant Dockerfile lines:

```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    git curl ca-certificates gnupg python3 python3-pip ...

RUN pip install 'claude-agent-sdk>=0.1.0,<0.2.0'

COPY agent_runner.py /usr/local/bin/agent_runner.py
```

Claude Code CLI (`@anthropic-ai/claude-code`) is still installed via npm --
it is used as the underlying runtime by the SDK.

---

## Auth simplification

`CollectAuthEnv()` in `sandbox/auth.go` forwards `ANTHROPIC_API_KEY` or
`CLAUDE_CODE_OAUTH_TOKEN` (based on an `auth_preference` config option) plus
`GH_TOKEN` into the container. The SDK handles API key negotiation natively,
so there is no longer a need to generate a `.claude.json` config file to
suppress onboarding dialogs or trust prompts. The old `GenerateClaudeConfig()`
function was removed.

**Typical auth flow:** You set `ANTHROPIC_API_KEY` and `GH_TOKEN` in your
shell environment. `godark run` picks them up, forwards them into the
container, and `agent_runner.py` passes them to the SDK via the `env` option.
No additional configuration required.
