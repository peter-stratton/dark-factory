# Scenario: RunDataWriter package

Relates to: Issue #94

## Setup
- The `internal/rundata` package is tested directly via Go unit tests
- All tests use a temporary base directory (override `os.UserHomeDir` or accept
  a base path parameter) so nothing is written to the real `~/.godark/`
- No external services, Docker, or GitHub API required
- `StepResult` fields are populated manually by callers (no dependency on
  `agent.Result`)

## Cases

### New creates run directory
Call `New("owner/repo", "Phase 7", []int{94, 95})` with a temp base directory.
- A directory matching `<base>/owner/repo/<YYYYMMDD-HHMMSS>/` is created
- The directory exists and is writable

### run.json written at creation
After `New()`, read `run.json` from the run directory.
- JSON contains `started_at` as a non-empty ISO 8601 timestamp
- JSON contains `repo` set to `"owner/repo"`
- JSON contains `milestone` set to `"Phase 7"`
- JSON contains `issue_numbers` set to `[94, 95]`
- `finished_at` is null
- `summary` is null

### FinalizeRun updates run.json
After `New()`, call `FinalizeRun(RunSummary{Total: 2, Implemented: 1, Failed: 1})`.
- `run.json` now has a non-null `finished_at` timestamp
- `summary.total` equals 2
- `summary.implemented` equals 1
- `summary.failed` equals 1

### WriteImplementResult creates correct file
Call `WriteImplementResult(42, step)` with a populated `StepResult`.
- File `issues/42/implement.json` exists in the run directory
- JSON contains the `StepResult` fields

### WriteReviewResult creates quality review file
Call `WriteReviewResult(42, "quality", step)`.
- File `issues/42/quality-review.json` exists
- JSON contains the step data

### WriteReviewResult creates functional review file
Call `WriteReviewResult(42, "functional", step)`.
- File `issues/42/functional-review.json` exists

### WriteReviewResult rejects invalid kind
Call `WriteReviewResult(42, "bad", step)`.
- An error is returned
- The error message mentions the invalid kind value

### WriteRetryResult creates retry file
Call `WriteRetryResult(42, 1, step)`.
- File `issues/42/retries/1/retry.json` exists
- JSON contains the step data

### WriteRetryReviewResult creates retry review file
Call `WriteRetryReviewResult(42, 2, step)`.
- File `issues/42/retries/2/quality-review.json` exists

### WriteOutcome creates outcome file
Call `WriteOutcome(Outcome{IssueNumber: 42, Status: "implemented", PRNumber: 57})`.
- File `issues/42/outcome.json` exists
- JSON contains `issue_number`, `status`, `pr_number`

### Path traversal rejected
Call `New("../evil/../../etc", "M", []int{1})`.
- An error is returned
- No directory is created outside the base path

### Dir returns run directory path
After `New()`, call `Dir()`.
- Returns the full path to the run directory
- The path ends with a `YYYYMMDD-HHMMSS` segment

### Timestamp format in directory name
After `New()`, inspect the created directory name.
- The directory name matches the pattern `YYYYMMDD-HHMMSS`
- The timestamp is close to `time.Now()` (within a few seconds)
