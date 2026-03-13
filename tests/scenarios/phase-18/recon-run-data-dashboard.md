# Scenario: Recon run data and dashboard

Relates to: Issue #370

## Setup
- The `internal/rundata/` package contains `Writer` implementing `RunDataHook`
- A temporary directory is used as the run data root
- The dashboard templates are in `internal/dashboard/templates/`

## Cases

### WriteReconResult creates recon.json
Call `WriteReconResult(42, step)` where `step` has `SessionID: "sess-1"`,
`Cost: 0.05`, `Duration: 30s`, and `Output: "Found 3 files"`.
- A file `<rundir>/42/recon.json` is created
- The JSON contains `"session_id":"sess-1"`
- The JSON contains `"cost":0.05`
- The JSON contains `"output":"Found 3 files"`

### WriteReconResult with empty output
Call `WriteReconResult(42, step)` where `step` has empty `Output`.
- `recon.json` is created successfully
- The JSON contains `"output":""`

### Read old run data without recon.json
Load a run data directory that has `implement.json` but no `recon.json`.
- No error is returned
- Recon data is nil or zero-valued

### Dashboard shows recon step when present
Render the issue detail page for a run that includes `recon.json`.
- The review chain timeline includes a "Recon" step before the "Implement" step
- The recon step shows duration and cost
- The recon brief text is expandable

### Dashboard omits recon step when absent
Render the issue detail page for a run without `recon.json`.
- The review chain timeline does not include a "Recon" step
- No error or blank section is rendered
