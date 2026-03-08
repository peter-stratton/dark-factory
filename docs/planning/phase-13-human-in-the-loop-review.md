# Phase 13: Human-in-the-Loop Review

> **Goal:** Humans can review godark-created PRs and request changes that the
> agent automatically picks up and fixes. Teams adopt godark with full human
> oversight and gradually increase autonomy as trust builds. This is the
> critical path for org adoption — most teams will not start with auto-merge.

## Milestone

`Phase 13`

---

## Issue 238: Config field for `auto_merge`

### Description

Replace the existing `NoMerge bool` field with `AutoMerge string` in the config
struct. Valid values are `none` (stop at PR, human merges), `low_risk`
(auto-merge small/safe PRs, stop for rest), and `all` (auto-merge everything,
human spot-checks only). Default is `none`.

This is a breaking change from the current `no_merge` behavior — the old field
is removed entirely, not deprecated.

### Key constraints

- Modify `internal/config/config.go`:
  - Remove `NoMerge bool` field and `yaml:"no_merge"` tag
  - Add `AutoMerge string` field with yaml tag `auto_merge`, default `"none"`
  - Remove `NoMerge *bool` from `CLIFlags`; add `AutoMerge *string`
  - Add validation: value must be one of `none`, `low_risk`, `all`
- Modify `internal/cmd/run.go`:
  - Replace `--no-merge` flag with `--auto-merge` string flag
- Modify `internal/cmd/implement.go`:
  - Replace `--no-merge` flag with `--auto-merge` string flag
- Modify `internal/agent/loop.go`:
  - Replace `cfg.NoMerge` check with `cfg.AutoMerge == "none"` (preserves
    current skip-merge behavior for `none`)
  - `low_risk` and `all` are wired in a later issue
- Update all tests referencing `NoMerge`

### Acceptance criteria

- [ ] `Config` has `AutoMerge` field, default `"none"`
- [ ] `NoMerge` field is fully removed from config, CLI flags, and tests
- [ ] Validation rejects values other than `none`, `low_risk`, `all`
- [ ] `auto_merge: none` skips merge (same behavior as old `no_merge: true`)
- [ ] `auto_merge: all` merges after review (same behavior as old default)

### Test cases

- **Config default**: New config has `AutoMerge == "none"`
- **Valid values**: `auto_merge: none`, `auto_merge: low_risk`, and
  `auto_merge: all` each parse correctly
- **Invalid value**: `auto_merge: always` fails validation with descriptive
  error
- **Flag override**: `--auto-merge all` overrides config file value
- **None skips merge**: `auto_merge: none` causes `ProcessIssue` to return
  `ready-to-merge` status without calling `gh pr merge`
- **All merges**: `auto_merge: all` causes `ProcessIssue` to merge the PR

---

## Issue 239: Config field for `watch` block

### Description

Add a `Watch` struct to the config for the `godark watch` polling settings.
Contains the poll interval as a duration string.

This is config-only — the watch command is handled in a follow-on issue.

### Key constraints

- Modify `internal/config/config.go`:
  - New type:
    ```go
    type Watch struct {
        PollInterval string `yaml:"poll_interval"`
    }
    ```
  - Add `Watch *Watch` field to `Config` with yaml tag `watch` (pointer so
    nil = not configured, uses default interval)
  - Default: `nil` (watch command uses hardcoded default `"60s"` when nil)
- Add validation in `validate()`:
  - If `watch` is set and `poll_interval` is non-empty, it must parse as a
    `time.Duration`

### Acceptance criteria

- [ ] `Config` has `Watch` field, nil by default
- [ ] Setting `watch: {poll_interval: "30s"}` in YAML is reflected in parsed
  config
- [ ] Validation rejects invalid poll interval duration
- [ ] Nil `watch` is valid (watch command uses default)

### Test cases

- **Config defaults**: New config has nil `Watch`
- **Valid config**: `watch: {poll_interval: "30s"}` parses correctly
- **Invalid interval**: `poll_interval: "not-a-duration"` fails validation
- **Not configured**: No `watch` in YAML, field is nil

---

## Issue 240: Config field for `risk_thresholds` block

### Description

Add a `RiskThresholds` struct to the config with configurable thresholds for
the `low_risk` auto-merge classification. When `auto_merge: low_risk` is set,
PRs are evaluated against these thresholds to decide whether a human must
review.

This is config-only — the risk classifier logic is handled in a follow-on
issue.

### Key constraints

- Modify `internal/config/config.go`:
  - New type:
    ```go
    type RiskThresholds struct {
        MaxLines int `yaml:"max_lines"`
        MaxFiles int `yaml:"max_files"`
    }
    ```
  - Add `RiskThresholds *RiskThresholds` field to `Config` with yaml tag
    `risk_thresholds` (pointer so nil = use defaults)
  - Defaults when nil: `MaxLines: 200`, `MaxFiles: 10` (applied by the
    classifier, not the config layer)
- Add validation in `validate()`:
  - If set, `max_lines` and `max_files` must be positive integers

### Acceptance criteria

- [ ] `Config` has `RiskThresholds` field, nil by default
- [ ] Setting `risk_thresholds: {max_lines: 100, max_files: 5}` in YAML is
  reflected in parsed config
- [ ] Validation rejects zero or negative values
- [ ] Nil `risk_thresholds` is valid (classifier uses defaults)

### Test cases

- **Config defaults**: New config has nil `RiskThresholds`
- **Valid config**: `risk_thresholds: {max_lines: 100, max_files: 5}` parses
  correctly
- **Zero lines**: `max_lines: 0` fails validation
- **Negative files**: `max_files: -1` fails validation
- **Not configured**: No `risk_thresholds` in YAML, field is nil

---

## Issue 241: `internal/label/` package — PR lifecycle labels and state machine

### Description

New package in the foundation layer defining label constants for PR lifecycle
states and a helper to validate state transitions. The PR lifecycle labels
communicate state to humans at a glance:

- `godark:awaiting-human-review` — AI review complete, waiting for human
- `godark:fixing-review-feedback` — agent is fixing human-requested changes
- `godark:ready-to-merge` — all reviews passed, safe to merge (used when
  `auto_merge: none`)

The package also defines a `Transition(from, to string) bool` function that
validates whether a label transition is legal (e.g., `awaiting-human-review`
→ `fixing-review-feedback` is valid, but `ready-to-merge` →
`fixing-review-feedback` is not).

### Key constraints

- New package: `internal/label/label.go`
- Foundation layer — zero dependencies, importable by all layers
- Constants:
  ```go
  const (
      AwaitingHumanReview  = "godark:awaiting-human-review"
      FixingReviewFeedback = "godark:fixing-review-feedback"
      ReadyToMerge         = "godark:ready-to-merge"
  )
  ```
- `Transition(from, to string) bool` — returns true if the transition is valid
- `All() []string` — returns all PR lifecycle labels for bulk operations
  (e.g., ensuring labels exist in the repo)
- Valid transitions:
  - `""` → `AwaitingHumanReview` (first time labeling)
  - `""` → `ReadyToMerge` (auto-merge mode, no human review needed)
  - `AwaitingHumanReview` → `FixingReviewFeedback`
  - `FixingReviewFeedback` → `AwaitingHumanReview`
  - `AwaitingHumanReview` → `ReadyToMerge`
  - Any → `""` (removing labels on merge/close)

### Acceptance criteria

- [ ] Package exists at `internal/label/`
- [ ] Three PR lifecycle label constants are exported
- [ ] `Transition` validates legal state changes
- [ ] `Transition` rejects illegal state changes
- [ ] `All()` returns all three labels

### Test cases

- **Valid transition**: `Transition("", AwaitingHumanReview)` returns true
- **Valid loop**: `Transition(AwaitingHumanReview, FixingReviewFeedback)`
  returns true
- **Valid loop back**: `Transition(FixingReviewFeedback, AwaitingHumanReview)`
  returns true
- **Valid approval**: `Transition(AwaitingHumanReview, ReadyToMerge)` returns
  true
- **Invalid skip**: `Transition(ReadyToMerge, FixingReviewFeedback)` returns
  false
- **Clear labels**: `Transition(ReadyToMerge, "")` returns true
- **All labels**: `All()` returns slice of length 3

---

## Issue 243: Migrate `LockLabel` to `internal/label/`

### Description

Move the `LockLabel` constant from `internal/lock/lock.go` into the new
`internal/label/` package. Update all imports. This consolidates all
godark-managed GitHub labels into a single package.

### Key constraints

- Add to `internal/label/label.go`:
  ```go
  const InProgress = "godark-in-progress"
  ```
- Remove `LockLabel` constant from `internal/lock/lock.go`
- Update `internal/lock/lock.go` to import `label.InProgress` instead of
  using the local constant
- Update any other files referencing `lock.LockLabel` to use
  `label.InProgress`
- Update `internal/cmd/init.go` where `LockLabel` is used for
  `createLockLabel`

### Acceptance criteria

- [ ] `label.InProgress` constant exists with value `"godark-in-progress"`
- [ ] `lock.LockLabel` is removed
- [ ] `internal/lock/` imports `internal/label/` for the constant
- [ ] All references to `lock.LockLabel` updated to `label.InProgress`
- [ ] All existing tests pass without modification (except import paths)

### Test cases

- **Constant value**: `label.InProgress` equals `"godark-in-progress"`
- **Lock uses label**: `internal/lock/` references `label.InProgress`, not a
  local constant
- **Init uses label**: `internal/cmd/init.go` references `label.InProgress`
- **Existing lock tests pass**: No behavioral change

---

## Issue 244: Wire PR labels into orchestrator

**Blocked by**: #241, #238

### Description

Modify the agent loop to set PR lifecycle labels at state transitions. After
AI review completes and the PR is approved:

- If `auto_merge: none` → label `godark:awaiting-human-review`
- If `auto_merge: low_risk` → label determined by risk classifier (wired in
  a later issue); for now, label `godark:awaiting-human-review`
- If `auto_merge: all` → merge (no label needed, current behavior)

When the orchestrator exhausts retries and labels `needs-human-review`, also
apply `godark:awaiting-human-review`.

On merge or close, remove all PR lifecycle labels.

### Key constraints

- Modify `internal/agent/loop.go`:
  - After the `cfg.AutoMerge == "none"` branch (currently line ~496), apply
    `label.AwaitingHumanReview` via `github.AddIssueLabel` on the PR
  - After merge, remove all lifecycle labels via `label.All()` loop
  - After `needs-human-review` escalation, apply
    `label.AwaitingHumanReview`
- Modify `internal/orchestrator/orchestrator.go`:
  - At startup, ensure all lifecycle labels exist in the repo via
    `github.EnsureLabel` (same pattern as `LockLabel` initialization)
- Label colors: use a consistent palette (suggest blue for awaiting, yellow
  for fixing, green for ready)

### Acceptance criteria

- [ ] `auto_merge: none` applies `godark:awaiting-human-review` label to PR
- [ ] Merge removes all lifecycle labels
- [ ] `needs-human-review` escalation applies `godark:awaiting-human-review`
- [ ] Lifecycle labels are ensured at orchestrator startup
- [ ] Label operations use `github.AddIssueLabel` / `github.RemoveIssueLabel`

### Test cases

- **None labels PR**: `auto_merge: none` and approved → PR gets
  `awaiting-human-review` label
- **All skips label**: `auto_merge: all` and approved → no lifecycle label,
  PR merged
- **Escalation labels**: Max retries exhausted → `awaiting-human-review`
  label applied
- **Merge cleans labels**: After merge, lifecycle labels are removed
- **Labels ensured**: Orchestrator startup calls `EnsureLabel` for all
  lifecycle labels

---

## Issue 246: `godark watch` command scaffold

**Blocked by**: #239, #244

### Description

New `godark watch` Cobra command that polls GitHub for PRs labeled
`godark:awaiting-human-review` and detects when a human submits a
`CHANGES_REQUESTED` review. Runs as a long-lived foreground process (Ctrl+C
to stop).

This issue implements the polling loop and detection only — the agent
resumption (feeding feedback to the implementer) is handled in a follow-on
issue. When a change-requesting review is detected, this issue logs the event
and applies the `godark:fixing-review-feedback` label.

### Key constraints

- New file: `internal/cmd/watch.go`
- Add `watchCmd` to root command
- Required flag: `--repo` (or from `godark.yaml`)
- Poll loop:
  1. List open PRs with label `godark:awaiting-human-review` using
     `gh pr list --label "godark:awaiting-human-review" --repo <repo>
     --json number,headRefName`
  2. For each PR, fetch reviews using `gh api
     repos/{owner}/{repo}/pulls/{number}/reviews --jq
     '.[] | select(.state == "CHANGES_REQUESTED")'`
  3. Track which reviews have already been processed (by review ID) to
     avoid re-processing on subsequent polls
  4. On new `CHANGES_REQUESTED` review: log it, apply
     `godark:fixing-review-feedback` label, remove
     `godark:awaiting-human-review` label
- Poll interval from `cfg.Watch.PollInterval` (default `"60s"` if nil)
- Graceful shutdown on context cancellation (SIGINT/SIGTERM)
- New function in `internal/github/` for fetching PR reviews:
  ```go
  func FetchPRReviews(repo string, prNum int) ([]PRReview, error)

  type PRReview struct {
      ID     int    `json:"id"`
      State  string `json:"state"`
      Body   string `json:"body"`
      Author string `json:"author"`
  }
  ```

### Acceptance criteria

- [ ] `godark watch` command exists and is registered
- [ ] Polls for PRs with `godark:awaiting-human-review` label
- [ ] Detects `CHANGES_REQUESTED` reviews on labeled PRs
- [ ] Applies `fixing-review-feedback` label on detection
- [ ] Removes `awaiting-human-review` label on detection
- [ ] Tracks processed review IDs to avoid duplicates
- [ ] Respects configured poll interval
- [ ] Shuts down cleanly on SIGINT

### Test cases

- **Command registered**: `godark watch --help` shows usage
- **Poll finds PR**: Mock `gh pr list` returns one PR → reviews are fetched
- **Review detected**: `CHANGES_REQUESTED` review → labels swapped
- **Duplicate skipped**: Same review ID on next poll → no action taken
- **No PRs**: No labeled PRs → poll sleeps and retries
- **Default interval**: Nil watch config uses 60s default
- **Custom interval**: `poll_interval: "10s"` uses 10s

---

## Issue 249: Human feedback agent resumption

**Blocked by**: #246

### Description

When `godark watch` detects a human `CHANGES_REQUESTED` review, it feeds the
review comments to the implementer agent via session resumption, waits for the
fix, and re-labels the PR as `godark:awaiting-human-review`.

The implementer resumes its prior session (`GODARK_SESSION_ID`), so it has
full context: original implementation reasoning, AI reviewer feedback from
prior rounds, and now the human's comments. Human comments are formatted the
same way as AI reviewer comments — the implementer sees a unified feedback
stream.

### Key constraints

- Modify `internal/cmd/watch.go`:
  - After detecting a `CHANGES_REQUESTED` review, invoke the implementer
    agent with the human's review comments
  - Load config and prompt templates (same as `godark implement`)
  - Retrieve the session ID from run data (stored during the original
    `ProcessIssue` run)
  - Call `agent.Retry()` with the human's review body as the feedback
    context
  - After the agent pushes, remove `godark:fixing-review-feedback` and
    apply `godark:awaiting-human-review`
- New function in `internal/github/` for fetching review comments:
  ```go
  func FetchReviewComments(repo string, prNum int, reviewID int) ([]string, error)
  ```
- Session ID retrieval — need to read from run data. Add a function to
  find the most recent session ID for a given PR number:
  ```go
  // In internal/rundata/
  func FindSessionID(runsDir, repo string, prNum int) (string, error)
  ```
  This scans run directories for the repo, finds outcomes matching the PR
  number, and reads the session ID from the implement or retry step result.
- Run data for watch-initiated fixes should be written to a new run
  directory (the watch command creates its own `rundata.Writer`)

### Acceptance criteria

- [ ] Human review comments are fed to implementer via `agent.Retry()`
- [ ] Agent resumes prior session using stored session ID
- [ ] After fix, PR is re-labeled `godark:awaiting-human-review`
- [ ] `godark:fixing-review-feedback` label is removed after fix
- [ ] Run data is written for watch-initiated fix cycles
- [ ] Missing session ID falls back to cold start (no session resume)

### Test cases

- **Feedback fed**: Human comment body is passed to `Retry()` as review
  feedback
- **Session resumed**: Session ID from run data is passed to agent
- **Labels swapped**: After fix push, `fixing-review-feedback` removed and
  `awaiting-human-review` applied
- **No session ID**: Missing session ID still invokes agent (cold start)
- **Run data written**: Watch-initiated fix creates run data directory
- **Multiple comments**: Multiple review comments are concatenated into a
  single feedback string

---

## Issue 245: Risk classifier

**Blocked by**: #240

### Description

Add a risk classification function to `internal/quality/` that evaluates a PR
against configurable and non-configurable risk gates. Returns a risk
assessment indicating whether the PR is safe for auto-merge.

A PR is classified as `low_risk` only if ALL of these gates pass:

1. Lines changed ≤ `max_lines` (configurable, default 200)
2. Files changed ≤ `max_files` (configurable, default 10)
3. No protected paths touched
4. Verify pipeline passed on first attempt (zero fix cycles)
5. No quality flags raised on the final review

Any single gate failing → the PR is not low-risk.

### Key constraints

- New file: `internal/quality/risk.go`
- Types:
  ```go
  type RiskInput struct {
      LinesChanged   int
      FilesChanged   int
      ChangedFiles   []string       // file paths in the PR diff
      ProtectedPaths []string       // from config
      FixCycles      int            // verify fix attempts used
      QualityFlags   []Flag         // from review quality checks
  }

  type RiskAssessment struct {
      IsLowRisk    bool              `json:"is_low_risk"`
      Gates        []RiskGate        `json:"gates"`
  }

  type RiskGate struct {
      Name    string `json:"name"`
      Passed  bool   `json:"passed"`
      Detail  string `json:"detail"`
  }
  ```
- Function:
  ```go
  func ClassifyRisk(input RiskInput, maxLines, maxFiles int) RiskAssessment
  ```
- Default thresholds (used when config is nil): `maxLines=200`,
  `maxFiles=10`
- Protected path matching reuses the same `strings.HasPrefix` logic used
  by the existing protected path checks
- The assessment struct is written to run data so humans can audit the
  classification (wired in a later issue)

### Acceptance criteria

- [ ] `ClassifyRisk` returns `IsLowRisk: true` when all gates pass
- [ ] `ClassifyRisk` returns `IsLowRisk: false` when any gate fails
- [ ] Each gate result is individually reported in the assessment
- [ ] Protected path matching uses prefix matching
- [ ] Default thresholds are 200 lines and 10 files

### Test cases

- **All pass**: Small PR, no protected paths, no fix cycles, no flags →
  `IsLowRisk: true`
- **Lines exceeded**: 201 lines changed → `IsLowRisk: false`, `max_lines`
  gate failed
- **Files exceeded**: 11 files changed → `IsLowRisk: false`, `max_files`
  gate failed
- **Protected path touched**: Changed file matches protected path →
  `IsLowRisk: false`
- **Fix cycles used**: `FixCycles: 1` → `IsLowRisk: false`
- **Quality flags raised**: One quality flag → `IsLowRisk: false`
- **Multiple gates fail**: Lines and files both exceeded → both gates
  reported as failed

---

## Issue 247: Wire auto_merge and risk classification into merge decision

**Blocked by**: #245, #244

### Description

Integrate the risk classifier into the merge flow in `ProcessIssue`. After AI
review approves a PR:

- `auto_merge: all` → merge (current behavior, unchanged)
- `auto_merge: none` → label `awaiting-human-review`, return (already wired)
- `auto_merge: low_risk` → run risk classifier; if low-risk, merge; if not,
  label `awaiting-human-review`

The risk assessment is written to run data so humans can audit why a PR was
or wasn't auto-merged.

### Key constraints

- Modify `internal/agent/loop.go`:
  - After review approval, add `auto_merge: low_risk` branch
  - Gather `RiskInput` from available data:
    - `LinesChanged` and `FilesChanged`: fetch from `gh pr view --json
      additions,deletions,changedFiles`
    - `ChangedFiles`: fetch from `gh pr diff --name-only`
    - `ProtectedPaths`: from `cfg.ProtectedPaths`
    - `FixCycles`: count of verify fix attempts (already tracked in loop)
    - `QualityFlags`: from review quality checks (already computed in loop)
  - Call `quality.ClassifyRisk()` with thresholds from
    `cfg.RiskThresholds` (or defaults)
  - If low-risk: merge
  - If not low-risk: label `awaiting-human-review`, return
    `ready-to-merge` status
- New function in `internal/github/`:
  ```go
  func FetchPRStats(repo string, prNum int) (additions, deletions, fileCount int, err error)
  func FetchPRChangedFiles(repo string, prNum int) ([]string, error)
  ```
- Write risk assessment to run data via `RunDataHook`:
  - Add `WriteRiskAssessment(issueNumber int, assessment quality.RiskAssessment) error`
    to `RunDataHook` interface

### Acceptance criteria

- [ ] `auto_merge: low_risk` runs risk classifier before merge decision
- [ ] Low-risk PR is auto-merged
- [ ] Non-low-risk PR is labeled `awaiting-human-review`
- [ ] Risk assessment is written to run data
- [ ] `auto_merge: all` still merges unconditionally
- [ ] `auto_merge: none` still skips merge

### Test cases

- **Low risk merges**: Small PR, no flags → merged
- **High risk stops**: Large PR → labeled `awaiting-human-review`, not merged
- **All mode ignores risk**: `auto_merge: all` with large PR → still merged
- **None mode ignores risk**: `auto_merge: none` with small PR → still not
  merged
- **Risk written**: Risk assessment appears in run data
- **Protected path blocks**: PR touching protected path → not auto-merged

---

## Issue 248: Dashboard human review views

**Blocked by**: #244

### Description

Surface PRs awaiting human review prominently in the dashboard. Add filtering
and display for human review state in existing dashboard views.

### Key constraints

- Modify `internal/dashboard/`:
  - Run detail view: add a "PRs Awaiting Review" section at the top showing
    issues with `ready-to-merge` status that haven't been merged
  - Add filter/sort by `awaiting_human` state in the run detail issue list
  - Issue detail view: show human feedback rounds in the dialogue timeline
    (if `dialogue.json` contains entries from human reviewers, display them
    with a distinct visual style — different background color or icon)
  - Run list view: add a column or badge showing count of PRs awaiting
    human review per run
- Read human review state from run data outcomes (status `ready-to-merge`)
  and from risk assessment data when available

### Acceptance criteria

- [ ] Run detail shows "PRs Awaiting Review" section
- [ ] Issues can be filtered by awaiting-human state
- [ ] Human feedback rounds display in dialogue timeline
- [ ] Run list shows awaiting-review count per run

### Test cases

- **Awaiting section**: Run with `ready-to-merge` outcomes shows awaiting
  section
- **No awaiting**: Run with all `implemented` outcomes hides awaiting section
- **Human dialogue**: Dialogue entry with human author displays with distinct
  style
- **Count badge**: Run list row shows "2 awaiting" when two issues are
  `ready-to-merge`

---

## Issue 242: Update architecture.json for Phase 13 packages

### Description

Add `internal/label/` to the foundation layer in `docs/architecture.json`.
Verify that all other new code lives within existing package directories.
Run `godark vet architecture` to confirm.

### Key constraints

- Modify `docs/architecture.json`:
  - Add `"internal/label/"` to the foundation layer `paths` array
- Run `godark vet architecture` and confirm it passes
- Verify no other new package directories were created outside existing
  layer paths

### Acceptance criteria

- [ ] `internal/label/` is listed in the foundation layer
- [ ] `godark vet architecture` passes after all Phase 13 issues are merged
- [ ] No new package directories exist outside of defined layer paths

### Test cases

- **Vet passes**: `godark vet architecture` exits 0 with no findings
- **Label in foundation**: `architecture.json` foundation layer includes
  `internal/label/`
