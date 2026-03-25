# Scenario: Judge interventions in TUI and notifications

Relates to: Issue #648

## Setup
- `internal/tui/messages.go` contains a `JudgeIntervention` message type
- `internal/tui/model.go` handles the message and updates issue status display
- `internal/notify/notify.go` dispatches `judge_intervention` events

## Cases

### TUI renders intervention reason
Send a `JudgeIntervention` message to the TUI model for an issue.
- The issue row displays the intervention reason (e.g., "Killed: idle 300s")
- The status is visually distinct from a normal failure

### Notification dispatched on judge kill
Trigger a judge kill event through the notification system.
- The notification provider receives an event with type `judge_intervention`
- The event payload contains the rule name and detail message

### Nil intervention does not crash TUI
Send a step result with nil `JudgeIntervention` to the TUI model.
- No panic occurs
- The issue row displays its normal status
