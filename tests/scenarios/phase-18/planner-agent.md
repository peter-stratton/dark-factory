# Scenario: Planner agent function

Relates to: Issue #345

## Setup
- The `internal/agent/` package with mock `Runner` function variable
- Config with `Planning.Enabled: true` and valid repo
- A loaded `Prompts` struct with non-empty `Planner` template
- Mock runner captures the role and prompt passed to `Run`

## Cases

### Plan invocation uses reviewer role
Call `Plan()` with a valid issue and config.
- The agent is invoked with role `"reviewer"`
- The rendered prompt contains the issue title, issue body, and repo name

### Plan result contains plan text
Mock runner returns a successful result with `ResultText` populated.
- Returned `Result.ResultText` contains the plan content
- `Result.SessionID` is populated (but not used for resumption)

### Prompt data includes architecture context
Call `Plan()` with config pointing to existing architecture and conventions docs.
- Rendered prompt contains architecture doc content
- Rendered prompt contains conventions doc content

### Prompt data includes module context
Call `Plan()` with config containing a `modules:` block.
- Rendered prompt contains module names and dependency relationships

### Plan timeout returns TimedOut
Mock runner returns a result with `TimedOut: true`.
- Returned `Result.TimedOut` is `true`
- No error is returned (timeout is a result, not an error)

### Plan runner error
Mock runner returns an error.
- `Plan()` returns the error to the caller
- No result is returned
