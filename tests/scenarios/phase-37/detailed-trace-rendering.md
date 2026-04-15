# Scenario: Detailed trace rendering

Relates to: Issue #788

## Setup
- A run data directory exists under `~/.godark/runs/<owner>/<repo>/<timestamp>/` with `run.json`, issue subdirectories, and step result JSON files
- A stats.db database contains matching run, outcome, and step_result records with trace IDs
- The `godark trace` CLI command is available

## Cases

### Detail flag renders all step sections
- GIVEN a run data directory with recon, implement, verify, quality-review, and functional-review steps for issue #1
- WHEN `godark trace 1 --detail` is executed
- THEN the output contains section headers for each step (RECON, IMPLEMENT, VERIFY, QUALITY REVIEW, FUNCTIONAL REVIEW, OUTCOME)

### Detail renders prompt when captured
- GIVEN a run data directory where `implement.json` has a non-empty `prompt` field
- WHEN `godark trace <issue> --detail` is executed
- THEN the IMPLEMENT section displays the first 3 lines of the prompt

### Detail shows not-captured for missing prompt
- GIVEN a run data directory where `implement.json` has no `prompt` field (old run)
- WHEN `godark trace <issue> --detail` is executed
- THEN the IMPLEMENT section displays "[not captured]" for the prompt

### Detail renders retries in chronological order
- GIVEN a run data directory with an initial implementation and one retry cycle (retry-0)
- WHEN `godark trace <issue> --detail` is executed
- THEN the retry step appears between the initial implementation and the final review

### Detail renders risk assessment gates
- GIVEN a run data directory with a `risk-assessment.json` containing gate results
- WHEN `godark trace <issue> --detail` is executed
- THEN the RISK ASSESSMENT section shows each gate name with PASS or FAIL

### Detail renders verify check results
- GIVEN a run data directory with `verify-0.json` containing build, lint, and test check results
- WHEN `godark trace <issue> --detail` is executed
- THEN the VERIFY section shows each check name with PASS or FAIL

### Detail renders dialogue entries
- GIVEN a run data directory with `dialogue.json` containing implementer and reviewer entries
- WHEN `godark trace <issue> --detail` is executed
- THEN dialogue entries appear inline in the output

### Detail falls back when run data directory is missing
- GIVEN a stats.db entry for a trace whose run data directory has been deleted
- WHEN `godark trace <issue> --detail` is executed
- THEN the output shows the standard summary table with a note that run data was not found on disk

### Detail and JSON flags are mutually exclusive
- GIVEN the `godark trace` command
- WHEN both `--detail` and `--json` flags are passed
- THEN an error message is printed and the command exits with a non-zero code

### QueryRunByID returns matching record
- GIVEN a stats.db with a run record with ID "20260414-120000"
- WHEN `QueryRunByID` is called with "20260414-120000"
- THEN it returns the matching `RunRecord` with correct repo and milestone fields

### QueryRunByID returns nil for unknown ID
- GIVEN a stats.db with no run record for ID "99999999-999999"
- WHEN `QueryRunByID` is called with "99999999-999999"
- THEN it returns nil and no error
