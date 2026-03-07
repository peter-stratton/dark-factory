# Scenario: Architecture JSON verification for Phase 12

Relates to: Issue #226

## Setup
- All Phase 12 issues have been merged
- The `docs/architecture.json` layer definitions
- The `godark vet architecture` command

## Cases

### Vet passes after all Phase 12 merges
Run `godark vet architecture` from the repo root after all Phase 12 PRs are merged.
- Command exits 0
- No findings or violations printed

### No new package directories outside layer definitions
List all `.go` files created or modified in Phase 12.
- Every file is within a path covered by an existing layer in `docs/architecture.json`
- No new top-level package directories were created outside of `internal/config/`, `internal/agent/`, `internal/skills/`, or other defined layer paths

### Config changes stay in foundation layer
All modifications to `internal/config/` remain in the foundation layer.
- `internal/config/` is listed in the foundation layer's `paths` array
- No new imports from higher layers (cmd, orchestration, service, etc.) introduced in config package
