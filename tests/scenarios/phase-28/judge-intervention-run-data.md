# Scenario: JudgeIntervention in run data

Relates to: Issue #643

## Setup
- `internal/rundata/writer.go` contains the `JudgeIntervention` type and `WriteJudgeIntervention` method
- `internal/rundata/reader.go` loads interventions into `IssueDetail.JudgeInterventions`
- Run data directory is a temp dir with standard structure

## Cases

### Write single intervention
Create a `Writer` for a temp run directory.
Call `WriteJudgeIntervention(42, intervention)` with a populated `JudgeIntervention`.
- File `issues/42/judge-interventions.json` exists
- File contains valid JSON with the intervention's rule, judgment, detail, and step

### Append multiple interventions
Call `WriteJudgeIntervention(42, first)` then `WriteJudgeIntervention(42, second)`.
- File contains a JSON array with both interventions
- Order is preserved (first, then second)

### Reader loads interventions
Write two interventions for issue 42 via the Writer.
Load the run via the Reader.
- `IssueDetail` for issue 42 has `JudgeInterventions` with length 2
- Each intervention has correct rule and step fields

### Missing file returns empty slice
Load a run via the Reader where no `judge-interventions.json` exists for an issue.
- `IssueDetail.JudgeInterventions` is an empty slice (not nil)
- No error returned
