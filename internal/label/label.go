package label

// PR lifecycle label constants communicate state to humans at a glance.
const (
	AwaitingHumanReview  = "godark:awaiting-human-review"
	FixingReviewFeedback = "godark:fixing-review-feedback"
	ReadyToMerge         = "godark:ready-to-merge"
)

// validTransitions lists the allowed from→to label pairs. The empty string
// represents "no label" (initial state or after merge/close).
var validTransitions = map[string]map[string]bool{
	"": {
		AwaitingHumanReview: true,
		ReadyToMerge:        true,
	},
	AwaitingHumanReview: {
		FixingReviewFeedback: true,
		ReadyToMerge:         true,
		"":                   true,
	},
	FixingReviewFeedback: {
		AwaitingHumanReview: true,
		"":                  true,
	},
	ReadyToMerge: {
		"": true,
	},
}

// Transition reports whether moving from label from to label to is a legal
// state transition. The empty string represents the absence of a PR lifecycle
// label (e.g. before the first label is applied, or after merge/close).
func Transition(from, to string) bool {
	tos, ok := validTransitions[from]
	if !ok {
		return false
	}
	return tos[to]
}

// All returns all PR lifecycle label strings. Useful for bulk operations such
// as ensuring labels exist in the repository before applying them.
func All() []string {
	return []string{AwaitingHumanReview, FixingReviewFeedback, ReadyToMerge}
}
