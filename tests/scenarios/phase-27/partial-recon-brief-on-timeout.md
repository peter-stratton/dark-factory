# Scenario: Partial recon brief on timeout

Relates to: Issue #632

## Setup
- `handleNonBlockingResult()` in `internal/agent/loop.go` is the function under test
- `Result` struct contains `TimedOut bool` and `ResultText string`
- A `writeHook` function captures the `StepResult` written to run data

## Cases

### Partial output returned on timeout
Create a `Result` with `TimedOut: true` and
`ResultText: "## Files\n- config.go\n- label.go"`.
Call `handleNonBlockingResult`.
- Return value is non-empty
- Return value contains "config.go"
- Return value is prefixed with a timeout notice

### Empty output on timeout returns empty string
Create a `Result` with `TimedOut: true` and empty `ResultText`.
Call `handleNonBlockingResult`.
- Return value is `""`

### Normal completion returns full text without prefix
Create a `Result` with `TimedOut: false` and
`ResultText: "## Full recon brief"`.
Call `handleNonBlockingResult`.
- Return value equals the full `ResultText`
- Return value does not contain a timeout notice prefix

### Run data annotated with partial timeout
Create a `Result` with `TimedOut: true` and non-empty `ResultText`.
Call `handleNonBlockingResult` with a write hook.
- The `StepResult` passed to the hook contains an error field mentioning timeout
- The `StepResult` contains the partial output text

### Spec generator and verify also get partial output
Create a `Result` with `TimedOut: true` and non-empty `ResultText`.
Call `handleNonBlockingResult` for a non-recon agent (e.g. spec generator).
- Return value is non-empty (same behavior as recon)
