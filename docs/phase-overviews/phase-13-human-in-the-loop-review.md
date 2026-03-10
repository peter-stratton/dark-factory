# Phase 13: Human-in-the-Loop Review

Most teams will not hand an AI full merge authority on day one. Phase 13 adds the machinery for humans to stay in the loop: PR lifecycle labels that communicate state at a glance, a `godark watch` command that detects human review feedback and feeds it back to the agent, a risk classifier that decides which PRs are safe to auto-merge, and graduated autonomy settings that let teams dial up trust as they gain confidence. This is the critical adoption path -- teams start with full oversight and relax it over time.

---

## Graduated Auto-Merge

**What it does:** Replaces the old `no_merge` boolean with a three-level `auto_merge` setting that controls what happens after the AI reviewer approves a PR. Teams choose how much autonomy to grant.

**Example:** A team starting with godark sets `auto_merge: none` in their config (this is the default). After two months of reviewing every PR, they trust the pipeline for small changes and switch to `low_risk`:

```yaml
auto_merge: low_risk
risk_thresholds:
  max_lines: 200
  max_files: 10
```

The CLI flag works the same way -- `--auto-merge low_risk` overrides whatever is in the config file. The three modes:

- `none` -- AI reviewer approves, PR is labeled `godark:awaiting-human-review`, and godark stops. A human merges when ready.
- `low_risk` -- AI reviewer approves, the risk classifier evaluates the PR against five gates. If all pass, godark merges. If any gate fails, the PR is labeled for human review.
- `all` -- AI reviewer approves, godark merges immediately. The human spot-checks after the fact.

Invalid values are caught at config validation:

```
$ godark run --auto-merge always
Error: auto_merge must be one of: none, low_risk, all
```

---

## PR Lifecycle Labels

**What it does:** Three GitHub labels track where each PR is in the human review cycle. Labels are the source of truth -- any human or external tool can see the current state by looking at the PR.

**Example:** An agent finishes implementing issue #42 and the AI reviewer approves it. With `auto_merge: none`, the orchestrator applies the `godark:awaiting-human-review` label (blue, #4A90E2). The PR now shows up in GitHub's label filter and in the godark dashboard.

A developer reviews the PR and requests changes. `godark watch` detects the review, removes `godark:awaiting-human-review`, and applies `godark:fixing-review-feedback` (yellow, #F5D76E). The agent fixes the issues and pushes. The label flips back to `godark:awaiting-human-review`.

When the human finally approves and the PR merges, all lifecycle labels are removed.

The three labels and their transitions form a state machine defined in `internal/label/label.go`:

```
(initial) ──→ godark:awaiting-human-review ──→ godark:fixing-review-feedback
                        ↑                              │
                        └──────────────────────────────┘
                        │
                        ↓
               godark:ready-to-merge
```

Invalid transitions are rejected by `label.Transition()`. You cannot go from `ready-to-merge` back to `fixing-review-feedback` -- that transition does not exist. The `InProgress` label (`godark-in-progress`), previously defined in the lock package, was also consolidated here so all godark-managed labels live in one place.

Labels are created during `godark init` via `github.EnsureLabel()` for each entry in `label.Specs`.

---

## The Watch Command

**What it does:** `godark watch` is a long-running foreground process that polls GitHub for PRs labeled `godark:awaiting-human-review`, detects when a human submits a `CHANGES_REQUESTED` review, and automatically feeds that feedback to the implementer agent.

**Example:** After a run finishes with several PRs awaiting review, a developer starts the watcher:

```
$ godark watch --repo myorg/myservice
Watching myorg/myservice for human review feedback (poll interval: 60s)...
```

The watcher polls on a configurable interval:

```yaml
watch:
  poll_interval: 30s
```

When it detects a `CHANGES_REQUESTED` review on PR #87, the sequence is:

1. Fetch the review body and inline comments via `github.FetchPRReviews()` and `github.FetchReviewComments()`
2. Swap labels: remove `awaiting-human-review`, apply `fixing-review-feedback`
3. Extract the issue number from the branch name (format: `{issueNum}-{slug}`)
4. Look up the prior session ID via `rundata.FindSessionID()` -- this scans `~/.godark/runs/` for the most recent session associated with that PR
5. Resume the implementer agent with `agent.Retry()`, passing the human's comments as feedback context
6. After the agent pushes its fix, swap labels back: remove `fixing-review-feedback`, apply `awaiting-human-review`

The watcher tracks processed review IDs to avoid re-triggering on reviews it has already handled. If no prior session ID is found, the agent cold-starts without session resumption. Ctrl+C shuts down cleanly via context cancellation.

Run data for watch-initiated fix cycles is written to its own run directory under `~/.godark/runs/`, using the same `rundata.New()` writer as `godark run` and `godark implement`.

---

## Risk Classification

**What it does:** When `auto_merge: low_risk` is set, a deterministic classifier evaluates each approved PR against five gates. All five must pass for the PR to be auto-merged. Any single failure routes the PR to human review.

**Example:** The AI reviewer approves PR #63, a small bug fix that changed 45 lines across 2 files. The orchestrator gathers stats via `github.FetchPRStats()` and `github.FetchPRChangedFiles()`, then calls `quality.ClassifyRisk()`:

```go
input := quality.RiskInput{
    LinesChanged:   45,    // additions + deletions
    FilesChanged:   2,
    ChangedFiles:   []string{"internal/agent/loop.go", "internal/agent/loop_test.go"},
    ProtectedPaths: []string{".github/", "godark.yaml"},
    FixCycles:      0,     // passed verify on first attempt
    QualityFlags:   nil,   // no quality flags raised
}
assessment := quality.ClassifyRisk(input, 200, 10)
// assessment.IsLowRisk == true, all 5 gates passed
```

The five gates:

| Gate | Condition | Default |
|------|-----------|---------|
| `max_lines` | Lines changed <= threshold | 200 |
| `max_files` | Files changed <= threshold | 10 |
| `protected_paths` | No changed file matches a protected path | -- |
| `no_fix_cycles` | Zero verify-fix attempts used | -- |
| `no_quality_flags` | No quality flags raised on the review | -- |

Now consider PR #71, a larger refactor touching 380 lines and a protected CI config. The classifier returns:

```json
{
  "is_low_risk": false,
  "gates": [
    {"name": "max_lines", "passed": false, "detail": "380 lines changed, max 200"},
    {"name": "max_files", "passed": true, "detail": "4 files changed, max 10"},
    {"name": "protected_paths", "passed": false, "detail": "touches protected path: .github/"},
    {"name": "no_fix_cycles", "passed": true, "detail": "0 fix cycles"},
    {"name": "no_quality_flags", "passed": true, "detail": "0 quality flags"}
  ]
}
```

Two gates failed, so the PR is labeled `godark:awaiting-human-review` instead of being merged. The risk assessment is written to run data via `hook.WriteRiskAssessment()` so humans can audit the classification decision.

---

## Notification System

**What it does:** Sends notifications at key events during a run. Delivery is best-effort -- failures are logged as warnings and never block execution. The system uses a pluggable provider model, with Telegram supported at launch.

**Example:** A team wants Telegram alerts when runs finish or abort:

```yaml
notify:
  - provider: telegram
    events: [run_complete, implementation_complete, abort]
    settings:
      bot_token: ${TELEGRAM_BOT_TOKEN}
      chat_id: "123456789"
```

Settings use `${VAR}` syntax for environment variable expansion, keeping secrets out of the config file. Three event types fire at different points:

- `implementation_complete` -- fired after each issue is processed (from the implement command)
- `run_complete` -- fired after the orchestrator finishes all issues in a milestone
- `abort` -- fired when a run is stopped early

Each notification includes the repo name and a message describing what happened. The Telegram provider formats messages in Markdown:

```
✅ *Run complete* — myorg/myservice
Milestone "Phase 5": 8 implemented, 0 failed, 1 awaiting review
```

Notifications are dispatched via `notify.Fire()` with a 10-second timeout per provider. If the Telegram API is down, the run continues unaffected.

---

## Dashboard Human Review Views

**What it does:** Surfaces PRs awaiting human review in the `godark status` web dashboard, making it easy to see which PRs need attention without checking GitHub directly.

**Example:** After a run completes with `auto_merge: none`, three PRs are awaiting review. Opening `godark status` shows:

**Run list view** -- each run row includes an "awaiting" badge. A run with two PRs pending shows "2 awaiting" next to the run summary.

**Run detail view** -- a "PRs Awaiting Review" section appears at the top, listing only issues with `ready-to-merge` status. This section is hidden when no PRs are waiting. The full issue list below supports filtering with `?filter=awaiting_human` to show only those issues.

**Issue detail view** -- the dialogue timeline shows human feedback rounds alongside AI reviewer comments. Human entries are visually distinct: purple background (#a78bfa) with a person icon, clearly separated from the AI reviewer's notes. If `godark watch` processed multiple human review rounds, each appears chronologically in the timeline.

Status labels in the issue list map to the PR lifecycle:
- "Ready to Merge" -- green success badge (outcome status `ready-to-merge`)
- "Needs Human Review" -- yellow warning badge (outcome status `needs-human-review`, for PRs that exhausted retries)

---

## Session Resumption for Human Feedback

**What it does:** When a human requests changes, the implementer agent resumes its prior session rather than starting from scratch. The agent retains its full context -- original implementation reasoning, AI reviewer feedback from earlier rounds, and now the human's comments.

**Example:** PR #42 was created by the implementer agent in session `abc123`. The AI reviewer approved it, but a human reviewer points out an edge case in the error handling. `godark watch` detects the review and calls `rundata.FindSessionID()`, which scans `~/.godark/runs/myorg/myservice/` for the most recent run data containing PR #42.

The function walks run directories from newest to oldest, checking each `outcome.json` for a matching PR number. When it finds a match, it reads the session ID from the latest retry step result, or falls back to the implement step result.

With the session ID in hand, `agent.Retry()` resumes the implementer with the human's feedback formatted as review comments -- the same format used for AI reviewer feedback. The agent sees a unified stream:

```
Round 1 (AI reviewer): APPROVED
Round 2 (human): CHANGES_REQUESTED — "The error handling in handler.go doesn't account for..."
```

If the run data is missing or the session ID cannot be found -- perhaps the runs directory was cleaned up -- the agent falls back to a cold start. The fix still works; it just costs more tokens because the agent has to rediscover its prior context.
