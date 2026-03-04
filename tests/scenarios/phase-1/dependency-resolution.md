# Scenario: Dependency resolution

Relates to: Issue #4

## Setup
- A set of `github.Issue` structs with various `Body` contents containing dependency declarations
- A closed-issue set (map of issue numbers that are considered closed)
- The deps package (`internal/deps`) is imported directly
- No external services or network access required

## Cases

### Parse standard dependency declaration
An issue body contains `**Blocked by**: #1 (Project scaffold)`.
- `ParseDeps` returns `[1]`

### Parse multiple dependencies on one line
An issue body contains `Depends on: #3, #5`.
- `ParseDeps` returns `[3, 5]`

### Parse case-insensitive dependency keywords
An issue body contains `BLOCKED BY: #4`.
- `ParseDeps` returns `[4]`

### No dependencies returns empty slice
An issue body contains no dependency declarations (just normal text with no `Blocked by` or `Depends on` lines).
- `ParseDeps` returns an empty/nil slice

### Unrelated issue references are not treated as dependencies
An issue body mentions `See #10 for context` but has no `Blocked by` or `Depends on` line.
- `ParseDeps` returns an empty/nil slice
- Only lines matching the dependency pattern are considered

### Filter unblocked issues with all deps closed
Three issues: A (no deps), B (depends on #1, which is closed), C (depends on #99, which is open).
- `FilterUnblocked` returns A and B
- C is excluded because #99 is not in the closed set

### Filter when all issues are blocked
Two issues, both depending on open issues.
- `FilterUnblocked` returns an empty slice

### Filter when no issues have dependencies
Three issues with no dependency declarations.
- `FilterUnblocked` returns all three issues in the original order

### HasDeps detects dependency lines
An issue body with `**Blocked by**: #1`.
- `HasDeps` returns `true`

### HasDeps returns false for plain text
An issue body with no dependency declarations.
- `HasDeps` returns `false`

### ClosedSet builds lookup map
Given issue numbers `[1, 5, 10]`.
- `ClosedSet` returns a map where `1`, `5`, and `10` are `true`
- Other numbers are `false` (not present)
