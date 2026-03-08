# Scenario: Wire auto_merge and risk classification into merge decision

Relates to: Issue #247

## Setup
- The `internal/agent/` package tested with stubbed `GuardRunner` and `Runner`
- Mock `gh pr view` and `gh pr diff` for PR stats
- Config with various `auto_merge` and `risk_thresholds` settings

## Cases

### Low risk PR auto-merged
Config has `auto_merge: low_risk`. PR has 50 lines, 3 files, no protected paths, no fix cycles, no flags.
- Risk classifier runs
- `IsLowRisk` is `true`
- PR is merged
- Outcome status is `"implemented"`

### High risk PR sent to human
Config has `auto_merge: low_risk`. PR has 500 lines, 20 files.
- Risk classifier runs
- `IsLowRisk` is `false`
- `github.AddIssueLabel` called with `"godark:awaiting-human-review"`
- Outcome status is `"ready-to-merge"`
- PR is not merged

### All mode ignores risk
Config has `auto_merge: all`. PR has 500 lines.
- Risk classifier is NOT called
- PR is merged regardless of size

### None mode ignores risk
Config has `auto_merge: none`. PR has 10 lines, 1 file (very low risk).
- Risk classifier is NOT called
- PR is not merged
- Outcome status is `"ready-to-merge"`

### Risk assessment written to run data
Config has `auto_merge: low_risk`. PR is classified.
- `hook.WriteRiskAssessment` is called with the `RiskAssessment` struct
- Assessment includes all gate results

### Protected path blocks auto-merge
Config has `auto_merge: low_risk`, `protected_paths: ["CLAUDE.md"]`. PR changes `CLAUDE.md`.
- `IsLowRisk` is `false`
- PR is labeled for human review, not merged
