# Phase 23: Watch & Daemon Mode

Before Phase 23, a `godark run` that left PRs in `awaiting-human-review` simply exited. A human had to review the PR, then manually re-run godark to process any issues that were unblocked by the merge. Phase 23 validates the `godark watch` command end-to-end, extracts its polling logic into a shared package, adds a `--watch` flag to `godark run` that keeps the process alive after the first pass, and builds TUI and dashboard views for monitoring watched PRs. The result is a daemon mode that closes the loop: run issues, wait for human reviews, and automatically process anything that gets unblocked -- all in one invocation.

---

## Shared Watch Package

**What it does:** The core polling logic lives in `internal/watch/`, an orchestration-layer package reused by both `godark watch` and `godark run --watch`. The `Watch` struct owns the poll loop, review handling, and external merge detection. The command layer is a thin wrapper that creates a `Watch` instance and calls `Run()` or `RunUntilDone()`.

**Example:** When `godark watch` starts, the command creates the shared watcher and hands off control:

```go
w := watch.New(cfg, prompts, authEnv, logger)
err := w.Run(ctx) // polls indefinitely until ctx is cancelled
```

The `Watch` struct tracks three maps to maintain state across poll cycles:

```go
type Watch struct {
    cfg              *config.Config
    prompts          *agent.Prompts
    authEnv          map[string]string
    logger           *slog.Logger
    processed        map[int]bool  // review IDs already handled
    mergedIssues     map[int]bool  // issue numbers with merged PRs
    seenIssues       map[int]bool  // issues ever seen in awaiting label
    onMergesDetected func(ctx context.Context, mergedNums []int)
}
```

`Run()` polls indefinitely (for `godark watch`). `RunUntilDone()` polls until no PRs carry the `godark:awaiting-human-review` label, then exits -- this is the variant used by daemon mode.

---

## Review Handling

**What it does:** Each poll cycle fetches PRs labeled `godark:awaiting-human-review`, inspects their reviews, and acts on `CHANGES_REQUESTED` or `APPROVED` verdicts. The `processed` map ensures each review is handled exactly once.

**Example:** A human submits a `CHANGES_REQUESTED` review on PR #42. On the next poll cycle, `PollOnce` detects it:

1. Fetches PRs with `godark:awaiting-human-review` via `ListPRsWithLabel`
2. For each PR, fetches reviews via `FetchPRReviews`
3. Finds the unprocessed `CHANGES_REQUESTED` review
4. Calls `HandleChangesRequested(ctx, pr, review)`:
   - Swaps the label to `godark:fixing-review-feedback`
   - Invokes the implementer agent to address the feedback, resuming the prior session
   - On success, swaps the label back to `godark:awaiting-human-review`

When the human later submits an `APPROVED` review, `HandleApproved` merges the PR, closes the linked issue, and removes the label. The `processed` map records the review ID so neither review is acted on twice.

---

## Daemon Mode (--watch Flag)

**What it does:** The `--watch` flag on `godark run` keeps the process alive after the first pass completes. Instead of exiting, it enters a polling loop that watches for external merges on PRs left in `awaiting-human-review`. When a merge is detected, it re-resolves the dependency graph and processes any newly unblocked issues.

**Example:** A milestone has three issues: A (no deps), B (blocked by A), and C (blocked by B). The initial run implements A and opens a PR that lands in `awaiting-human-review`:

```
$ godark run --milestone "Phase 4" --repo myorg/myapp --watch
```

The run processes A, skips B and C (blocked), then transitions to watch mode. A human reviews and merges A's PR. On the next poll cycle:

1. `pollExternalMerges` detects A's PR was merged
2. The merge callback fires with `mergedNums: [42]` (A's issue number)
3. `orchestrator.ReResolveAndProcess` re-fetches milestone issues, finds B is now unblocked, and processes it through the full agent loop
4. B's PR lands in `awaiting-human-review`, and the watch loop continues
5. When B is merged, C becomes processable -- same cycle repeats
6. After C is merged and no PRs remain in `awaiting-human-review`, the watch loop exits

The re-resolution function is extracted from the orchestrator's existing wave logic so the behavior is identical whether issues are unblocked during a normal run or during daemon mode:

```go
func ReResolveAndProcess(
    ctx context.Context,
    allIssues []github.Issue,
    noDarkNums []int,
    seen map[int]bool,
    cfg *config.Config,
    milestone string,
    logger *slog.Logger,
    reporter progress.ProgressReporter,
    notifiers []notify.Notifier,
) (bool, error)
```

---

## External Merge Detection

**What it does:** Each poll cycle checks whether PRs that were in `awaiting-human-review` have been merged externally -- by a human or by the watch loop's own `APPROVED` handler. Merged issue numbers are accumulated and passed to the re-resolution callback. The `mergedIssues` map prevents the same merge from triggering re-resolution twice.

**Example:** During daemon mode, the watch loop calls `DetectMergedPRs` on each tick:

```go
func (w *Watch) DetectMergedPRs(ctx context.Context, repo string, issueNumbers []int) ([]int, error)
```

If issues #42 and #43 both had open PRs last cycle and both are now merged, `DetectMergedPRs` returns `[42, 43]`. These are recorded in `mergedIssues` so the next cycle won't report them again. The callback registered via `SetMergeCallback` receives the newly merged numbers and triggers `ReResolveAndProcess`.

---

## Seen Map Seeding

**What it does:** When `godark run --watch` transitions from the initial run to watch mode, it pre-populates the `seenIssues` map with issue numbers from PRs that already carry godark labels. This prevents the watch loop from re-processing issues that were already handled during the initial run.

**Example:** The initial run created PRs for issues #10, #11, and #12. Before entering watch mode, `seedSeenFromProcessedPRs` scans for PRs carrying any of:

- `godark:awaiting-human-review`
- `godark:ready-to-merge`
- `godark:fixing-review-feedback`

The resulting map `{10: true, 11: true, 12: true}` is passed to the `Watch` instance, ensuring the daemon loop only processes issues that become unblocked after the initial pass.

---

## Watch TUI

**What it does:** A Bubble Tea model renders a live terminal view for `godark watch` showing the repo being watched, a table of PRs with their label states, a scrolling activity log, and polling status. The TUI replaces structured log output when running interactively.

**Example:** Running `godark watch --repo myorg/myapp` in a terminal shows:

```
godark watch · myorg/myapp
watching for PRs awaiting human review

  #   TITLE                              STATUS
  42  Add user authentication            Awaiting Review
  43  Fix pagination bug                 Fixing Feedback

  Activity
  ──────────────────────────────────────
  CHANGES_REQUESTED detected on #43
  Agent invoked for #43
  PR #41 merged

  watching · last poll: 12s ago · press ctrl+c to stop
```

The `WatchModel` handles several message types:

- `PRUpdateMsg` — refreshes the PR table and updates the "last poll" timestamp
- `ActivityMsg` — appends to the activity log (ring buffer, last 10 entries)
- `WatchDoneMsg` — sets the done flag, hint text changes to "press q to exit"

Label badges are color-coded: blue for awaiting review, yellow for fixing feedback, green for approved/merged, red for failed. A separate goroutine (`watchTUIPoller`) fetches the PR list on each interval and sends `PRUpdateMsg` to the model.

The `--no-tui` flag disables the Bubble Tea interface and falls back to standard logger output, consistent with `godark run --no-tui` and `godark implement --no-tui`.

---

## Run TUI Watch Transition

**What it does:** When `godark run --watch` finishes its initial pass and enters watch mode, the existing run TUI transitions seamlessly. The issue table stays visible (showing final outcomes from the first pass), but the hint text and spinner update to indicate active polling.

**Example:** The run command sends a `WatchingMsg` to the Bubble Tea program when entering watch mode:

```go
type WatchingMsg struct{}
```

When the model receives this message, it sets `m.watching = true`. The hint text changes from "press q to exit" to "watching for merges -- press ctrl+c to cancel", and the spinner continues to indicate the process is active. If re-resolution processes new issues during watch mode, `IssueStartedMsg` and `IssueCompletedMsg` messages add new rows to the existing table. When the watch loop exits, `RunDoneMsg` sets `m.done = true` and the user can exit with q.

---

## Watch Dashboard Page

**What it does:** The web dashboard at `/watch` surfaces PRs being monitored by `godark watch`, showing their current label state, time in state, and links to GitHub. This gives visibility into the review cycle without needing a terminal session.

**Example:** Navigating to the watch page in `godark status` shows:

| # | Title | Status | Time in State |
|---|-------|--------|---------------|
| 42 | Add user authentication | Awaiting Review | 2h 15m |
| 43 | Fix pagination bug | Fixing Feedback | 45m |

The handler queries GitHub for PRs carrying either `godark:awaiting-human-review` or `godark:fixing-review-feedback`, deduplicates by PR number, and converts each to a `WatchPRView`:

```go
type WatchPRView struct {
    Number       int
    Title        string
    CurrentLabel string
    BadgeClass   string   // "badge--info", "badge--warning", "badge--secondary"
    BadgeLabel   string   // "Awaiting Review", "Fixing Feedback", "Unknown"
    TimeInState  string
    GitHubURL    string
}
```

Badge classes map to the same color scheme as the TUI: blue for awaiting, yellow for fixing feedback, gray for unknown states. A repo selector dropdown filters the view when multiple repos are configured. If no `WatchQuerier` is provided (watch not active), the page shows a disabled state.

---

## Configurable Poll Interval

**What it does:** The poll interval for both `godark watch` and daemon mode is configurable via `godark.yaml`. The default is 60 seconds.

**Example:** To poll every 30 seconds:

```yaml
watch:
  poll_interval: "30s"
```

The config validation ensures the value is a valid positive Go duration string. If the `watch` block is omitted or `poll_interval` is empty, the default 60-second interval applies. Both `godark watch` and `godark run --watch` read from the same config field.
