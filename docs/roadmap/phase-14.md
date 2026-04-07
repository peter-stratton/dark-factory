## Phase 14: Bounded Concurrency

**Goal**: Independent issues within a run execute in parallel, bounded by a
configurable worker pool. Dependent issues still respect topological ordering.
Wave-barrier dispatch groups independent issues, processes them concurrently,
then merges all successes serially before re-resolving dependencies for the
next wave.

**Milestone**: `Phase 14` | **Label**: `phase-14`

- Concurrency config block (`concurrency.max_workers`, default 1)
- `--with-compose` flag forcing single-worker mode when compose is configured
- Thread-safe run data writer (mutex on run.json read-modify-write methods)
- Per-issue log files (`issues/{num}/debug.log`) for concurrent debuggability
- Extract per-issue processing into `runOneIssue` worker function (pure refactor)
- Wave barrier dispatcher with bounded goroutines and channel-based result collection
- Post-wave serial merge with continuation on failure (no abort) and dependency re-resolution
- Rate-limit handling at wave boundaries (sleep until reset, re-dispatch rate-limited issues)
- TUI concurrent status display (multiple spinners, worker count in summary bar)
- Dashboard wave grouping (wave metadata in run data, wall-clock savings)

**Issues**: #745–#754

**Planning doc**: `docs/planning/phase-14-bounded-concurrency.md`
