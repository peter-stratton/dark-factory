# Phase 31: Planner Agent

Before this phase, the implementer agent jumped straight from reconnaissance to writing code. The recon brief told it what code exists, but not how to approach the implementation. Phase 31 inserts a planner agent between recon and implementation that produces a structured brief - approach, key decisions, task breakdown, and risk flags - so the implementer starts with a strategy rather than improvising one. The planner is non-blocking: if it fails or times out, implementation proceeds without a plan, the same pattern used for recon and spec generation.

---

## Planner Agent Function and Prompt

**What it does:** The planner agent reads the issue, recon brief, architecture documentation, and coding conventions, then outputs a structured implementation plan. It has read-only permissions - it reasons about code but does not modify it.

**Example:** The `Plan()` function in `internal/agent/planner.go` follows the standard agent pattern:

```go
func Plan(ctx context.Context, issue github.Issue, cfg *config.Config,
    prompts *Prompts, authEnv map[string]string, logger *slog.Logger,
    reconBrief string) (*Result, error) {
    slug := Slugify(issue.Title)
    data := newPromptData(issue, cfg, slug)
    data.ReconBrief = reconBrief

    rendered, err := RenderPrompt(prompts.Planner, data)
    if err != nil {
        return nil, fmt.Errorf("rendering planner prompt: %w", err)
    }

    opts, err := newRunOpts(rendered, cfg, authEnv, "planner")
    if err != nil {
        return nil, err
    }

    return Run(ctx, opts, logger)
}
```

The prompt at `prompts/planner.txt` receives the full project context - issue body, recon brief, architecture doc, architecture graph, and conventions doc - then instructs the agent to produce four sections:

```
## Approach
Describe the overall implementation strategy. Which existing patterns to follow,
what the main moving parts are, and how they connect.

## Key Decisions
List any non-obvious design choices, trade-offs, or alternatives considered.
Explain why the chosen approach is preferred.

## Task Breakdown
Provide an ordered list of implementation steps. Each step should name the file(s)
to create or modify and briefly describe the change.

## Risk Flags
Identify anything that could go wrong: tricky edge cases, potential breaking
changes, unclear requirements, or areas that need extra test coverage.
```

The prompt explicitly states: "Do NOT modify any files. Do NOT run git commands. Your output IS the plan." The `planner` role is configured with read-only permissions (`Read`, `Glob`, `Grep`) - no `Edit`, `Write`, or `Bash`.

---

## Pipeline Integration

**What it does:** The planner runs as a non-blocking step in `ProcessIssue()` between recon and implementation. If it fails or times out, a warning is logged and the implementer proceeds without a plan. The planner brief is passed to the implementer via the `PlannerBrief` template variable.

**Example:** In `ProcessIssue()` in `internal/agent/loop.go`, the planner block sits between recon and implement:

```go
// Optional planner step: produce a structured plan before implementation.
plannerBrief := ""
if prompts.Planner != "" {
    if reporter != nil {
        reporter.IssueStageChanged(issue.Number, "plan")
    }
    planResult, planErr := Plan(ctx, issue, cfg, prompts, authEnv, logger, reconBrief)
    if planResult != nil {
        if handleJudgeIntervention(issue.Number, "plan", planResult, hook, reporter, logger) {
            runPostMortem(issue.Number, planResult, hook, logger)
        }
    }
    var planWriteHook func(rundata.StepResult) error
    if hook != nil {
        planWriteHook = func(step rundata.StepResult) error {
            step.TraceID = traceID
            return hook.WritePlannerResult(issue.Number, step)
        }
    }
    plannerBrief = handleNonBlockingResult(planResult, planErr, "planner agent", logger, planWriteHook)
}
```

The `handleNonBlockingResult` function handles the three-way dispatch: on error it logs a warning and returns empty string, on timeout it logs and returns empty, on success it returns the `ResultText`. This is the same pattern used for recon and spec generation.

The planner is skipped entirely when `prompts.Planner` is empty - no stage change, no agent call, no run data. The planner brief then flows directly into the implementer:

```go
implResult, err := Implement(ctx, issue, cfg, prompts, authEnv, logger, reconBrief, plannerBrief)
```

---

## Implementer Prompt Injection

**What it does:** When the planner produces a brief, it's injected into the implementer prompt as a `## Plan` section. The implementer is instructed to follow the approach, task breakdown, and key decisions unless it discovers information that invalidates them.

**Example:** The `Implement()` function in `internal/agent/implementer.go` sets the brief on the prompt data:

```go
func Implement(ctx context.Context, issue github.Issue, cfg *config.Config,
    prompts *Prompts, authEnv map[string]string, logger *slog.Logger,
    reconBrief string, plannerBrief string) (*Result, error) {
    slug := Slugify(issue.Title)
    data := newPromptData(issue, cfg, slug)
    data.ReconBrief = reconBrief
    data.PlannerBrief = plannerBrief
    // ...
}
```

The `PlannerBrief` field on `PromptData` in `internal/agent/prompt.go`:

```go
// PlannerBrief holds the output from the planner agent. When non-empty
// it is injected into the implementer prompt as a structured plan.
PlannerBrief string
```

In `prompts/implementer.txt`, the plan section is conditional:

```
{{- if .PlannerBrief}}

## Plan

The following plan was produced by the planner agent. Use it to guide your
implementation - follow the approach, task breakdown, and key decisions unless
you discover information that invalidates them.

{{.PlannerBrief}}
{{- end}}
```

When the planner didn't run or failed, `PlannerBrief` is empty and this section is omitted from the rendered prompt entirely - no wasted context.

---

## Config and Prompt Loading

**What it does:** The planner prompt is loaded from a configurable path in `godark.yaml` with fallback to the embedded default, following the same optional-prompt pattern as recon and merge coordinator.

**Example:** The project's `godark.yaml` uses a cheaper model for the planner since it only needs to reason, not write code:

```yaml
model: opus
model_overrides:
  recon: sonnet
  spec_generator: sonnet
  planner: sonnet

prompts:
  planner: prompts/planner.txt
```

The config `Prompts` struct in `internal/config/config.go`:

```go
Planner string `yaml:"planner"`
```

Loading in `LoadPrompts()` in `internal/agent/prompt.go`:

```go
planner, err := loadPromptFile(cfg.Prompts.Planner, "planner.txt")
```

The prompt is also registered in `harnessPromptFiles` in `internal/cmd/scaffold.go`:

```go
{"planner.txt", "prompts/planner.txt"},
```

This means `godark init` and `godark new` install the planner prompt template into new projects automatically.

---

## Run Data and Dashboard Timeline

**What it does:** The planner result is persisted to `planner.json` per issue and appears as a "Planner" step in the dashboard issue detail timeline, positioned between Recon and Implement.

**Example:** The `RunDataHook` interface in `internal/agent/runhook.go` includes:

```go
WritePlannerResult(issueNumber int, step rundata.StepResult) error
```

The writer persists to `issues/<issueNum>/planner.json`:

```go
func (w *Writer) WritePlannerResult(issueNum int, step StepResult) error {
    path := filepath.Join(w.issueDir(issueNum), "planner.json")
    return writeJSONMkdirs(path, step)
}
```

The reader loads it into `IssueDetail.Planner` in `internal/rundata/reader.go`:

```go
Planner: r.readStep(filepath.Join(issueDir, "planner.json")),
```

In the dashboard, `buildTimeline()` in `internal/dashboard/handlers.go` inserts the planner between recon and implement:

```go
if hasStepData(issue.Recon) {
    steps = append(steps, stepToView("Recon", issue.Recon))
}
if hasStepData(issue.Planner) {
    steps = append(steps, stepToView("Planner", issue.Planner))
}
if hasStepData(issue.Implement) {
    steps = append(steps, stepToView("Implement", issue.Implement))
}
```

When the planner didn't run (no `planner.json`), the timeline skips straight from Recon to Implement with no gap.

---

## TUI Stage Transitions

**What it does:** The TUI displays a `plan` stage during planner execution, giving operators visibility into where in the pipeline each issue currently sits.

**Example:** The full stage progression with the planner is:

```
recon -> plan -> implement -> verify -> review -> merge-coordinate -> merged
```

The stage is emitted in `ProcessIssue()` before calling `Plan()`:

```go
if reporter != nil {
    reporter.IssueStageChanged(issue.Number, "plan")
}
```

In the TUI, issue #42 might show:

```
#42  Add rate limiting to API endpoints    [plan]
```

This transitions to `[implement]` once the planner completes and the implementer starts. If the planner is not configured (empty prompt), the stage goes directly from `[recon]` to `[implement]`.
