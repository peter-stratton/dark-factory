# Phase 33: Semi-Structured Review

> **Goal:** The functional reviewer produces auditable, structured reasoning
> (premises -> traces -> conclusion) that is machine-verifiable for consistency,
> reducing false approvals on subtle bugs.

## Milestone

`Phase 33: Semi-Structured Review`

---

## Issue 728: Semi-formal reviewer prompt

### Description

Create a standalone prompt template for the semi-formal functional reviewer.
This is a full replacement for `reviewer.txt` that adds structured reasoning
sections (PREMISES, ACCEPTANCE TRACE, REGRESSION TRACE, UNCOVERED PATHS,
FORMAL CONCLUSION) before the verdict. The reviewer must derive its verdict
from the formal conclusion — it cannot print APPROVED if any acceptance
criterion is NOT SATISFIED or any regression test is BROKEN.

### Key constraints

- Create `prompts/reviewer_semiformal.txt` as a standalone file (not a
  partial or include — full prompt, self-contained)
- Start from the current `reviewer.txt` (99 lines) as the base, preserving
  all existing steps (checkout, read spec, read implementation notes, review
  diff, architecture check, run tests, write integration tests)
- Insert the semi-formal analysis block between "run integration tests" and
  "post verdict." The block requires these sections in order:
  - **PREMISES** — for each changed file, state what the change does (one
    sentence per premise, labeled P1, P2, ...)
  - **ACCEPTANCE TRACE** — for each acceptance criterion in the issue body or
    scenario spec, trace which premises satisfy it, state SATISFIED / NOT
    SATISFIED / UNTESTABLE, cite the test that exercises it
  - **REGRESSION TRACE** — for each existing test touching modified code,
    state prior behavior, new behavior, and PRESERVED / CHANGED (justified) /
    BROKEN
  - **UNCOVERED PATHS** — code paths introduced by the patch not exercised by
    any trace, with HIGH / MEDIUM / LOW risk rating
  - **FORMAL CONCLUSION** — derive verdict from traces: any NOT SATISFIED or
    BROKEN or HIGH-risk uncovered path means CHANGES_REQUESTED
- The prompt must state: "Your AGENT_RESULT line MUST match your FORMAL
  CONCLUSION. If any AC is NOT SATISFIED, any RT is BROKEN, or any uncovered
  path is HIGH risk, you MUST print AGENT_RESULT=CHANGES_REQUESTED."
- Use the same template variables as `reviewer.txt` — no new PromptData
  fields needed
- The PR comment format adds `### Semi-Formal Analysis` above the existing
  `### Architecture Compliance` section, containing the full structured
  analysis
- The `prompts/` directory uses `//go:embed *.txt` in `prompts/embed.go`, so
  the new file is automatically embedded — no changes to `embed.go` needed
- Add `{"reviewer_semiformal.txt", "prompts/reviewer_semiformal.txt"}` to the
  `harnessPromptFiles` list in `internal/cmd/scaffold.go` so both `godark
  init` and `godark new` install it
- Add `reviewer_semiformal: prompts/reviewer_semiformal.txt` to the
  `configTail` constant in `internal/cmd/init.go` (in the `prompts:` section)
  so new godark.yaml files include the path

### Acceptance criteria

- [ ] `prompts/reviewer_semiformal.txt` exists and is a valid Go template
      using the same variables as `reviewer.txt`
- [ ] Prompt contains PREMISES, ACCEPTANCE TRACE, REGRESSION TRACE, UNCOVERED
      PATHS, and FORMAL CONCLUSION sections with the format described above
- [ ] `harnessPromptFiles` in `scaffold.go` includes the new prompt
- [ ] `configTail` in `init.go` includes `reviewer_semiformal:` in the
      prompts section
- [ ] `go build ./...` passes

### Test cases

- **Template renders**: Render `reviewer_semiformal.txt` with a populated
  PromptData — verify output contains all five semi-formal section headers
- **Template renders without spec**: Render with `HasScenarioSpec=false` —
  verify conditional sections (integration test steps) are absent but
  semi-formal block is still present
- **Scaffold installs prompt**: Call `writeHarnessPrompts` — verify
  `reviewer_semiformal.txt` is written to the destination directory

---

## Issue 729: Config and prompt selection

**Blocked by**: #728

### Description

Add configuration to toggle semi-formal review and wire prompt selection into
the orchestrator. When enabled, the functional reviewer receives the
semi-formal prompt instead of the standard one. A separate flag enables
semi-formal only on retry cycles (where structured reasoning has the highest
payoff for token cost).

### Key constraints

- Add a `Review` struct to `internal/config/config.go`:
  ```go
  type Review struct {
      Semiformal        bool `yaml:"semiformal"`
      SemiformalOnRetry bool `yaml:"semiformal_on_retry"`
  }
  ```
  Add `Review Review `yaml:"review"`` field to the `Config` struct.
- Add `ReviewerSemiformal string` field to the `Prompts` struct in
  `internal/agent/prompt.go` (after `Reviewer`)
- Add loading in `LoadPrompts()` following the existing pattern — load from
  `cfg.Prompts.ReviewerSemiformal` with fallback to embedded
  `reviewer_semiformal.txt`
- Add `ReviewerSemiformal string `yaml:"reviewer_semiformal"`` to the config
  `Prompts` struct in `internal/config/config.go`
- In `internal/agent/loop.go`, in `runFunctionalReviewCycle`, select the
  prompt before calling `Review()`:
  ```go
  reviewerPrompt := prompts.Reviewer
  if cfg.Review.Semiformal || (cfg.Review.SemiformalOnRetry && attempt > 0) {
      reviewerPrompt = prompts.ReviewerSemiformal
  }
  ```
  Then pass `reviewerPrompt` to `Review()` instead of relying on
  `prompts.Reviewer` inside the function.
- Modify `Review()` in `internal/agent/reviewer.go` to accept the prompt
  string as a parameter instead of reading `prompts.Reviewer` directly. This
  is a signature change — update the one call site in `loop.go`.
- Both config fields default to `false` — no behavior change unless opted in
- Do NOT add the `review:` section to `configTail` in `init.go` — these are
  opt-in fields that users add when ready. The default godark.yaml stays
  unchanged.

### Acceptance criteria

- [ ] `Review` struct exists in config with `Semiformal` and
      `SemiformalOnRetry` fields
- [ ] `ReviewerSemiformal` field exists on both config `Prompts` and agent
      `Prompts` structs
- [ ] `LoadPrompts()` loads the semiformal reviewer prompt
- [ ] `Review()` accepts a prompt string parameter
- [ ] `runFunctionalReviewCycle` selects semiformal prompt when
      `cfg.Review.Semiformal` is true
- [ ] `runFunctionalReviewCycle` selects semiformal prompt on retry when
      `cfg.Review.SemiformalOnRetry` is true and `attempt > 0`
- [ ] Standard reviewer prompt is used when both config fields are false
- [ ] `go build ./...` passes
- [ ] `go test ./internal/agent/...` passes

### Test cases

- **Config unmarshals review section**: Unmarshal YAML with `review:
  semiformal: true` — verify `cfg.Review.Semiformal` is true
- **Config defaults to false**: Unmarshal YAML without `review:` section —
  verify both fields are false
- **LoadPrompts loads semiformal**: Load prompts from directory containing
  `reviewer_semiformal.txt` — verify `Prompts.ReviewerSemiformal` is non-empty
- **Review accepts prompt param**: Call `Review()` with a custom prompt string
  — verify the rendered prompt uses the provided string, not the default
- **Prompt selection semiformal enabled**: With `Semiformal=true`, verify the
  semiformal prompt is selected on attempt 0
- **Prompt selection semiformal on retry**: With `SemiformalOnRetry=true`,
  verify standard prompt on attempt 0 and semiformal on attempt 1

---

## Issue 731: Consistency quality gate and wiring

**Blocked by**: #729

### Description

Add a quality check that parses the reviewer's semi-formal analysis from
stdout and flags when the verdict contradicts the stated traces. Wire it into
`computeReviewFlags()` so inconsistencies trigger automatic re-run, following
the same pattern as `no_review_tests_written`.

### Key constraints

- Add `CheckSemiformalConsistency(output string) *Flag` to
  `internal/quality/quality.go`, following the existing check function pattern
  (returns `*Flag` or nil)
- Use string matching, not section parsing:
  1. Check if output contains "FORMAL CONCLUSION" — if not, return nil (not a
     semiformal review, skip the check)
  2. Scan ACCEPTANCE TRACE section for "NOT SATISFIED"
  3. Scan REGRESSION TRACE section for ": BROKEN"
  4. Scan UNCOVERED PATHS section for "Risk: HIGH"
  5. Check if output contains `AGENT_RESULT=APPROVED`
  6. If any of (2), (3), or (4) is true AND (5) is true, return a flag with
     code `"semiformal_inconsistency"` and a message citing what was found
- In `computeReviewFlags()` in `loop.go`, call
  `CheckSemiformalConsistency(result.Output)` and append the flag if non-nil.
  Only call this when the semiformal prompt was used (pass a bool parameter
  or check based on config).
- In `runFunctionalReviewCycle`, after computing flags, check for
  `semiformal_inconsistency` using the existing `hasQualityFlag()` helper.
  When found: log a warning, delete the stale review comment, and `continue`
  the retry loop (same pattern as `no_review_tests_written` at lines 770-780
  of loop.go).
- Flag code: `"semiformal_inconsistency"`
- Flag message should cite what was found, e.g.: "verdict APPROVED but
  acceptance trace contains NOT SATISFIED"

### Acceptance criteria

- [ ] `CheckSemiformalConsistency` exists in `internal/quality/quality.go`
- [ ] Returns nil when output has no FORMAL CONCLUSION section
- [ ] Returns a flag when APPROVED verdict contradicts NOT SATISFIED trace
- [ ] Returns a flag when APPROVED verdict contradicts BROKEN regression
- [ ] Returns a flag when APPROVED verdict contradicts HIGH-risk uncovered path
- [ ] Returns nil when verdict is CHANGES_REQUESTED (no contradiction)
- [ ] Returns nil when APPROVED and no contradictions exist
- [ ] `computeReviewFlags` calls the new check for semiformal reviews
- [ ] `runFunctionalReviewCycle` triggers re-run on `semiformal_inconsistency`
- [ ] `go build ./...` passes
- [ ] `go test ./internal/quality/...` passes
- [ ] `go test ./internal/agent/...` passes

### Test cases

- **No formal conclusion**: Output without FORMAL CONCLUSION — returns nil
- **Clean approval**: Output with all SATISFIED, no BROKEN, no HIGH risk, and
  APPROVED — returns nil
- **NOT SATISFIED with APPROVED**: Output with "AC1: ... Verdict: NOT
  SATISFIED" and AGENT_RESULT=APPROVED — returns flag with code
  `semiformal_inconsistency`
- **BROKEN with APPROVED**: Output with "Status: BROKEN" and
  AGENT_RESULT=APPROVED — returns flag
- **HIGH risk with APPROVED**: Output with "Risk: HIGH" and
  AGENT_RESULT=APPROVED — returns flag
- **NOT SATISFIED with CHANGES_REQUESTED**: Output with NOT SATISFIED and
  AGENT_RESULT=CHANGES_REQUESTED — returns nil (no contradiction)
- **Re-run triggered**: Mock a review result with `semiformal_inconsistency`
  flag — verify the retry loop continues (does not return approved)

---

## Issue 730: Dashboard render semi-formal analysis

**Blocked by**: #728

### Description

The review chain view in the dashboard already renders the full reviewer
output as markdown. No special parsing or formatting is needed — the
semi-formal sections (PREMISES, ACCEPTANCE TRACE, etc.) will render naturally
as markdown headers and lists in the existing output display. This issue
verifies the rendering works correctly and adds a test case for it.

### Key constraints

- No changes to `internal/dashboard/handlers.go` or
  `internal/dashboard/templates/issue-detail.html` — the existing
  `stepToView` function at line 654 already captures full output and the
  template renders it as-is
- Add a test in `internal/dashboard/handlers_review_chain_test.go` that
  verifies semi-formal output renders without errors in the review chain
  template
- The test should use a `StepResult` with `Output` containing a realistic
  semi-formal analysis (all five sections with sample traces and a FORMAL
  CONCLUSION)
- If the existing output rendering truncates or strips markdown headers,
  fix that — but based on current code this should work out of the box

### Acceptance criteria

- [ ] Review chain view renders semi-formal analysis sections as markdown
      without errors
- [ ] Section headers (PREMISES, ACCEPTANCE TRACE, etc.) are visible in the
      rendered output
- [ ] Test exists verifying semi-formal output in the review chain

### Test cases

- **Semiformal output renders**: Build a `StepResult` with output containing
  all five semi-formal sections, render through the review chain template —
  verify no template errors and section headers appear in HTML output
- **Semiformal with flags**: Build a `StepResult` with semiformal output AND
  quality flags (including `semiformal_inconsistency`) — verify both the
  flags and the analysis render correctly

---

## Integration chain audit

```
reviewer_semiformal.txt created in Issue 1 (prompt)
  -> embedded via go:embed *.txt (automatic, no issue needed)
  -> registered in harnessPromptFiles in Issue 1 (scaffold.go)
  -> registered in configTail in Issue 1 (init.go)
  -> loaded by LoadPrompts() in Issue 2 (prompt.go)
  -> stored in Prompts.ReviewerSemiformal in Issue 2 (prompt.go)
  -> selected by runFunctionalReviewCycle in Issue 2 (loop.go)
  -> passed to Review() in Issue 2 (reviewer.go signature change)
  -> rendered and sent to agent (existing RenderPrompt path, no change needed)

Review.Semiformal config field defined in Issue 2 (config.go)
  -> read by runFunctionalReviewCycle in Issue 2 (loop.go)
  -> controls prompt selection (no other consumers)

Review.SemiformalOnRetry config field defined in Issue 2 (config.go)
  -> read by runFunctionalReviewCycle in Issue 2 (loop.go)
  -> controls prompt selection on retry (no other consumers)

CheckSemiformalConsistency defined in Issue 3 (quality.go)
  -> called by computeReviewFlags in Issue 3 (loop.go)
  -> flag checked by hasQualityFlag in runFunctionalReviewCycle in Issue 3 (loop.go)
  -> triggers re-run (continue in retry loop) in Issue 3 (loop.go)

Dashboard rendering in Issue 4
  -> no new types or fields — uses existing StepResult.Output path
```

All hops covered. No gaps.
