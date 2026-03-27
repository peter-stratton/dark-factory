# Scenario: Spec delta package

Relates to: Issue #679

## Setup
- New package at `internal/specdelta/` in the domain layer
- `docs/architecture.json` domain layer paths include `internal/specdelta/`
- Scenario specs use `# Scenario:`, `## Setup`, `## Cases`, `### <Case name>`
  structure

## Cases

### All cases added
Call `Diff("", after)` where after contains 3 cases.
- `Delta.AddedCases` has 3 entries matching the case names
- `Delta.RemovedCases` is empty
- `Delta.ChangedCases` is empty
- `IsEmpty()` returns false

### All cases removed
Call `Diff(before, "")` where before contains 2 cases.
- `Delta.RemovedCases` has 2 entries
- `Delta.AddedCases` is empty

### No change returns empty delta
Call `Diff(spec, spec)` with identical before and after.
- `IsEmpty()` returns true
- All slices are empty
- `SetupChanged` is false

### Mixed changes detected
Before has cases A, B, C. After has B (modified content), C (unchanged), D (new).
- `Delta.RemovedCases` contains "A"
- `Delta.ChangedCases` contains one entry with Name "B" and differing Before/After
- `Delta.AddedCases` contains "D"
- Case C does not appear in any delta list

### Setup change detected
Same cases in before and after, but different `## Setup` content.
- `Delta.SetupChanged` is true
- Case lists are empty

### Format produces markdown for non-empty delta
Call `Format()` on a delta with added, removed, and changed cases.
- Output contains a heading for added cases
- Output contains a heading for removed cases
- Output contains a heading for changed cases
- Output is valid markdown

### Format returns empty string for empty delta
Call `Format()` on a delta where `IsEmpty()` is true.
- Return value is ""

### Architecture layer compliance
Read `docs/architecture.json`.
- `internal/specdelta/` appears in the domain layer paths
- Package imports only `foundation` or `content` layer packages (or stdlib)

### Build and vet pass
Run `go build ./...` and `go vet ./...`.
- Both complete with exit code 0
