# Scenario: godark vet architecture subcommand

Relates to: Issue #128

## Setup
- The `internal/vet` package's `ValidateArchitecture` function is tested
  directly via Go unit tests with in-memory `layers.Definition` structs
- The CLI subcommand is tested by writing temp `architecture.json` files and
  invoking the command
- No external services, Docker, or GitHub API required

## Cases

### Valid DAG produces no findings
Pass a `Definition` with layers: types (no imports), config (imports types),
orchestrator (imports config).
- `ValidateArchitecture` returns a report with zero findings

### Simple cycle detected
Pass a `Definition` with layers: A (imports B), B (imports A).
- The report contains an error finding
- The finding message names both layers A and B

### Transitive cycle detected
Pass a `Definition` with layers: A (imports B), B (imports C), C (imports A).
- The report contains an error finding
- The finding message names all three layers A, B, and C

### Self-import detected
Pass a `Definition` with a layer A that imports itself.
- The report contains an error finding
- The finding message names layer A

### Clean 5-layer architecture
Pass a well-structured `Definition` with 5 layers in a strict hierarchy (types,
config, deps, github, orchestrator) with correct import chains.
- The report contains zero findings

### Missing architecture file exits 0
Run `godark vet architecture` in a directory without `docs/architecture.json`.
- The command exits with code 0
- Output contains an info message about the missing file

### JSON output flag
Run `godark vet architecture --json` with a valid `docs/architecture.json`.
- Output is valid JSON
- The JSON contains a `findings` array

### Architecture file flag
Write an `architecture.json` to a non-default path. Run
`godark vet architecture --architecture-file <path>`.
- The command reads and validates the file at the specified path
