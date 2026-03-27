# Scenario: Require --tag on vet scenarios and scope to phase subdirectory

Relates to: Issue #678

## Setup
- `godark vet scenarios` currently accepts `--tag` as optional
- Scenario specs are organized in `tests/scenarios/phase-N/` subdirectories
- `milestoneToLabel()` in `vet_helpers.go` converts milestone titles to phase
  labels (e.g., "Phase 30: Spec Tightening" to "phase-30")
- `vet.ValidateScenarios()` accepts a `scenarioDir` string parameter

## Cases

### Missing tag returns error
Run `godark vet scenarios --repo owner/repo` without `--tag` or `--milestone`.
- Command exits with a non-zero exit code
- Error message contains "required"

### Missing repo returns error
Run `godark vet scenarios --tag phase-30` without `--repo`.
- Command exits with a non-zero exit code
- Error message mentions "--repo"

### Scoped to phase subdirectory
Create a temp directory with `phase-1/` and `phase-2/` subdirectories, each
containing a valid scenario spec. Run with `--tag phase-1`.
- Only specs in `phase-1/` are validated
- Specs in `phase-2/` are not mentioned in findings

### Missing phase subdirectory errors
Run with `--tag phase-99` where `tests/scenarios/phase-99/` does not exist.
- Command exits with a non-zero exit code
- Error message names the missing path (e.g., "tests/scenarios/phase-99")

### Valid invocation succeeds
Run `godark vet scenarios --tag phase-1 --repo owner/repo` with valid specs
in the `phase-1/` subdirectory.
- Command exits with code 0 (assuming specs are valid)
- Findings reference only files in the phase-1 subdirectory

### Build and vet pass
Run `go build ./...` and `go vet ./...`.
- Both complete with exit code 0
