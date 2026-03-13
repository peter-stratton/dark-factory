# Scenario: Consolidate config and tag/milestone resolution

Relates to: Issue #395

## Setup
- `internal/cmd/run.go` uses `resolveTag()` from `vet_helpers.go`
- A stub `ResolveMilestoneByTag` returns a milestone title

## Cases

### Tag resolves milestone
Set `--tag v1.0` and `--repo owner/repo` on the run command. Stub `ResolveMilestoneByTag` to return `"Phase 1"`.
- The resolved milestone is `"Phase 1"`

### Mutual exclusivity enforced
Set both `--tag v1.0` and `--milestone "Phase 1"` on the run command.
- Returns an error indicating mutual exclusivity

### Missing repo with tag errors
Set `--tag v1.0` without `--repo` and no config file.
- Returns an error indicating `--repo` is required

### No inline resolution in run.go
Read `internal/cmd/run.go`.
- No inline `cmd.Flags().Changed("tag")` block with nested config loading
- Uses `resolveTag(cmd)` or equivalent shared helper
