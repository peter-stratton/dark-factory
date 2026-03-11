# Scenario: Quality reviewer value-add analysis

Relates to: Issue #344

## Setup
- The `internal/analysis/` package tested via Go unit tests
- Fabricated `rundata.RunDetail` slices with various quality and functional
  review outcomes
- The `internal/cmd/analyze.go` command with captured stdout

## Cases

### No quality reviews in any run
Compute stats from runs where no quality review data exists.
- `QualityReviewerStats.RunsWithQualityReview` is `0`
- `QualityReviewerStats.AvgQualityCostUSD` is `0`
- `QualityReviewerStats.TokenCostTotal` is `0`
- No division-by-zero panic

### All quality reviews approved
Compute stats from runs where quality review always approved on first pass.
- `QualityChangesRequested` is `0`
- `QualityOnlyBlocks` is `0`
- `AvgQualityCostUSD` reflects the total quality review cost divided by count

### Quality blocks then functional approves
Compute stats from a run where quality review requested changes, implementer
fixed, and functional review approved on first pass.
- `QualityChangesRequested` is `1`
- `QualityOnlyBlocks` is `1` (quality was the only blocker)

### Both quality and functional block
Compute stats from a run where both quality review and functional review
requested changes.
- `QualityChangesRequested` is `1`
- `QualityOnlyBlocks` is `0` (functional would have caught it too)

### Cost aggregation across runs
Compute stats from three runs with quality review costs of $0.10, $0.20, $0.30.
- `TokenCostTotal` is `$0.60`
- `AvgQualityCostUSD` is `$0.20`

### Mixed runs with and without quality review
Compute stats from five runs where only three have quality review data.
- `TotalRuns` is `5`
- `RunsWithQualityReview` is `3`
- Percentages are computed against `RunsWithQualityReview`, not `TotalRuns`

### Human-readable output
Run `godark analyze` with quality review data present.
- Output contains "Quality Reviewer" section header
- Output contains the percentage of runs where quality caught issues
- Output contains average and total cost figures

### JSON output
Run `godark analyze --json` with quality review data present.
- JSON output contains `quality_reviewer_stats` key
- All fields of `QualityReviewerStats` are present in the JSON
