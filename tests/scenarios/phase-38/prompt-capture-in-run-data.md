# Scenario: Prompt capture in run data

Relates to: Issue #787

## Setup
- An `agent.Result` struct is available with a `Prompt` field
- A `rundata.StepResult` struct is available with a `Prompt` field
- A stats.db database has been migrated with the `prompt` column on `step_results`
- A `stats.StepResultRecord` struct is available with a `Prompt` field

## Cases

### ResultToStep copies prompt field
- GIVEN an `agent.Result` with `Prompt` set to a non-empty string
- WHEN `ResultToStep` is called
- THEN the returned `rundata.StepResult` has `Prompt` matching the input

### ResultToStep handles empty prompt
- GIVEN an `agent.Result` with `Prompt` set to an empty string
- WHEN `ResultToStep` is called
- THEN the returned `rundata.StepResult` has `Prompt` as empty string

### Schema migration adds prompt column idempotently
- GIVEN a stats.db database
- WHEN `migrate` is called twice
- THEN no error occurs on the second call (duplicate column is suppressed)

### StepResult prompt round-trips through stats.db
- GIVEN a `StepResultRecord` with `Prompt` set to a multi-line string
- WHEN written via `doWriteStepResult` and queried back via `QueryStepsByTraceID`
- THEN the returned record's `Prompt` field matches the original

### StepResult prompt omitted from JSON when empty
- GIVEN a `rundata.StepResult` with `Prompt` set to an empty string
- WHEN marshaled to JSON
- THEN the `"prompt"` key is absent from the output

### StepResult prompt present in JSON when set
- GIVEN a `rundata.StepResult` with `Prompt` set to a non-empty string
- WHEN marshaled to JSON
- THEN the `"prompt"` key is present with the correct value

### Implementer run captures rendered prompt
- GIVEN a `godark run` execution that processes an issue through the implementer step
- WHEN the implementer step completes and its result is written to `implement.json`
- THEN the `implement.json` file contains a non-empty `prompt` field with the rendered implementer prompt

### Reviewer run captures rendered prompt
- GIVEN a `godark run` execution that processes an issue through functional review
- WHEN the functional review step completes and its result is written to `functional-review.json`
- THEN the `functional-review.json` file contains a non-empty `prompt` field with the rendered reviewer prompt
