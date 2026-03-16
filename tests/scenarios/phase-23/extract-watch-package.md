# Scenario: Extract polling logic into internal/watch/ package

Relates to: Issue #516

## Setup
- `internal/watch/` package exists with `Watch` struct, `New()`, `Run()`, `PollOnce()` methods
- `internal/cmd/watch.go` reduced to thin wrapper calling `watch.New()` and `w.Run()`
- `docs/architecture.json` updated with `internal/watch/` in orchestration layer
- Testability seams moved from cmd to watch package

## Cases

### Watch struct initializes
Call `watch.New(cfg, prompts, authEnv, logger)`.
- Returns a non-nil `*Watch`
- Processed map is initialized (empty)

### Run exits on context cancel
Call `w.Run(ctx)` with an immediately cancelled context.
- Returns nil (clean exit)
- No panic

### PollOnce processes PRs
Stub `ListPRsWithLabel` to return 1 PR. Stub `FetchPRReviews` to return a CHANGES_REQUESTED review. Call `w.PollOnce(ctx)`.
- HandleChangesRequested is invoked for the PR
- Review ID is added to the processed map

### Cmd layer is thin wrapper
Read `internal/cmd/watch.go`.
- File creates a `watch.Watch` instance and calls `Run()`
- No polling logic remains in the cmd layer

### Architecture vet passes
Run `godark vet architecture`.
- No violations for `internal/watch/` imports
- `internal/watch/` listed in orchestration layer paths

### Existing tests pass
Run `go test ./internal/watch/`.
- All migrated tests pass
- Same coverage as before extraction
