# Scenario: Extract CLI flag parser helper

Relates to: Issue #394

## Setup
- A `parseCLIFlags(cmd)` helper exists in `internal/cmd/`
- Both `run.go` and `implement.go` use the helper

## Cases

### All flags parsed
Create a Cobra command with all flags set: `--repo owner/repo`, `--max-retries 5`, `--no-sandbox`, `--auto-merge-feature all`, `--auto-merge-rollup manual`, `--base-branch develop`, `--default-branch main`. Call `parseCLIFlags(cmd)`.
- Returned `CLIFlags.Repo` points to `"owner/repo"`
- Returned `CLIFlags.MaxRetries` points to `5`
- Returned `CLIFlags.NoSandbox` points to `true`

### No flags changed
Create a Cobra command with no flags explicitly set. Call `parseCLIFlags(cmd)`.
- All pointer fields in the returned `CLIFlags` are nil

### Partial flags
Create a Cobra command with only `--repo` set. Call `parseCLIFlags(cmd)`.
- `CLIFlags.Repo` is non-nil
- Other pointer fields are nil

### Run.go uses helper
Read `internal/cmd/run.go`.
- No inline `cmd.Flags().Changed()` blocks for the 7 shared flags

### Implement.go uses helper
Read `internal/cmd/implement.go`.
- No inline `cmd.Flags().Changed()` blocks for the 7 shared flags
