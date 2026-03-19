# Phase 14: Bounded Concurrency

> **Goal:** Independent issues within a run execute in parallel, bounded by a
> configurable worker pool. Dependent issues still respect topological ordering.
> Merge serialization ensures `main` stays linear. A dedicated merge coordinator
> agent resolves rebase conflicts that arise from concurrent branches.

## Milestone

`Phase 14`

---

## Issue 593: Add concurrency config block with max_workers field

### Description

Add a `Concurrency` struct and `concurrency` YAML block to the config. The only
field for now is `max_workers`, which controls how many issues are processed in
parallel within a wave. Default is 1 (preserving current serial behavior). The
nested struct leaves room for future knobs (cost caps, rate limits) without
restructuring the config.

### Key constraints

- Add `Concurrency` struct to `internal/config/config.go` with field
  `MaxWorkers int` (`yaml:"max_workers"`)
- Add `Concurrency *Concurrency` field to `Config` struct (nil = use defaults)
- Add `validateConcurrency()`: `max_workers` must be >= 1
- In `defaults()`, set `Concurrency` to `&Concurrency{MaxWorkers: 1}`
- When `concurrency` block is absent from YAML, defaults apply (max_workers: 1)

### Acceptance criteria

- [ ] `concurrency.max_workers` is parsed from `godark.yaml`
- [ ] Validation rejects `max_workers: 0` and negative values
- [ ] Absent `concurrency` block defaults to `max_workers: 1`
- [ ] Existing configs without `concurrency` block load without error

### Test cases

- **Valid config**: Parse config with `concurrency.max_workers: 3`; verify struct populated
- **Default value**: Parse config without `concurrency` block; verify `MaxWorkers == 1`
- **Zero rejected**: Parse config with `max_workers: 0`; verify validation error
- **Negative rejected**: Parse config with `max_workers: -1`; verify validation error
- **Partial block**: Parse config with `concurrency:` block but no `max_workers`; verify default applies

---

## Issue 597: Add --with-compose flag and concurrent mode logic

**Blocked by**: #593

### Description

Add a `--with-compose` CLI flag to `godark run` and `godark implement` that
forces single-worker integration mode when compose is configured. When
`max_workers > 1` and `--with-compose` is not set, nil out `DockerCompose` on
the config so compose services are skipped. When `--with-compose` is set,
override `max_workers` to 1. This keeps the mode decision in config resolution
rather than scattering conditions through the orchestrator.

Note: this issue touches 4 files but each change is 1–6 lines following the
established CLI flag pattern (see `parseCLIFlags()` in `cmdutil.go`).

### Key constraints

- Add `WithCompose *bool` field to `CLIFlags` in `internal/config/config.go`
- Add `f.Bool("with-compose", false, ...)` to flag definitions in both
  `internal/cmd/run.go` and `internal/cmd/implement.go`
- Add `--with-compose` parsing to `parseCLIFlags()` in `internal/cmd/cmdutil.go`
  (same `cmd.Flags().Changed()` pattern as existing flags)
- In `applyFlags()`: if `max_workers > 1` and `WithCompose` is not set, set
  `cfg.DockerCompose = nil` (compose skipped)
- In `applyFlags()`: if `WithCompose` is set, set `cfg.Concurrency.MaxWorkers = 1`
- Log a message when compose is skipped due to concurrent mode
- Warn (do not error) if `--with-compose` is set but no `docker_compose` block
  exists in config

### Acceptance criteria

- [ ] `--with-compose` flag is available on `godark run` and `godark implement`
- [ ] `max_workers > 1` without `--with-compose` nils out `DockerCompose`
- [ ] `--with-compose` forces `max_workers` to 1 regardless of config value
- [ ] Warning logged when `--with-compose` used without `docker_compose` config
- [ ] Compose services start normally when `max_workers: 1` (default behavior)

### Test cases

- **Concurrent skips compose**: Config with `max_workers: 3` and `docker_compose` set; verify `DockerCompose` is nil after flag resolution
- **With-compose forces serial**: Config with `max_workers: 3` and `--with-compose`; verify `MaxWorkers == 1` and `DockerCompose` preserved
- **No compose config warning**: Pass `--with-compose` without `docker_compose` block; verify warning logged
- **Default serial preserves compose**: Config with `max_workers: 1` and `docker_compose` set; verify compose preserved without flag
- **Flag not set no-op**: Config with `max_workers: 1`, no `--with-compose`; verify no change to compose or workers

---

## Issue 594: Per-issue log files

### Description

Replace the single run-level `debug.log` with per-issue log files at
`issues/{num}/debug.log`. Each worker creates its own `slog.Logger` pointing at
its issue's log file before calling `ProcessIssue`. This eliminates interleaved
log output when issues run concurrently and makes debugging specific issues
easier even in serial mode.

### Key constraints

- Add a function to `internal/logging/` that creates a logger for a given
  directory path (reuse existing `NewLoggerFileOnly` pattern)
- In the orchestrator's per-issue processing (currently lines 706–821 of
  `processIssues()`), create a per-issue logger using
  `writer.IssueDir(issue.Number)` as the log directory
- Pass the per-issue logger to `ProcessIssue()` instead of the run-level logger
- The orchestrator's own coordination events (wave dispatch, merge decisions,
  abort reasons) continue using the run-level logger
- `logFactory` parameter already supports creating loggers for arbitrary
  directories

### Acceptance criteria

- [ ] Each issue's agent execution logs write to `issues/{num}/debug.log`
- [ ] Concurrent issues do not interleave log output
- [ ] Orchestrator coordination events still log to run-level `debug.log`
- [ ] Per-issue log files are created inside existing `IssueDir` structure

### Test cases

- **Per-issue log created**: Process an issue; verify `issues/{num}/debug.log` exists
- **Separate log content**: Process two issues; verify each log contains only its own issue's events
- **Orchestrator log separate**: Verify wave and merge events appear in run-level `debug.log`, not per-issue logs
- **Log directory creation**: Verify `IssueDir` is created before logger attempts to write

---

## Issue 595: Extract per-issue processing into worker function

### Description

Refactor the per-issue processing body in `processIssues()` (currently lines
706–821 of `internal/orchestrator/orchestrator.go`) into a standalone function
that takes an issue and returns a result struct. This is a pure refactor — no
behavior change — that prepares for concurrent dispatch in the next issue. The
function encapsulates: calling `ProcessIssue`, writing dialogue, calculating
cost, and determining the outcome status.

### Key constraints

- New function in `internal/orchestrator/orchestrator.go` (not a new file)
- Define a `waveResult` struct (or similar) that captures: issue number,
  outcome status, PR number, cost, error, and whether the issue was merged
- The function must not reference shared mutable state (`runStats`,
  `implementedIssues`, `allLockedNums`, `seen`) — those are updated by the
  caller after the function returns
- Punchlist enrichment goroutine stays in the caller (it has its own mutex)
- The existing behavior must be byte-for-byte identical after refactor

### Acceptance criteria

- [ ] Per-issue processing body extracted into a named function
- [ ] Function returns a result struct with all data needed by the caller
- [ ] No shared mutable state accessed inside the function
- [ ] All existing tests pass without modification
- [ ] Run behavior is identical before and after refactor

### Test cases

- **Existing test suite passes**: Run `go test ./internal/orchestrator/...` with no changes to tests
- **Successful issue**: Call worker function with a passing issue; verify result contains StatusImplemented and PR number
- **Failed issue**: Call worker function with a failing issue; verify result contains StatusFailed and error
- **No shared state mutation**: Verify function signature takes only immutable inputs (issue, config, prompts, etc.)

---

## Issue 598: Wave barrier dispatcher with concurrent workers

**Blocked by**: #593, #595

### Description

Replace the serial issue loop in `processIssues()` with a wave-barrier
dispatcher. Each wave identifies processable (unblocked) issues, dispatches up
to `min(max_workers, wave_size)` goroutines running the extracted worker
function, waits for all to complete via `sync.WaitGroup`, then collects results.
Shared state (`runStats`, `implementedIssues`, `allLockedNums`) is updated by
the main goroutine after the wave completes — not by workers.

### Key constraints

- Modify the wave loop in `processIssues()` (`internal/orchestrator/orchestrator.go`)
- Workers send results to a channel; main goroutine reads all results after
  `WaitGroup.Wait()`
- Worker count per wave: `min(cfg.Concurrency.MaxWorkers, len(batch))`
- Each worker receives its own per-issue logger (from the per-issue log files
  issue)
- `seen` map is populated before dispatch (not by workers)
- Context cancellation must propagate to all workers
- When `max_workers: 1`, behavior is identical to current serial execution

### Acceptance criteria

- [ ] Independent issues in a wave execute concurrently
- [ ] Worker count does not exceed `max_workers` or wave size
- [ ] Shared state is only updated after wave completes (no mutex needed on counters)
- [ ] Context cancellation stops all workers
- [ ] `max_workers: 1` produces identical behavior to pre-concurrency code

### Test cases

- **Serial mode unchanged**: Run with `max_workers: 1`; verify identical output to current behavior
- **Concurrent dispatch**: Run with `max_workers: 3` and 3 independent issues; verify all three start before any finishes (use timing or mock agents)
- **Worker cap respected**: Run with `max_workers: 2` and 5 independent issues; verify at most 2 run simultaneously
- **Context cancellation**: Cancel context mid-wave; verify all workers exit
- **Result collection**: Verify all worker results are collected after wave, with correct issue numbers and statuses

---

## Issue 599: Post-wave merge serializer and failure abort

**Blocked by**: #598

### Description

After a wave completes, merge all successful issues serially and handle
failures. Successful issues (StatusImplemented) are squash-merged one at a time
with a rebase check between each merge (using existing
`runPreMergeRebasePhase`). If any issue in the wave failed, merge the successes
and then abort the run — do not dispatch further waves. Failed and blocked
issues are reported in the run summary.

### Key constraints

- Modify the post-wave section of `processIssues()` in
  `internal/orchestrator/orchestrator.go`
- Merge order: by issue number (stable, deterministic)
- After each merge, the existing rebase phase runs for the next PR in the queue
- If rebase fails and exhausts `max_rebase_attempts`, that issue is flagged
  `needs-human-review` (existing behavior)
- After all merges, if any issue in the wave had StatusFailed, set an abort
  reason and skip further waves
- Issues blocked by a failed issue are reported as StatusBlocked in the summary
- Dependency re-resolution runs only if at least one merge succeeded and no
  failures occurred

### Acceptance criteria

- [ ] Successful issues merge serially after wave completes
- [ ] Merge order is by issue number
- [ ] Rebase check runs between consecutive merges
- [ ] Wave failure aborts the run after merging successes
- [ ] Blocked issues are counted in the run summary
- [ ] Re-resolution runs only when all issues in the wave succeeded

### Test cases

- **All succeed**: Wave of 3 issues all pass; verify all 3 merge in order and re-resolution runs
- **Mixed results**: Wave of 3 issues, 1 fails; verify 2 merge, run aborts, remaining issues reported blocked
- **All fail**: Wave of 3 issues all fail; verify no merges attempted, run aborts
- **Merge order**: Verify issues merge in ascending issue number order regardless of completion order
- **Rebase between merges**: Verify rebase phase runs for 2nd and 3rd merges after the 1st
- **Rebase failure**: One PR fails rebase after max attempts; verify it's labeled needs-human-review, other successes still merge

---

## Issue 596: Merge coordinator prompt template

### Description

Add a new prompt template for the merge coordinator agent — a focused agent
whose only job is to rebase a branch onto the updated base and resolve conflicts.
This replaces the current fallback to the retry agent for conflict resolution,
which uses the full implementer context and is slower than necessary.

### Key constraints

- Create `prompts/merge_coordinator.txt` with template variables for branch
  name, base branch, conflict description (git output), and PR context
- Add `MergeCoordinator string` field to `Prompts` struct in
  `internal/config/config.go` (`yaml:"merge_coordinator"`)
- Add `MergeCoordinator` field to agent `Prompts` struct in
  `internal/agent/prompt.go`
- Update `LoadPrompts()` in `internal/agent/prompt.go` to load
  `merge_coordinator.txt` (same `loadPromptFile` pattern as existing prompts)
- Prompt should instruct the agent to: check out the branch, rebase onto base,
  resolve conflicts preserving intent of both sides, run build/test to verify,
  push the result
- Agent permissions: `Read`, `Edit`, `Bash` (for git commands), `Glob`, `Grep`

### Acceptance criteria

- [ ] `prompts/merge_coordinator.txt` exists with conflict resolution instructions
- [ ] `merge_coordinator` path configurable in `godark.yaml` prompts block
- [ ] `LoadPrompts()` loads the template without error
- [ ] Template renders with branch, base branch, and conflict context variables

### Test cases

- **Template loads**: Call `LoadPrompts` with default paths; verify merge coordinator prompt loaded
- **Template renders**: Render with sample PromptData including branch and conflict info; verify output contains branch names
- **Custom path**: Set `prompts.merge_coordinator` in config; verify custom file loaded
- **Missing file fallback**: Verify embedded default is used when custom path not set

---

## Issue 600: Merge coordinator agent function and wiring

**Blocked by**: #596

### Description

Add a `MergeCoordinate()` function in `internal/agent/` that invokes the merge
coordinator agent, and wire it into `runPreMergeRebasePhase()` in
`internal/agent/loop.go` as the fallback when `gh pr update-branch` (automatic
rebase) fails. This replaces the current fallback to the retry agent for
conflict resolution.

Note: this issue creates a new file (`merge_coordinator.go`) and modifies
`loop.go`. The new file is a single function following the exact pattern of
existing agent functions (`Recon()`, `Review()`, etc.), and the modification is
replacing one function call in `runPreMergeRebasePhase`.

### Key constraints

- New file `internal/agent/merge_coordinator.go` with `MergeCoordinate()`
  function following the existing agent function pattern (`newRunOpts` +
  `Run()`)
- Role name: `"merge_coordinator"`
- The function receives: rendered prompt, config, auth env, logger
- Modify `runPreMergeRebasePhase()` in `internal/agent/loop.go` (around line
  1329) to call `MergeCoordinate()` instead of the retry agent when automatic
  rebase fails
- Bounded by `cfg.MaxRebaseAttempts` (existing field, default 1)
- After successful conflict resolution, re-run verify pipeline (existing
  behavior)
- Write merge coordinator result to run data (duration, cost, session ID)

### Acceptance criteria

- [ ] `MergeCoordinate()` function exists and follows agent function pattern
- [ ] Automatic rebase failure invokes merge coordinator instead of retry agent
- [ ] Merge coordinator is bounded by `max_rebase_attempts`
- [ ] Verify pipeline re-runs after successful conflict resolution
- [ ] Merge coordinator result recorded in run data

### Test cases

- **Successful conflict resolution**: Stub agent to succeed; verify branch is rebased and verify pipeline runs
- **Failed resolution**: Stub agent to fail; verify PR labeled needs-human-review after max attempts
- **Max attempts respected**: Set `max_rebase_attempts: 2`; verify coordinator called at most twice
- **No conflict no-op**: Automatic rebase succeeds; verify merge coordinator is not invoked
- **Run data recorded**: Verify merge coordinator step result includes duration and cost

---

## Issue 601: TUI concurrent status display

**Blocked by**: #598

### Description

Update the TUI to display multiple in-progress issues simultaneously and show
a worker utilization indicator. The existing message model already keys on issue
number, so concurrent `IssueStageChangedMsg` messages work without changes to
the message types. The visual changes are: multiple rows showing spinners at
once, and a worker count in the summary bar.

### Key constraints

- Modify `internal/tui/model.go` to track active worker count
- Add a `WorkersActiveMsg` message type to `internal/tui/messages.go`
  (sent by orchestrator at wave start/end)
- Update the summary bar in `internal/tui/view.go` to show
  `"N/M workers active"` alongside existing counts
- Multiple table rows can show the in-progress spinner simultaneously (verify
  this works — the current model may already support it)
- No changes needed to `IssueStageChangedMsg` or `IssueCompletedMsg`

### Acceptance criteria

- [ ] Multiple issues show in-progress spinners simultaneously during a wave
- [ ] Summary bar displays active worker count
- [ ] Worker count updates at wave boundaries
- [ ] Serial mode (`max_workers: 1`) displays identically to current behavior

### Test cases

- **Concurrent spinners**: Send two `IssueStartedMsg` without completing the first; verify both rows show in-progress state
- **Worker count display**: Send `WorkersActiveMsg{Active: 3, Total: 5}`; verify summary bar shows "3/5 workers active"
- **Wave transition**: Complete a wave and start a new one; verify worker count resets
- **Serial mode**: Run with `max_workers: 1`; verify no worker count displayed (or "1/1")

---

## Issue 602: Dashboard concurrent status display

**Blocked by**: #598

### Description

Surface concurrent worker information in the `godark status` dashboard run
detail view. Show which issues ran concurrently (wave grouping), worker
utilization per wave, and total wall-clock time saved versus serial execution.

### Key constraints

- Modify run detail template in `internal/dashboard/` to show wave grouping
- Add wave metadata to run data: which issues were in each wave, wave start/end
  times
- This requires a small addition to `internal/rundata/writer.go` — a
  `WriteWaveResult()` method that records wave number, issue numbers, and timing
- Display wall-clock comparison: actual duration vs sum of individual issue
  durations (shows concurrency benefit)

### Acceptance criteria

- [ ] Run detail view shows issues grouped by wave
- [ ] Wave timing (start, end, duration) displayed per wave
- [ ] Wall-clock savings shown (serial estimate vs actual)
- [ ] Serial runs (single wave, one issue at a time) display without wave grouping

### Test cases

- **Wave grouping visible**: Run with 2 waves; verify dashboard shows wave 1 and wave 2 sections
- **Timing display**: Verify each wave shows duration
- **Wall-clock savings**: Run with 3 concurrent issues; verify savings calculation is positive
- **Serial fallback**: Run with `max_workers: 1`; verify no wave grouping shown
