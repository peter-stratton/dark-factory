# Scenario: Duration trends by step metric

Relates to: Issue #466

## Setup
- `internal/analysis/trends.go` with `ComputeTrends()` function
- `TrendPoint` struct extended with `AvgImplementDuration` and `AvgReviewDuration` (seconds)
- Test data with step results containing duration_seconds for implement and review steps across multiple runs

## Cases

### Single run with implement and review durations
One run with implement at 300s and quality-review at 120s.
- `TrendPoint.AvgImplementDuration` is 300.0
- `TrendPoint.AvgReviewDuration` is 120.0

### Multiple runs show duration over time
3 runs with increasing implement durations: 200s, 300s, 400s.
- Trend points are in chronological order
- Each point has the correct `AvgImplementDuration`

### Missing duration data produces zero
A run with no step results (or step results with 0 duration).
- `AvgImplementDuration` is 0.0
- `AvgReviewDuration` is 0.0
- No NaN or infinity values

### Average across multiple issues in one run
A run with 3 issues: implement durations 200s, 300s, 400s.
- `AvgImplementDuration` is 300.0 (average)

### CLI output shows duration summary
Run `godark analyze` with duration data.
- Output includes average implement and review durations
- Formatted as human-readable time (e.g., "5m00s", "2m00s")

### Dashboard shows duration trend chart
View `/analysis` with multi-run duration data.
- A duration trend chart is visible
- Shows implement and review duration lines over time

### Duration helps identify timeout needs
A run with implement durations approaching 1800s (30m default timeout).
- The trend point clearly shows the high duration
- Helps the user decide to increase `agent_timeout` in `godark.yaml`
