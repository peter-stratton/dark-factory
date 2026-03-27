## Phase 23: Watch & Daemon Mode ✅

**Goal**: `godark watch` is validated end-to-end and handles both
`CHANGES_REQUESTED` and `APPROVED` reviews reliably. A new `--watch` flag on
`godark run` keeps the run alive after its first pass, polling for human merges
and automatically processing newly unblocked issues — eliminating the need to
manually re-run after human approvals.

**Milestone**: `Phase 23` | **Label**: `phase-23`

### Watch command validation and fixes
- Smoke test `godark watch` against a real repo — verify polling, label
  swapping, and agent invocation work e2e
- Fix any bugs discovered during validation (likely: stale lock handling,
  branch checkout issues, error recovery)
- Validate APPROVED → merge flow (#456 already implemented but never tested
  live)
- Add `--no-tui` flag to `godark watch` for consistency with run/implement
- Watch TUI view — Bubble Tea model showing polling status, PRs being watched,
  recent activity log
- Watch dashboard view — surface watch-managed PRs and their review cycles in
  `godark status`

### Daemon mode (`godark run --watch`)
- `--watch` flag on `godark run` — after the first pass completes, enter a
  polling loop instead of exiting
- Poll for merged PRs that were left in `awaiting-human-review` during the run
- When a merge is detected, re-resolve dependencies and process newly unblocked
  issues
- Reuse the existing wave re-resolution logic from `processIssues()`
- TUI transitions from "run complete" to "watching for merges" state with
  appropriate hint text
- Graceful exit: `ctrl+c` during watch mode cancels and exits cleanly
- Stats DB: write stats for daemon-mode-initiated issue processing (same as
  normal runs)

### Shared infrastructure
- Extract polling logic into a shared package (`internal/watch/` or similar)
  used by both `godark watch` and `godark run --watch`
- Configurable poll interval from `watch.poll_interval` in `godark.yaml`
  (existing config, reused)

**Issues**: #515–#523

**Planning doc**: `docs/planning/phase-23-watch-and-daemon-mode.md`

