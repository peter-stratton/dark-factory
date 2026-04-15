# Scenario: Issue snapshot export and import

Relates to: Issue #780

## Setup
- A GitHub repo with a milestone containing 3+ issues, some with `Blocked by` dependencies
- godark CLI built with the `bench` subcommand available

## Cases

### Export issues for a milestone
- GIVEN a milestone "Phase 37: Benchmarking Framework" with 4 issues in the repo
- WHEN `godark bench snapshot --milestone "Phase 37: Benchmarking Framework"` is run
- THEN a JSON file is written containing all 4 issues with titles, bodies, labels, and dependency info

### Snapshot includes metadata
- GIVEN a milestone with issues
- WHEN a snapshot is exported
- THEN the JSON file includes `repo`, `milestone`, `godark_version`, and `created_at` fields

### Restore creates issues from snapshot
- GIVEN a snapshot JSON file with 3 issues
- WHEN `godark bench restore <snapshot-file>` is run against a repo with no existing issues
- THEN 3 new issues are created in the repo with matching titles, bodies, and labels

### Dependency references remapped on restore
- GIVEN a snapshot where issue B has `Blocked by #100` referencing issue A (original #100)
- WHEN the snapshot is restored and issue A is created as #200
- THEN issue B's body contains `Blocked by #200` (the new number), not `#100`

### Duplicate detection on restore
- GIVEN a snapshot with 3 issues and a repo that already has an issue with a matching title
- WHEN `godark bench restore` is run
- THEN a warning is printed for the duplicate and it is not recreated

### Empty milestone produces valid snapshot
- GIVEN a milestone with no issues
- WHEN `godark bench snapshot` is run
- THEN a JSON file is written with an empty `issues` array and valid metadata

### Restore assigns milestone
- GIVEN a snapshot with `milestone: "Phase 37: Benchmarking Framework"`
- WHEN the snapshot is restored
- THEN all created issues are assigned to the matching milestone in the target repo
