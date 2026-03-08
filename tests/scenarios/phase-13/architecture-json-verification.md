# Scenario: Update architecture.json for Phase 13 packages

Relates to: Issue #242

## Setup
- The `docs/architecture.json` file with layer definitions
- The `godark vet architecture` command

## Cases

### Label package in foundation layer
Read `docs/architecture.json` and inspect the foundation layer.
- `"internal/label/"` is listed in the foundation layer `paths` array

### Vet architecture passes
Run `godark vet architecture` after all Phase 13 code is merged.
- Command exits with code 0
- No findings reported

### No unknown package directories
List all Go package directories under `internal/`.
- Every directory is covered by at least one layer's `paths` in architecture.json
