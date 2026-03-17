# Scenario: Surface resource stats in analyze CLI output

Relates to: Issue #545

## Setup
- Stats database populated with step results that include resource fields
- `internal/analysis/analysis.go` with `Aggregate()` function
- `internal/cmd/analyze.go` with `printAnalyzeReport()` function

## Cases

### Report includes resource stats
Aggregate runs where step results have non-zero `peak_memory_bytes` and `cpu_nanoseconds`.
- `Report.ResourceStats` is populated
- `MaxPeakMemoryBytes` reflects the highest single step value
- `AvgPeakMemoryBytes` reflects the mean across non-zero steps
- `TotalCPUNanoseconds` reflects the sum across all steps
- `ResourceByStep` has entries for each step name

### Per-step breakdown computed
Aggregate runs with implement steps using 200MB and review steps using 100MB.
- `ResourceByStep["implement"].MaxMemoryBytes` equals the implement peak
- `ResourceByStep["quality-review"].MaxMemoryBytes` equals the review peak
- Each step has independent averages

### Section omitted for old runs
Aggregate runs where all resource fields are zero.
- `Report.ResourceStats` has all zero values
- CLI output does not print a "Resource Usage" section

### Mixed old and new runs
Aggregate a mix of runs with and without resource data.
- Averages computed only over non-zero entries
- Maximums reflect only runs that have data

### JSON output includes resource stats
Run `godark analyze --json` against runs with resource data.
- JSON output contains `resource_stats` key
- Per-step breakdown is present in JSON
