# Scenario: Non-blocking agent result handler

Relates to: Issue #393

## Setup
- The `internal/agent/loop.go` file contains `handleNonBlockingResult()`
- A fake `writeHook` function that records calls

## Cases

### Error case returns empty and writes hook
Call `handleNonBlockingResult(nil, errors.New("fail"), "spec-gen", logger, fakeHook)`.
- Returns empty string
- `fakeHook` was called once with a `StepResult` containing error text

### Timeout case returns empty and writes hook
Call `handleNonBlockingResult(&Result{TimedOut: true}, nil, "recon", logger, fakeHook)`.
- Returns empty string
- `fakeHook` was called once with a `StepResult` where `Error` contains "timed out"

### Success case returns result text and writes hook
Call `handleNonBlockingResult(&Result{ResultText: "findings..."}, nil, "recon", logger, fakeHook)`.
- Returns `"findings..."`
- `fakeHook` was called once with a `StepResult` containing the output

### Nil hook does not panic
Call `handleNonBlockingResult(&Result{ResultText: "ok"}, nil, "spec-gen", logger, nil)`.
- Returns `"ok"`
- Does not panic

### Spec-gen block uses helper
Read `internal/agent/loop.go` spec-gen handling (around line 79).
- The block calls `handleNonBlockingResult` instead of inline error/timeout/success branches

### Recon block uses helper
Read `internal/agent/loop.go` recon handling (around line 123).
- The block calls `handleNonBlockingResult` instead of inline branches
