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

### Kill with usable result proceeds to verify
Stub agent `Run` to return `JudgeKilled: true` with non-empty `ResultText` (agent completed work before going idle).
- The loop proceeds to the verify/review step as normal
- The intervention is still written to run data
- The issue is not treated as failed

### Kill with no result is terminal for current attempt
Stub agent `Run` to return `JudgeKilled: true` with empty `ResultText` (agent stalled before producing output).
- The step is not retried within the same attempt cycle
- The issue does not proceed to verify/review

### Fresh attempt permitted after kill with no result
Stub agent `Run` to return judge-killed result with empty `ResultText`. Config has `MaxRetries: 2`.
- A new attempt is started (fresh container) after the judge kill
- The new attempt is a full retry, not a same-container retry
