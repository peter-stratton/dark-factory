# Phase 18: Adaptive Agent Loop

Before Phase 18, agents went into each issue cold -- no awareness of the codebase beyond what the issue description and scenario specs provided. If a retry loop exhausted its resume context, the agent kept hammering the same approach. Phase 18 adds a recon step that scouts the codebase before implementation begins, and a hybrid retry strategy that switches from session resumption to fresh-start-with-handoff when retries stall. Issues late in a milestone now execute as reliably as early ones because the system accounts for codebase reality and adapts when stuck.

---

## Recon Agent

**What it does:** Before the implementer starts work, an optional recon agent explores the codebase and produces a free-form analysis. The implementer receives this analysis as structured context in its prompt, eliminating the cold-start discovery phase that previously consumed tokens and sometimes led agents down wrong paths.

**Example:** A team is running Phase 19 of their project. Issue #389 asks to "extract a unified `ParseVerdict` from `ParseReviewResult` and `ParseQualityResult`." The recon agent reads the issue, then explores the codebase:

```
INFO starting recon agent  issue_number=389 issue_title="Extract unified ParseVerdict"
```

The recon prompt (`prompts/recon.txt`) instructs the agent to answer specific questions: which files need to change, what are the current function signatures, which architecture layers are involved, what utilities already exist, and whether anything in the codebase differs from what the issue assumes. The agent's response is free-form -- no structured output format, just specific findings with file paths and quoted code.

The recon brief is injected into the implementer prompt via the `{{.ReconBrief}}` template variable:

```
## Pre-implementation context (recon brief)

The following is a summary of relevant files, signatures, and architectural
notes gathered by the recon agent before implementation. Use it as context
when reading code and planning your approach.

<recon agent output here>
```

Recon is non-blocking. If the recon agent fails or times out, the implementer proceeds without it:

```go
reconBrief = handleNonBlockingResult(reconResult, reconErr, "recon agent", logger, reconWriteHook)
```

The recon step is configured via `prompts.recon` in `godark.yaml`. If the field is empty, recon is skipped entirely.

---

## Recon Role Permissions

**What it does:** The recon agent runs with read-only permissions -- `Read`, `Glob`, and `Grep` only. It cannot modify files, run shell commands, or create branches. This keeps the codebase in a known state before implementation begins.

**Example:** The `Recon()` function in `internal/agent/recon.go` constructs run options with the `"recon"` role:

```go
opts, err := newRunOpts(rendered, cfg, authEnv, "recon")
```

The role maps to `allowed_tools=["Read", "Glob", "Grep"]` in the agent runner. If the recon agent attempts to write a file or run a bash command, the tool call is rejected with a system message explaining why. This is the same permission model used by the reviewer role, applied to a different purpose.

---

## Recon Run Data

**What it does:** Recon results are persisted to run data alongside implementation, review, and verify steps. Cost, duration, session ID, and the full recon brief are recorded for every issue that runs recon.

**Example:** After recon completes for issue #389, the orchestrator writes:

```
~/.godark/runs/owner/repo/2026-03-10T14:30:00Z/issues/389/recon.json
```

The file contains a `StepResult` with timing, cost, and output:

```json
{
  "output": "Files to change: internal/agent/verdict.go ...",
  "started_at": "2026-03-10T14:30:05Z",
  "finished_at": "2026-03-10T14:30:42Z",
  "duration_seconds": 37.2,
  "cost_usd": 0.08,
  "session_id": "sess_abc123"
}
```

Recon cost is included in the per-issue cost total shown in the dashboard run detail view. The `buildTimeline()` function in `internal/dashboard/handlers.go` includes the recon step in the issue detail timeline when data exists:

```go
if hasStepData(issue.Recon) {
    steps = append(steps, stepToView("Recon", issue.Recon))
}
```

---

## Hybrid Retry Strategy

**What it does:** When the implementer fails review and retries, the system decides between two strategies: resume the prior session (preserving full context) or start a fresh session with a structured handoff document. Early retries resume. Later retries start fresh. The threshold is configurable.

**Example:** A team has `max_resume_retries: 2` in their `godark.yaml` (the default). Issue #204 fails its first quality review:

- **Retry 1** (attempt 0): Resume the prior session. The implementer picks up where it left off, reads the reviewer's feedback, and pushes a fix. The agent remembers its reasoning, which files it changed, and why.
- **Retry 2** (attempt 1): Resume again. Same approach -- the agent has been working on this continuously.
- **Retry 3** (attempt 2): `shouldHandoff(2, 2)` returns `true`. The system starts a fresh session instead of resuming.

The decision is a single function in `internal/agent/loop.go`:

```go
func shouldHandoff(attempt int, maxResumeRetries int) bool {
    return attempt >= maxResumeRetries
}
```

When handoff triggers, `assembleHandoffContext()` extracts structured dialogue from the PR comments -- Implementation Notes, Review Notes, and Quality Review Notes -- and joins them chronologically:

```go
headings := []string{
    "## Implementation Notes",
    "## Review Notes",
    "## Quality Review Notes",
}
```

The fresh implementer receives this handoff in its retry prompt with an explicit instruction:

```
This is a fresh session. Prior implementation attempts and reviewer feedback
are summarized below. Do NOT resume a prior session -- start from scratch
using this context to understand what was tried and what failed.
```

The key insight is that a stuck agent often needs a different approach, not more context in the same thread. Fresh sessions can re-evaluate the problem from scratch while still knowing what was already tried and why it was rejected.

---

## Handoff Context Assembly

**What it does:** When a fresh session is triggered, `assembleHandoffContext()` fetches the PR's body and all comments, then extracts the structured dialogue sections that agents post during implementation and review. This gives the fresh agent a complete history of what was attempted and why it failed.

**Example:** After two failed review cycles on PR #847, the PR has accumulated three structured comments:

1. Implementer's `## Implementation Notes` -- what approach was taken and why
2. Quality reviewer's `## Quality Review Notes` -- what issues were found
3. Implementer's second `## Implementation Notes` -- what was fixed

`assembleHandoffContext()` calls `gh pr view 847 --json body,comments`, walks through all bodies in chronological order, and extracts sections matching the known headings. The result is a multi-section string joined with `---` separators:

```
## Implementation Notes
Extracted the ParseVerdict function into internal/agent/verdict.go...

---

## Quality Review Notes
The extraction is incomplete — ParseReviewResult still has inline parsing...

---

## Implementation Notes
Moved all remaining inline parsing to use the unified ParseVerdict...
```

If the PR has no structured comments or the `gh` call fails, `assembleHandoffContext()` returns an empty string and the retry proceeds without handoff context. Best-effort, never blocking.
