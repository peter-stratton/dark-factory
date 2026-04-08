# Phase 14: Bounded Concurrency

Until Phase 14, godark processed issues strictly serially: one agent at a time, one merge at a time, even when the issue dependency graph was wide and flat. Phase 14 introduces a wave-barrier concurrency model -- independent issues within a wave run in parallel under a bounded worker pool, then merge serially at the wave boundary before dependencies are re-resolved for the next wave. A mid-phase addendum replaced the first `--with-compose` flag design with an ephemeral `RunMode` struct built per invocation, keeping config as a verbatim capability declaration and pushing per-run intent into flags.

---

## Concurrency Config Block

**What it does:** Declares the project's parallelism ceiling as capability in `godark.yaml`. This is the maximum worker count a run is ever allowed to use; per-invocation intent is set by flags on top of it.

**Example:** A team that has tuned their sandbox image and GitHub App rate budget for four parallel issues writes:

```yaml
concurrency:
  max_workers: 4
```

`internal/config/config.go` defines the struct and applies a default of 1 so existing configs without a `concurrency` block keep the old serial behavior:

```go
type Concurrency struct {
    MaxWorkers int `yaml:"max_workers"`
}
```

Validation rejects anything below 1:

```
concurrency.max_workers must be >= 1, got 0
```

The zero value is caught in `applyDefaults()` and promoted to 1 before validation runs, so an entirely absent block loads cleanly.

---

## RunMode: Per-Invocation Intent

**What it does:** Replaces the short-lived `--with-compose` flag design with an ephemeral `RunMode{Workers, Integration}` struct built per command by `config.BuildRunMode(cfg, flags)`. The config remains a verbatim capability declaration; `RunMode` is the read-only, per-run resolution of flags against that ceiling.

**Example:** A developer wants to run an integration pass against the compose stack for this run only:

```
$ godark run --integration
```

`BuildRunMode` sees `Integration = true`, forces `Workers = 1`, and returns. The orchestrator and agent layer both consume `runMode.Integration` instead of looking at `cfg.DockerCompose != nil`. Compose services are never silently skipped or activated -- intent is explicit.

A different invocation asks for two workers:

```
$ godark run --workers 2
```

With `concurrency.max_workers: 4` in config, `BuildRunMode` returns `RunMode{Workers: 2, Integration: false}`. If the user overshoots the ceiling, they get a clean error rather than a silent clamp:

```
$ godark run --workers 10
Error: --workers N exceeds concurrency.max_workers ceiling M
```

Other validation rules surface similarly:

```
--integration requires a docker_compose block in config
--integration cannot be combined with --workers > 1; integration services are shared and not safe under parallel workers
```

Default with no flags is `Workers = cfg.Concurrency.MaxWorkers`, `Integration = false`. The old `--with-compose` flag was removed outright, not aliased -- scripts using it get an unknown-flag error so the mistake is loud.

---

## Thread-Safe Run Data Writer

**What it does:** Adds a mutex to `rundata.Writer` to protect the handful of methods that read-modify-write the shared `run.json`. Per-issue writes under `issues/<num>/` are untouched -- concurrent workers touch disjoint files -- so the lock scope stays minimal.

**Example:** Two workers in the same wave both try to update run-level state around the same time -- worker A finalizes rate-limit metadata, worker B updates issue-title mapping. Without the mutex, each would read `run.json`, mutate an in-memory copy, and race on the write-back. With Phase 14:

```go
type Writer struct {
    mu sync.Mutex
    // ...
}

func (w *Writer) SetRateLimit(resetsAt time.Time) error {
    w.mu.Lock()
    defer w.mu.Unlock()
    // read run.json, set RateLimitResetsAt, write back
}
```

The locked methods are `WriteIssueDeps`, `WriteIssueTitles`, `SetRateLimit`, `ClearRateLimit`, and `FinalizeRun`. `WriteJudgeIntervention` appends to a per-issue file and remains mutex-free by contract: concurrent workers always operate on distinct issue numbers.

---

## Per-Issue Log Files

**What it does:** Each concurrent worker gets its own `*slog.Logger` writing to `issues/{num}/debug.log`, so interleaved agent output from two parallel workers doesn't produce unreadable log files. The orchestrator's own coordination events (wave start, merge, lock, rate-limit hold) continue flowing to the run-level logger.

**Example:** After a concurrent run completes, the run directory layout looks like:

```
~/.godark/runs/myorg/myservice/2026-04-08T14-03-12Z/
├── run.json
├── run.log               <- wave dispatch, merges, rate-limit events
├── waves/
│   ├── 1.json
│   └── 2.json
└── issues/
    ├── 745/
    │   ├── debug.log     <- issue 745's implementer/verify/reviewer trace
    │   ├── outcome.json
    │   └── dialogue.json
    └── 748/
        └── debug.log     <- isolated from 745
```

The wave-dispatcher creates the per-issue logger before the goroutine starts; if `writer` is nil (dry-run mode) the worker falls back to the run-level logger without panicking.

---

## runOneIssue Worker Function

**What it does:** A pure refactor that extracts the per-issue processing body (agent run, dialogue write, cost calculation, outcome status determination, punchlist prep) into a standalone `runOneIssue` function that takes only values and per-issue data -- no shared mutable state. This is the unit the wave dispatcher fans out.

**Example:** The signature in `internal/orchestrator/orchestrator.go`:

```go
func runOneIssue(ctx context.Context, issue github.Issue, cfg *config.Config,
    runMode config.RunMode, prompts *agent.Prompts, authEnv map[string]string,
    logger *slog.Logger, hook agent.RunDataHook,
    reporter progress.ProgressReporter, writer *rundata.Writer) waveResult
```

It returns a `waveResult` capturing everything the caller needs to apply back to shared state after the wave joins:

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

`runStats`, `seen`, `implementedIssues`, and `justMergedNums` are all mutated by the main goroutine after collecting results -- never inside the worker. That invariant is what makes the serial merge phase safe without further locking.

---

## Wave Barrier Dispatcher

**What it does:** Replaces the serial issue loop with a fan-out / join-wait pattern. For each wave of unblocked issues, the dispatcher fires up to `min(runMode.Workers, len(batch))` goroutines through a counting semaphore, waits for all of them to return their `waveResult` into a buffered channel, then proceeds to the merge phase. Context cancellation is honored both at semaphore acquisition and inside the workers.

**Example:** A wave has 5 independent issues and `runMode.Workers = 2`. The dispatcher does roughly:

```go
results := make(chan waveResult, len(batch))
sem := make(chan struct{}, maxWorkers)
var wg sync.WaitGroup
var activeWorkers atomic.Int32
dispatch:
for _, issue := range batch {
    select {
    case sem <- struct{}{}:
    case <-ctx.Done():
        break dispatch
    }
    reporter.IssueStarted(issue.Number, issue.Title)
    wg.Add(1)
    go func(iss github.Issue) {
        defer wg.Done()
        defer func() { <-sem }()
        reporter.WorkersActive(int(activeWorkers.Add(1)), maxWorkers)
        defer func() {
            reporter.WorkersActive(int(activeWorkers.Add(-1)), maxWorkers)
        }()
        results <- runOneIssue(ctx, iss, cfg, runMode, prompts, authEnv, logger, hook, reporter, writer)
    }(issue)
}
wg.Wait()
close(results)
```

All five issues are marked `seen` before dispatch so concurrent re-entry can't double-process an issue. Results are drained into a slice and sorted by issue number for a deterministic merge order downstream. When `runMode.Workers == 1`, the semaphore of size 1 yields behavior byte-identical to the pre-Phase-14 serial loop.

---

## Post-Wave Serial Merge With Continuation

**What it does:** After join-wait, the main goroutine walks collected results in ascending issue-number order, merges each approved PR one at a time on the base branch, updates stats, and spawns the existing punchlist enrichment goroutines. Failures do not abort the run -- they're counted and reported, and issues that depended on a failed issue simply never become processable in the next wave.

**Example:** A wave of three issues {745, 748, 750} where 748 fails verify and 745/750 pass:

- The dispatcher join-waits until all three goroutines return.
- Collected results are sorted: 745, 748, 750.
- 745 is approved-ready-for-merge; `agent.MergeApprovedPR` runs, branch protection is satisfied because no other merge is competing, base branch is updated.
- 748 is failed: `runStats.failed++`, `reporter.IssueCompleted(..., "failed", ...)`, no merge attempt, no abort reason set.
- 750 is approved; it merges after 745, on the newly-updated base, so the Phase 26 merge-coordinator rebase phase has a clean input.
- At least one merge succeeded, so `refreshAndCategorize` runs to re-resolve dependencies for wave 2. Any issue that transitively depended on 748 stays in the `blocked` list and is counted in the final `runStats.blocked`.

The serial merge is deliberate: concurrent `gh pr merge` calls against the same base would fight each other over the "branch up to date" requirement of branch protection. Fan out the expensive agent work; funnel the cheap merge step.

---

## Rate-Limit Handling at Wave Boundaries

**What it does:** When one or more workers in a wave return `UsageLimited: true`, the orchestrator still merges the successful peers first, then partitions the rate-limited results, sleeps until the latest `ResetsAt + 30s`, refreshes the GitHub App token, and re-dispatches the rate-limited issues in the next wave. If any reset time exceeds the 6-hour `maxHold`, those issues are failed instead of held.

**Example:** Wave of three issues -- #745 succeeds, #748 hits Claude's weekly usage limit resetting in 2h, #750 succeeds. The sequence:

1. All three workers return; results collected and sorted.
2. #745 and #750 merge serially on the main goroutine. `runStats.implemented` is incremented twice.
3. The rate-limited partition finds #748 with `ResetsAt = now + 2h`. 2h is under `maxHold`, so it's retryable.
4. `delete(seen, 748)` so #748 re-enters the processable set next wave.
5. `reporter.RateLimited(resetsAt)` lights up the TUI indicator, `writer.SetRateLimit(resetsAt)` persists the hold.
6. Orchestrator sleeps until `resetsAt + 30s` buffer, honoring `ctx.Done()` so Ctrl+C exits cleanly.
7. Token refreshed, `reporter.RateLimitCleared()`, `writer.ClearRateLimit()`, wave loop continues. Wave 2 picks up #748.

If instead `ResetsAt = now + 7h`, #748 is failed immediately:

```
usage-limited issue failed (reset exceeds max hold)  issue_number=748 resets_at=...
```

---

## TUI Concurrent Status Display

**What it does:** The Bubble Tea TUI now shows multiple in-progress issues simultaneously (spinners on multiple rows) and displays a worker utilization counter in the summary bar when `runMode.Workers > 1`. The existing per-issue message model already keyed on issue number, so concurrent `IssueStageChangedMsg` messages interleaved cleanly once a new `WorkersActiveMsg` type was added.

**Example:** During a wave with 3 of 4 workers busy, the summary bar renders `3/4 workers`. A new interface method was added to `progress.ProgressReporter`:

```go
WorkersActive(active, total int)
```

`TUIReporter` sends a `WorkersActiveMsg{Active: active, Total: total}` into the Bubble Tea program; `TextReporter` logs a single line (`N/M workers active`). The orchestrator wires it into the dispatch goroutine via an `atomic.Int32` so each worker reports its own transition:

```go
reporter.WorkersActive(int(activeWorkers.Add(1)), maxWorkers)
defer func() {
    reporter.WorkersActive(int(activeWorkers.Add(-1)), maxWorkers)
}()
```

In serial mode (`Workers == 1`) the counter is hidden so the summary bar stays uncluttered for the common case.

---

## Dashboard Wave Grouping

**What it does:** Persists per-wave metadata to `waves/<N>.json` and surfaces it in the `godark status` dashboard. The run detail page groups issues by wave when wave data exists, and displays a "Concurrency saved Xm Ys" banner computed from the difference between summed per-issue durations and the actual wall-clock run duration. Serial runs (a single wave) render without the grouping so existing dashboards look unchanged.

**Example:** `rundata.WaveResult` is the persisted shape:

```go
type WaveResult struct {
    Wave         int       `json:"wave"`
    IssueNumbers []int     `json:"issue_numbers"`
    StartedAt    time.Time `json:"started_at"`
    FinishedAt   time.Time `json:"finished_at"`
}
```

At each wave boundary the orchestrator calls `writer.WriteWaveResult(...)`. The dashboard reader loads `waves/` into `RunDetail.Waves`; `handlers.go` renders `WaveView` sections only when `len(detail.Waves) > 1`:

```go
if len(detail.Waves) > 1 {
    for _, w := range detail.Waves {
        // build WaveView, append to data.Waves
    }
}
```

The run-detail template (`internal/dashboard/templates/run-detail.html`) shows a card for each wave, plus a headline banner when savings are positive:

```
Concurrency saved 4m 17s
Wave 1 - 3 issues - 5m 42s
Wave 2 - 2 issues - 6m 01s
```

Runs with a single wave or no wave data fall through to the flat issue list used before Phase 14.
