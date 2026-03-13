# Scenario: Update outcome status consumers

Relates to: Issue #404

## Setup
- `OutcomeStatus` type and constants exist in `internal/agent/loop.go`
- Consumer files have been updated: `implement.go`, `orchestrator.go`,
  `dashboard/handlers.go`

## Cases

### Implement.go uses constants
Read `internal/cmd/implement.go` status switch.
- Cases use `agent.StatusImplemented`, `agent.StatusReadyToMerge`, `agent.StatusNeedsHumanReview`
- No bare status string literals in the switch

### Orchestrator uses constants
Read `internal/orchestrator/orchestrator.go` status switch.
- Cases use `agent.StatusImplemented`, `agent.StatusReadyToMerge`, `agent.StatusNeedsHumanReview`

### Dashboard uses constants
Read `internal/dashboard/handlers.go` badge switch.
- Cases use `agent.StatusImplemented`, `agent.StatusReadyToMerge`, `agent.StatusNeedsHumanReview`, `agent.StatusFailed`

### Existing tests pass
Run `go test ./internal/cmd/... ./internal/orchestrator/... ./internal/dashboard/...`.
- All tests pass
