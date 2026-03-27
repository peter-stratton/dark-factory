## Phase 14: Bounded Concurrency

**Goal**: Independent issues within a run execute in parallel, bounded by a
configurable worker pool. Dependent issues still respect topological ordering.
Merge serialization ensures `main` stays linear.

**Milestone**: `Phase 14` | **Label**: `phase-14`

- Worker pool with configurable max concurrency (`concurrency.max_workers`, default 1)
- Wave barrier scheduling: process independent issues in parallel, wait for wave, merge all, re-resolve dependencies, next wave
- Dependency-aware scheduling from existing topological sort
- Per-worker sandbox containers with isolated git worktrees
- Concurrent/integration run modes: compose skipped when `max_workers > 1`; `--with-compose` forces single-worker integration mode
- Single-goroutine merge serializer (squash-merge, rebase, signal next)
- Merge coordinator agent (from Phase 26) used for post-wave conflict resolution
- Thread-safe run data writer (mutex or per-issue writers)
- Per-issue log files for concurrent debuggability
- Active workers indicator and concurrent status badges in dashboard

**Issues**: #593–#602

**Planning doc**: `docs/planning/phase-14-bounded-concurrency.md`

