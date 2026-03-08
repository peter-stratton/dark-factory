# Scenario: PR lifecycle labels and state machine

Relates to: Issue #241

## Setup
- The `internal/label/` package is tested via Go unit tests
- No external dependencies — pure constants and logic

## Cases

### Label constants exported
Import `internal/label` and reference the three PR lifecycle constants.
- `label.AwaitingHumanReview` equals `"godark:awaiting-human-review"`
- `label.FixingReviewFeedback` equals `"godark:fixing-review-feedback"`
- `label.ReadyToMerge` equals `"godark:ready-to-merge"`

### All returns all labels
Call `label.All()`.
- Returns a slice of length 3
- Contains all three PR lifecycle label constants

### Valid transition from empty to awaiting
Call `label.Transition("", label.AwaitingHumanReview)`.
- Returns `true`

### Valid transition from empty to ready
Call `label.Transition("", label.ReadyToMerge)`.
- Returns `true`

### Valid transition awaiting to fixing
Call `label.Transition(label.AwaitingHumanReview, label.FixingReviewFeedback)`.
- Returns `true`

### Valid transition fixing to awaiting
Call `label.Transition(label.FixingReviewFeedback, label.AwaitingHumanReview)`.
- Returns `true`

### Valid transition awaiting to ready
Call `label.Transition(label.AwaitingHumanReview, label.ReadyToMerge)`.
- Returns `true`

### Invalid transition ready to fixing
Call `label.Transition(label.ReadyToMerge, label.FixingReviewFeedback)`.
- Returns `false`

### Clear labels always valid
Call `label.Transition(label.ReadyToMerge, "")`.
- Returns `true`

Call `label.Transition(label.AwaitingHumanReview, "")`.
- Returns `true`
