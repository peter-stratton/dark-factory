# Scenario: Extract vet data fetcher helper

Relates to: Issue #396

## Setup
- `internal/cmd/vet_helpers.go` contains `fetchVetData(repo, milestone)`
- Stub GitHub API functions for `FetchMilestoneIssues` and `FetchAllIssueNumbers`

## Cases

### Milestone mode returns both
Call `fetchVetData("owner/repo", "Phase 1")` with stubs returning 3 issues and 10 issue numbers.
- `issues` slice has 3 elements
- `allNums` map has 10 entries

### Repo-only mode returns allNums only
Call `fetchVetData("owner/repo", "")` with stub returning 10 issue numbers.
- `issues` is nil
- `allNums` map has 10 entries

### Fetch error wrapped
Stub `FetchMilestoneIssues` to return an error. Call `fetchVetData("owner/repo", "Phase 1")`.
- Returns an error containing "fetching milestone issues"

### Vet issues uses helper
Read `internal/cmd/vet_issues.go`.
- No inline `FetchMilestoneIssues` + `FetchAllIssueNumbers` block

### Vet roadmap uses helper
Read `internal/cmd/vet_roadmap.go`.
- No inline fetch block

### Vet scenarios uses helper
Read `internal/cmd/vet_scenarios.go`.
- No conditional duplicate fetch paths
