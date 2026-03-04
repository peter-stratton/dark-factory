# Scenario: Structured output parsing

Relates to: Issue #40

## Setup
- The agent package (`internal/agent`) is imported directly
- The `Runner` variable in `launcher.go` is stubbed to return controlled stdout containing structured JSON with `verdict` and `tool_trace` fields
- `ParseReviewResult` and `Review()` are tested with various combinations of structured JSON and raw stdout
- `agent_runner.py` verdict extraction is tested via Python unit tests
- No real SDK, Claude API, or Docker daemon required

## Cases

### Structured verdict APPROVED
Stub the runner to return stdout ending with `{"session_id": "s1", "result": "All good", "cost_usd": 0.30, "is_error": false, "verdict": "APPROVED"}`.
- `Result.Verdict` is `"APPROVED"`
- `Review()` returns `ReviewResult.Verdict` as `"APPROVED"`

### Structured verdict CHANGES_REQUESTED
Stub the runner to return stdout ending with `{"session_id": "s2", "result": "Tests fail", "cost_usd": 0.25, "is_error": false, "verdict": "CHANGES_REQUESTED"}`.
- `Result.Verdict` is `"CHANGES_REQUESTED"`

### Fallback to stdout scanning when no structured verdict
Stub the runner to return stdout with `REVIEW_RESULT=APPROVED` on a line but no `verdict` key in the structured JSON.
- Verdict is `"APPROVED"` via the fallback path
- No error is raised

### Fallback to stdout scanning when no structured JSON at all
Stub the runner to return plain text stdout containing `REVIEW_RESULT=CHANGES_REQUESTED` with no JSON line.
- Verdict is `"CHANGES_REQUESTED"` via the fallback path

### No verdict from either source
Stub the runner to return stdout with no `verdict` in JSON and no `REVIEW_RESULT=` sentinel.
- Verdict is `""` (empty string)
- `Review()` treats this as a failure (no verdict found)

### Implementer result has no verdict field
Stub the runner to return structured JSON for an implementer role (no `verdict` key).
- `Result.Verdict` is `""` (empty string)
- No error is raised — verdict is only expected from reviewers

### Tool trace captured from implementer
Stub the runner to return structured JSON with `"tool_trace": ["Read internal/foo/bar.go", "Edit internal/foo/bar.go", "Bash: go test ./..."]`.
- `Result.ToolTrace` contains all three entries
- Entries preserve the tool name and summary

### Tool trace is empty when not present
Stub the runner to return structured JSON without a `tool_trace` key.
- `Result.ToolTrace` is nil or empty
- No error is raised

### Python runner extracts verdict from ResultMessage
In `agent_runner.py`, simulate a `ResultMessage` with `result` text containing `REVIEW_RESULT=APPROVED`.
- The structured output JSON includes `"verdict": "APPROVED"`

### Python runner extracts CHANGES_REQUESTED verdict
In `agent_runner.py`, simulate a `ResultMessage` with `result` text containing `REVIEW_RESULT=CHANGES_REQUESTED`.
- The structured output JSON includes `"verdict": "CHANGES_REQUESTED"`

### Python runner omits verdict for non-reviewer roles
In `agent_runner.py` with `GODARK_ROLE=implementer`, simulate a `ResultMessage`.
- The structured output JSON does not contain a `verdict` key

### Python runner collects tool trace from messages
In `agent_runner.py`, simulate a stream of `AssistantMessage` objects containing `ToolUseBlock` entries for Read, Edit, and Bash.
- The structured output JSON includes a `tool_trace` array summarizing each tool call
