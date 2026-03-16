# Phase 23: Watch & Daemon Mode

> **Goal:** `godark watch` is validated end-to-end and handles both
> `CHANGES_REQUESTED` and `APPROVED` reviews reliably. A new `--watch` flag on
> `godark run` keeps the run alive after its first pass, polling for human
> merges and automatically processing newly unblocked issues — eliminating the
> need to manually re-run after human approvals.

## Milestone

`Phase 23`

---

## Issue 515: Smoke test godark watch end-to-end

### Description

**This is a human-only task (label: `nodark`).** Manually run `godark watch`
against a real repo with PRs in `godark:awaiting-human-review` state. Verify
that the polling loop starts, detects reviews, invokes the implementer agent
on `CHANGES_REQUESTED`, and merges on `APPROVED`. Document any bugs found as
follow-up issues.

### Key constraints

- Label this issue `nodark` — godark agents must not attempt to implement it
- Run `godark watch` from the dark-factory repo with at least one PR labeled
  `godark:awaiting-human-review`
- Submit a `CHANGES_REQUESTED` review and verify the agent is invoked
- Submit an `APPROVED` review and verify the PR is merged
- Document findings in the issue comments

### Acceptance criteria

- [ ] `godark watch` starts without error and logs poll activity
- [ ] `CHANGES_REQUESTED` review detected and agent invoked
- [ ] `APPROVED` review detected and PR merged
- [ ] Any bugs found filed as separate issues

### Test cases

- **Manual validation**: Human runs the command and documents results

---

## Issue 516: Extract polling logic into internal/watch/ package

### Description

Extract the core watch polling logic from `internal/cmd/watch.go` into a new
`internal/watch/` package in the orchestration layer. This makes the polling
infrastructure reusable by both `godark watch` and `godark run --watch`
(daemon mode). The cmd layer becomes a thin wrapper that creates a `Watch`
instance and calls `Run()`.

Update `docs/architecture.json` to add `internal/watch/` to the orchestration
layer paths.

### Key constraints

- New package `internal/watch/`:
  - `Watch` struct holding config, prompts, authEnv, logger, and processed map
  - `New(cfg, prompts, authEnv, logger) *Watch` constructor
  - `Run(ctx) error` — the polling loop (extracted from `runWatch()`)
  - `PollOnce(ctx) error` — single poll cycle (extracted from `pollOnce()`)
  - `HandleChangesRequested(ctx, pr, review)` — extracted from cmd
  - `HandleApproved(ctx, pr, review)` — extracted from cmd
- Move testability seams (`watchRetryFn`, `watchFindSessionIDFn`, etc.) into
  the new package
- Modify `internal/cmd/watch.go`:
  - Reduce to thin wrapper: create `watch.New(...)`, call `w.Run(ctx)`
  - Remove all extracted logic
- Update `docs/architecture.json`:
  - Add `"internal/watch/"` to orchestration layer paths
- All existing watch tests must continue to pass (move tests to
  `internal/watch/` or update imports)

### Acceptance criteria

- [ ] `internal/watch/` package exists with `Watch` struct and `Run()` method
- [ ] `internal/cmd/watch.go` is a thin wrapper calling `watch.Run()`
- [ ] `docs/architecture.json` includes `"internal/watch/"` in orchestration
- [ ] `godark vet architecture` passes
- [ ] All existing watch tests pass

### Test cases

- **Watch starts**: `watch.New()` + `w.Run(ctx)` with immediate context cancel
  returns nil
- **PollOnce delegates**: Stubbed GitHub calls, `PollOnce` processes PRs
  correctly
- **Existing tests pass**: All tests from `watch_test.go` pass after migration

---

## Issue 517: Add --no-tui flag to godark watch

**Blocked by**: #516

### Description

Add a `--no-tui` flag to `godark watch` for consistency with `godark run` and
`godark implement`. When TUI is not active, watch output goes through the
existing logger. This prepares the command for TUI integration in a later
issue.

### Key constraints

- Modify `internal/cmd/watch.go`:
  - Add `--no-tui` flag (bool, default false)
  - Terminal detection: `!noTUI && term.IsTerminal(int(os.Stdout.Fd()))`
  - When TUI: use `logging.NewLoggerFileOnly()` (logger writes to file only)
  - When no TUI: use `logging.NewLogger()` (current behavior)
  - TUI mode is a no-op for now (just selects the logger); actual TUI
    rendering comes in the next issue

### Acceptance criteria

- [ ] `--no-tui` flag exists on `godark watch`
- [ ] Interactive terminal uses file-only logger when TUI mode is active
- [ ] `--no-tui` forces standard logger regardless of terminal
- [ ] Non-terminal stdout uses standard logger

### Test cases

- **Flag registered**: `godark watch --help` shows `--no-tui` flag
- **Logger selection**: TUI mode creates file-only logger
- **No-tui forces text**: `--no-tui` with interactive terminal uses standard
  logger

---

## Issue 520: Watch TUI view

**Blocked by**: #517

### Description

Create a Bubble Tea model for the watch command that shows polling status,
a list of PRs being watched, and a recent activity log. The TUI replaces the
structured log output when running interactively.

### Key constraints

- New file `internal/tui/watch_model.go`:
  - `WatchModel` struct with: repo, poll interval, last poll time, PR list
    (number, title, label state), activity log (recent events), done/cancelling
    state
  - `NewWatchModel(repo string, interval time.Duration, cancelFn func())`
  - `Init()`, `Update()`, `View()` implementing `tea.Model`
  - Message types: `PollTickMsg`, `PRUpdateMsg`, `ActivityMsg`, `WatchDoneMsg`
- New file `internal/tui/watch_view.go`:
  - Header: "godark watch" logo + repo name
  - PR table: number, title, current label state
  - Activity log: last N events (e.g., "CHANGES_REQUESTED detected on #42",
    "PR #42 merged")
  - Footer: "watching · last poll: 30s ago · press ctrl+c to stop"
- Modify `internal/cmd/watch.go`:
  - When TUI mode: create `WatchModel`, run Bubble Tea program, feed events
    from the `watch.Watch` polling loop
- Activity log limited to last 10 entries (ring buffer or slice trim)

### Acceptance criteria

- [ ] Watch TUI renders header with repo name
- [ ] PR table shows PRs with current label state
- [ ] Activity log shows recent events
- [ ] ctrl+c exits cleanly
- [ ] Non-TUI mode unchanged (logger output)

### Test cases

- **PR table updates**: `PRUpdateMsg` adds/updates a row in the PR table
- **Activity log**: `ActivityMsg` appends to the log, oldest entries trimmed
  at 10
- **Done state**: `WatchDoneMsg` sets done flag, user can exit with q
- **Empty state**: No PRs found — shows "no PRs awaiting review"

---

## Issue 518: Watch dashboard view

**Blocked by**: #516

### Description

Surface watch-managed PRs and their review cycles in the `godark status`
dashboard. Add a section to the run detail page or a new dedicated page
showing PRs in `awaiting-human-review` and `fixing-review-feedback` states
with their review history.

### Key constraints

- Modify `internal/dashboard/handlers.go`:
  - New handler `handleWatchStatus()` or extend existing run detail page
  - Query GitHub for PRs with godark labels (`awaiting-human-review`,
    `fixing-review-feedback`)
  - Display: PR number, title, current label, last review state, time in
    current state
- Modify or create `internal/dashboard/templates/watch.html`:
  - Table of watched PRs with status badges matching TUI label colors
  - Link to each PR on GitHub
- Add navigation link to the dashboard sidebar

### Acceptance criteria

- [ ] Dashboard shows PRs in `awaiting-human-review` state
- [ ] Dashboard shows PRs in `fixing-review-feedback` state
- [ ] Each PR links to GitHub
- [ ] Navigation sidebar includes watch page link

### Test cases

- **PRs displayed**: Two PRs with different labels — both shown with correct
  badges
- **No PRs**: No labeled PRs — shows empty state message
- **Link works**: PR link points to correct GitHub URL

---

## Issue 519: Add --watch flag to godark run

**Blocked by**: #516

### Description

Add a `--watch` flag to `godark run` that keeps the run alive after the first
pass completes. Instead of exiting, the command enters a polling loop that
watches for external merges on PRs left in `awaiting-human-review`. This is
the entry point for daemon mode — the actual re-resolution logic is a separate
issue.

### Key constraints

- Modify `internal/cmd/run.go`:
  - Add `--watch` flag (bool, default false)
  - After `orchestrator.Run()` returns, if `--watch` is set and there are
    unmerged PRs (status `ready-to-merge` or `needs-human-review`), enter the
    watch polling loop using `watch.Watch` from `internal/watch/`
  - Pass the same config, prompts, authEnv, and logger
  - The watch loop handles `CHANGES_REQUESTED` and `APPROVED` as usual
  - When no more PRs are labeled `awaiting-human-review`, exit the watch loop
- The `--watch` flag is independent of `--no-tui` — both can be combined

### Acceptance criteria

- [ ] `--watch` flag exists on `godark run`
- [ ] After first pass, run enters polling loop if unmerged PRs exist
- [ ] Watch loop handles CHANGES_REQUESTED and APPROVED
- [ ] Run exits when no more PRs are awaiting review
- [ ] ctrl+c cancels both the watch loop and exits cleanly

### Test cases

- **Watch entered**: Run completes with 1 needs-human-review issue — enters
  watch mode
- **Watch skipped**: Run completes with all issues implemented — exits normally
  (no watch)
- **Watch exits on empty**: Last awaiting PR is merged — watch loop exits
- **Cancel during watch**: ctrl+c during watch polling cancels and exits
- **Flag not set**: Run without `--watch` exits as before (no behavior change)

---

## Issue 521: Daemon mode: detect external merges during watch polling

**Blocked by**: #519

### Description

During the `--watch` polling loop after a `godark run`, detect when a PR that
was left in `awaiting-human-review` has been merged externally (by a human or
by the watch command's APPROVED handler). Track which issues have been
unblocked by these merges so the re-resolution step can process them.

### Key constraints

- Modify `internal/watch/` package:
  - Add `DetectMergedPRs(ctx, repo, issueNumbers) ([]int, error)` — checks
    which of the given issues now have merged PRs (via `gh pr list --state
    merged` or checking issue closed state)
  - Returns the issue numbers whose PRs have been merged since the last check
  - Uses the same poll interval as the review detection loop
- The detection runs alongside the existing review polling (same tick cycle)
- Merged issue numbers are accumulated and passed to the re-resolution step

### Acceptance criteria

- [ ] `DetectMergedPRs` returns issue numbers with newly merged PRs
- [ ] Detection runs on each poll cycle alongside review detection
- [ ] Already-detected merges are not re-reported (idempotent)
- [ ] Detection works for PRs merged by humans and by the APPROVED handler

### Test cases

- **Detect merged PR**: Issue #42's PR was open, now merged — #42 returned
- **Already detected**: #42 was detected last cycle — not returned again
- **No merges**: All PRs still open — empty slice returned
- **Multiple merges**: #42 and #43 both merged — both returned

---

## Issue 522: Daemon mode: re-resolve dependencies and process unblocked issues

**Blocked by**: #521

### Description

When the watch polling loop detects that PRs have been merged (issues closed),
re-resolve the dependency graph and process any newly unblocked issues. This
reuses the existing wave re-resolution logic from `processIssues()` but
triggered by external merges rather than internal ones.

### Key constraints

- Modify `internal/orchestrator/orchestrator.go`:
  - Extract the dependency re-resolution logic from `processIssues()` into a
    reusable function: `reResolveAndProcess(ctx, allIssues, closedSet, cfg,
    ...)`
  - The function: re-fetches closed issues, re-categorizes, identifies newly
    unblocked issues, processes them through the agent loop
  - This is called by `processIssues()` in the existing wave loop (refactor,
    no behavior change) AND by the daemon mode watch loop when merges are
    detected
- Modify `internal/cmd/run.go`:
  - After the watch loop detects merges, call `reResolveAndProcess()` with
    the updated closed set
  - Loop: poll → detect merges → re-resolve → process → poll again
  - Exit when no more issues are processable and no PRs are awaiting review
- Stats DB: write stats for daemon-mode-processed issues (same as normal runs)
- Reporter: send TUI messages for newly processed issues

### Acceptance criteria

- [ ] Re-resolution detects newly unblocked issues after external merges
- [ ] Unblocked issues are processed through the full agent loop
- [ ] Stats written for daemon-mode-processed issues
- [ ] The existing wave loop in `processIssues()` still works unchanged
- [ ] Run exits when all issues are complete and no PRs are awaiting review

### Test cases

- **Re-resolve after merge**: Issue B blocked by A. A's PR merged externally.
  Re-resolution finds B unblocked, processes it.
- **Multiple unblocked**: A and C merged. B (blocked by A) and D (blocked by C)
  both become processable.
- **No new unblocked**: Merge doesn't unblock anything — no processing, poll
  continues
- **Existing wave loop unchanged**: `processIssues()` without `--watch` behaves
  identically to before

---

## Issue 523: TUI watch-mode transition

**Blocked by**: #520, #519

### Description

When `godark run --watch` transitions from the initial run to watch mode, the
TUI should reflect the state change. The issue table remains visible (showing
final outcomes), but the hint text and status change from "press q to exit" to
"watching for merges" with the polling status.

### Key constraints

- Modify `internal/tui/model.go`:
  - Add `watching bool` state (distinct from `done`)
  - New message type: `WatchingMsg{}` — sent when the run transitions to watch
    mode
  - When `watching == true`:
    - Hint shows "watching for merges · press ctrl+c to cancel"
    - Spinner continues (indicating active polling)
    - New issue rows can still be added (if re-resolution processes new issues)
  - When a merge is detected during watch mode, update the relevant issue row
  - `RunDoneMsg` still transitions to `done` when watch mode exits
- Modify `internal/cmd/run.go`:
  - Send `WatchingMsg{}` to the TUI program when entering watch mode
  - Continue sending `IssueStartedMsg`/`IssueCompletedMsg` for daemon-mode
    processed issues

### Acceptance criteria

- [ ] TUI shows "watching for merges" hint after run completes with `--watch`
- [ ] Spinner continues during watch mode
- [ ] New issues from re-resolution appear in the table
- [ ] ctrl+c during watch mode cancels and shows "press q to exit"
- [ ] TUI exits cleanly after watch mode completes

### Test cases

- **Transition to watching**: `WatchingMsg` sets `watching = true`, hint updates
- **New issue during watch**: `IssueStartedMsg` during watch mode adds a row
- **Cancel during watch**: ctrl+c sets `cancelling = true`, hint updates
- **Watch complete**: `RunDoneMsg` during watch mode sets `done = true`
