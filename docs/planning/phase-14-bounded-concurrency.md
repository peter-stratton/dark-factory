# Phase 14: Bounded Concurrency

> **Goal:** Independent issues within a run execute in parallel, bounded by a
> configurable worker pool. Dependent issues still respect topological ordering.
> Wave-barrier dispatch groups independent issues into waves, processes them
> concurrently, then merges all successes serially before re-resolving
> dependencies for the next wave.

## Milestone

`Phase 14`

---

## Issue 745: Add concurrency config block with max_workers field

### Description

Add a `Concurrency` struct and `concurrency` YAML block to the config. The only
field for now is `MaxWorkers`, which controls how many issues are processed in
parallel within a wave. Default is 1, preserving current serial behavior.

This issue touches 2 files: `internal/config/config.go` (add struct + field +
validation) and `internal/config/config_test.go` (add tests).

### Key constraints

- Add to `internal/config/config.go`:
  ```go
  type Concurrency struct {
      MaxWorkers int `yaml:"max_workers"`
  }
  ```
- Add `Concurrency Concurrency `yaml:"concurrency"`` field to `Config` struct
  (not a pointer - zero value MaxWorkers=0 is caught by validation)
- Add `validateConcurrency()`: if `Concurrency.MaxWorkers` is set (non-zero),
  it must be >= 1. When absent from YAML (zero value), default to 1.
- Apply default in `applyDefaults()`: if `MaxWorkers == 0`, set to 1
- No behavior changes anywhere else - this is config-only

### Acceptance criteria

- [ ] `concurrency.max_workers` is parsed from `godark.yaml`
- [ ] Absent `concurrency` block defaults to `max_workers: 1`
- [ ] Validation rejects `max_workers: 0` and negative values
- [ ] Existing configs without `concurrency` block load without error

### Test cases

- **Valid config**: Parse config with `concurrency.max_workers: 3`; verify
  `cfg.Concurrency.MaxWorkers == 3`
- **Default value**: Parse config without `concurrency` block; verify
  `MaxWorkers == 1`
- **Negative rejected**: Parse config with `max_workers: -1`; verify
  validation error

---

## Issue 748: Add --with-compose flag and concurrent mode logic

**Blocked by**: #745

### Description

Add a `--with-compose` CLI flag to `godark run` and `godark implement` that
forces single-worker mode when compose is configured. When `max_workers > 1`
and `--with-compose` is not set, nil out `DockerCompose` on the config so
compose services are skipped. When `--with-compose` is set, override
`max_workers` to 1.

This issue touches 4 files but each change is 1-6 lines following the
established CLI flag pattern. Files: `internal/config/config.go` (add
`WithCompose` to `CLIFlags`), `internal/cmd/run.go` (add flag definition),
`internal/cmd/implement.go` (add flag definition), `internal/cmd/cmdutil.go`
(parse flag in `parseCLIFlags`).

### Key constraints

- Add `WithCompose *bool` field to `CLIFlags` in `internal/config/config.go`
- Add `f.Bool("with-compose", false, ...)` to flag definitions in both
  `internal/cmd/run.go` and `internal/cmd/implement.go`
- Add `--with-compose` parsing to `parseCLIFlags()` in
  `internal/cmd/cmdutil.go` (same `cmd.Flags().Changed()` pattern as existing
  flags)
- In `applyFlags()`: if `MaxWorkers > 1` and `WithCompose` is not set, set
  `cfg.DockerCompose = nil` and log "compose skipped: max_workers > 1"
- In `applyFlags()`: if `WithCompose` is set, set
  `cfg.Concurrency.MaxWorkers = 1`
- Warn (do not error) if `--with-compose` is set but no `docker_compose` block
  exists in config

### Acceptance criteria

- [ ] `--with-compose` flag is available on `godark run` and `godark implement`
- [ ] `max_workers > 1` without `--with-compose` nils out `DockerCompose`
- [ ] `--with-compose` forces `max_workers` to 1 regardless of config value
- [ ] Warning logged when `--with-compose` used without `docker_compose` config

### Test cases

- **Concurrent skips compose**: Config with `max_workers: 3` and
  `docker_compose` set; verify `DockerCompose` is nil after flag resolution
- **With-compose forces serial**: Config with `max_workers: 3` and
  `--with-compose`; verify `MaxWorkers == 1` and `DockerCompose` preserved
- **No compose config warning**: Pass `--with-compose` without `docker_compose`
  block; verify warning logged
- **Default serial preserves compose**: Config with `max_workers: 1` and
  `docker_compose` set; verify compose preserved without flag

---

## Issue 746: Thread-safe run data writer

### Description

Add a mutex to `Writer` in `internal/rundata/writer.go` to protect methods that
perform read-modify-write on `run.json`. Per-issue Write* methods (which write
to separate `issues/<num>/` directories) are already safe for concurrent calls
to different issue numbers, but the run-level methods race when called from
concurrent workers.

This issue touches 2 files: `internal/rundata/writer.go` (add mutex, wrap
methods) and `internal/rundata/writer_test.go` (add concurrent test).

### Key constraints

- Add `mu sync.Mutex` field to the `Writer` struct (line 108)
- Wrap the following methods with `w.mu.Lock()` / `defer w.mu.Unlock()`:
  - `WriteIssueDeps()` (line 318) - reads run.json, sets IssueDeps, writes back
  - `WriteIssueTitles()` (line 340) - reads run.json, sets IssueTitles, writes
    back
  - `SetRateLimit()` (line 538) - reads run.json, sets RateLimitResetsAt,
    writes back
  - `ClearRateLimit()` (line 560) - reads run.json, clears RateLimitResetsAt,
    writes back
  - `FinalizeRun()` (line 581) - reads run.json, sets FinishedAt and Summary,
    writes back
- `WriteJudgeIntervention()` (line 470) does a read-append-write on
  `judge-interventions.json` per issue. Since concurrent workers process
  different issues, this is safe without a mutex. Add a code comment noting
  this assumption.
- Per-issue methods (`WriteOutcome`, `WriteImplementResult`, etc.) do NOT need
  the mutex - they write to `issues/<num>/` which is unique per worker
- Do not change method signatures - the mutex is internal to `Writer`

### Acceptance criteria

- [ ] `Writer` struct has a `mu sync.Mutex` field
- [ ] `WriteIssueDeps`, `WriteIssueTitles`, `SetRateLimit`, `ClearRateLimit`,
      and `FinalizeRun` acquire the mutex
- [ ] Per-issue Write* methods do not acquire the mutex
- [ ] Concurrent calls to different Write* methods do not corrupt run.json

### Test cases

- **Concurrent run.json writes**: Launch 10 goroutines calling
  `SetRateLimit`/`ClearRateLimit` concurrently; verify run.json is valid JSON
  after all complete
- **Per-issue writes unblocked**: Call `WriteImplementResult` for two different
  issue numbers concurrently; verify both files are written correctly
- **Existing tests pass**: All existing writer tests pass without modification

---

## Issue 747: Per-issue log files

### Description

Create per-issue log files at `issues/{num}/debug.log` so concurrent workers
don't interleave log output. Each worker gets its own `*slog.Logger` pointing
at its issue's log directory. The orchestrator's coordination events (wave
dispatch, merge decisions) continue using the run-level logger.

This issue touches 2 files: `internal/orchestrator/orchestrator.go` (create
per-issue logger before calling processIssueFn) and `internal/logging/` (add a
helper if one doesn't already exist for creating file-based loggers).

### Key constraints

- Add a function to `internal/logging/` (or reuse an existing one) that creates
  a `*slog.Logger` writing to a file in a given directory:
  ```go
  func NewFileLogger(dir string) (*slog.Logger, error)
  ```
- In the per-issue processing section of `processIssues()` (currently line 760),
  create a per-issue logger using `writer.IssueDir(issue.Number)` as the
  directory, and pass it to `processIssueFn` instead of the run-level logger
- If `writer` is nil (e.g., dry-run mode), fall back to the run-level logger
- The per-issue logger must be created before `processIssueFn` is called and
  does not need cleanup (file is closed when the logger is garbage collected,
  or use a deferred close)
- Orchestrator coordination events (wave start, merge, abort, lock/unlock)
  continue using the run-level `logger` parameter

### Acceptance criteria

- [ ] Each issue's agent execution logs write to `issues/{num}/debug.log`
- [ ] Orchestrator coordination events log to run-level logger, not per-issue
- [ ] Per-issue log files are created inside existing `IssueDir` structure
- [ ] Nil writer falls back to run-level logger gracefully

### Test cases

- **Per-issue log created**: Process an issue with a writer; verify
  `issues/{num}/debug.log` exists and contains log entries
- **Orchestrator log separate**: Verify wave-start and merge events do not
  appear in per-issue logs
- **Nil writer fallback**: Process an issue without a writer; verify no panic
  and logging goes to run-level logger

---

## Issue 749: Extract per-issue processing into worker function

**Blocked by**: #747

### Description

Refactor the per-issue processing body in `processIssues()` (lines 752-897)
into a standalone function that takes an issue and returns a result struct. This
is a pure refactor with no behavior change, preparing for concurrent dispatch.

The function encapsulates: calling `processIssueFn`, writing dialogue,
calculating cost, determining outcome status, and spawning the background
punchlist enrichment goroutine.

This issue touches 2 files: `internal/orchestrator/orchestrator.go` (extract
function) and `internal/orchestrator/orchestrator_test.go` (verify existing
tests pass).

### Key constraints

- New unexported function in `internal/orchestrator/orchestrator.go`:
  ```go
  func runOneIssue(ctx context.Context, issue github.Issue, cfg *config.Config,
      prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger,
      hook agent.RunDataHook, reporter progress.ProgressReporter,
      writer *rundata.Writer) waveResult
  ```
- Define `waveResult` struct:
  ```go
  type waveResult struct {
      IssueNumber  int
      Issue        github.Issue
      Outcome      agent.IssueOutcome
      IssueCost    float64
      Merged       bool
      UsageLimited bool
      ResetsAt     time.Time
  }
  ```
- The function must NOT access shared mutable state (`runStats`, `seen`,
  `implementedIssues`, `allLockedNums`, `justMergedNums`). Those are updated
  by the caller after the function returns.
- Punchlist background enrichment goroutine moves inside `runOneIssue` but
  uses the existing `plMu`/`plWg`/`plEntries` passed as parameters (or
  returned for the caller to handle). Simplest: return punchlist data in
  `waveResult` and let the caller spawn the goroutine.
- The existing `for _, issue := range batch` loop calls `runOneIssue` and then
  applies the result to shared state, exactly reproducing the current behavior
- All existing orchestrator tests must pass without modification

### Acceptance criteria

- [ ] Per-issue processing body extracted into `runOneIssue` function
- [ ] `waveResult` struct captures all data needed by the caller
- [ ] Function does not access shared mutable state
- [ ] All existing tests pass without modification
- [ ] Run behavior is identical before and after refactor

### Test cases

- **Existing test suite passes**: Run `go test ./internal/orchestrator/...`
  with no test changes
- **Successful issue result**: Call `runOneIssue` with a passing stub; verify
  result has `Merged: true` and correct PR number
- **Failed issue result**: Call `runOneIssue` with a failing stub; verify
  result has `Merged: false` and error populated
- **No shared state mutation**: Verify function signature takes only values
  and pointers to immutable/per-issue data

---

## Issue 750: Wave barrier dispatcher with concurrent workers

**Blocked by**: #745, #749

### Description

Replace the serial issue loop in `processIssues()` with a wave-barrier
dispatcher. Each wave identifies processable (unblocked) issues, dispatches up
to `min(max_workers, wave_size)` goroutines running `runOneIssue`, waits for
all to complete, then collects results via a channel. Shared state is updated
by the main goroutine after the wave completes.

This issue touches 2 files: `internal/orchestrator/orchestrator.go` (replace
serial loop with concurrent dispatch) and
`internal/orchestrator/orchestrator_test.go` (add concurrency tests).

### Key constraints

- Replace the `for _, issue := range batch` loop (currently line 752) with:
  ```go
  results := make(chan waveResult, len(batch))
  sem := make(chan struct{}, cfg.Concurrency.MaxWorkers)
  var wg sync.WaitGroup
  for _, issue := range batch {
      sem <- struct{}{}
      wg.Add(1)
      go func(iss github.Issue) {
          defer wg.Done()
          defer func() { <-sem }()
          issueLogger := createIssueLogger(writer, iss.Number, logger)
          results <- runOneIssue(ctx, iss, cfg, prompts, authEnv, issueLogger, hook, reporter, writer)
      }(issue)
  }
  wg.Wait()
  close(results)
  ```
- Mark all issues in the batch as `seen` before dispatch (not inside workers)
- Each worker gets its own per-issue logger (from the per-issue log files
  issue)
- Context cancellation propagates to all workers via the shared `ctx`
- When `max_workers: 1`, behavior is identical to current serial execution
  (one goroutine at a time, same ordering)
- The semaphore pattern within a wave gives us bounded concurrency while
  the wave barrier gives us the clean merge point
- Do NOT handle rate limits or merges in this issue - those come in the
  next two issues. This issue just dispatches and collects results.

### Acceptance criteria

- [ ] Independent issues in a wave execute concurrently
- [ ] Worker count does not exceed `max_workers` or wave size
- [ ] Context cancellation stops all workers
- [ ] `max_workers: 1` produces identical behavior to pre-concurrency code
- [ ] All issues in batch are marked `seen` before dispatch

### Test cases

- **Serial mode unchanged**: Run with `max_workers: 1`; verify identical
  output to current behavior
- **Concurrent dispatch**: Run with `max_workers: 3` and 3 independent issues;
  verify all three workers run (use timing or stub agent with sleep)
- **Worker cap respected**: Run with `max_workers: 2` and 5 independent issues;
  verify at most 2 run simultaneously (use a shared counter with mutex)
- **Context cancellation**: Cancel context mid-wave; verify all workers exit
  and results channel is drained
- **Result collection**: Verify all worker results are collected with correct
  issue numbers and statuses

---

## Issue 751: Post-wave merge serializer and continuation

**Blocked by**: #750

### Description

After a wave completes, process the collected results: merge all successful
issues serially, handle failures (continue, don't abort), and re-resolve
dependencies for the next wave. Failed issues are marked failed but do not
stop the run - any issues unblocked by the successes are processed in the
next wave.

This issue touches 2 files: `internal/orchestrator/orchestrator.go` (post-wave
result processing) and `internal/orchestrator/orchestrator_test.go` (add
tests).

### Key constraints

- After the wave completes and results are collected from the channel:
  1. Sort results by issue number for deterministic merge order
  2. For each result, update shared state (`runStats`, `implementedIssues`,
     report completion via `reporter.IssueCompleted`)
  3. Spawn punchlist enrichment goroutines for issues with PRs (existing
     `plWg`/`plMu`/`plEntries` pattern)
  4. Collect successfully merged issues into `justMergedNums`
  5. If any results are `Merged: true`, run `refreshAndCategorize` to
     re-resolve dependencies and continue to the next wave
  6. If no results are merged and no rate-limit hold, break the wave loop
- Failed issues: increment `runStats.failed`, report via reporter, but do
  NOT set an abort reason. The run continues.
- Issues that depend on a failed issue will not appear in `processable` after
  re-resolution - they remain in the `blocked` list and are counted in
  `runStats.blocked` at the end
- Merge order: ascending issue number (stable, deterministic)
- The existing `runPreMergeRebasePhase` and merge coordinator infrastructure
  (from Phase 26) handles rebase conflicts between consecutive merges
- After each merge, the base branch is updated via `PullAfterMerge` (existing
  behavior)

### Acceptance criteria

- [ ] Successful issues merge serially after wave completes
- [ ] Merge order is by ascending issue number
- [ ] Failed issues do not abort the run
- [ ] Issues blocked by failed issues are reported as blocked in summary
- [ ] Re-resolution runs when at least one merge succeeded
- [ ] No re-resolution when nothing merged and no rate-limit hold

### Test cases

- **All succeed**: Wave of 3 issues all pass; verify all 3 merge in order
  and re-resolution runs
- **Mixed results**: Wave of 3 issues, 1 fails; verify 2 merge, run
  continues, failed issue counted
- **All fail**: Wave of 3 issues all fail; verify no merges attempted, wave
  loop exits
- **Merge order**: Issues complete in order 3, 1, 2; verify merge order is
  1, 2, 3
- **Blocked by failure**: Issue B depends on issue A. Wave 1: A fails, C
  succeeds. Wave 2: B is not in processable. Final summary: B is blocked.
- **Rebase between merges**: Verify rebase phase runs between consecutive
  merges after a wave

---

## Issue 752: Rate-limit handling in concurrent waves

**Blocked by**: #751

### Description

Handle Claude usage limits during concurrent waves. When any worker returns
with `UsageLimited: true`, the orchestrator lets the wave finish, merges any
successes, sleeps until the rate limit resets, then re-dispatches the
rate-limited issues in the next wave.

This issue touches 1 file: `internal/orchestrator/orchestrator.go` (add
rate-limit logic to post-wave result processing).

### Key constraints

- After collecting wave results and processing merges:
  1. Check if any result has `UsageLimited: true`
  2. If so, find the latest `ResetsAt` across all rate-limited results
  3. Remove rate-limited issues from `seen` so they are retried
  4. Sleep until `ResetsAt + 30s` buffer (existing pattern)
  5. Refresh GitHub App token after the hold (existing pattern)
  6. Report via `reporter.RateLimited(resetsAt)` /
     `reporter.RateLimitCleared()`
  7. If `writer != nil`, call `writer.SetRateLimit(resetsAt)` /
     `writer.ClearRateLimit()`
  8. Continue the wave loop - rate-limited issues will be re-batched
- The `const maxHold = 6 * time.Hour` check applies: if any reset time
  exceeds max hold, fail those issues instead of sleeping
- Context cancellation during the sleep breaks the hold (existing pattern)
- This replaces the current per-issue rate-limit handling in the serial
  loop (lines 762-798). That code is removed and replaced by the post-wave
  check.

### Acceptance criteria

- [ ] Rate-limited issues are retried after the hold period
- [ ] Non-rate-limited successes in the same wave still merge
- [ ] Latest `ResetsAt` is used when multiple workers hit the limit
- [ ] Max hold exceeded fails the rate-limited issues
- [ ] Context cancellation during sleep exits cleanly

### Test cases

- **Single rate limit**: Wave of 3 issues, 1 hits rate limit, 2 succeed;
  verify 2 merge, sleep until reset, rate-limited issue retried in next wave
- **All rate limited**: Wave of 3 issues all rate limited; verify no merges,
  sleep, all retried
- **Mixed rate limit and failure**: 1 rate limited, 1 failed, 1 succeeded;
  verify success merges, failure counted, rate-limited retried
- **Max hold exceeded**: Rate limit resets in 7 hours; verify issue is failed
  instead of holding
- **Context cancelled during hold**: Cancel context during sleep; verify
  clean exit

---

## Issue 753: TUI concurrent status display

**Blocked by**: #750

### Description

Update the TUI to display multiple in-progress issues simultaneously and show
a worker utilization indicator. The existing Bubble Tea message model already
keys on issue number, so concurrent `IssueStageChangedMsg` messages work
without changes to the message types.

This issue touches 4 files in `internal/tui/`: `messages.go` (add
`WorkersActiveMsg`), `model.go` (track active count, handle new message),
`view.go` (render worker count in summary bar), `reporter.go` (add
`WorkersActive` method). Also touches `internal/progress/reporter.go`
(add interface method) and `internal/progress/text.go` (add stub method).

### Key constraints

- Add `WorkersActiveMsg` message type to `internal/tui/messages.go`:
  ```go
  type WorkersActiveMsg struct {
      Active int
      Total  int
  }
  ```
- Add `WorkersActive(active, total int)` method to the `ProgressReporter`
  interface in `internal/progress/reporter.go`
- Implement in `TUIReporter`: send `WorkersActiveMsg` via `r.p.Send()`
- Implement in `TextReporter`: log "N/M workers active" (one line)
- In `internal/tui/model.go`: add `activeWorkers`, `totalWorkers` fields to
  `Model`, handle `WorkersActiveMsg` in `Update()`
- In `internal/tui/view.go`: render `"N/M workers"` in the summary bar when
  `totalWorkers > 1` (hide for serial mode)
- The orchestrator sends `WorkersActive(len(batch), cfg.Concurrency.MaxWorkers)`
  at wave start and `WorkersActive(0, cfg.Concurrency.MaxWorkers)` at wave end
  (wired in the wave dispatcher issue's orchestrator code, but the reporter
  method must exist first)
- Multiple table rows already support concurrent in-progress spinners - verify
  this works and fix if needed
- Update all test stubs implementing `ProgressReporter` to include the new
  method (grep for types implementing the interface)

### Acceptance criteria

- [ ] Multiple issues show in-progress spinners simultaneously
- [ ] Summary bar displays active worker count when `max_workers > 1`
- [ ] Worker count hidden in serial mode (`max_workers: 1`)
- [ ] All ProgressReporter implementations compile with new method

### Test cases

- **Concurrent spinners**: Send two `IssueStartedMsg` without completing the
  first; verify both rows show in-progress state
- **Worker count display**: Send `WorkersActiveMsg{Active: 3, Total: 5}`;
  verify summary bar contains "3/5 workers"
- **Serial mode hidden**: Send `WorkersActiveMsg{Active: 1, Total: 1}`;
  verify no worker count in summary bar
- **Worker count updates**: Send Active: 3 then Active: 1; verify display
  updates

---

## Issue 754: Dashboard wave grouping

**Blocked by**: #750

### Description

Surface concurrent worker information in the dashboard run detail view. Show
which issues ran in each wave, wave timing, and wall-clock time saved versus
serial execution.

This issue touches 3 files: `internal/rundata/writer.go` (add
`WriteWaveResult`), `internal/rundata/reader.go` (add wave data to
`RunDetail`), and `internal/dashboard/handlers.go` + template (render wave
sections).

### Key constraints

- Add `WaveResult` struct to `internal/rundata/writer.go`:
  ```go
  type WaveResult struct {
      Wave         int       `json:"wave"`
      IssueNumbers []int     `json:"issue_numbers"`
      StartedAt    time.Time `json:"started_at"`
      FinishedAt   time.Time `json:"finished_at"`
  }
  ```
- Add `WriteWaveResult(wave WaveResult) error` to `Writer` - writes to
  `waves/<N>.json`
- Add `Waves []WaveResult` field to `RunDetail` in reader
- Load wave data in `LoadRun()` by scanning `waves/` directory
- In the dashboard run detail template, group issues by wave number when
  wave data exists. Show wave duration and issue count per wave.
- Calculate wall-clock savings: sum of individual issue durations minus
  actual run duration. Display as "Concurrency saved Xm Ys" when positive.
- Serial runs (no wave data or single wave) display without wave grouping
- The orchestrator calls `writer.WriteWaveResult()` at wave boundaries
  (wired in the post-wave merge serializer)

### Acceptance criteria

- [ ] `WriteWaveResult` writes wave metadata to `waves/<N>.json`
- [ ] Dashboard groups issues by wave when wave data exists
- [ ] Wall-clock savings displayed for concurrent runs
- [ ] Serial runs display without wave grouping

### Test cases

- **Wave data round-trip**: Write 2 wave results via Writer; read via
  LoadRun; verify both waves present with correct issue numbers
- **Dashboard grouping**: Load a run with 2 waves; verify HTML contains
  wave-1 and wave-2 section headings
- **Serial fallback**: Load a run with no wave data; verify no wave sections
- **Wall-clock savings**: Run with 2 concurrent issues (5m each, 6m wall);
  verify savings shows ~4m

---

## Integration chain audit

```
Config.Concurrency.MaxWorkers defined in config.go (issue 1)
  -> validated by validateConcurrency() in config.go (issue 1)
  -> checked by applyFlags() for compose conflict (issue 2)
  -> read by processIssues() wave dispatcher (issue 6)
  -> controls semaphore capacity (issue 6)
  -> sent to reporter.WorkersActive() (issue 9)

CLIFlags.WithCompose defined in config.go (issue 2)
  -> parsed by parseCLIFlags() in cmdutil.go (issue 2)
  -> applied by applyFlags() in config.go (issue 2)
  -> nils DockerCompose or overrides MaxWorkers (issue 2)

Writer.mu (sync.Mutex) added in writer.go (issue 3)
  -> acquired by WriteIssueDeps, WriteIssueTitles (called before waves)
  -> acquired by SetRateLimit, ClearRateLimit (called between waves, issue 8)
  -> acquired by FinalizeRun (called after all waves)
  -> NOT acquired by per-issue Write* methods (safe: separate directories)

Per-issue logger created in orchestrator.go (issue 4)
  -> passed to runOneIssue() (issue 5)
  -> passed to processIssueFn/ProcessIssue (issue 5)
  -> used by all agent steps within ProcessIssue (automatic)

waveResult struct defined in orchestrator.go (issue 5)
  -> returned by runOneIssue() (issue 5)
  -> sent to results channel by wave dispatcher goroutine (issue 6)
  -> consumed by post-wave merge serializer (issue 7)
  -> checked for UsageLimited by rate-limit handler (issue 8)

Wave dispatcher goroutines in orchestrator.go (issue 6)
  -> bounded by semaphore (capacity = MaxWorkers from issue 1)
  -> uses per-issue logger from (issue 4)
  -> calls runOneIssue from (issue 5)
  -> sends results to channel consumed by (issue 7)
  -> reporter.WorkersActive() at wave start/end (issue 9)
  -> writer.WriteWaveResult() at wave end (issue 10)

Post-wave merge serializer in orchestrator.go (issue 7)
  -> reads results channel from (issue 6)
  -> uses existing runPreMergeRebasePhase, MergeCoordinate (Phase 26)
  -> calls refreshAndCategorize for dependency re-resolution (existing)
  -> feeds remaining issues back to wave loop (existing pattern)

Rate-limit handler in orchestrator.go (issue 8)
  -> checks waveResult.UsageLimited from (issue 7)
  -> calls writer.SetRateLimit / ClearRateLimit (mutex-protected, issue 3)
  -> removes issues from seen map (main goroutine only, no mutex needed)
  -> refreshes authEnv GH_TOKEN (main goroutine only)

ProgressReporter.WorkersActive defined in reporter.go (issue 9)
  -> implemented by TUIReporter in tui/reporter.go (issue 9)
  -> implemented by TextReporter in progress/text.go (issue 9)
  -> called by wave dispatcher in orchestrator.go (issue 6)
  -> displayed in TUI summary bar (issue 9)

WaveResult written by Writer in orchestrator.go (issue 10)
  -> written at wave boundaries by post-wave serializer (issue 7)
  -> read by LoadRun in reader.go (issue 10)
  -> consumed by dashboard handlers (issue 10)
  -> rendered in run detail template (issue 10)

ProgressReporter stub updates needed (issue 9):
  -> fakeReporter in orchestrator_test.go
  -> stubProgressReporter in cmd/implement_test.go
  -> mockReporter in agent/loop_test.go
```

All hops covered. No gaps.

---

# Addendum: Refactor `--with-compose` to Runtime `RunMode`

> Follow-up to #748 / PR #765 (issue #766). The current `applyFlags` silently
> nils `cfg.DockerCompose` when `max_workers > 1`, which violates the
> "explicit over implicit in config" rule. Replace it with an ephemeral
> `RunMode` struct built per-invocation from CLI flags, leaving config as a
> verbatim capability declaration.

## Model

- **Config** declares *capability*: `concurrency.max_workers` is the project's
  parallelism ceiling and default; `docker_compose` lists services that are
  *available*.
- **Flags** declare *per-run intent*:
  - `--integration` activates compose services for this run; forces
    `Workers = 1`.
  - `--workers N` selects worker count for this run (1..max_workers).
- **`RunMode`** is the ephemeral struct flowed from cmd → orchestrator → agent
  layer:
  ```go
  type RunMode struct {
      Workers     int
      Integration bool
  }
  ```
- Default behaviour with no flags: `Workers = cfg.Concurrency.MaxWorkers`,
  `Integration = false`. Compose stays off in all cases unless `--integration`
  is passed.
- `--with-compose` is removed outright (not aliased). Anyone scripting it
  gets a clean "unknown flag" error.

---

## Issue 768: Define `RunMode` type and `BuildRunMode` constructor with validation

### Description

Introduce the `RunMode` struct and a `BuildRunMode(cfg, flags)` constructor in
the `config` package. The constructor centralises all per-invocation
validation. No call sites are touched in this issue — this is pure new code
with its own unit tests.

### Key constraints

- New file `internal/config/runmode.go` in package `config`.
- Type:
  ```go
  type RunMode struct {
      Workers     int
      Integration bool
  }
  ```
- Add two pointer fields to `CLIFlags` in `internal/config/config.go`:
  `Workers *int` and `Integration *bool`. Do NOT remove `WithCompose` yet —
  removal happens in the cleanup issue.
- Constructor signature:
  ```go
  func BuildRunMode(cfg *Config, flags CLIFlags) (RunMode, error)
  ```
- Validation rules (return errors verbatim, no slog warnings):
  - If `flags.Integration != nil && *flags.Integration` and
    `cfg.DockerCompose == nil`: return error
    `"--integration requires a docker_compose block in config"`.
  - If `flags.Integration != nil && *flags.Integration` and
    `flags.Workers != nil && *flags.Workers > 1`: return error
    `"--integration cannot be combined with --workers > 1; integration services are shared and not safe under parallel workers"`.
  - If `flags.Workers != nil && *flags.Workers > cfg.Concurrency.MaxWorkers`:
    return error
    `"--workers N exceeds concurrency.max_workers ceiling M"` (interpolating
    N and M).
  - If `flags.Workers != nil && *flags.Workers < 1`: return error
    `"--workers must be >= 1"`.
- Worker resolution order:
  1. If `flags.Integration` is set and true → `Workers = 1` (forced).
  2. Else if `flags.Workers` is set → `Workers = *flags.Workers`.
  3. Else → `Workers = cfg.Concurrency.MaxWorkers`.
- `Integration = flags.Integration != nil && *flags.Integration`.
- Do NOT mutate `cfg`. The constructor is read-only over its inputs.

### Acceptance criteria

- [ ] `internal/config/runmode.go` defines `RunMode` and `BuildRunMode`.
- [ ] `CLIFlags` has new `Workers *int` and `Integration *bool` fields.
- [ ] All four error rules are exercised by unit tests.
- [ ] All three worker-resolution branches are exercised by unit tests.
- [ ] `BuildRunMode` provably does not mutate its `*Config` argument
      (verified by a test that snapshots the config before and after).

### Test cases

- **Default no flags**: `cfg.Concurrency.MaxWorkers=4`, no flags → returns
  `RunMode{Workers: 4, Integration: false}`.
- **Explicit workers**: `--workers 2` with ceiling 4 → `RunMode{Workers: 2, Integration: false}`.
- **Integration forces serial**: `--integration` with ceiling 4 →
  `RunMode{Workers: 1, Integration: true}`.
- **Integration without compose block**: `cfg.DockerCompose == nil` and
  `--integration` → error mentioning "requires a docker_compose block".
- **Integration plus workers > 1**: `--integration --workers 2` → error
  mentioning "cannot be combined".
- **Workers exceeds ceiling**: `--workers 10` with ceiling 4 → error
  mentioning "exceeds concurrency.max_workers".
- **Config not mutated**: snapshot `cfg`, call `BuildRunMode` with each
  combination above, assert deep-equal snapshot afterwards.

---

## Issue 769: Plumb explicit `integration bool` parameter through agent layer

**Blocked by**: #768

### Description

Behaviour-preserving signature change. Replace every runtime use of
`cfg.DockerCompose != nil` *inside the agent layer* with an explicit
`integration bool` parameter passed in by callers. All call sites continue to
source the parameter from `cfg.DockerCompose != nil` for now — the actual
flag-driven source switch happens in the next issue. After this issue, the
agent layer no longer reads `cfg.DockerCompose != nil` to decide compose
activation; it reads the parameter.

### Key constraints

- Files modified (5):
  - `internal/agent/loop.go` — both `sandboxCommandRunner` call sites
    (currently `:352` and `:453`) take an `integration` parameter from the
    enclosing function. Add `integration bool` to the enclosing function
    signatures and propagate.
  - `internal/agent/verify.go` — `:173` call to `sandboxCommandRunner`
    likewise; thread `integration` through the verify entry function.
  - `internal/agent/implementer.go` — `:277` `MountDockerSocket: cfg.DockerCompose != nil`
    becomes `MountDockerSocket: integration`. `:141`
    `ComposeServices: buildComposeServices(cfg.DockerCompose, cfg.HostServices)`
    becomes guarded so services are only listed when `integration == true`
    (when false, pass an empty compose-services value so the prompt does not
    advertise unavailable services). Add `integration bool` to the
    implementer entry function signature.
  - `internal/orchestrator/orchestrator.go` — every call site that invokes
    the agent layer functions (loop, verify, implementer) now passes
    `cfg.DockerCompose != nil` as the new `integration` argument. Do NOT
    change `:373`, `:733`, `:749` gating yet — those are flipped in the next
    issue.
  - Test files for the changed agent functions (loop_test, verify_test,
    implementer_test, modules_test) — update call signatures only; no
    behaviour changes.
- After this issue, `cfg.DockerCompose != nil` must not appear inside any
  function in `internal/agent/`. (Validated by grep.)
- `internal/cmd/implement.go:155` and `internal/cmd/doctor.go:36` are NOT
  touched. Doctor reports declarative capability and stays as-is. The
  implement command's call into the agent layer is updated in the next
  issue alongside flag parsing.

### Acceptance criteria

- [ ] No occurrences of `cfg.DockerCompose != nil` remain inside
      `internal/agent/`.
- [ ] All four agent entry functions touched accept an explicit
      `integration bool` parameter.
- [ ] All orchestrator call sites pass `cfg.DockerCompose != nil` as the new
      argument.
- [ ] `go build ./...` and `go test ./internal/agent/... ./internal/orchestrator/...`
      pass with no behaviour change.
- [ ] When `integration == false`, `buildBasePromptData` produces an empty
      `ComposeServices` value regardless of `cfg.DockerCompose`.

### Test cases

- **Build still passes**: full `go build ./...` exits 0.
- **Agent tests still pass**: existing `internal/agent/...` tests pass with
  call-site updates.
- **Implementer omits compose services when integration=false**: call
  `buildBasePromptData` with `cfg.DockerCompose` populated but
  `integration=false` → returned `ComposeServices` is empty.
- **Implementer includes compose services when integration=true**: same
  config, `integration=true` → `ComposeServices` matches
  `buildComposeServices(cfg.DockerCompose, cfg.HostServices)`.
- **MountDockerSocket follows parameter**: integration=true → mount; false →
  no mount.
- **Grep gate**: assert via test (or document as a manual check) that
  `cfg.DockerCompose != nil` does not appear in `internal/agent/`.

---

## Issue 770: Add `--integration` and `--workers` flags; thread `RunMode` from cmd into orchestrator

**Blocked by**: #768, #769

### Description

Wire the new flags into both `godark run` and `godark implement`. Build a
`RunMode` in the cmd layer via `BuildRunMode`. Thread it as a new parameter
on the orchestrator entry function and on the `godark implement` single-issue
path. Switch the source of `integration` for all agent call sites from
`cfg.DockerCompose != nil` to `runMode.Integration`. Switch the orchestrator's
own compose-start gating (`:749`) and `DockerConfigFromConfig` calls
(`:373`, `:733`) to consult `runMode.Integration`. Switch the worker
semaphore cap (`:858`) to `runMode.Workers`.

### Key constraints

- Files modified (5):
  - `internal/cmd/run.go` — register `--integration` (bool) and `--workers`
    (int) flags. Remove the `--with-compose` flag registration at `:299` ONLY
    if it does not break parseCLIFlags compilation; otherwise leave it
    registered as a no-op string and remove it in the cleanup issue. (The
    cleanup issue is responsible for full removal.) Build `RunMode` via
    `BuildRunMode(cfg, flags)` after `parseCLIFlags`. Surface validation
    errors with the same error path as other flag-validation failures. Pass
    the resulting `RunMode` into the orchestrator entry function.
  - `internal/cmd/implement.go` — same flag registration, same `RunMode`
    construction. Pass `RunMode` into the single-issue path. The compose
    docker-config decision at `:155` becomes
    `sandbox.DockerConfigFromConfig(cfg.Docker, cfg.Runtime, cfg.SandboxEnv, composeForRun)`
    where `composeForRun = cfg.DockerCompose if runMode.Integration else nil`.
  - `internal/cmd/cmdutil.go` — extend `parseCLIFlags` to populate
    `flags.Integration` and `flags.Workers` when the flags are `Changed`.
    Leave the existing `flags.WithCompose` block in place (cleanup issue
    removes it).
  - `internal/orchestrator/orchestrator.go` — orchestrator entry function
    accepts a new `runMode config.RunMode` parameter. Internal usages:
    - `:373` and `:733` — `DockerConfigFromConfig(... , composeForRun)`
      where `composeForRun = cfg.DockerCompose if runMode.Integration else nil`.
    - `:749` — `if runMode.Integration` (replaces `if cfg.DockerCompose != nil`).
    - `:858` — `maxWorkers := runMode.Workers` (replaces
      `cfg.Concurrency.MaxWorkers`). Keep the `< 1` defensive clamp.
    - All call sites that previously passed `cfg.DockerCompose != nil` to
      agent functions (added in the previous issue) now pass
      `runMode.Integration`.
  - `internal/orchestrator/orchestrator_test.go` — tests construct a
    `config.RunMode` and pass it to the entry function. Existing tests that
    set `cfg.Concurrency.MaxWorkers = N` to drive parallelism now also set
    `runMode.Workers = N`. Existing tests that set `cfg.DockerCompose` to
    drive compose activation now also set `runMode.Integration = true`.
- `internal/cmd/doctor.go:36` is unchanged — doctor reports configured
  capability, not runtime intent.
- `BuildRunMode` errors are surfaced before any side-effecting work
  (logging, locking, container start). Mirror the existing config-validation
  error path so users see them at the same point as a bad `godark.yaml`.
- `--integration` is mutually exclusive with `--workers > 1`; this exclusion
  is enforced by `BuildRunMode`, not by Cobra annotations. The error message
  is asserted by tests.

### Acceptance criteria

- [ ] `--integration` and `--workers` flags are registered on `run` and
      `implement`.
- [ ] `RunMode` is constructed via `BuildRunMode` in both commands and
      passed into the orchestrator/implement entry functions.
- [ ] Orchestrator semaphore cap reads `runMode.Workers`.
- [ ] All compose-activation branches (orchestrator `:373`, `:733`, `:749`;
      implement `:155`; agent layer parameters) source from
      `runMode.Integration`.
- [ ] After this issue, `cfg.DockerCompose != nil` does not appear in
      `internal/orchestrator/orchestrator.go` or `internal/agent/` for the
      purpose of *deciding* compose activation. (Reading `cfg.DockerCompose`
      to obtain the *content* of the compose declaration is fine.)
- [ ] Existing orchestrator tests pass after being updated to pass
      `RunMode`.

### Test cases

- **No flags, parallel config**: `cfg.Concurrency.MaxWorkers=4`, no flags →
  `runMode.Workers=4`, `runMode.Integration=false`; orchestrator dispatches
  with sem cap 4, compose not started even if `cfg.DockerCompose` is set.
- **Explicit `--workers 2`**: ceiling 4 → sem cap 2, compose not started.
- **Explicit `--integration`**: compose configured → sem cap 1, compose
  start path invoked, agents receive `integration=true`.
- **`--integration --workers 3`**: error from `BuildRunMode` surfaces
  before any work begins.
- **`--workers 10` with ceiling 4**: error before any work begins.
- **`--integration` with no compose block**: error before any work begins.
- **Implement single-issue with `--integration`**: `cfg.DockerCompose`
  populated → `DockerConfigFromConfig` receives the compose config; without
  `--integration` → receives nil for the compose argument.
- **Config not mutated**: after a `--workers 4` run, `cfg.DockerCompose`
  still equals its loaded value and `cfg.Concurrency.MaxWorkers` is
  unchanged.

---

## Issue 771: Remove `--with-compose` flag and `applyFlags` config mutation

**Blocked by**: #770

### Description

Destructive cleanup. The previous issues introduce `--integration` /
`--workers` and switch all runtime decisions to `runMode`. This issue rips
out the legacy `--with-compose` flag and the `applyFlags` mutation in
`internal/config/config.go` so config is verifiably read-only after load.
Updates the Phase 14 scenario to reflect the new behaviour.

### Key constraints

- Files modified (3):
  - `internal/config/config.go` — delete the entire block at `:628–:636`
    (`if flags.WithCompose != nil && *flags.WithCompose { ... } else if
    cfg.Concurrency.MaxWorkers > 1 && cfg.DockerCompose != nil { ... }`).
    Delete the `WithCompose` field from `CLIFlags`. Delete its
    `applyFlags` handling. Drop the now-unused `slog` import if it has no
    other consumers in the file.
  - `internal/cmd/run.go` and `internal/cmd/implement.go` — remove the
    `--with-compose` flag registrations at `run.go:299` and
    `implement.go:603`. Remove the `if cmd.Flags().Changed("with-compose")`
    block from `internal/cmd/cmdutil.go:48–:51`.
  - `internal/config/config_test.go` — delete or rewrite tests that
    asserted the mutation behaviour (existing
    `TestApplyFlagsWithComposeMutatesMaxWorkers`-style cases). Add a test
    that asserts `applyFlags` does not mutate `cfg.DockerCompose` or
    `cfg.Concurrency` for any input.
  - `tests/scenarios/phase-14/with-compose-flag.md` — replace assertions
    that depend on `cfg.DockerCompose` being nilled. The new scenario
    asserts: (a) no flags + parallel config → compose stays unstarted but
    `cfg.DockerCompose` is preserved verbatim; (b) `--integration` →
    compose started, `Workers=1`; (c) `--integration --workers 2` →
    validation error before any side effect.
- After this issue, `cfg.DockerCompose = nil` and `cfg.Concurrency.MaxWorkers = `
  do not appear anywhere in `internal/config/config.go` outside of
  `applyDefaults` (where defaulting is legitimate).
- `--with-compose` is gone — `godark run --with-compose` exits with Cobra's
  unknown-flag error.

### Acceptance criteria

- [ ] The `flags.WithCompose` mutation block is deleted from
      `internal/config/config.go`.
- [ ] `WithCompose` is removed from the `CLIFlags` struct.
- [ ] `--with-compose` is no longer registered on `run` or `implement`.
- [ ] `internal/cmd/cmdutil.go` no longer references `with-compose`.
- [ ] A test asserts `applyFlags` does not mutate `cfg.DockerCompose` or
      `cfg.Concurrency` for any flag combination.
- [ ] `tests/scenarios/phase-14/with-compose-flag.md` is updated (or
      renamed to `integration-flag.md`) to assert the new behaviour.
- [ ] `go build ./...` and `go test ./...` pass.

### Test cases

- **Mutation gone**: load a config with `docker_compose` set and
  `concurrency.max_workers: 4`; call `applyFlags` with empty `CLIFlags`;
  assert `cfg.DockerCompose` deep-equals the original and
  `cfg.Concurrency.MaxWorkers == 4`.
- **`--with-compose` rejected**: invoke `godark run --with-compose` →
  exit code non-zero, stderr contains `unknown flag`.
- **Scenario passes**: the rewritten Phase 14 scenario file passes against
  a fresh build.

---

## Integration chain audit (addendum)

```
RunMode defined in internal/config/runmode.go (issue A)
  -> built by BuildRunMode(cfg, flags) in cmd layer (issue B2)
     -> called from internal/cmd/run.go after parseCLIFlags (issue B2)
     -> called from internal/cmd/implement.go after parseCLIFlags (issue B2)
  -> passed into orchestrator entry function (issue B2)
     -> orchestrator semaphore cap reads runMode.Workers (issue B2, replaces orchestrator.go:858)
     -> orchestrator compose start gates on runMode.Integration (issue B2, replaces orchestrator.go:749)
     -> orchestrator passes runMode.Integration to agent functions (issue B2)
        -> agent/loop.go sandboxCommandRunner (parameter added in issue B1, source switched in B2)
        -> agent/verify.go sandboxCommandRunner (parameter added in issue B1, source switched in B2)
        -> agent/implementer.go MountDockerSocket + ComposeServices (parameter added in issue B1, source switched in B2)
  -> passed into single-issue path in internal/cmd/implement.go (issue B2)
     -> DockerConfigFromConfig at implement.go:155 receives composeForRun (issue B2)

applyFlags mutation block at internal/config/config.go:628-636 removed (issue C)
  -> WithCompose field removed from CLIFlags (issue C)
  -> --with-compose flag deregistered from run.go:299 and implement.go:603 (issue C)
  -> cmdutil.go:48-51 with-compose handling removed (issue C)
  -> phase-14 with-compose-flag scenario updated (issue C)
```

All hops covered. Doctor's `cfg.DockerCompose != nil` at `cmd/doctor.go:36`
intentionally preserved — it reports declarative capability, not runtime
intent.
