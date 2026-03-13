# Scenario: Recon orchestrator integration

Relates to: Issue #369

## Setup
- The `internal/agent/` package contains `Recon()`, `Implement()`, and
  `ProcessIssue()`
- A stub `Run()` function is used to simulate agent invocations
- `Prompts.Recon` is configurable (empty string to disable)

## Cases

### Recon runs before implement when configured
Call `ProcessIssue()` with `Prompts.Recon` set to a valid template.
- `Recon()` is invoked before `Implement()`
- The recon agent uses the `recon` role

### Recon output passed to implementer
Stub `Run()` to return `ResultText: "Found 3 files to modify"` for the recon call.
- `Implement()` is called with `reconBrief` equal to `"Found 3 files to modify"`
- The implementer's rendered prompt contains the recon brief text

### Recon skipped when unconfigured
Call `ProcessIssue()` with `Prompts.Recon` set to `""`.
- `Recon()` is not called
- `Implement()` is called with empty `reconBrief`
- No recon-related log messages are emitted

### Recon failure is non-blocking
Stub `Run()` to return an error for the recon call.
- A warning is logged mentioning the recon failure
- `Implement()` is still called with empty `reconBrief`
- `ProcessIssue()` does not return a failed outcome due to recon

### Recon timeout is non-blocking
Stub `Run()` to return a result with `TimedOut: true` for the recon call.
- A warning is logged
- `Implement()` is still called with empty `reconBrief`

### Implement signature accepts reconBrief
Call `Implement()` directly with `reconBrief: "some context"`.
- `PromptData.ReconBrief` is set to `"some context"` in the rendered prompt
- The agent is invoked with the `implementer` role (not `recon`)
