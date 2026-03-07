# Scenario: CI check gate in merge flow

Relates to: Issue #224

## Setup
- The `internal/agent/` package merge flow with CI check polling
- A mock `GuardRunner` simulating `gh pr checks` output
- Config with `wait_for_checks:` block

## Cases

### All required checks pass
Config has `wait_for_checks: {timeout: "10m", required: [golangci-lint]}`.
Mock `gh pr checks` returns `golangci-lint` with state `completed`, conclusion `success`.
- Merge proceeds immediately
- No fix cycle triggered

### Required check fails triggers fix cycle
Config has `wait_for_checks: {timeout: "10m", required: [test]}`.
Mock `gh pr checks` returns `test` with state `completed`, conclusion `failure`.
- Fix cycle is triggered
- Implementer receives the check failure output
- After fix, checks are re-polled

### Fix succeeds on retry
First poll: required check fails. Implementer fixes. Second poll: check passes.
- Fix cycle runs once
- Merge proceeds after second poll

### Timeout with pending checks
Config has `wait_for_checks: {timeout: "1s", required: [slow-check]}`.
Mock `gh pr checks` always returns `slow-check` with state `queued`.
- Issue fails with timeout error
- Error message mentions "CI checks timed out"

### Not configured merges immediately
Config has no `wait_for_checks:` block.
- Merge happens immediately after review approval
- No polling occurs

### Unrequired check failure ignored
Config has `wait_for_checks: {timeout: "10m", required: [lint]}`.
Mock `gh pr checks` returns `lint` with `success` and `optional-check` with `failure`.
- Merge proceeds (only required checks matter)

### Fix attempts exhausted
Config has `wait_for_checks` and `verify: {max_fix_attempts: 1}`.
Required check fails, fix attempt fails, check still fails.
- Issue fails after exhausting fix attempts
- Error indicates fix attempts exhausted

### Polling interval
Config has `wait_for_checks: {timeout: "5m", required: [ci]}`.
First poll: check is `pending`. Second poll: check is `success`.
- Two polls occur with ~30 second interval between them
- Merge proceeds after second poll
