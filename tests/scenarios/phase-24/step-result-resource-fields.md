# Scenario: Add resource fields to StepResult and step_results table

Relates to: Issue #544

## Setup
- `internal/rundata/writer.go` with `StepResult` struct
- `internal/stats/schema.go` with migration function
- `internal/stats/write.go` and `internal/stats/query.go` for persistence
- A temporary SQLite database for each test case

## Cases

### StepResult JSON includes resource fields
Write a `StepResult` with `PeakMemoryBytes: 209715200` and `CPUNanoseconds: 3000000000` to a JSON file.
- File contains `"peak_memory_bytes": 209715200`
- File contains `"cpu_nanoseconds": 3000000000`

### Migration adds columns to fresh database
Open a new stats database (no prior tables).
- `step_results` table has `peak_memory_bytes` column
- `step_results` table has `cpu_nanoseconds` column

### Migration adds columns to existing database
Open a stats database created before this change (columns absent).
- `ALTER TABLE ADD COLUMN` succeeds for both columns
- Existing rows are preserved with NULL values for new columns

### Double migration is idempotent
Run the migration function twice on the same database.
- Second call does not error
- Table structure unchanged

### Round-trip through stats DB
Insert a step result with `peak_memory_bytes = 104857600` and `cpu_nanoseconds = 2000000000`.
- Query the same step result back
- `PeakMemoryBytes` equals `104857600`
- `CPUNanoseconds` equals `2000000000`

### Zero values handled correctly
Insert a step result with `peak_memory_bytes = 0` and `cpu_nanoseconds = 0`.
- Query returns zero values (not NULL)
- No error on insert or query
