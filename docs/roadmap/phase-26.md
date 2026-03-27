## Phase 26: Merge Coordinator Agent

**Goal**: A dedicated merge coordinator agent resolves branch conflicts and
divergence anywhere in the pipeline — per-issue pre-merge, rollup merge, and
both `godark run` and `godark implement` modes. It appears as a visible step
in the review chain with full telemetry. Replaces the current fallback to the
implementer retry agent for conflict resolution, which uses the full
implementer context and is slower than necessary.

**Milestone**: `Phase 26: Merge Coordinator Agent` | **Label**: `phase-26`

### Prompt template and agent role
- `prompts/merge_coordinator.txt` with template variables for branch name,
  base branch, conflict description (git output), and PR context
- Agent permissions: `Read`, `Edit`, `Bash` (for git commands), `Glob`, `Grep`
- `merge_coordinator` path configurable in `godark.yaml` prompts block
- Prompt instructs agent to: check out the branch, rebase onto base, resolve
  conflicts preserving intent of both sides, run build/test to verify, push

### Agent function and config wiring
- `MergeCoordinate()` function in `internal/agent/merge_coordinator.go`
  following existing agent function pattern (`newRunOpts` + `Run()`)
- Role name: `"merge_coordinator"`
- Add `MergeCoordinator string` field to `Prompts` struct in config and
  agent prompt loader
- Bounded by `max_rebase_attempts` (existing config field, default 1)

### Per-issue pre-merge integration
- Replace `Retry()` fallback in `runPreMergeRebasePhase()` in
  `internal/agent/loop.go` with `MergeCoordinate()` when `gh pr update-branch`
  fails
- After successful conflict resolution, re-run verify pipeline (existing
  behavior preserved)
- Works in both `godark run` and `godark implement` (the agent loop is shared)

### Rollup merge conflict handling
- Add conflict detection in `handleRollupPR()` in
  `internal/orchestrator/orchestrator.go` before `mergeRollupPRFn`
- Check PR mergeable status; if CONFLICTING, invoke merge coordinator
- Bounded by `max_rebase_attempts`; if exhausted, leave PR open for human
  review (same pattern as per-issue)

### Review chain visibility
- "Merge Coordinate" appears as a dedicated step in the dashboard issue detail
  review chain timeline with duration, cost, peak memory, CPU time
- Surface in TUI as a stage transition (recon → implement → verify → review →
  merge coordinate → merged)

### Run data
- `merge_coordinate.json` per issue recording duration, cost, session ID,
  attempt count, outcome (resolved/exhausted/error)
- Rollup merge coordinate result written alongside rollup verify result
- Quality flags for merge coordinator failures surfaced in dashboard

**Issues**: #605–#611

**Planning doc**: `docs/planning/phase-26-merge-coordinator-agent.md`

