# Scenario: Session resumption for retries

Relates to: Issue #39

## Setup
- The agent package (`internal/agent`) is imported directly
- The `Runner` variable in `launcher.go` is stubbed to return controlled stdout containing structured JSON with `session_id` fields
- `ProcessIssue` is tested with stubbed `Implement`, `Review`, and `Retry` functions to verify session ID threading
- `agent_runner.py` resume logic is tested via Python unit tests
- No real SDK, Claude API, or Docker daemon required

## Cases

### Session ID captured from implementer result
Stub the runner to return stdout ending with `{"session_id": "sess-abc123", "result": "done", "cost_usd": 0.50, "is_error": false}`.
- `Result.SessionID` is `"sess-abc123"`

### First implement does not set GODARK_SESSION_ID
Call `Implement()` with a stubbed runner and inspect the environment variables.
- `GODARK_SESSION_ID` is not present in the subprocess environment (or is empty)

### Retry passes previous session ID
Call `Retry()` with a previous session ID of `"sess-abc123"`.
- `GODARK_SESSION_ID` is set to `"sess-abc123"` in the subprocess environment

### ProcessIssue threads session ID from implement to retry
Run `ProcessIssue` where the implementer returns `session_id: "sess-first"` and the reviewer returns `CHANGES_REQUESTED`.
- The first `Retry()` call receives `GODARK_SESSION_ID=sess-first` in its environment

### ProcessIssue threads session ID across multiple retries
Run `ProcessIssue` where the implementer returns `session_id: "sess-first"`, the first retry returns `session_id: "sess-second"`, and the reviewer returns `CHANGES_REQUESTED` twice.
- The first `Retry()` receives `GODARK_SESSION_ID=sess-first`
- The second `Retry()` receives `GODARK_SESSION_ID=sess-second`

### Reviewer does not use session resumption
Call `Review()` with a stubbed runner.
- `GODARK_SESSION_ID` is not present in the subprocess environment
- Each review invocation starts a fresh session

### agent_runner.py passes resume option when session ID is set
Invoke `agent_runner.py` with `GODARK_SESSION_ID=sess-abc123` set in the environment.
- The `query()` call includes `resume="sess-abc123"` in `ClaudeAgentOptions`

### agent_runner.py starts fresh when no session ID
Invoke `agent_runner.py` without `GODARK_SESSION_ID` set.
- The `query()` call does not include a `resume` option
- A new session is created

### Resume fallback on invalid session
Invoke `agent_runner.py` with `GODARK_SESSION_ID` set to an expired or invalid session ID, and the SDK raises an error on resume.
- The runner falls back to a fresh session with the same prompt
- A warning is logged to stderr indicating the resume failed
- The process exits successfully (non-zero exit only if the fresh session also fails)

### Session ID survives timeout
Stub the runner to return a result with `session_id` present but `TimedOut: true`.
- `Result.SessionID` is still captured from the output
- The session ID is available for potential retry even after a timeout
