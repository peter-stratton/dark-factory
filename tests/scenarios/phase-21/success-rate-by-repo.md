# Scenario: Success rate by repo and surface verify stats

Relates to: Issue #467

## Setup
- `internal/analysis/` package with `Aggregate()` function
- `Report` struct extended with `RepoStats map[string]RepoSummary`
- `RepoSummary` struct: `Total`, `Implemented`, `Failed`, `SuccessRate`
- `Report.VerifyStats` already computed but currently not displayed
- Test data with runs across multiple repos and verify check results

## Cases

### Single repo success rate
One repo with 8 implemented, 2 failed issues.
- `RepoStats["org/repo-a"].SuccessRate` is 0.80
- `RepoStats["org/repo-a"].Total` is 10

### Multiple repos with different rates
Repo-a: 8/10 success. Repo-b: 3/5 success.
- `RepoStats` has 2 entries
- Repo-a rate is 0.80, repo-b rate is 0.60

### CLI shows per-repo table
Run `godark analyze` with multi-repo data.
- Output includes a "Success by repo" table
- Each row shows repo name, total, implemented, failed, success rate

### Dashboard shows repo breakdown
View `/analysis` with multi-repo data.
- Repo success rate card is visible
- Each repo listed with its success rate

### Verify stats displayed in CLI
Run `godark analyze` with verify check data (lint failed 3/10, test failed 1/10).
- Output includes a "Verify check failures" table
- Shows: "lint: 3 failures (30.0%)", "test: 1 failure (10.0%)"

### Verify stats displayed in dashboard
View `/analysis` with verify check data.
- Verify check failure card is visible
- Shows check names with failure counts and rates

### No verify data omits section
Run `godark analyze` with no verify results across any runs.
- Verify section is omitted entirely (not an empty table)

### Single repo omits table header
When only one repo exists in the data.
- Repo stats still displayed (even if less useful with one repo)
- No errors from single-entry map
