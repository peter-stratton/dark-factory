# Scenario: Wire judge into agent loop — intervention events and run data

Relates to: Issue #646

## Setup
- `internal/agent/loop.go` checks `result.JudgeKilled` and `result.JudgeIntervention` after each agent call
- Run data writer is available via hook for writing interventions
- Agent `Run`/`Retry`/`VerifyFix` functions are stubbed to return controlled results

## Cases

### Intervention written to run data
Stub agent `Run` to return a result with `JudgeKilled: true` and a populated `JudgeIntervention`.
Process the issue through the loop.
- `WriteJudgeIntervention` is called on the hook
- The written intervention has the correct `Step` field (e.g., "implement")
- The written intervention has the correct rule name from the judge

### Kill logged with rule detail
Stub agent to return judge-killed result.
Process the issue.
- Log output contains the rule name (e.g., "idle_timeout")
- Log output indicates the step was killed by the judge

### No retry on judge kill within same attempt
Stub agent `Run` to return judge-killed result on first call.
- The step is not retried within the same attempt cycle
- The issue proceeds to the next phase of handling (not re-run)

### Fresh attempt permitted after judge kill
Stub agent `Run` to return judge-killed result. Config has `MaxRetries: 2`.
- A new attempt is started (fresh container) after the judge kill
- The new attempt is a full retry, not a same-container retry
