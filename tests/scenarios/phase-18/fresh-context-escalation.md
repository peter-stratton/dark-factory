# Scenario: Fresh-context escalation on retries

Relates to: Issue #346

## Setup
- The `internal/agent/` package with mock `Retry` function that captures its
  `prevSessionID` argument
- Config with `Planning.FreshRestartAfter` set to various values
- Config with `MaxRetries: 3` (4 total attempts: 0, 1, 2, 3)
- Mock reviewer that returns `CHANGES_REQUESTED` on every attempt (to force
  retries through the full loop)
- Session ID `"sess_001"` returned from initial `Implement()`

## Cases

### Fresh restart at threshold
Process an issue with `FreshRestartAfter: 2` and `MaxRetries: 3`.
- Retry attempt 0: `Retry()` called with `prevSessionID = "sess_001"`
- Retry attempt 1: `Retry()` called with the session ID from attempt 0
- Retry attempt 2: `Retry()` called with `prevSessionID = ""` (fresh context)
- An info log entry mentions "fresh-context" or "escalating"

### Never escalate
Process an issue with `FreshRestartAfter: 0` and `MaxRetries: 3`.
- All retry attempts pass the current session ID to `Retry()`
- No "fresh-context" or "escalating" log entry is produced

### Immediate fresh on second retry
Process an issue with `FreshRestartAfter: 1` and `MaxRetries: 2`.
- Retry attempt 0: `Retry()` called with `prevSessionID = "sess_001"`
- Retry attempt 1: `Retry()` called with `prevSessionID = ""` (fresh context)

### Quality review loop respects threshold
Process an issue with a quality reviewer configured, `FreshRestartAfter: 1`,
and quality review returning `CHANGES_REQUESTED` repeatedly.
- Quality retry attempt 0: `Retry()` called with session ID
- Quality retry attempt 1: `Retry()` called with `prevSessionID = ""`

### Fresh restart logged
Process an issue where fresh-context escalation triggers.
- An info-level log entry is produced containing the issue number and
  attempt number
