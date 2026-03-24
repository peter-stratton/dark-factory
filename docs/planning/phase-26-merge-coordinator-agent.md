# Phase 26: Merge Coordinator Agent

> **Goal:** A dedicated merge coordinator agent resolves branch conflicts and
> divergence anywhere in the pipeline — per-issue pre-merge, rollup merge, and
> both `godark run` and `godark implement` modes. It appears as a visible step
> in the review chain with full telemetry. Replaces the current fallback to the
> implementer retry agent for conflict resolution.

## Milestone

`Phase 26: Merge Coordinator Agent`

---

## Issue 605: Merge coordinator prompt template and role

### Description

Add the merge coordinator prompt template and agent role. The merge coordinator
is a focused agent whose only job is to rebase a branch onto the updated base
and resolve conflicts. It understands both sides of the conflict and preserves
the intent of each change.

### Key constraints

- New file `prompts/merge_coordinator.txt` with template variables:
  - `{{.BaseBranch}}` — the target branch to rebase onto
  - `{{.IssueTitle}}`, `{{.IssueBody}}` — issue context for understanding
    intent of the feature branch changes
  - `{{.ConflictInfo}}` — git output describing the conflict (passed at render
    time, not from PromptData; use a dedicated template variable)
  - Standard `PromptData` fields available for architecture/conventions context
- Prompt instructs the agent to:
  - Check out the feature branch
  - Rebase onto the base branch
  - Resolve conflicts preserving intent of both sides
  - Run `{{.BuildCommand}}` and `{{.TestCommand}}` to verify the resolution
  - Push the result
- New `merge_coordinator` entry in `_ROLE_PERMISSIONS` in
  `internal/agent/runner/agent_runner.py` (line ~38):
  - `allowed_tools: ["Read", "Edit", "Bash", "Glob", "Grep"]`
  - `disallowed_tools: ["Write"]` — edits only, no new files; conflict
    resolution modifies existing files
- Update `prompts/embed.go` if needed (the `//go:embed` directive uses `*.txt`
  glob so it should pick up the new file automatically)

### Acceptance criteria

- [ ] `prompts/merge_coordinator.txt` template exists and renders with standard
  `PromptData` fields plus a `ConflictInfo` variable
- [ ] `merge_coordinator` role defined in `_ROLE_PERMISSIONS` with
  `allowed_tools: [Read, Edit, Bash, Glob, Grep]` and
  `disallowed_tools: [Write]`
- [ ] Role permissions tested (same pattern as existing role tests in
  `test_hooks.py`)

### Test cases

- **Role permissions**: `merge_coordinator` role allows Read, Edit, Bash, Glob,
  Grep and disallows Write
- **Prompt renders**: `RenderPrompt` with standard `PromptData` plus
  `ConflictInfo` produces valid prompt text containing the branch name and
  conflict description

---

## Issue 606: Merge coordinator config and prompt loading

**Blocked by**: #605

### Description

Add the `merge_coordinator` config field and wire it into prompt loading so the
merge coordinator prompt template is loaded alongside existing prompts.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `MergeCoordinator string` with yaml tag `merge_coordinator` to the
    `Prompts` struct (line ~338)
- Modify `internal/agent/prompt.go`:
  - Add `MergeCoordinator string` field to the agent `Prompts` struct (line ~25)
  - Add `ConflictInfo string` field to `PromptData` struct (line ~78) — the
    merge coordinator prompt needs this to describe the conflict
  - Add load logic in `LoadPrompts()` for `merge_coordinator.txt` — optional
    pattern (same as recon: empty string if not configured, embedded default
    used)
- The `ConflictInfo` field on `PromptData` is set to empty string by default
  in `newPromptData()` and populated by the caller before rendering

### Acceptance criteria

- [ ] `prompts.merge_coordinator` path configurable in `godark.yaml`
- [ ] `LoadPrompts()` loads `merge_coordinator.txt` without error
- [ ] `ConflictInfo` field exists on `PromptData` and renders in templates

### Test cases

- **Config parses**: YAML with `prompts.merge_coordinator: custom/path.txt`
  deserializes correctly
- **LoadPrompts includes merge coordinator**: Call `LoadPrompts` with default
  paths; verify merge coordinator prompt loaded (non-empty string)
- **ConflictInfo renders**: Render merge coordinator template with `ConflictInfo`
  set; verify output contains the conflict text

---

## Issue 607: MergeCoordinate() agent function

**Blocked by**: #606

### Description

Add the `MergeCoordinate()` function in a new file
`internal/agent/merge_coordinator.go`, following the existing agent function
pattern (`newRunOpts` + `Run()`). This is the callable entry point that the
agent loop and orchestrator will use.

### Key constraints

- New file `internal/agent/merge_coordinator.go`
- Function signature:
  ```go
  func MergeCoordinate(ctx context.Context, issue github.Issue, prNum int,
      conflictInfo string, cfg *config.Config, prompts *Prompts,
      authEnv map[string]string, logger *slog.Logger) (*Result, error)
  ```
- Follows the `Recon()` pattern: `newPromptData` → set `ConflictInfo` on data →
  `RenderPrompt` → `newRunOpts` with role `"merge_coordinator"` → `Run()`
- The `conflictInfo` parameter is injected into `PromptData.ConflictInfo` before
  rendering
- Returns `(*Result, error)` — same as all other agent functions

### Acceptance criteria

- [ ] `MergeCoordinate()` function exists in
  `internal/agent/merge_coordinator.go`
- [ ] Function follows the `newRunOpts` + `Run()` pattern with role
  `"merge_coordinator"`

### Test cases

- **Function compiles**: `MergeCoordinate` is callable with the expected
  signature (compile-time check via test that references it)
- **ConflictInfo injected**: Verify that calling `MergeCoordinate` with a
  `conflictInfo` string results in `PromptData.ConflictInfo` being set when
  the prompt is rendered (use a test prompt template containing
  `{{.ConflictInfo}}`)

---

## Issue 608: Per-issue pre-merge integration

**Blocked by**: #607

### Description

Replace the `Retry()` fallback in `runPreMergeRebasePhase()` with
`MergeCoordinate()` when `gh pr update-branch` (automatic rebase) fails. This
is the primary insertion point — it applies to both `godark run` and
`godark implement` since the agent loop is shared.

### Key constraints

- Modify `runPreMergeRebasePhase()` in `internal/agent/loop.go` (line ~1277):
  - When `github.UpdateBranch()` fails (line ~1318), call `MergeCoordinate()`
    instead of `Retry()`
  - Pass the conflict info string (git error output) as `conflictInfo`
  - The merge coordinator does not use or update `sessionID` — it runs as an
    independent agent, not a session continuation of the implementer
  - After successful merge coordinator run, re-run `runVerifyPhase()` (existing
    behavior preserved)
  - If merge coordinator returns an error or non-zero exit, treat the attempt
    as failed and continue the retry loop (same as current Retry behavior)
- Bounded by existing `cfg.MaxRebaseAttempts` (no config changes needed)
- The `fixCycles` counter is not incremented for merge coordinator runs — it
  tracks implementer fix cycles, not conflict resolution

### Acceptance criteria

- [ ] `runPreMergeRebasePhase` calls `MergeCoordinate()` instead of `Retry()`
  when automatic rebase fails
- [ ] Verify pipeline re-runs after successful conflict resolution
- [ ] Max rebase attempts still respected
- [ ] Works in both `godark run` and `godark implement` (shared code path)

### Test cases

- **Successful conflict resolution**: Stub `CheckMergeable` to return
  CONFLICTING then MERGEABLE; stub `UpdateBranch` to fail; stub
  `MergeCoordinate` to succeed; verify verify pipeline runs and function
  returns `(false, nil)`
- **Failed resolution exhausts attempts**: Stub merge coordinator to succeed
  but `CheckMergeable` keeps returning CONFLICTING; verify function returns
  `(true, nil)` after `MaxRebaseAttempts`
- **Automatic rebase succeeds**: Stub `UpdateBranch` to succeed; verify
  `MergeCoordinate` is not invoked
- **No conflict no-op**: Stub `CheckMergeable` to return MERGEABLE; verify
  function returns immediately
- **Merge coordinator error**: Stub `MergeCoordinate` to return error; verify
  attempt is counted and loop continues

---

## Issue 609: Run data and RunDataHook wiring

**Blocked by**: #607

### Description

Add the run data recording infrastructure for merge coordinator results so they
persist to disk and are available to the dashboard and analysis pipeline.

### Key constraints

- Modify `internal/agent/runhook.go`:
  - Add `WriteMergeCoordinatorResult(issueNumber int, step rundata.StepResult) error`
    to the `RunDataHook` interface
- Modify `internal/rundata/writer.go`:
  - Add `WriteMergeCoordinatorResult(issueNum int, step StepResult) error`
    method on `Writer`
  - Writes to `issues/<issueNum>/merge_coordinator.json`
  - Follows the `WriteReconResult` pattern exactly
- Modify `internal/rundata/reader.go`:
  - Add `MergeCoordinator StepResult` field to `IssueDetail` struct (line ~29)
  - Load from `merge_coordinator.json` in the issue reader (same pattern as
    `recon.json`)
- Update any test mocks that implement `RunDataHook` — grep for types that
  implement the interface and add the new method

### Acceptance criteria

- [ ] `RunDataHook` interface includes `WriteMergeCoordinatorResult`
- [ ] `Writer.WriteMergeCoordinatorResult` writes `merge_coordinator.json`
- [ ] `IssueDetail` includes `MergeCoordinator` field populated from JSON
- [ ] All `RunDataHook` implementations updated (no compile errors)

### Test cases

- **Write round-trip**: Write a merge coordinator result via Writer, read it
  back via reader, verify fields match
- **Missing file**: Reader returns zero-value `StepResult` when
  `merge_coordinator.json` does not exist
- **Hook interface satisfied**: Verify all concrete types implementing
  `RunDataHook` compile with the new method
- **Stats DB columns**: Verify merge coordinator step results flow through
  `FinalizeRun` to the stats database (if step results are written to stats)

---

## Issue 610: Rollup merge conflict handling

**Blocked by**: #607, #609

### Description

Add conflict detection and merge coordinator invocation in `handleRollupPR()`
so that rollup merges can recover from conflicts instead of just failing. When
the rollup PR (base branch → default branch) has conflicts, the merge
coordinator agent resolves them before the merge attempt.

### Key constraints

- Modify `handleRollupPR()` in `internal/orchestrator/orchestrator.go`
  (line ~1009):
  - After `upsertRollupPRFn()` creates the PR and before `mergeRollupPRFn()`
    merges it, check `github.CheckMergeable()` on the rollup PR
  - If CONFLICTING, invoke `agent.MergeCoordinate()` with rollup context
  - Bounded by `cfg.MaxRebaseAttempts` — if exhausted, leave the PR open for
    human review (do not error, just log and set `reporter.RollupCreated` with
    `merged: false`)
  - After successful conflict resolution, re-run rollup verify
    (`runRollupVerifyFn`)
  - The rollup merge coordinator needs a synthetic `github.Issue` or the
    function signature needs to accept conflict context differently — the
    simplest approach is to construct a minimal `github.Issue` with the rollup
    PR title/body for prompt rendering context
- Write rollup merge coordinator result to run data via `writer` if available
  (new method `WriteRollupMergeCoordinatorResult` on Writer, or reuse existing
  method with a sentinel issue number like 0)

### Acceptance criteria

- [ ] `handleRollupPR` checks mergeable status before merge attempt
- [ ] CONFLICTING rollup PR invokes merge coordinator
- [ ] Max rebase attempts respected; PR left open if exhausted
- [ ] Rollup verify re-runs after successful conflict resolution

### Test cases

- **Clean rollup**: Stub `CheckMergeable` to return MERGEABLE; verify merge
  coordinator is not invoked
- **Conflict resolved**: Stub CONFLICTING then MERGEABLE; verify merge
  coordinator called and rollup merges
- **Conflict unresolvable**: Stub CONFLICTING for all attempts; verify PR left
  open for human review, no error returned
- **Verify re-run**: After conflict resolution, verify `runRollupVerifyFn` is
  called again

---

## Issue 611: Dashboard and TUI integration

**Blocked by**: #609

### Description

Surface the merge coordinator as a visible step in the dashboard review chain
timeline and as a stage in the TUI, with the same duration/cost/memory/CPU
tooltip fields as other steps.

### Key constraints

- Modify `buildTimeline()` in `internal/dashboard/handlers.go` (line ~592):
  - Add a "Merge Coordinate" step after verify results and before the final
    merge outcome:
    ```go
    if hasStepData(issue.MergeCoordinator) {
        steps = append(steps, stepToView("Merge Coordinate", issue.MergeCoordinator))
    }
    ```
  - Position in timeline: after all review steps, before merged/failed outcome
- TUI stage updates in `internal/tui/`:
  - The TUI uses string-based stage labels sent via `IssueStageChangedMsg`
  - Add `"merge-coordinate"` stage emission in the agent loop (loop.go) when
    the merge coordinator is invoked — this is just a
    `reporter.IssueStageChanged(issue.Number, "merge-coordinate")` call
  - No struct changes needed in the TUI package itself — stages are free-form
    strings

### Acceptance criteria

- [ ] "Merge Coordinate" appears in dashboard review chain when
  `merge_coordinator.json` exists
- [ ] Step shows duration, cost, peak memory, and CPU time with hover tooltips
- [ ] TUI displays "merge-coordinate" stage during merge coordinator execution

### Test cases

- **Timeline includes step**: Build timeline from `IssueDetail` with
  `MergeCoordinator` populated; verify "Merge Coordinate" node appears
- **Timeline omits when absent**: Build timeline from `IssueDetail` with
  zero-value `MergeCoordinator`; verify no "Merge Coordinate" node
- **TUI stage**: Verify `IssueStageChangedMsg` with stage "merge-coordinate"
  updates the issue row display
