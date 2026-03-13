# Scenario: Migrate callers to mdutil.WalkMarkdownFiles

Relates to: Issue #400

## Setup
- The `internal/mdutil` package exists with `WalkMarkdownFiles`
- The three caller files have been updated: `punchlist/punchlist.go`,
  `vet/scenarios.go`, `agent/guardrails.go`

## Cases

### Punchlist reads scenario specs
Call `punchlist.ReadScenarioSpec()` with a scenario directory containing a spec file with `Relates to: Issue #42`.
- The function returns the spec content for issue 42
- No `filepath.WalkDir` call remains inline in punchlist.go

### Vet validates scenarios
Call `vet.ValidateScenarios()` with a scenario directory containing valid `.md` files.
- Validation collects all `.md` files
- No `filepath.WalkDir` call remains inline in vet/scenarios.go

### Guardrails detects scenario spec
Call `guardrails.HasScenarioSpec()` with a directory containing a matching spec.
- Returns true
- No `filepath.WalkDir` call remains inline in guardrails.go

### Init.go unchanged
Read `internal/cmd/init.go`.
- It still uses `fs.WalkDir` on the embedded filesystem (not `mdutil`)
