# Phase 26: Merge Coordinator Agent

When multiple issues land sequentially on a base branch, the feature branch for the next issue can diverge and develop merge conflicts. Before Phase 26, the system fell back to the full implementer retry agent to resolve conflicts -- slow, expensive, and carrying the entire implementer context when all it needed to do was rebase and fix a few markers. Phase 26 introduces a dedicated merge coordinator agent that does exactly one thing: resolve conflicts and push. It plugs into both per-issue pre-merge and rollup merge flows, appears as a first-class step in the dashboard and TUI, and writes full telemetry to run data.

---

## Prompt Template and Agent Role

**What it does:** A focused prompt template instructs the merge coordinator to rebase a feature branch onto the base branch, resolve conflicts preserving intent from both sides, verify the result with build and test commands, and push. The agent is deliberately restricted -- it can read, edit, search, and run shell commands, but cannot create new files.

**Example:** The template at `prompts/merge_coordinator.txt` receives the same `PromptData` as other agents, plus a `ConflictInfo` field injected at call time:

```
You are a merge coordinator resolving rebase conflicts for issue #{{.IssueNumber}}
in the {{.Repo}} repository.

Issue title: {{.IssueTitle}}
Issue body:
{{.IssueBody}}

Conflict details:
{{.ConflictInfo}}

Steps:
1. Check out the feature branch for this issue
2. Rebase onto origin/{{.BaseBranch}}:
   git rebase origin/{{.BaseBranch}}
3. If conflicts occur, resolve each one:
   - Read both sides of every conflict marker carefully
   - Use the issue context above to understand the intent of the feature branch changes
   - Preserve the intent of both the base branch updates and the feature branch changes
   ...
4. Verify the resolution:
   - Run build: {{.BuildCommand}}
   - Run tests: {{.TestCommand}}
5. Push the rebased branch: git push --force-with-lease

CRITICAL RULES:
- Do NOT create new files -- only edit existing files to resolve conflicts
- Do NOT change any logic beyond what is necessary to resolve the conflicts
```

The "no new files" constraint is enforced at the role level -- `merge_coordinator` allows `Read`, `Edit`, `Bash`, `Glob`, and `Grep` but disallows `Write`.

---

## MergeCoordinate() Agent Function

**What it does:** A dedicated entry point in `internal/agent/merge_coordinator.go` follows the standard agent function pattern: build prompt data, inject conflict info, render the template, create run options with the `merge_coordinator` role, and delegate to `Run()`.

**Example:** The function signature mirrors other agent functions like `Recon()` and `Implement()`:

```go
func MergeCoordinate(ctx context.Context, issue github.Issue, prNum int,
    conflictInfo string, cfg *config.Config, prompts *Prompts,
    authEnv map[string]string, logger *slog.Logger) (*Result, error)
```

The `conflictInfo` parameter is set on `PromptData.ConflictInfo` before rendering -- this is the git error output or a description of what conflicts exist. The caller constructs it from the failure context. A typical invocation from the agent loop:

```go
conflictInfo := fmt.Sprintf(
    "PR #%d has merge conflicts with the base branch that could not be "+
    "automatically resolved.\n\nError: %v\n\nPlease resolve the merge "+
    "conflicts, push the changes, and ensure the branch is up to date "+
    "with the base branch.",
    prNum, updateErr,
)
mcResult, mcErr := MergeCoordinate(ctx, issue, prNum, conflictInfo, cfg, prompts, authEnv, logger)
```

---

## Config and Prompt Loading

**What it does:** The merge coordinator prompt is loadable from a custom path via `godark.yaml` or falls back to the embedded default, following the same optional-prompt pattern as the recon and planner agents.

**Example:** To override the default prompt, add a path under the `prompts` block in `godark.yaml`:

```yaml
prompts:
  merge_coordinator: "custom/prompts/my-merge-coordinator.txt"
```

The config struct in `internal/config/config.go` has the corresponding field:

```go
type Prompts struct {
    // ...
    MergeCoordinator string `yaml:"merge_coordinator"`
}
```

In `internal/agent/prompt.go`, `LoadPrompts()` loads it the same way as other optional prompts -- try the configured path first, fall back to the embedded `merge_coordinator.txt`:

```go
mc, err := loadPromptFile(cfg.Prompts.MergeCoordinator, "merge_coordinator.txt")
if err != nil {
    p.MergeCoordinator = ""
} else {
    p.MergeCoordinator = mc
}
```

---

## Per-Issue Pre-Merge Integration

**What it does:** When the automatic `gh pr update-branch` rebase fails due to conflicts, the agent loop now invokes `MergeCoordinate()` instead of falling back to the full implementer retry agent. After a successful resolution, the verify pipeline re-runs to confirm the rebased code still passes. The attempt is bounded by `max_rebase_attempts` in config.

**Example:** A run is processing issue #340. Three earlier issues have already merged into the base branch, and the PR for #340 now conflicts. In `runPreMergeRebasePhase()` in `internal/agent/loop.go`:

1. `github.UpdateBranch()` attempts an automatic rebase and fails
2. The TUI stage updates to `merge-coordinate`:
   ```go
   reporter.IssueStageChanged(issue.Number, "merge-coordinate")
   ```
3. `MergeCoordinate()` runs with the error output as conflict info
4. The result is written to run data via the `RunDataHook`:
   ```go
   step.TraceID = traceID
   hook.WriteMergeCoordinatorResult(issue.Number, step)
   ```
5. If successful, `runVerifyPhase()` re-runs to validate the resolution
6. If the merge coordinator fails or the PR is still conflicting, the attempt counter increments and the loop retries up to `MaxRebaseAttempts`

This code path is shared between `godark run` and `godark implement` -- both use the same agent loop, so both get the merge coordinator automatically.

---

## Rollup Merge Conflict Handling

**What it does:** When a rollup PR (base branch to default branch) has merge conflicts, the orchestrator invokes the merge coordinator before attempting the merge. If the conflicts can't be resolved after `max_rebase_attempts`, the PR is left open for human review instead of failing the run.

**Example:** A run completes five issues on a `sprint-42` base branch. The rollup PR merging `sprint-42` into `main` has conflicts because someone pushed directly to `main`. In `HandleRollupPR()` in `internal/orchestrator/orchestrator.go`:

```go
logger.Info("rollup PR is conflicting, invoking merge coordinator",
    "pr_number", prNum,
    "attempt", attempt+1,
    "max_attempts", cfg.MaxRebaseAttempts,
)

conflictInfo := fmt.Sprintf(
    "Rollup PR #%d (%s -> %s) has merge conflicts that need to be resolved.",
    prNum, cfg.BaseBranch, defaultBranch,
)

result, mcErr := mergeCoordinateFn(ctx, syntheticIssue, prNum, conflictInfo, cfg, prompts, authEnv, logger)
```

The orchestrator constructs a synthetic `github.Issue` from the rollup PR context so the merge coordinator prompt has enough information to understand both sides. After resolution, `runRollupVerifyFn` re-runs to validate the merged code. Each attempt is recorded to `rollup/merge_coordinator-<attempt>.json` via `WriteRollupMergeCoordinatorResult`.

If all attempts are exhausted:

```go
logger.Warn("rollup PR still conflicting after all merge coordinator attempts -- leaving PR open for human review",
    "pr_number", prNum,
    "max_attempts", cfg.MaxRebaseAttempts,
)
reporter.RollupCreated(prNum, prURL, false)
```

The run completes without error -- the PR exists but isn't merged, and the human can take over.

---

## Run Data and Telemetry

**What it does:** Merge coordinator results are persisted as first-class step artifacts, queryable through the run data reader and available to the dashboard and analysis pipeline.

**Example:** After a merge coordinator run on issue #340, the result lands at `~/.godark/runs/owner/repo/<timestamp>/issues/340/merge_coordinator.json`:

```json
{
  "output": "Rebased and resolved 2 conflicts in cmd/root.go and internal/config/config.go",
  "cost_usd": 0.12,
  "duration_seconds": 45,
  "peak_memory_bytes": 134217728,
  "cpu_nanoseconds": 8500000000,
  "session_id": "abc123",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "tool_trace": ["Edit cmd/root.go", "Edit internal/config/config.go", "Bash go build ./..."]
}
```

The `RunDataHook` interface in `internal/agent/runhook.go` includes the method:

```go
WriteMergeCoordinatorResult(issueNumber int, step rundata.StepResult) error
```

The reader in `internal/rundata/reader.go` loads it into `IssueDetail.MergeCoordinator` from `merge_coordinator.json`, returning a zero-value `StepResult` when the file is absent (most issues won't need conflict resolution). Rollup merge coordinator results use a separate path: `rollup/merge_coordinator-<attempt>.json` to track per-attempt results.

---

## Dashboard and TUI Integration

**What it does:** The merge coordinator appears as a visible step in the dashboard review chain timeline, positioned after the functional review and before the final merge outcome. The TUI shows a `merge-coordinate` stage transition during execution.

**Example:** In the dashboard, `buildTimeline()` in `internal/dashboard/handlers.go` adds the step conditionally:

```go
if hasStepData(issue.FunctionalReview) {
    steps = append(steps, stepToView("Functional Review", issue.FunctionalReview))
}
if hasStepData(issue.MergeCoordinator) {
    steps = append(steps, stepToView("Merge Coordinate", issue.MergeCoordinator))
}
```

When present, the "Merge Coordinate" card displays duration, cost, peak memory, and CPU time -- the same tooltip fields as every other step. When `merge_coordinator.json` is absent (the common case), no card appears.

In the TUI, the stage transition fires when the merge coordinator starts:

```go
reporter.IssueStageChanged(issue.Number, "merge-coordinate")
```

This updates the live issue row so the operator can see the system is resolving conflicts rather than waiting silently on a rebase.
