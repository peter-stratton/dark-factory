# Scenario: Fresh agent with structured handoff on retry 3+

Relates to: Issue #371

## Setup
- The `internal/agent/` package contains `Retry()` and `ProcessIssue()`
- A stub `Run()` function is used to simulate agent invocations
- A stub GitHub API returns PR comments containing `## Implementation Notes`
  and `## Review Notes` sections
- `PromptData` has a `HandoffContext` field

## Cases

### Fresh mode skips session ID
Call `Retry()` with `prevSessionID: "sess-1"` and `handoffContext: "Prior attempts summary..."`.
- `GODARK_SESSION_ID` is NOT set in the agent's environment
- The agent starts a fresh session

### Resume mode preserved when handoff is empty
Call `Retry()` with `prevSessionID: "sess-1"` and `handoffContext: ""`.
- `GODARK_SESSION_ID` is set to `"sess-1"` in the agent's environment

### Handoff context rendered in retry prompt
Render `implementer_retry.txt` with `HandoffContext` set to `"Attempt 1: added widget\nReview: rejected for missing tests"`.
- The rendered prompt contains the handoff text
- The rendered prompt contains a preamble indicating this is a fresh session

### Handoff context omitted when empty
Render `implementer_retry.txt` with `HandoffContext` set to `""`.
- The rendered prompt does not contain a fresh-session preamble
- The rendered prompt does not contain a handoff section

### Handoff assembly extracts structured comments
Given PR comments containing:
  - Comment 1: `## Implementation Notes\nAdded widget package`
  - Comment 2: `## Review Notes\n### Changes Requested\nMissing error handling`
  - Comment 3: `## Implementation Notes\nFixed error handling`
Call the handoff assembly function.
- The result contains "Added widget package"
- The result contains "Missing error handling"
- The result contains "Fixed error handling"
- The entries are in chronological order

### Handoff assembly with no structured comments
Given PR comments that contain no `## Implementation Notes` or `## Review Notes` sections.
- The handoff assembly function returns an empty string

### Handoff assembly includes quality review notes
Given PR comments containing a `## Quality Review Notes` section.
- The result includes the quality review content

### All callers of Retry updated
Call `Retry()` from the quality review retry path with empty `handoffContext`.
- The call compiles and behaves identically to pre-change behavior
- Session resumption works as before
