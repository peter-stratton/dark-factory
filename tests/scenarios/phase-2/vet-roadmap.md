# Scenario: Roadmap validation

Relates to: Issue #17

## Setup
- The vet roadmap package (`internal/vet`) is imported directly
- A temporary directory containing planning doc markdown files
- The `github.CommandRunner` variable is stubbed to return controlled issue and milestone data
- No external services or network access required

## Cases

### Valid planning doc produces no findings
A planning doc that references issues `## Issue 14:` through `## Issue 17:`, all of which exist on GitHub and are in the correct milestone with matching phase labels.
- No error findings are produced
- No warning findings are produced

### Phantom issue reference
A planning doc contains `## Issue 99:` but issue #99 does not exist on GitHub.
- A warning finding is produced
- The finding message mentions issue #99
- The finding location is the planning doc file path

### Orphaned issue in milestone
Issue #5 is in the Phase 2 milestone on GitHub but is not mentioned in any planning doc in `docs/planning/`.
- A warning finding is produced
- The finding message mentions issue #5 and "not referenced in any planning doc"

### Phase label does not match milestone
An issue has label `phase-1` but is assigned to the `Phase 2` milestone.
- An error finding is produced
- The finding message mentions the label-milestone mismatch
- The finding location identifies the issue number

### Issues referenced across multiple planning docs
Issue #14 is mentioned in `phase-2-quality-and-vetting.md` and also referenced in another planning doc.
- Issue #14 is considered covered
- No orphan warning is produced for issue #14

### No planning docs found
The `docs/planning/` directory is empty (or does not exist).
- A warning finding is produced
- The finding message mentions that no planning docs were found
