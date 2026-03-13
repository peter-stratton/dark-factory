# Scenario: Truncation limits grouped into struct

Relates to: Issue #389

## Setup
- A `TruncationLimits` struct is defined with `VerifyOutput` and `PRDiff` fields
- `verify.go` and `punchlist.go` read limits from the struct

## Cases

### Default verify output limit
Run the verify step with default limits.
- Output is truncated at 4096 bytes (matching the original `verifyOutputLimit`)

### Default PR diff limit
Generate a punchlist with default limits and a PR diff exceeding 30000 bytes.
- The diff passed to the punchlist agent is truncated at 30000 bytes

### Custom limits override defaults
Set `TruncationLimits{VerifyOutput: 1024, PRDiff: 5000}`.
- Verify output truncates at 1024
- PR diff truncates at 5000

### No package-level constants remain
Search `internal/agent/verify.go` for `verifyOutputLimit` as a package-level const.
- Not found (value comes from struct)
Search `internal/agent/punchlist.go` for `maxPRDiffLen` as a package-level const.
- Not found (value comes from struct)
