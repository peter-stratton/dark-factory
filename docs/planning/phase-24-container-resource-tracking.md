# Phase 24: Container Resource Tracking

> **Goal:** Every agent container execution records peak memory and CPU usage.
> These metrics flow through run data, the stats database, the analyze command,
> the dashboard, and sprint reports — giving operators the data they need to plan
> bounded concurrency.

## Milestone

`Phase 24`

---

## Issue 543: Capture container resource stats via docker inspect

### Description

After a container finishes (or times out / OOM-kills), call `docker inspect`
to capture peak memory and CPU usage before the cleanup `docker rm -f` runs.
Return the values alongside the existing `RunResult`.

Use `docker inspect --format '{{json .State}}'` (or similar) to extract:
- `MemoryStats.MaxUsage` — peak RSS in bytes (persisted by cgroup even after OOM)
- `CpuStats.CpuUsage.TotalUsage` — total CPU time in nanoseconds

### Key constraints

- Modify `sandbox.RunResult` to add `PeakMemoryBytes int64` and `CPUNanoseconds int64`
- Call `docker inspect` after `docker wait` returns but before the deferred `docker rm -f`
- Parse the JSON response; log a warning and leave fields zero on parse failure (best-effort)
- Must work on OOM-killed containers (inspect succeeds on stopped containers)
- `CommandRunner` seam must be used for testability

### Acceptance criteria

- [ ] `RunResult` includes `PeakMemoryBytes` and `CPUNanoseconds` fields
- [ ] `docker inspect` is called after container completion
- [ ] Fields are populated on successful container runs
- [ ] Fields are populated on OOM-killed containers
- [ ] Parse failure logs a warning and returns zero values (does not fail the run)

### Test cases

- **Inspect parsed successfully**: Stub `CommandRunner` to return valid inspect JSON; verify `PeakMemoryBytes` and `CPUNanoseconds` are populated on the result
- **Inspect parse failure**: Stub `CommandRunner` to return malformed JSON; verify fields are zero and no error is returned
- **Inspect command failure**: Stub `CommandRunner` to return an error for the inspect call; verify fields are zero and no error is returned
- **Timed out container**: Stub a timed-out run; verify inspect is still called and fields are populated

---

## Issue 544: Add resource fields to StepResult and step_results table

**Blocked by**: #543

### Description

Add `PeakMemoryBytes` and `CPUNanoseconds` to the `StepResult` struct in
rundata so they are written to per-step JSON files. Add corresponding columns
to the `step_results` table in the stats database. Update the write path to
persist the new fields.

### Key constraints

- Add `PeakMemoryBytes int64` and `CPUNanoseconds int64` to `rundata.StepResult`
- Add `peak_memory_bytes INTEGER` and `cpu_nanoseconds INTEGER` columns to `step_results` via `ALTER TABLE ADD COLUMN` in the migration function — ignore "duplicate column" errors for idempotency
- Update `stats.InsertStepResult` (or equivalent write function) to include the new columns
- Update `stats.QueryStepResults` (or equivalent read function) to scan the new columns
- Update `analysis.StepDetail` (or equivalent conversion struct) to carry the new fields

### Acceptance criteria

- [ ] `StepResult` JSON files include `peak_memory_bytes` and `cpu_nanoseconds` fields
- [ ] `step_results` table has the new columns after migration
- [ ] Migration is idempotent — running on an already-migrated database does not error
- [ ] New fields are written to and read from the stats database
- [ ] Existing databases without the new columns are migrated transparently on open

### Test cases

- **StepResult serialization**: Write a StepResult with resource fields set; read it back and verify values
- **Migration on fresh DB**: Open a new database; verify columns exist
- **Migration on existing DB**: Open a database created before this change; verify columns are added without data loss
- **Double migration**: Run migration twice; verify no error
- **Round-trip through stats DB**: Insert a step result with resource fields; query it back and verify values match

---

## Issue 545: Surface resource stats in analyze CLI output

**Blocked by**: #544

### Description

Add resource usage metrics to the `analysis.Report` struct and display them
in the `godark analyze` CLI output. Show per-step and per-issue aggregates
for peak memory and CPU time.

### Key constraints

- Add a `ResourceStats` section to `analysis.Report` with: `MaxPeakMemoryBytes`, `AvgPeakMemoryBytes`, `TotalCPUNanoseconds`, `AvgCPUNanosecondsPerIssue`, and `ResourceByStep map[string]StepResourceStats`
- `StepResourceStats` holds `MaxMemoryBytes`, `AvgMemoryBytes`, `TotalCPUNanoseconds`
- Aggregate from `step_results` query data in `analysis.Aggregate()`
- Add a "Resource Usage" table to `printAnalyzeReport()` showing per-step max/avg memory and CPU
- Include in JSON output when `--json` is passed
- Skip the section entirely when all values are zero (older runs without resource data)

### Acceptance criteria

- [ ] `analysis.Report` includes `ResourceStats` with per-step breakdowns
- [ ] `godark analyze` prints a resource usage table
- [ ] `godark analyze --json` includes resource stats in JSON output
- [ ] Section is omitted when no resource data exists (backward compatible)

### Test cases

- **Report with resource data**: Aggregate runs that have resource fields; verify stats are computed correctly
- **Report without resource data**: Aggregate runs from before this feature; verify section is omitted
- **Mixed data**: Some runs have resource data, some don't; verify aggregation only counts non-zero entries
- **JSON output**: Verify JSON includes resource stats structure
- **Per-step breakdown**: Verify each step (implement, quality-review, etc.) has its own resource stats

---

## Issue 546: Surface resource stats in dashboard issue detail view

**Blocked by**: #544

### Description

Add peak memory and CPU columns to the per-step table in the dashboard
issue detail view. Display human-friendly units (MB for memory, seconds for CPU).

### Key constraints

- Modify the issue detail HTML template in `internal/dashboard/`
- Add memory and CPU columns to the existing step results table
- Format memory as MB (bytes / 1048576, one decimal place)
- Format CPU as seconds (nanoseconds / 1e9, two decimal places)
- Show "—" for steps with zero values (older runs)

### Acceptance criteria

- [ ] Issue detail view shows peak memory and CPU columns
- [ ] Values are formatted in human-readable units
- [ ] Steps without resource data show "—" instead of zero
- [ ] Page renders correctly for older runs without resource data

### Test cases

- **Step with resource data**: Render issue detail with resource fields set; verify columns show formatted values
- **Step without resource data**: Render issue detail from a pre-feature run; verify "—" placeholders
- **Mixed steps**: Some steps have data, some don't; verify each row is correct

---

## Issue 547: Include resource summary in report sprint output

**Blocked by**: #545

### Description

Add a resource usage summary to the `godark report` sprint output. Show
peak memory high-water mark and total CPU-seconds across the report window.
Useful for capacity planning discussions.

### Key constraints

- Add resource fields to the report data model in `internal/report/`
- Include in terminal, markdown, and HTML output formats
- Show: peak memory high-water mark (max across all steps), total CPU time, avg CPU per issue
- Skip section when no resource data exists in the report window
- If `--no-summary` is set, still include the raw numbers (only the LLM summary is skipped)

### Acceptance criteria

- [ ] `godark report` includes resource usage summary
- [ ] All three output formats (terminal, markdown, html) include the section
- [ ] Section is omitted when no resource data exists
- [ ] Peak memory shows the single highest value across all steps in the window

### Test cases

- **Terminal format**: Generate report with resource data; verify section appears
- **Markdown format**: Generate markdown report; verify section renders correctly
- **No resource data**: Generate report for older runs; verify section is omitted
- **Peak memory correctness**: Verify the highest single step value is shown, not an average

---

## Issue 548: Resource stats for --no-sandbox host mode

**Blocked by**: #544

### Description

When running in `--no-sandbox` mode, there is no Docker container to inspect.
Capture the agent process's resource usage via `syscall.Getrusage` after it
exits, and populate the same `PeakMemoryBytes` and `CPUNanoseconds` fields
on `StepResult`.

### Key constraints

- Modify the host-mode agent execution path in `internal/agent/runner/`
- Use `syscall.Getrusage(syscall.RUSAGE_CHILDREN, &usage)` after the agent process exits
- `usage.Maxrss` gives peak RSS (in bytes on Linux, kilobytes on macOS — normalize)
- `usage.Utime + usage.Stime` gives total CPU time
- Populate the same `StepResult` fields as the sandbox path
- Platform differences: `Maxrss` units differ between Linux and macOS; use build tags or runtime check
- Best-effort: log warning and leave fields zero if `Getrusage` fails

### Acceptance criteria

- [ ] Host-mode runs populate `PeakMemoryBytes` and `CPUNanoseconds` on `StepResult`
- [ ] Values are normalized across platforms (bytes, not kilobytes)
- [ ] `Getrusage` failure logs a warning and does not fail the run
- [ ] Resource fields flow through to stats DB, analyze, and dashboard identically to sandbox mode

### Test cases

- **Host mode captures resources**: Run an agent in host mode; verify resource fields are non-zero
- **Getrusage failure**: Stub `Getrusage` to fail; verify fields are zero and no error
- **Platform normalization**: Verify memory is in bytes regardless of OS
