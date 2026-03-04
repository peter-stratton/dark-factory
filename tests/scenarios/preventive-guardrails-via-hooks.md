# Scenario: Preventive guardrails via hooks

Relates to: Issue #38

## Setup
- The hook logic in `agent_runner.py` is tested via Python unit tests
- `GODARK_PROTECTED_PATHS` environment variable is set to control which paths are blocked
- Hook callback functions are invoked directly with simulated tool input dicts
- The Go side (`internal/agent`) is tested with stubbed `Runner` to verify env var forwarding
- No real SDK, Claude API, or Docker daemon required

## Cases

### Exact path is blocked
Invoke the `PreToolUse` hook with tool name `Write` and `file_path` set to `CLAUDE.md` while `GODARK_PROTECTED_PATHS=CLAUDE.md,tests/scenarios/`.
- The hook returns a dict with `decision` set to `block`
- The hook returns a non-empty `systemMessage` mentioning `CLAUDE.md`

### Directory prefix is blocked
Invoke the `PreToolUse` hook with tool name `Edit` and `file_path` set to `tests/scenarios/foo.md` while `GODARK_PROTECTED_PATHS=tests/scenarios/`.
- The hook returns a dict with `decision` set to `block`
- The `systemMessage` mentions the protected directory

### Nested file under protected directory is blocked
Invoke the `PreToolUse` hook with tool name `Write` and `file_path` set to `tests/scenarios/sub/deep.md` while `GODARK_PROTECTED_PATHS=tests/scenarios/`.
- The hook returns a dict with `decision` set to `block`

### Non-protected path is allowed
Invoke the `PreToolUse` hook with tool name `Write` and `file_path` set to `internal/foo/bar.go` while `GODARK_PROTECTED_PATHS=CLAUDE.md,tests/scenarios/`.
- The hook returns an empty dict (no `decision` key, or `decision` is not `block`)
- The write is allowed to proceed

### Read tool is not intercepted
Invoke the `PreToolUse` hook with tool name `Read` and `file_path` set to `CLAUDE.md` while `GODARK_PROTECTED_PATHS=CLAUDE.md`.
- The hook does not block the read (hook matcher only matches `Write|Edit`)
- Reading protected paths is always allowed

### Empty protected paths blocks nothing
Invoke the `PreToolUse` hook with tool name `Write` and `file_path` set to `CLAUDE.md` while `GODARK_PROTECTED_PATHS` is empty or unset.
- The hook does not block the write
- No error is raised

### System message is actionable
Invoke the `PreToolUse` hook with a blocked path.
- The `systemMessage` explains which path was blocked
- The `systemMessage` tells the agent that the path is protected and cannot be modified

### Audit hook logs tool calls
Invoke the `PostToolUse` hook with tool name `Bash` and a sample input.
- A JSON line is written to stderr
- The JSON contains a `tool` key with value `Bash`
- The JSON contains an `input_summary` key with a non-empty value
- The JSON contains a `timestamp` key

### Audit hook logs for all tools
Invoke the `PostToolUse` hook with tool names `Read`, `Write`, `Edit`, `Glob`, `Grep` in sequence.
- A JSON line is written to stderr for each tool call
- Each line contains the correct tool name

### Go side sets GODARK_PROTECTED_PATHS
Call `Run()` with a config that has `ProtectedPaths: ["CLAUDE.md", "tests/scenarios/"]`.
- `GODARK_PROTECTED_PATHS` is set in the subprocess environment to `CLAUDE.md,tests/scenarios/`

### CheckProtectedDrift remains as fallback
After the SDK migration, `CheckProtectedDrift` in `guardrails.go` still exists and runs after agent execution.
- The function is still called in `ProcessIssue` after the implementer finishes
- Both the hook (preventive) and drift check (detective) coexist
