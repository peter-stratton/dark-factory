# Scenario: Risk classifier

Relates to: Issue #245

## Setup
- The `internal/quality/` package tested via Go unit tests
- `RiskInput` structs with various combinations of risk signals

## Cases

### All gates pass
Call `ClassifyRisk` with 50 lines, 3 files, no protected paths touched, 0 fix cycles, no quality flags. Thresholds: 200 lines, 10 files.
- `IsLowRisk` is `true`
- All gates report `Passed: true`

### Lines exceeded
Call `ClassifyRisk` with 201 lines changed. Threshold: 200.
- `IsLowRisk` is `false`
- `max_lines` gate reports `Passed: false`
- Detail mentions the line count and threshold

### Files exceeded
Call `ClassifyRisk` with 11 files changed. Threshold: 10.
- `IsLowRisk` is `false`
- `max_files` gate reports `Passed: false`

### Protected path touched
Call `ClassifyRisk` with `ChangedFiles: ["CLAUDE.md"]` and `ProtectedPaths: ["CLAUDE.md"]`.
- `IsLowRisk` is `false`
- `protected_paths` gate reports `Passed: false`

### Protected path prefix match
Call `ClassifyRisk` with `ChangedFiles: ["tests/scenarios/phase-1/foo.md"]` and `ProtectedPaths: ["tests/scenarios/"]`.
- `IsLowRisk` is `false`
- `protected_paths` gate reports `Passed: false`

### Fix cycles used
Call `ClassifyRisk` with `FixCycles: 1`.
- `IsLowRisk` is `false`
- `no_fix_cycles` gate reports `Passed: false`

### Quality flags raised
Call `ClassifyRisk` with one quality flag (`low_cost`).
- `IsLowRisk` is `false`
- `no_quality_flags` gate reports `Passed: false`

### Multiple gates fail
Call `ClassifyRisk` with 201 lines and 11 files.
- `IsLowRisk` is `false`
- Both `max_lines` and `max_files` gates report `Passed: false`
- Other gates still report their individual results
