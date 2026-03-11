# Phase 18: Agent Planning & Fresh-Context Restart

> **Goal:** Apply planner-worker-judge lessons to the dev loop. A dedicated
> planner agent analyzes issues and produces structured implementation plans
> before the worker starts. Retries that exhaust session context escalate to
> fresh-context restarts. Quality reviewer ROI becomes measurable so teams can
> decide whether to keep or fold it.

## Milestone

`Phase 18`

---

## Issue 343: Planning config and prompt template

### Description

Add the `planning:` config block, the `Planner` prompt path in `Prompts`, and
a new `prompts/planner.txt` template. The planner prompt instructs a read-only
agent to analyze the issue and codebase and produce a structured implementation
plan. Also add a `PlanOutput` field to `PromptData` so downstream templates
(implementer) can receive the plan.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `Planning` struct:
    ```go
    type Planning struct {
        Enabled            bool `yaml:"enabled"`
        FreshRestartAfter  int  `yaml:"fresh_restart_after"`
    }
    ```
  - Add `Planning Planning` field to `Config` with yaml tag `planning`
  - Defaults: `Enabled: true`, `FreshRestartAfter: 2`
  - Add validation: `FreshRestartAfter` must be >= 0 when `Enabled` is true
  - Add `Planner string` field to config `Prompts` struct with yaml tag
    `planner`
- Modify `internal/agent/prompt.go`:
  - Add `Planner string` field to agent `Prompts` struct
  - Load `planner.txt` in `LoadPrompts` (optional, same pattern as
    `QualityReviewer`)
  - Add `PlanOutput string` field to `PromptData`
- New file: `prompts/planner.txt` — embedded via existing `//go:embed` in
  `prompts/embed.go`
- Template variables: `{{.Repo}}`, `{{.IssueNumber}}`, `{{.IssueTitle}}`,
  `{{.IssueBody}}`, `{{.ArchitectureDocContent}}`, `{{.ArchitectureJSON}}`,
  `{{.ConventionsDocContent}}`, `{{.HasScenarioSpec}}`, `{{.BaseBranch}}`,
  `{{.ModuleContext}}`
- The planner prompt should instruct the agent to:
  - Read the issue body and explore the codebase
  - Identify which files need to change and in what order
  - Describe a test strategy (what to test, which packages)
  - Flag risk areas and ambiguities
  - Output a structured plan in a parseable format (markdown with headers)
  - NOT make any changes — read-only exploration only

### Acceptance criteria

- [ ] `Config` has `Planning` struct with `Enabled` and `FreshRestartAfter`
- [ ] Config defaults: `Enabled: true`, `FreshRestartAfter: 2`
- [ ] `Prompts` struct (both config and agent) has `Planner` field
- [ ] `PromptData` has `PlanOutput` field
- [ ] `prompts/planner.txt` exists and is embedded
- [ ] `LoadPrompts` loads planner template

### Test cases

- **Config defaults**: New config has `Planning.Enabled: true`,
  `Planning.FreshRestartAfter: 2`
- **Config override**: Setting `planning: {enabled: false}` in YAML disables
  planning
- **FreshRestartAfter override**: Setting `fresh_restart_after: 0` is valid
  (means never escalate to fresh context)
- **Planner prompt loads**: `LoadPrompts` returns non-empty `Planner` field
- **Custom planner path**: Setting `prompts: {planner: "custom/plan.txt"}`
  loads from that path
- **Template renders**: `RenderPrompt` with planner template and populated
  `PromptData` produces valid output

---

## Issue 345: Planner agent function

**Blocked by**: #343

### Description

New file `internal/agent/planner.go` containing the `Plan()` function. Follows
the same pattern as `Implement()` — renders the planner prompt, builds
`RunOpts`, and calls `Run()`. The planner uses the `reviewer` role permissions
(read-only: Read, Glob, Grep, Bash — no Write/Edit) since it should only
explore the codebase, not modify it.

The function extracts the plan text from the agent's `ResultText` for passing
to the implementer.

### Key constraints

- New file: `internal/agent/planner.go`
- Exported function:
  ```go
  // Plan runs the planner agent to produce an implementation plan for the
  // given issue. The planner is read-only — it explores the codebase and
  // outputs a structured plan without making changes.
  func Plan(ctx context.Context, issue github.Issue, cfg *config.Config,
      prompts *Prompts, authEnv map[string]string,
      logger *slog.Logger) (*Result, error)
  ```
- Uses `newPromptData(issue, cfg, slug)` to build template data (same as
  `Implement`)
- Renders `prompts.Planner` template
- Calls `newRunOpts(rendered, cfg, authEnv, "reviewer")` — reuses reviewer
  role for read-only permissions
- No new role in `agent_runner.py` — `reviewer` role already enforces
  read-only tool access
- No session resumption — planner runs fresh every time
- No branch checkout needed — planner reads the default/base branch

### Acceptance criteria

- [ ] `Plan()` function exists in `internal/agent/planner.go`
- [ ] Planner uses `reviewer` role (read-only permissions)
- [ ] Planner renders the planner prompt template with issue data
- [ ] `Result.ResultText` contains the plan output
- [ ] No files are modified by the planner agent

### Test cases

- **Plan invocation**: `Plan()` renders the planner prompt and calls `Run`
  with role `"reviewer"`
- **Plan result**: Returned `Result` has `ResultText` populated with plan
  content
- **Prompt data populated**: Rendered prompt contains issue title, body,
  architecture doc content
- **Timeout handling**: Timed-out planner returns `Result{TimedOut: true}`

---

## Issue 347: Wire planner into ProcessIssue

**Blocked by**: #345

### Description

Insert the planning step into `ProcessIssue` in `loop.go`, between step 0
(spec generation) and step 1 (implement). When `cfg.Planning.Enabled` is true
and `prompts.Planner` is non-empty, call `Plan()` and pass the resulting plan
text to the implementer via `PromptData.PlanOutput`.

The planner is best-effort: if it fails or times out, log a warning and proceed
to implementation without a plan (graceful degradation, same pattern as spec
generation).

### Key constraints

- Modify `internal/agent/loop.go`:
  - After step 0 (spec generation) and before step 1 (implement), add new
    step 0.5: planning
  - Guard: `if cfg.Planning.Enabled && prompts.Planner != ""`
  - Call `Plan(ctx, issue, cfg, prompts, authEnv, logger)`
  - On success: extract `planResult.ResultText` and store in a local
    `planOutput` variable
  - On error or timeout: log warning, set `planOutput = ""`, continue
  - Before calling `Implement()`, set `data.PlanOutput = planOutput` — this
    requires modifying `Implement()` to accept an optional plan output
- Modify `internal/agent/implementer.go`:
  - Change `Implement()` signature to accept `planOutput string` parameter
  - Set `data.PlanOutput = planOutput` before rendering the implementer prompt
  - Update all call sites of `Implement()` (only `loop.go`)
- Modify `prompts/implementer.txt`:
  - Add conditional section: when `{{.PlanOutput}}` is non-empty, include
    it under a "## Implementation Plan" header before the main instructions
  - Instruct the implementer to follow the plan's file list and change
    sequence, adjusting if the plan is inaccurate

### Acceptance criteria

- [ ] Planning step runs between spec generation and implementation
- [ ] Plan output is passed to implementer via `PromptData.PlanOutput`
- [ ] Planner failure is non-fatal (logs warning, continues without plan)
- [ ] Planner timeout is non-fatal (same graceful degradation)
- [ ] `Planning.Enabled: false` skips the planning step entirely
- [ ] Empty `prompts.Planner` skips the planning step

### Test cases

- **Plan then implement**: With planning enabled, `Plan()` runs before
  `Implement()`, and plan output appears in implementer prompt
- **Plan disabled**: With `Planning.Enabled: false`, `Plan()` is not called
- **Plan failure graceful**: When `Plan()` returns an error, implementation
  proceeds with empty `PlanOutput`
- **Plan timeout graceful**: When `Plan()` returns `TimedOut: true`,
  implementation proceeds with empty `PlanOutput`
- **No planner prompt**: When `prompts.Planner` is empty, planning step is
  skipped regardless of config

---

## Issue 346: Fresh-context escalation on retries

**Blocked by**: #343

### Description

Modify the retry path in the functional review loop so that after
`FreshRestartAfter` consecutive failed retries with session resumption, the
next retry discards the session ID and starts a fresh agent. The fresh agent
reads the PR diff and review comments cold, without any prior session context.

This implements the Cursor insight that fresh context prevents drift and stale
assumptions on long retry chains, while preserving token-efficient session
resumption for early retries.

### Key constraints

- Modify `internal/agent/loop.go`:
  - In the step 5 review/retry loop, track the current retry attempt number
    (already exists as `attempt`)
  - Before calling `Retry()`, check:
    `if cfg.Planning.FreshRestartAfter > 0 && attempt >= cfg.Planning.FreshRestartAfter`
  - When true: pass empty string for `prevSessionID` instead of `sessionID`
  - Log the escalation: `"escalating to fresh-context retry"`
  - The same logic applies to the quality review retry loop (step 4)
- No changes to `Retry()` itself — it already handles empty `prevSessionID`
  by starting a fresh session
- `FreshRestartAfter: 0` means never escalate (always use session resumption,
  current behavior)
- `FreshRestartAfter: 1` means the first retry uses session resumption, the
  second retry is fresh
- No changes to `VerifyFix` — verify-fix retries are mechanically different
  (fixing specific errors) and benefit less from fresh context

### Acceptance criteria

- [ ] After `FreshRestartAfter` retries, session ID is cleared for subsequent
  retries
- [ ] Early retries (below threshold) still use session resumption
- [ ] `FreshRestartAfter: 0` preserves current behavior (always resume)
- [ ] Fresh-context escalation is logged
- [ ] Both functional review and quality review retry loops respect the setting

### Test cases

- **Fresh restart at threshold**: With `FreshRestartAfter: 2`, attempts 0 and
  1 pass `sessionID` to `Retry()`, attempt 2 passes empty string
- **Never escalate**: With `FreshRestartAfter: 0`, all retries pass
  `sessionID` regardless of attempt number
- **Immediate fresh**: With `FreshRestartAfter: 1`, attempt 0 passes
  `sessionID`, attempt 1 passes empty string
- **Quality review loop**: Fresh-context escalation applies to quality review
  retries with the same threshold
- **Log message**: Fresh-context escalation produces an info-level log entry

---

## Issue 348: Planner run data and dashboard

**Blocked by**: #347

### Description

Extend the run data system to record planner results, and surface them in the
dashboard issue detail view. The planner step is recorded alongside existing
steps (spec generation, implementation, review). The dashboard shows the plan
in a collapsible section.

### Key constraints

- Modify `internal/agent/runhook.go`:
  - Add `WritePlanResult(issueNumber int, step rundata.StepResult) error` to
    `RunDataHook` interface
- Modify `internal/rundata/writer.go`:
  - Implement `WritePlanResult` — writes to
    `issues/<issueNumber>/plan.json`
- Modify `internal/rundata/reader.go`:
  - Add `PlanResult *StepResult` field to `IssueDetail` with json tag
    `plan_result,omitempty`
  - Read `plan.json` in `ReadIssueDetail` when present
- Modify `internal/agent/loop.go`:
  - After successful `Plan()` call, write result via
    `hook.WritePlanResult(issue.Number, ResultToStep(planResult))`
- Modify `internal/dashboard/templates/issue-detail.html`:
  - Add collapsible "Implementation Plan" section showing plan result text
  - Show cost and duration alongside the plan
  - Only render when `PlanResult` is non-nil
- No new Go handler code needed — `IssueDetail` already flows to the template

### Acceptance criteria

- [ ] `RunDataHook` interface has `WritePlanResult` method
- [ ] `plan.json` is written to the correct issue directory
- [ ] `ReadIssueDetail` loads plan result when present
- [ ] Missing `plan.json` does not break existing run data reads
- [ ] Dashboard shows plan in issue detail view when present

### Test cases

- **Write plan result**: `WritePlanResult(42, step)` creates
  `issues/42/plan.json`
- **Read plan result**: `ReadIssueDetail` returns `PlanResult` when
  `plan.json` exists
- **Missing plan backwards compatible**: `ReadIssueDetail` returns nil
  `PlanResult` when `plan.json` is absent
- **Hook called in loop**: Mock hook verifies `WritePlanResult` called after
  planner step
- **Dashboard renders plan**: Issue detail page with `PlanResult` set
  contains "Implementation Plan" section

---

## Issue 344: Quality reviewer value-add analysis

### Description

Add a new report to `godark analyze` that measures the quality reviewer's
value-add across runs. The report compares quality review outcomes to
functional review outcomes to answer: "Does the quality reviewer catch issues
that the functional reviewer misses?" This gives teams data to decide whether
to keep the quality reviewer as a separate pass or fold its concerns into the
functional reviewer prompt.

### Key constraints

- Modify `internal/analysis/analysis.go`:
  - Add `QualityReviewerStats` struct:
    ```go
    type QualityReviewerStats struct {
        TotalRuns               int     `json:"total_runs"`
        RunsWithQualityReview   int     `json:"runs_with_quality_review"`
        QualityChangesRequested int     `json:"quality_changes_requested"`
        FunctionalApprovedAfter int     `json:"functional_approved_after_quality_fix"`
        QualityOnlyBlocks       int     `json:"quality_only_blocks"`
        AvgQualityCostUSD       float64 `json:"avg_quality_cost_usd"`
        TokenCostTotal          float64 `json:"token_cost_total"`
    }
    ```
  - Add `ComputeQualityReviewerStats(runs []rundata.RunDetail) QualityReviewerStats`
    function
  - Logic: iterate runs, for each issue check whether quality review requested
    changes and whether functional review subsequently approved. Count issues
    where quality review was the only blocker (quality blocked but functional
    approved on first pass after quality fix)
  - Sum quality review costs across all runs
- Modify `internal/cmd/analyze.go`:
  - Include `QualityReviewerStats` in the analyze output
  - Human-readable summary: "Quality reviewer caught issues in X/Y runs
    (Z%). Average cost: $N per issue. Total cost: $M."
  - JSON output: include the stats struct

### Acceptance criteria

- [ ] `QualityReviewerStats` struct captures key metrics
- [ ] `ComputeQualityReviewerStats` correctly aggregates across runs
- [ ] `godark analyze` output includes quality reviewer section
- [ ] Runs without quality review are counted but don't skew percentages

### Test cases

- **No quality reviews**: Runs with no quality review data produce zero counts
  and zero cost
- **All quality approved**: Runs where quality always approves show 0
  `QualityChangesRequested`
- **Quality blocks then functional approves**: Issue where quality requested
  changes, implementer fixed, functional approved — counted as
  `QualityOnlyBlocks`
- **Both block**: Issue where both quality and functional request changes —
  not counted as `QualityOnlyBlocks` (functional would have caught it too)
- **Cost aggregation**: Average and total costs computed correctly across
  multiple runs
- **JSON output**: `--json` flag includes `QualityReviewerStats` in output
