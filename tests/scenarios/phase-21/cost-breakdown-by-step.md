# Scenario: Cost breakdown by step metric

Relates to: Issue #465

## Setup
- `internal/analysis/` package with `Aggregate()` function
- `CostStats` struct extended with `CostByStep map[string]float64`
- Test data with step results containing cost_usd values for different step types

## Cases

### Single step type
All step results are "implement" with total cost $6.00.
- `CostByStep` contains `{"implement": 6.0}`
- Implement is 100% of total cost

### Multiple step types
Step results: implement $3.00, quality-review $1.00, retry-1 $0.50.
- `CostByStep` contains all three entries
- Percentages: implement ~66.7%, quality-review ~22.2%, retry-1 ~11.1%

### Zero cost run
No step results have cost data (all 0.0).
- `CostByStep` is empty or all zeros
- No division-by-zero errors

### CLI output shows cost table
Run `godark analyze` with multi-step cost data.
- Output includes a "Cost by step" table
- Steps sorted by cost descending
- Each row shows step name, total cost, and percentage

### Dashboard shows cost breakdown
View `/analysis` with multi-step cost data.
- Cost breakdown card or chart is visible
- Shows each step type with its cost and percentage

### Retry steps aggregated separately
Step results include "retry-1" ($0.30), "retry-2" ($0.40), "retry-3" ($0.20).
- Each retry attempt appears as a separate entry in `CostByStep`
- Or retries are grouped under a single "retries" key (implementation decides)
