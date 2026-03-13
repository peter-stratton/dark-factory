# Scenario: Hybrid retry config — max_resume_retries

Relates to: Issue #372

## Setup
- The `internal/config/` package is tested via Go unit tests
- `ProcessIssue()` uses `cfg.MaxResumeRetries` to decide resume vs. fresh mode
- `cfg.MaxRetries` controls the total number of retry attempts

## Cases

### Config default value
Parse a minimal `godark.yaml` without `max_resume_retries`.
- `Config.MaxResumeRetries` is 2

### Config override
Parse a `godark.yaml` with `max_resume_retries: 0`.
- `Config.MaxResumeRetries` is 0

### Resume on attempt within threshold
With `MaxResumeRetries: 2` and `MaxRetries: 4`, process attempt 0 (first retry).
- `Retry()` is called with empty `handoffContext`
- `GODARK_SESSION_ID` is set in the agent's environment

### Resume on attempt at threshold boundary
With `MaxResumeRetries: 2` and `MaxRetries: 4`, process attempt 1 (second retry).
- `Retry()` is called with empty `handoffContext`
- `GODARK_SESSION_ID` is set in the agent's environment

### Fresh on attempt beyond threshold
With `MaxResumeRetries: 2` and `MaxRetries: 4`, process attempt 2 (third retry).
- `Retry()` is called with non-empty `handoffContext`
- `GODARK_SESSION_ID` is NOT set in the agent's environment

### All retries fresh when zero
With `MaxResumeRetries: 0` and `MaxRetries: 3`, process attempt 0 (first retry).
- `Retry()` is called with non-empty `handoffContext`
- `GODARK_SESSION_ID` is NOT set

### All retries resume when high threshold
With `MaxResumeRetries: 10` and `MaxRetries: 3`, process attempt 2 (third retry).
- `Retry()` is called with empty `handoffContext`
- `GODARK_SESSION_ID` is set

### Quality review retry respects threshold
With `MaxResumeRetries: 1`, the quality review retry loop on attempt 1 uses fresh mode.
- Handoff context is assembled from PR comments
- Fresh agent session is started

### Functional review retry respects threshold
With `MaxResumeRetries: 1`, the functional review retry loop on attempt 1 uses fresh mode.
- Handoff context is assembled from PR comments
- Fresh agent session is started
