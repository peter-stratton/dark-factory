# Scenario: Migrate test infrastructure from Runner to SandboxRunner stubs

Relates to: Issue #671

## Setup
- Issues #668 and #670 are complete: `NoSandbox` field and `Runner` var no
  longer exist in production code
- Test files still reference `NoSandbox: true` and/or stub `Runner`
- `setupLoopTest()` in `loop_test.go` stubs both `Runner` and sandbox runners
  (`sandbox.CommandRunner`, `sandbox.SplitRunner`)

## Cases

### No test references NoSandbox
Search all `*_test.go` files for `NoSandbox`.
- No matches found

### No test stubs host Runner
Search all `*_test.go` files for `Runner =` (excluding `SandboxRunner`).
- No matches found

### Loop test config updated
Read `loopConfig()` in `internal/agent/loop_test.go`.
- No `NoSandbox` field is set

### Host-only tests deleted
Search for `TestProcessIssue_VerifyHostModeUnchangedWithNoSandbox` and
`TestHostRunner_` in test files.
- No matches found

### Config NoSandbox tests deleted
Search `internal/config/config_test.go` for `TestNoSandbox`.
- No matches found

### Command NoSandbox tests deleted
Search `internal/cmd/run_test.go` for `TestNoSandbox`.
- No matches found

### Doctor NoSandbox tests deleted
Search `internal/doctor/doctor_test.go` for `NoSandbox`.
- No matches found

### Root test flag list updated
Read the flag list assertion in `internal/cmd/root_test.go`.
- `"no-sandbox"` does not appear in the expected flags

### Full test suite passes
Run `go test ./...`.
- Exits with code 0

### Race detector clean
Run `go test ./... -race`.
- Exits with code 0
