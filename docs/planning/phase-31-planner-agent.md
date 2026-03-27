# Phase 31: Planner Agent

> **Goal:** A new agent sits between recon and implementation, producing a
> design document and task breakdown before any code is written. The
> implementer receives both the recon brief (what code exists) and the plan
> (how to implement), reducing wasted iterations from wrong approaches.

## Milestone

`Phase 31: Planner Agent`

---

## Issue: Planner agent function and prompt template

### Description

Create the planner agent following the existing agent function pattern. The
planner reads the issue, recon brief, architecture doc, and conventions doc,
then produces a structured brief with approach, key decisions, task breakdown,
and risk flags. It has read-only permissions and does not modify code.

### Key constraints

- Create `internal/agent/planner.go` with:
  ```go
  func Plan(ctx context.Context, issue github.Issue, cfg *config.Config,
      prompts *Prompts, authEnv map[string]string, reconBrief string,
      logger *slog.Logger) (*Result, error)
  ```
  - Follow the pattern in `recon.go`: slugify, newPromptData, set
    `data.ReconBrief = reconBrief`, render prompt, newRunOpts with role
    `"planner"`, call `Run()`
- Create `prompts/planner.txt` with template variables:
  - `{{.IssueNumber}}`, `{{.IssueTitle}}`, `{{.IssueBody}}`
  - `{{.ReconBrief}}` — the recon output (may be empty if recon was skipped)
  - `{{.ArchitectureDocContent}}`, `{{.ArchitectureJSON}}`,
    `{{.ConventionsDocContent}}`
  - `{{.Repo}}`, `{{.BaseBranch}}`
- Prompt instructs the agent to produce a brief with these markdown sections:
  `## Approach`, `## Key Decisions`, `## Task Breakdown`, `## Risk Flags`
- Prompt must explicitly state: "Do NOT modify any files. Do NOT run git
  commands. Your output IS the plan — write it to stdout."
- Add `Planner string` field to `Prompts` struct in
  `internal/agent/prompt.go` (after `Recon`)
- Add `prompts.planner` to the config loader in `internal/agent/prompt.go`
  `LoadPrompts()` function
- Add `planner.txt` to the embedded prompt files in `prompts/embed.go`
- Add `planner.txt` to the `harnessPromptFiles` list in
  `internal/cmd/scaffold.go` so `godark init` installs it

### Acceptance criteria

- [ ] `internal/agent/planner.go` exists with `Plan()` function
- [ ] `prompts/planner.txt` exists with required template variables
- [ ] `Planner` field exists on `Prompts` struct
- [ ] `LoadPrompts()` loads the planner prompt
- [ ] `godark init` installs `planner.txt` to the project's prompts directory
- [ ] `go build ./...` passes
- [ ] `go test ./internal/agent/...` passes

### Test cases

- **Plan renders prompt**: Call `Plan()` with stubbed `SandboxRunner` — verify
  the rendered prompt contains the issue title and recon brief
- **Plan returns result**: Stubbed runner returns structured brief — verify
  `Result.ResultText` contains the brief
- **Plan with empty recon**: Call `Plan()` with empty `reconBrief` — verify
  prompt still renders without error
- **LoadPrompts includes planner**: Load prompts from a directory containing
  `planner.txt` — verify `Prompts.Planner` is non-empty

---

## Issue: Wire planner into ProcessIssue pipeline

**Blocked by**: Planner agent function and prompt template

### Description

Insert the planner step between recon and implementation in `ProcessIssue()`.
The planner is non-blocking — if it fails or times out, the implementer
proceeds without a plan (same pattern as recon). The planner brief is passed
to the implementer via a new `PlannerBrief` template variable.

### Key constraints

- In `internal/agent/loop.go` `ProcessIssue()`:
  - After the recon block (line ~140) and before the implement call (line ~146),
    add a planner block:
    ```go
    var plannerBrief string
    if prompts.Planner != "" {
        reporter.IssueStageChanged(issue.Number, "plan")
        planResult, planErr := Plan(ctx, issue, cfg, prompts, authEnv, reconBrief, logger)
        plannerBrief = handleNonBlockingResult(planResult, planErr, "planner", logger,
            func(step rundata.StepResult) error {
                if hook != nil { return hook.WritePlannerResult(issue.Number, step) }
                return nil
            })
    }
    ```
  - Pass `plannerBrief` to `Implement()` — add parameter to `Implement()`
    signature
- In `internal/agent/implementer.go`:
  - Add `plannerBrief string` parameter to `Implement()` function signature
  - Set `data.PlannerBrief = plannerBrief` before rendering the prompt
- In `internal/agent/prompt.go`:
  - Add `PlannerBrief string` field to `PromptData` struct
- In `prompts/implementer.txt`:
  - Add a conditional section that injects the planner brief when non-empty:
    `{{if .PlannerBrief}}## Plan\n\n{{.PlannerBrief}}{{end}}`
  - Place it after the recon brief section

### Acceptance criteria

- [ ] Planner runs between recon and implement when `prompts.Planner` is set
- [ ] Planner is skipped when `prompts.Planner` is empty
- [ ] Planner failure logs a warning and implementation proceeds
- [ ] Planner brief appears in the implementer prompt when available
- [ ] `go build ./...` passes
- [ ] `go test ./internal/agent/...` passes

### Test cases

- **Planner runs when configured**: Set `prompts.Planner` to a valid template,
  stub runner to return a brief — verify `Plan()` was called and brief appears
  in implementer prompt
- **Planner skipped when empty**: Set `prompts.Planner` to "" — verify no
  planner stage reported, implementer runs directly after recon
- **Planner failure non-blocking**: Stub runner to return error for planner
  role — verify warning logged and implementer still runs with empty
  PlannerBrief
- **Planner timeout non-blocking**: Stub runner to return timed-out result —
  verify warning logged and implementer still runs

---

## Issue: Planner run data, dashboard timeline, and TUI stage

**Blocked by**: Wire planner into ProcessIssue pipeline

### Description

Add planner as a visible step in the run data, dashboard timeline, and TUI
stage transitions. The planner result is persisted to `planner.json` per
issue and appears in the dashboard issue detail timeline between Recon and
Implement.

### Key constraints

- In `internal/agent/runhook.go`:
  - Add `WritePlannerResult(issueNumber int, step rundata.StepResult) error`
    to the `RunDataHook` interface
- In `internal/rundata/writer.go`:
  - Add `WritePlannerResult()` method to `Writer` — writes to
    `issues/<issueNum>/planner.json`
- In `internal/rundata/reader.go`:
  - Add `Planner StepResult` field to `IssueDetail` struct
  - Load from `planner.json` in `loadIssueDetail()`
- In `internal/dashboard/handlers.go`:
  - Add planner step to `buildTimeline()` — insert between Recon and
    Implement with label "Planner"
- In `internal/tui/reporter.go`:
  - The stage `"plan"` is already reported by the pipeline wiring issue —
    no additional changes needed unless stage display formatting is required
- In `internal/agent/loop_test.go`:
  - Add `WritePlannerResult` to `testRunDataHook` struct

### Acceptance criteria

- [ ] `WritePlannerResult` exists on `RunDataHook` interface
- [ ] `planner.json` written to run data when planner runs
- [ ] Dashboard timeline shows "Planner" step between Recon and Implement
- [ ] TUI displays "plan" stage during planner execution
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

### Test cases

- **Run data written**: Process an issue with planner enabled — verify
  `planner.json` exists in the issue's run data directory
- **Dashboard timeline order**: Load an issue detail with planner data —
  verify timeline order is Spec Generator, Recon, Planner, Implement, ...
- **Dashboard planner absent**: Load an issue detail without planner data —
  verify timeline skips Planner step (no gap)
- **Hook called**: Process an issue with testRunDataHook — verify
  `WritePlannerResult` was called with the correct issue number

---

## Integration chain audit

```
Prompts.Planner defined in prompt.go
  → loaded by LoadPrompts() in prompt.go                    ← Issue 1
  → checked by ProcessIssue() in loop.go                    ← Issue 2
  → rendered by Plan() in planner.go                        ← Issue 1

PromptData.PlannerBrief defined in prompt.go
  → set by ProcessIssue() in loop.go                        ← Issue 2
  → set by Implement() in implementer.go                    ← Issue 2
  → used by implementer.txt template                        ← Issue 2

Plan() defined in planner.go
  → called by ProcessIssue() in loop.go                     ← Issue 2
  → result handled by handleNonBlockingResult() in loop.go  ← Issue 2 (existing fn)

WritePlannerResult defined in runhook.go
  → implemented by Writer in writer.go                      ← Issue 3
  → called by ProcessIssue() in loop.go                     ← Issue 2
  → stubbed in testRunDataHook in loop_test.go              ← Issue 3

IssueDetail.Planner defined in reader.go
  → loaded by loadIssueDetail() in reader.go                ← Issue 3
  → consumed by buildTimeline() in handlers.go              ← Issue 3

Stage "plan" string
  → reported by ProcessIssue() in loop.go                   ← Issue 2
  → received by TUI via IssueStageChangedMsg                ← Issue 3 (existing msg type)
```

All hops covered. No gaps.
