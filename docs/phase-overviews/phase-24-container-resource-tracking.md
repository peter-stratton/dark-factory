# Phase 24: Container Resource Tracking

Before Phase 24, godark tracked cost and duration for every agent step but had no visibility into how much memory or CPU each container actually consumed. Planning for bounded concurrency (running multiple agents in parallel) requires knowing the resource envelope of a single agent. Phase 24 captures peak memory and CPU usage from every container execution, flows those metrics through run data, the stats database, the analyze command, the dashboard, and sprint reports -- giving operators the data they need to right-size their concurrency limits.

---

## Docker Stats Capture

**What it does:** After a container finishes (or times out, or gets OOM-killed), godark queries the Docker stats API to capture peak memory usage and total CPU time before the container is removed. The values are returned alongside the existing `RunResult`.

**Example:** When `RunContainer` in `internal/sandbox/container.go` completes a container run, it calls the Docker stats endpoint via the Unix socket:

```
GET /containers/{id}/stats?stream=false
```

The response is parsed into a `containerStats` struct that mirrors the Docker API shape:

```go
type containerStats struct {
    MemoryStats struct {
        MaxUsage int64 `json:"max_usage"`
    } `json:"memory_stats"`
    CpuStats struct {
        CpuUsage struct {
            TotalUsage int64 `json:"total_usage"`
        } `json:"cpu_usage"`
    } `json:"cpu_stats"`
}
```

The extracted values populate the `RunResult`:

```go
type RunResult struct {
    ExitCode        int
    Stdout          string
    Stderr          string
    TimedOut        bool
    PeakMemoryBytes int64  // from memory_stats.max_usage
    CPUNanoseconds  int64  // from cpu_stats.cpu_usage.total_usage
}
```

This works on OOM-killed containers because `docker inspect` succeeds on stopped containers -- the cgroup retains peak memory even after the process is gone. If the stats call fails (container already removed, API error), the fields are left at zero and a warning is logged. Resource capture is best-effort and never fails a run.

---

## StepResult Resource Fields

**What it does:** The `StepResult` struct in `internal/rundata/writer.go` carries resource metrics alongside the existing cost, duration, and flags. These fields are serialized to per-step JSON files in the run data directory.

**Example:** After an implement step completes on issue #42, the step result JSON file at `~/.godark/runs/<run-id>/issues/42/implement.json` includes:

```json
{
  "duration_seconds": 245.3,
  "cost_usd": 0.42,
  "peak_memory_bytes": 524288000,
  "cpu_nanoseconds": 18500000000,
  "flags": [],
  "session_id": "abc123"
}
```

The fields on the struct:

```go
type StepResult struct {
    // ... existing fields ...
    PeakMemoryBytes int64 `json:"peak_memory_bytes,omitempty"`
    CPUNanoseconds  int64 `json:"cpu_nanoseconds,omitempty"`
}
```

The `omitempty` tag means older step results without resource data produce no spurious zero fields, and existing tooling that reads these files is unaffected.

---

## Stats Database Columns

**What it does:** Two new columns on the `step_results` table in `~/.godark/stats.db` persist resource metrics for historical analysis. The migration is idempotent -- running against an already-migrated database is a no-op.

**Example:** The schema migration in `internal/stats/schema.go` adds the columns via `ALTER TABLE`:

```sql
ALTER TABLE step_results ADD COLUMN peak_memory_bytes INTEGER DEFAULT 0
ALTER TABLE step_results ADD COLUMN cpu_nanoseconds INTEGER DEFAULT 0
```

Duplicate-column errors are silently ignored, making the migration safe to run on every database open. The write path in `internal/stats/write.go` includes both fields:

```sql
INSERT OR REPLACE INTO step_results
    (run_id, issue_number, step_name, cost_usd, duration_seconds, flags,
     started_at, finished_at, peak_memory_bytes, cpu_nanoseconds)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
```

The query path scans them back into `StepResultRecord`:

```go
type StepResultRecord struct {
    RunID            string
    IssueNumber      int
    StepName         string
    CostUSD          float64
    DurationSeconds  float64
    Flags            []string
    StartedAt        time.Time
    FinishedAt       time.Time
    PeakMemoryBytes  int64
    CPUNanoseconds   int64
}
```

---

## Analysis Aggregation

**What it does:** The `analysis.Aggregate()` function computes resource statistics at multiple granularities: global maximums and averages, per-step breakdowns, and per-issue peaks. The result is a `ResourceStats` struct attached to the analysis report.

**Example:** After aggregating runs with resource data, the report contains:

```go
type ResourceStats struct {
    MaxPeakMemoryBytes        int64                        `json:"max_peak_memory_bytes"`
    AvgPeakMemoryBytes        int64                        `json:"avg_peak_memory_bytes"`
    TotalCPUNanoseconds       int64                        `json:"total_cpu_nanoseconds"`
    AvgCPUNanosecondsPerIssue int64                        `json:"avg_cpu_nanoseconds_per_issue"`
    ResourceByStep            map[string]StepResourceStats `json:"resource_by_step"`
}

type StepResourceStats struct {
    MaxMemoryBytes      int64 `json:"max_memory_bytes"`
    AvgMemoryBytes      int64 `json:"avg_memory_bytes"`
    TotalCPUNanoseconds int64 `json:"total_cpu_nanoseconds"`
}
```

`AvgPeakMemoryBytes` is the average of per-issue peaks (each issue's max across its steps), not the average of individual step readings. This reflects the actual memory envelope per issue, which is what matters for concurrency planning. When all values are zero (older runs without resource data), `ResourceStats` is nil and the section is omitted entirely.

---

## Analyze CLI Output

**What it does:** The `godark analyze` command prints a "Resource Usage" section with global stats and a per-step breakdown table. The same data is included in `--json` output.

**Example:** Running analyze against a repo with resource data:

```
$ godark analyze --repo myorg/myapp --since 2026-03-01

Resource Usage
  Max peak memory:    512.0 MB
  Avg peak memory:    384.2 MB
  Total CPU time:     2m 45s
  Avg CPU per issue:  18.5s

  STEP                MAX MEMORY    AVG MEMORY    TOTAL CPU
  implement           512.0 MB      410.3 MB      1m 52s
  quality-review      256.8 MB      198.4 MB      32s
  functional-review   245.2 MB      192.1 MB      21s
```

Two formatting helpers convert raw values to human-readable units:

- `formatBytes(b int64)` -- renders as GB, MB, KB, or B depending on magnitude
- `formatNanoseconds(ns int64)` -- renders as hours, minutes, seconds, or milliseconds

When no resource data exists in the query window (all values zero), the entire section is skipped.

---

## Dashboard Issue Detail View

**What it does:** The per-step timeline table in the dashboard's issue detail page includes peak memory and CPU time columns. Steps from older runs without resource data show a dash instead of zero.

**Example:** Viewing issue #42 in the dashboard shows each step with its resource usage:

| Step | Duration | Cost | Peak Memory | CPU Time | Verdict |
|------|----------|------|-------------|----------|---------|
| implement | 4m 05s | $0.42 | 512.0 MB | 18.50s | Passed |
| quality-review | 1m 30s | $0.15 | 256.8 MB | 8.23s | Passed |
| verify | 45s | -- | -- | -- | Passed |

The `TimelineStepView` struct in `internal/dashboard/handlers.go` carries the formatted values:

```go
type TimelineStepView struct {
    // ... existing fields ...
    PeakMemory string // "512.0 MB" or "—"
    CPUTime    string // "18.50s" or "—"
}
```

Formatting is handled by `formatMemoryMB` (bytes to MB with one decimal) and `formatCPUSecs` (nanoseconds to seconds with two decimals). Zero values render as "--" so the table reads cleanly for mixed-era data.

---

## Sprint Report

**What it does:** The `godark report` command includes a resource usage summary showing the peak memory high-water mark and total CPU time across the report window. The section appears in terminal, markdown, and HTML output formats.

**Example:** A sprint report covering the last two weeks:

```
Resource Usage
  Peak memory (max single step):  512.0 MB
  Total CPU time:                 45m 12s
  Avg CPU per issue:              18.5s
```

The `SprintReport` struct in `internal/report/report.go` carries the raw values:

```go
type SprintReport struct {
    // ... other fields ...
    PeakMemoryBytes           int64 // max single-step peak across all steps in window
    TotalCPUNanoseconds       int64 // sum of CPU nanoseconds across all steps
    AvgCPUNanosecondsPerIssue int64 // total / issues processed
}
```

These are populated directly from `ResourceStats` in the analysis report. The peak memory value is the single highest step reading, not an average -- it represents the worst-case memory requirement for a single container, which is the number that matters when sizing a concurrency pool. The section is omitted when no resource data exists in the report window.

---

## Host Mode Resource Capture

**What it does:** When running with `--no-sandbox` (no Docker container), resource stats are captured from the host process via `syscall.Getrusage` after the agent process exits. The same `PeakMemoryBytes` and `CPUNanoseconds` fields are populated, so downstream reporting is identical regardless of execution mode.

**Example:** In `internal/agent/launcher.go`, after the agent process completes:

```go
var usage syscall.Rusage
if err := GetrusageFn(syscall.RUSAGE_CHILDREN, &usage); err != nil {
    logger.Warn("getrusage failed, resource stats will be zero", "error", err)
} else {
    mem := int64(usage.Maxrss)
    if goosForRusage == "darwin" {
        mem *= 1024 // macOS reports Maxrss in KB; normalize to bytes
    }
    res.PeakMemoryBytes = mem
    res.CPUNanoseconds = (int64(usage.Utime.Sec)+int64(usage.Stime.Sec))*1e9 +
        (int64(usage.Utime.Usec)+int64(usage.Stime.Usec))*1e3
}
```

The platform difference is significant: Linux reports `Maxrss` in bytes, macOS in kilobytes. The `goosForRusage` check normalizes to bytes on both platforms. CPU time combines user and system time from the child process. Like the Docker path, this is best-effort -- a `Getrusage` failure logs a warning and leaves fields at zero without failing the run.
