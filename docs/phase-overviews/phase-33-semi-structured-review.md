# Phase 33: Semi-Structured Review

The functional reviewer has always been the last quality gate before a PR merges. But its reasoning was free-form text - hard to audit, harder to verify programmatically. Phase 33 adds a semi-formal analysis mode where the reviewer produces structured reasoning (premises, acceptance traces, regression traces, uncovered paths, formal conclusion) that the system can machine-verify for consistency. When the verdict contradicts the evidence, the review is automatically re-run. This catches false approvals on subtle bugs without adding human review overhead.

---

## Semi-Formal Reviewer Prompt

**What it does:** A new prompt template (`prompts/reviewer_semiformal.txt`) extends the standard reviewer with a structured reasoning block. The reviewer must work through five sections in order - PREMISES, ACCEPTANCE TRACE, REGRESSION TRACE, UNCOVERED PATHS, and FORMAL CONCLUSION - before rendering its verdict. The verdict must logically follow from the traces.

**Example:** The structured analysis block inserted between the test execution steps and the verdict:

```
## Semi-Formal Analysis

Before rendering your verdict, complete the following structured reasoning.
Work through each section in order.

### PREMISES
For each file changed in the PR diff, state in one sentence what the change does.
Label each premise P1, P2, ... in order.

### ACCEPTANCE TRACE
For each acceptance criterion in the issue body or scenario spec:
- Identify which premises (P1, P2, ...) satisfy it.
- State: SATISFIED / NOT SATISFIED / UNTESTABLE
- Cite the specific test (name or file) that exercises it, or state "no test."

### REGRESSION TRACE
For each existing test that touches a modified file:
- State the test's prior behavior.
- State the new behavior after the patch.
- State: PRESERVED / CHANGED (justified) / BROKEN

### UNCOVERED PATHS
List code paths introduced by the patch that are not exercised by any test.
Rate each: HIGH / MEDIUM / LOW risk.

### FORMAL CONCLUSION
Derive your verdict:
- If any acceptance criterion is NOT SATISFIED -> CHANGES_REQUESTED
- If any regression is BROKEN -> CHANGES_REQUESTED
- If any uncovered path is HIGH risk -> CHANGES_REQUESTED
- Otherwise -> APPROVED
```

The prompt enforces alignment between analysis and verdict with a critical rule:

```
CRITICAL: Your AGENT_RESULT line MUST match your FORMAL CONCLUSION. If any AC
is NOT SATISFIED, any RT is BROKEN, or any uncovered path is HIGH risk, you
MUST print AGENT_RESULT=CHANGES_REQUESTED.
```

The reviewer also posts the full semi-formal analysis as a PR comment under a `### Semi-Formal Analysis` header, making it visible in the GitHub dialogue trail and the run dashboard.

---

## Config Toggle

**What it does:** Two boolean fields in `godark.yaml` control when the semi-formal prompt is used. `review.semiformal` enables it for all review cycles. `review.semiformal_on_retry` enables it only on retry cycles (attempt > 0), where structured reasoning has the highest payoff relative to token cost. Both default to `false`.

**Example:** A team wants structured reasoning only when a review fails and the implementer retries - the first pass uses the cheaper standard review, and retries get the deeper analysis:

```yaml
review:
  semiformal_on_retry: true
```

Or to use semi-formal for every review:

```yaml
review:
  semiformal: true
```

The config struct in `internal/config/config.go`:

```go
type Review struct {
    Semiformal        bool `yaml:"semiformal"`
    SemiformalOnRetry bool `yaml:"semiformal_on_retry"`
}
```

When neither field is set (or both are false), the standard `reviewer.txt` prompt is used for all review cycles - no behavior change unless opted in.

---

## Prompt Selection Logic

**What it does:** The `selectReviewerPrompt` function in `internal/agent/reviewer.go` chooses between the standard and semi-formal prompt based on config and the current attempt number. The `Review()` function accepts the prompt string as a parameter rather than reading it from the `Prompts` struct directly.

**Example:** In the functional review loop in `internal/agent/loop.go`, the prompt is selected before each review call:

```go
reviewerPrompt := selectReviewerPrompt(cfg, prompts, attempt)
usedSemiformal := reviewerPrompt == prompts.ReviewerSemiformal && prompts.ReviewerSemiformal != ""
reviewResult, err := Review(ctx, issue, prNum, cfg, reviewerPrompt, authEnv, logger, hasSpec)
```

The selection function itself:

```go
func selectReviewerPrompt(cfg *config.Config, prompts *Prompts, attempt int) string {
    if cfg.Review.Semiformal || (cfg.Review.SemiformalOnRetry && attempt > 0) {
        if prompts.ReviewerSemiformal != "" {
            return prompts.ReviewerSemiformal
        }
    }
    return prompts.Reviewer
}
```

If `SemiformalOnRetry` is true, the first review (attempt 0) uses the standard prompt. When the implementer retries and the second review runs (attempt 1), it switches to the semi-formal prompt. The `usedSemiformal` flag is tracked so the consistency gate knows whether to run.

---

## Consistency Quality Gate

**What it does:** `CheckSemiformalConsistency` in `internal/quality/quality.go` scans the reviewer's output for contradictions between the structured traces and the verdict. If the reviewer says APPROVED but the traces contain evidence that should block approval, a `semiformal_inconsistency` flag fires and the review is automatically re-run.

**Example:** A reviewer approves a PR, but its own acceptance trace says one criterion is NOT SATISFIED. The consistency check catches this:

```go
func CheckSemiformalConsistency(output string) *Flag {
    if !strings.Contains(output, "FORMAL CONCLUSION") {
        return nil  // Not a semiformal review, skip
    }
    if !strings.Contains(output, "AGENT_RESULT=APPROVED") {
        return nil  // No contradiction possible on CHANGES_REQUESTED
    }
    if strings.Contains(output, "NOT SATISFIED") {
        return &Flag{
            Code:    "semiformal_inconsistency",
            Message: "verdict APPROVED but acceptance trace contains NOT SATISFIED",
        }
    }
    if strings.Contains(output, ": BROKEN") {
        return &Flag{
            Code:    "semiformal_inconsistency",
            Message: "verdict APPROVED but regression trace contains BROKEN",
        }
    }
    if strings.Contains(output, "Risk: HIGH") {
        return &Flag{
            Code:    "semiformal_inconsistency",
            Message: "verdict APPROVED but uncovered paths contain Risk: HIGH",
        }
    }
    return nil
}
```

Three contradiction types are detected:

| Evidence in traces | Verdict | Result |
|---|---|---|
| `NOT SATISFIED` in acceptance trace | APPROVED | `semiformal_inconsistency` flag |
| `: BROKEN` in regression trace | APPROVED | `semiformal_inconsistency` flag |
| `Risk: HIGH` in uncovered paths | APPROVED | `semiformal_inconsistency` flag |

When the flag fires, the review loop in `runFunctionalReviewCycle` handles it the same way as `no_review_tests_written` - delete the stale PR comment and continue the retry loop:

```go
if hasQualityFlag(fFlags, "semiformal_inconsistency") {
    logger.Warn("semiformal review inconsistency detected - re-running reviewer",
        "issue_number", issue.Number,
        "attempt", attempt+1,
    )
    if err := github.DeleteLastPRCommentWithHeader(cfg.Repo, prNum, "## Review Notes"); err != nil {
        logger.Warn("failed to delete stale review comment", "error", err)
    }
    continue
}
```

The inconsistency flag is only checked when the semi-formal prompt was actually used (`isSemiformal` parameter in `computeReviewFlags`), so standard reviews are unaffected.

---

## Dashboard Rendering

**What it does:** The semi-formal analysis sections render naturally in the dashboard's review chain view without any template changes. The existing `stepToView` function captures full output, and the template renders it as-is. Section headers (PREMISES, ACCEPTANCE TRACE, etc.) appear as markdown in the output display.

**Example:** When viewing the review chain for an issue that used semi-formal review, the functional review step output includes the full structured analysis:

```
## PREMISES
- P1: The function now returns a sorted list.
- P2: Input validation added for empty slices.

## ACCEPTANCE TRACE
1. Call sort([3,1,2]) -> expect [1,2,3]. Passes. Verdict: SATISFIED
2. Call sort([]) -> expect []. Passes. Verdict: SATISFIED

## REGRESSION TRACE
- No prior failures detected for this function. Status: PRESERVED

## UNCOVERED PATHS
- Error path when input contains nil elements. Risk: MEDIUM

## FORMAL CONCLUSION
All acceptance criteria SATISFIED. No regressions BROKEN. No HIGH-risk uncovered paths.
Verdict: APPROVED
```

A dashboard test in `internal/dashboard/handlers_review_chain_test.go` verifies that all five section headers appear in the rendered HTML output.

---

## Scaffold and Init Wiring

**What it does:** The semi-formal prompt is installed alongside other prompt templates when a project is initialized with `godark init` or scaffolded with `godark new`.

**Example:** The `harnessPromptFiles` list in `internal/cmd/scaffold.go` includes the new prompt:

```go
{"reviewer_semiformal.txt", "prompts/reviewer_semiformal.txt"},
```

And the `configTail` constant in `internal/cmd/init.go` includes the path in the prompts section:

```yaml
prompts:
  # ...
  reviewer_semiformal: prompts/reviewer_semiformal.txt
```

This means new projects get the prompt template installed and referenced in their `godark.yaml` by default, but the feature stays dormant until `review.semiformal` or `review.semiformal_on_retry` is set to `true`.
