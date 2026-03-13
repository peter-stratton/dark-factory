# Phase 18: Adaptive Agent Loop

> **Goal:** The agent loop adapts to codebase drift within a run, recovers
> intelligently from stuck retries, and produces better-informed
> implementations. Issues late in a milestone execute as reliably as early ones
> because the system accounts for changes made by prior issues.

## Milestone

`Phase 18`

---

## Issue 367: Recon agent prompt template and role

### Description

Add a `recon.txt` prompt template and `recon` agent role for pre-implementation
codebase exploration. The recon agent reads the current issue and explores the
codebase to produce a free-form analysis of what the implementer will
encounter — files that need changing, current function signatures, relevant
interfaces, infrastructure available from prior issues, and potential
constraint conflicts.

The prompt should be minimal: tell the agent what to investigate, not how to
format its output. The raw natural-language output will be passed directly to
the implementer as context. No structured output format — Go-side code captures
the agent's result text as-is.

### Key constraints

- New file `prompts/recon.txt` — instructs the agent to:
  - Read the issue body for requirements
  - Search for types, functions, and packages the issue would touch
  - Read key files to understand current signatures and coupling
  - Cross-reference against `docs/architecture.json` for layer awareness
  - Note anything that differs from what the issue description assumes
- New `recon` role in `internal/agent/runner/agent_runner.py`
  `_ROLE_PERMISSIONS` (line 38):
  - `allowed_tools: [Read, Glob, Grep]`
  - `disallowed_tools: [Write, Edit, Bash]`
- Follows the `punchlist` role pattern — strictly read-only
- No structured output format in the prompt; the agent writes its findings
  naturally

### Acceptance criteria

- [ ] `prompts/recon.txt` template exists and renders with standard `PromptData`
  fields
- [ ] `recon` role defined in `_ROLE_PERMISSIONS` with
  `allowed_tools: [Read, Glob, Grep]` and
  `disallowed_tools: [Write, Edit, Bash]`
- [ ] Role permissions tested (same pattern as existing role tests)

### Test cases

- **Role permissions**: `recon` role allows Read, Glob, Grep and disallows
  Write, Edit, Bash
- **Prompt renders**: `RenderPrompt` with standard `PromptData` produces valid
  prompt text containing the issue title and body

---

## Issue 368: Recon config and prompt data wiring

**Blocked by**: #367

### Description

Add `prompts.recon` config field and wire the recon brief into the implementer
prompt. When configured, the recon agent's output is available to the
implementer as additional context via a `ReconBrief` template variable.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `Recon string` with yaml tag `recon` to the `Prompts` struct (line 206)
- Modify `internal/agent/prompt.go`:
  - Add `ReconBrief string` field to `PromptData` struct (line 27)
  - Add load logic in `LoadPrompts()` for `recon.txt` — optional pattern
    (same as `QualityReviewer`, `VerifyFix`): empty string on missing file,
    no error
- Modify `prompts/implementer.txt`:
  - Add `{{if .ReconBrief}}` conditional block that includes the recon brief
    as pre-implementation context
- `ReconBrief` is NOT set in `newPromptData()` — it's set by the caller
  (`ProcessIssue`) after running the recon agent

### Acceptance criteria

- [ ] `prompts.recon` config field parsed from `godark.yaml`
- [ ] `ReconBrief` field exists on `PromptData`
- [ ] `Prompts` struct has `Recon` field, loaded by `LoadPrompts()`
- [ ] `implementer.txt` includes `{{if .ReconBrief}}` block
- [ ] Existing behavior unchanged when `prompts.recon` is empty/unset

### Test cases

- **Config parsing**: Setting `prompts: { recon: "custom/recon.txt" }` in YAML
  is reflected in parsed config
- **Config default**: Empty `prompts.recon` produces empty `Prompts.Recon`
- **Implementer with ReconBrief**: `RenderPrompt` with `ReconBrief` set
  includes the brief text in output
- **Implementer without ReconBrief**: `RenderPrompt` with empty `ReconBrief`
  does not include the recon section

---

## Issue 369: Recon orchestrator integration

**Blocked by**: #368

### Description

Invoke the recon agent before `Implement()` in `ProcessIssue()` and pass its
raw output as implementer context. The recon agent is non-blocking — if it
fails, the implementer proceeds without the brief.

### Key constraints

- New file `internal/agent/recon.go`:
  - `Recon()` function following the `GenerateSpec()` pattern (~35 lines)
  - Signature: `func Recon(ctx context.Context, issue github.Issue,
    cfg *config.Config, prompts *Prompts, authEnv map[string]string,
    logger *slog.Logger) (*Result, error)`
  - Renders `prompts.Recon` with `newPromptData()`
  - Calls `newRunOpts(rendered, cfg, authEnv, "recon")`
  - Returns `Run(ctx, opts, cfg.NoSandbox, logger)`
- Modify `internal/agent/implementer.go`:
  - Add `reconBrief string` parameter to `Implement()` signature
  - Set `data.ReconBrief = reconBrief` after calling `newPromptData()`
  - Same pattern as `reviewFeedback` on `Retry()`
- Modify `internal/agent/loop.go` (insert before `Implement()` call at
  line 121):
  - If `prompts.Recon != ""`, call `Recon()` and capture result
  - On success: extract `result.ResultText` as the recon brief, pass to
    `Implement()`
  - On failure: log warning, pass empty string to `Implement()` (graceful
    degradation)
  - Write recon result to hook (see run data issue)
- Update all existing callers of `Implement()` to pass the new `reconBrief`
  parameter (empty string when no recon is configured)

### Acceptance criteria

- [ ] `Recon()` function exists in `recon.go` and invokes the agent with
  `recon` role
- [ ] `Implement()` accepts a `reconBrief` parameter and sets
  `data.ReconBrief`
- [ ] `ProcessIssue()` calls `Recon()` before `Implement()` when configured
- [ ] Recon output flows into `ReconBrief` on `PromptData` for the implementer
- [ ] Recon failure is non-blocking (warning logged, implementation proceeds
  with empty brief)
- [ ] No behavior change when `prompts.recon` is not configured
- [ ] Existing callers of `Implement()` updated (pass empty string)

### Test cases

- **Recon runs before implement**: When `prompts.Recon` is configured,
  `Recon()` is called before `Implement()`
- **Recon output passed to implementer**: `Implement()` receives the recon
  agent's `ResultText` as `reconBrief`
- **Recon failure non-blocking**: When `Recon()` returns an error,
  `Implement()` is still called with empty `reconBrief`
- **Recon skipped when unconfigured**: When `prompts.Recon` is empty,
  `Recon()` is not called
- **Implement signature change**: Existing callers pass empty string when no
  recon brief is available

---

## Issue 370: Recon run data and dashboard

**Blocked by**: #369

### Description

Persist the recon result to run data and surface it in the dashboard issue
detail view. The full recon brief is persisted so it can be inspected after
runs for debugging prompt quality and recon accuracy.

### Key constraints

- Modify `internal/agent/runhook.go`:
  - Add `WriteReconResult(issueNumber int, step rundata.StepResult) error`
    to `RunDataHook` interface
- Modify `internal/rundata/writer.go`:
  - Implement `WriteReconResult` — writes `recon.json` to the issue's run
    data directory (same pattern as `WriteSpecGeneratorResult`)
  - `StepResult` already carries `SessionID`, `Cost`, `Duration`,
    `Output` (the brief text), and `ToolTrace`
- Modify `internal/agent/loop.go`:
  - Call `hook.WriteReconResult()` after the recon agent completes (both
    success and failure paths)
- Modify `internal/dashboard/templates/issue-detail.html`:
  - Add recon step to the review chain timeline, positioned before the
    implement step
  - Show duration and cost in the issue summary
  - Brief text expandable (same pattern as tool traces)
  - Skip the section when no recon data exists (backwards compatible)

### Acceptance criteria

- [ ] `WriteReconResult` exists on `RunDataHook` interface
- [ ] Recon result persisted as `recon.json` in issue run data directory
- [ ] `recon.json` contains session ID, cost, duration, and brief text
- [ ] Issue detail view in dashboard shows the recon brief when present
- [ ] Recon cost and duration visible in issue summary
- [ ] Dashboard renders correctly for runs without recon data

### Test cases

- **Write recon result**: `WriteReconResult` creates `recon.json` with expected
  fields
- **Read old run data**: Loading a run directory without `recon.json` does not
  error
- **Dashboard with recon**: Issue detail page for a run with recon data shows
  the recon step in the timeline
- **Dashboard without recon**: Issue detail page for a pre-Phase 18 run renders
  without recon section

---

## Issue 371: Fresh agent with structured handoff on retry 3+

### Description

On retries beyond a configurable threshold, start a fresh agent session instead
of resuming the prior session. Pass the PR comment dialogue (Implementation
Notes / Review Notes) as structured handoff context so the fresh agent
understands what was tried and what failed without inheriting a degraded
context window.

The handoff context is assembled by `ProcessIssue()` (not `Retry()`) to keep
`Retry()` a pure render-and-invoke function — same pattern as
`reviewFeedback`.

### Key constraints

- Modify `internal/agent/prompt.go`:
  - Add `HandoffContext string` field to `PromptData` struct
- Modify `internal/agent/implementer.go`:
  - Add `handoffContext string` parameter to `Retry()` signature (after
    `reviewFeedback`)
  - Set `data.HandoffContext = handoffContext` after `newPromptData()`
  - When `handoffContext` is non-empty, do NOT set
    `opts.Env["GODARK_SESSION_ID"]` even if `prevSessionID` is provided —
    fresh session is the point
  - When `handoffContext` is empty, existing session resumption behavior is
    unchanged
- Modify `prompts/implementer_retry.txt`:
  - Add `{{if .HandoffContext}}` block that presents the handoff context
    with a preamble like: "This is a fresh session. Prior implementation
    attempts and reviewer feedback are summarized below."
- Modify `internal/agent/loop.go`:
  - Handoff assembly function: fetch PR comments via
    `gh pr view <N> --repo <repo> --comments --json comments`, extract
    `## Implementation Notes` and `## Review Notes` / `## Quality Review Notes`
    sections, format chronologically
  - In the quality review retry path (line 255) and functional review retry
    path: when `attempt >= cfg.MaxResumeRetries`, assemble handoff and pass
    to `Retry()` instead of session ID
- Update all existing callers of `Retry()` to pass the new `handoffContext`
  parameter (empty string for resume mode)

### Acceptance criteria

- [ ] `HandoffContext` field exists on `PromptData`
- [ ] `Retry()` accepts `handoffContext` parameter
- [ ] When `handoffContext` is non-empty, `GODARK_SESSION_ID` is NOT set
  (fresh session)
- [ ] When `handoffContext` is empty, existing session resumption works
  unchanged
- [ ] `implementer_retry.txt` includes conditional handoff block
- [ ] Handoff assembly extracts Implementation Notes and Review Notes from PR
  comments
- [ ] All existing callers of `Retry()` updated with new parameter

### Test cases

- **Fresh mode skips session**: `Retry()` with non-empty `handoffContext` does
  not set `GODARK_SESSION_ID` in env, even when `prevSessionID` is provided
- **Resume mode unchanged**: `Retry()` with empty `handoffContext` and
  non-empty `prevSessionID` sets `GODARK_SESSION_ID` as before
- **Handoff rendering**: `RenderPrompt` with `HandoffContext` set includes the
  handoff text and fresh-session preamble
- **Handoff not rendered**: `RenderPrompt` with empty `HandoffContext` does not
  include the handoff section
- **Handoff assembly**: Given PR comments containing Implementation Notes and
  Review Notes sections, assembly function extracts and formats them
  chronologically
- **Handoff assembly empty**: Given PR with no structured comments, assembly
  returns empty string

---

## Issue 372: Hybrid retry config — max_resume_retries

**Blocked by**: #371

### Description

Add `max_resume_retries` config field that controls when retries switch from
session resumption to fresh agent with structured handoff.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `MaxResumeRetries int` with yaml tag `max_resume_retries` to
    `Config` struct
  - Default value: 2 (set in `applyDefaults()` or equivalent)
  - Value of 0 means all retries use fresh mode
  - Value >= `MaxRetries` means all retries use resume mode (preserves
    current behavior)
- Modify `internal/agent/loop.go`:
  - In the quality review retry path (around line 255): check
    `qAttempt >= cfg.MaxResumeRetries` to decide resume vs. fresh
  - In the functional review retry path (around line 328): same check with
    `attempt`
  - Pass empty `handoffContext` when resuming, assembled handoff when fresh

### Acceptance criteria

- [ ] `MaxResumeRetries` field parsed from `godark.yaml` with default value
  of 2
- [ ] Setting `max_resume_retries: 0` in YAML forces all retries to use fresh
  mode
- [ ] Setting `max_resume_retries` >= `max_retries` preserves current
  resume-always behavior
- [ ] Quality review retry loop uses config value for resume vs. fresh decision
- [ ] Functional review retry loop uses config value for resume vs. fresh
  decision

### Test cases

- **Config default**: New config has `MaxResumeRetries` of 2
- **Config override**: Setting `max_resume_retries: 0` in YAML is reflected in
  parsed config
- **Resume on attempt 1**: With `MaxResumeRetries: 2`, attempt 0 uses session
  resumption
- **Fresh on attempt 3**: With `MaxResumeRetries: 2`, attempt 2 uses fresh
  agent with handoff
- **All fresh**: With `MaxResumeRetries: 0`, all retry attempts use fresh mode
- **All resume**: With `MaxResumeRetries: 10` and `MaxRetries: 3`, all retry
  attempts use session resumption
