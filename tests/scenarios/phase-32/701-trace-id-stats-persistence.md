# Scenario: Trace ID persistence in stats.db

Relates to: Issue #701

## Setup
- An in-memory SQLite database with `EnsureSchema` called to create all tables and run migrations
- Sample `StepResultRecord` and `IssueOutcomeRecord` values with `TraceID` fields populated

## Cases

### Migration adds trace_id column to step_results
- GIVEN a fresh SQLite database
- WHEN `EnsureSchema` runs
- THEN the `step_results` table has a `trace_id` column

### Migration adds trace_id column to issue_outcomes
- GIVEN a fresh SQLite database
- WHEN `EnsureSchema` runs
- THEN the `issue_outcomes` table has a `trace_id` column

### Migration is idempotent
- GIVEN `EnsureSchema` has already been called once
- WHEN `EnsureSchema` is called a second time
- THEN no error occurs (column already exists)

### Step result round-trip with TraceID
- GIVEN a `StepResultRecord` with `TraceID` set to "abc-123"
- WHEN it is written via `WriteStepResult` and queried back via `QueryStepResults`
- THEN the returned record has `TraceID` equal to "abc-123"

### Issue outcome round-trip with TraceID
- GIVEN an `IssueOutcomeRecord` with `TraceID` set to "abc-123"
- WHEN it is written via `WriteIssueOutcome` and queried back via `QueryIssueOutcomes`
- THEN the returned record has `TraceID` equal to "abc-123"

### Empty TraceID backwards compatibility
- GIVEN a `StepResultRecord` with `TraceID` set to ""
- WHEN it is written and queried back
- THEN no error occurs and the returned `TraceID` is an empty string

### Convert copies TraceID from StepResult to StepResultRecord
- GIVEN a `rundata.StepResult` with `TraceID` set to "trace-xyz"
- WHEN it is converted to a `StepResultRecord`
- THEN `StepResultRecord.TraceID` equals "trace-xyz"

### Convert copies TraceID from Outcome to IssueOutcomeRecord
- GIVEN a `rundata.Outcome` with `TraceID` set to "trace-xyz"
- WHEN it is converted to an `IssueOutcomeRecord`
- THEN `IssueOutcomeRecord.TraceID` equals "trace-xyz"
