## Phase 28: Container Health Judge ✅

**Goal**: A Go-side health monitor watches container log streams in real-time
and intervenes when agents stall, thrash, or hit transport failures — cutting
losses in seconds instead of waiting for the 30-minute timeout. No LLM calls;
pure heuristic pattern matching on structured log output.

**Milestone**: `Phase 28: Container Health Judge` | **Label**: `phase-28`

### Real-time log streaming
- Change `RunContainer` to stream `docker logs --follow` via a goroutine
  instead of waiting for container exit then reading logs
- Feed lines to a callback as they arrive
- Preserve existing behavior: full stdout/stderr still captured in `RunResult`

### Pattern matcher and rules engine
- New `internal/agent/judge/` package with configurable rules:
  - **Idle timeout**: no tool call audit lines for N seconds (default 180s
    for recon/spec, 300s for implementer) — distinguishes "no output at all"
    from "streaming assistant text but no tool calls"
  - **Tool thrash**: 3+ ToolSearch calls for the same query pattern within
    60s — agent is searching for an unavailable tool
  - **Transport failure**: 10+ stream-closed errors with zero tool calls —
    SDK transport is dead
- Each rule produces a `Judgment` (kill, retry-container, warn, ignore)
- Rules are configurable via `judge:` config block in `godark.yaml`

### Structured diagnostics
- When the judge intervenes, produce a structured `JudgeIntervention` record:
  - What was detected (pattern, counts, timing)
  - Which rule triggered
  - What the operator should check (specific prompt file, config field, or
    env var when applicable)
- Intervention records written to run data for dashboard and `godark analyze`
- Surfaced in TUI and notifications

### Wire into agent loop
- Connect the judge to `RunContainer` via the log streaming callback
- Handle kill decisions: stop container, return partial result
- Integrate with retry logic: transport failures trigger container retry,
  tool thrash triggers step skip with diagnostic
- Judge interventions visible as a distinct event type in the review chain

### Container retry for transport failures
- When the judge detects a transport failure (zero tool calls + stream errors),
  automatically retry the container (not the whole step) up to N times
- Distinct from the existing agent retry logic which re-runs the full step
  with reviewer feedback

**Issues**: #640–#649

**Planning doc**: `docs/planning/phase-28-container-health-judge.md`

