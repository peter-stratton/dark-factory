# Scenario: Adopt CommandRunnerFunc in domain and orchestration

Relates to: Issue #402

## Setup
- The `internal/exec` package provides `CommandRunnerFunc`
- The domain/orchestration packages have been updated: `punchlist/punchlist.go`,
  `orchestrator/orchestrator.go`, `agent/guardrails.go`

## Cases

### Punchlist CommandRunner typed
Read `internal/punchlist/punchlist.go`.
- `var CommandRunner` declaration includes `exec.CommandRunnerFunc` type

### Orchestrator CommandRunner typed
Read `internal/orchestrator/orchestrator.go`.
- `var CommandRunner` declaration includes `exec.CommandRunnerFunc` type

### GuardRunner typed
Read `internal/agent/guardrails.go`.
- `var GuardRunner` declaration includes `exec.CommandRunnerFunc` type

### Existing test fakes still assignable
Run `go test ./internal/punchlist/... ./internal/orchestrator/... ./internal/agent/...`.
- All tests pass without modification to test fake assignments
