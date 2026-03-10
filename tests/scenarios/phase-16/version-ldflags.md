# Scenario: Version ldflags wiring

Relates to: Issue #289

## Setup
- The repo has at least one commit (so `git describe --always` returns a hash)
- `make build` produces `bin/godark`

## Cases

### Local build has git version
Run `make build` and then `bin/godark version`.
- Output contains a commit hash or tag-based version, not `dev`

### Build time preserved
Run `make build` and then `bin/godark version`.
- Output includes a timestamp (not `unknown`)

### Default fallback without ldflags
Run `go run ./cmd/godark version`.
- Output contains `dev` as the version string
