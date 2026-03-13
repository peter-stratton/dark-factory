# Scenario: Drift-check and handoff policy helpers

Relates to: Issue #406

## Setup
- The `internal/agent/loop.go` file contains `shouldHandoff()` and uses a
  consistent drift-check pattern

## Cases

### shouldHandoff returns true at threshold
Call `shouldHandoff(2, 2)`.
- Returns true

### shouldHandoff returns false below threshold
Call `shouldHandoff(1, 2)`.
- Returns false

### shouldHandoff zero threshold forces all fresh
Call `shouldHandoff(0, 0)`.
- Returns true (all retries use fresh mode)

### Quality retry uses shouldHandoff
Read the quality review retry path in `loop.go`.
- Uses `shouldHandoff(qAttempt, cfg.MaxResumeRetries)` instead of inline comparison

### Functional retry uses shouldHandoff
Read the functional review retry path in `loop.go`.
- Uses `shouldHandoff(attempt, cfg.MaxResumeRetries)` instead of inline comparison

### Drift check pattern is consistent
Read all `checkDriftAndClose` or `driftGuard` call sites in `loop.go`.
- All follow the same 3-line pattern: call, set status, return
