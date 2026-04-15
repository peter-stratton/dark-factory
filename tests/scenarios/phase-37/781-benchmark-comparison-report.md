# Scenario: Benchmark comparison report

Relates to: Issue #781

## Setup
- A stats.db with two completed runs containing step results with cost, duration, and model fields populated
- godark CLI built with the `bench compare` subcommand available

## Cases

### Two-run comparison produces table
- GIVEN two run records in stats.db with different costs and durations
- WHEN `godark bench compare <run-id-1> <run-id-2>` is run
- THEN a tabular report is printed with per-step cost, duration, and model for each run

### Delta column shows percentage change
- GIVEN run A with implement step cost $0.50 and run B with implement step cost $0.30
- WHEN `godark bench compare <A> <B>` is run
- THEN the delta column shows -40% for the implement step cost

### Model differences visible in output
- GIVEN run A where recon used "opus" and run B where recon used "sonnet"
- WHEN `godark bench compare <A> <B>` is run
- THEN both models are shown in the recon row

### JSON output flag
- GIVEN two run records in stats.db
- WHEN `godark bench compare <A> <B> --json` is run
- THEN valid JSON is printed to stdout with per-step and total comparisons

### Nonexistent run ID returns error
- GIVEN a stats.db with one run record
- WHEN `godark bench compare <valid-id> <nonexistent-id>` is run
- THEN an error message is printed indicating the run was not found

### Total row aggregates all steps
- GIVEN two runs each with 3 steps
- WHEN `godark bench compare <A> <B>` is run
- THEN a totals row shows summed cost and duration for each run with overall delta
