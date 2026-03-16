package label

// InProgress is the GitHub label applied to issues during an active godark run.
const InProgress = "godark-in-progress"

// NoDark is the GitHub label that marks an issue as a human-only task.
// Issues with this label are skipped by godark agents during run and implement.
const NoDark = "nodark"

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

// Spec holds the full metadata for a PR lifecycle label.
type Spec struct {
	Name        string
	Color       string
	Description string
}

// Specs is the authoritative list of all PR lifecycle labels with their
// metadata. Drive both EnsureLabel calls and the All helper from this slice
// so that adding a new label only requires a single change here.
var Specs = []Spec{
	{Name: AwaitingHumanReview, Color: "4A90E2", Description: "PR approved by AI, awaiting human review"},
	{Name: FixingReviewFeedback, Color: "F5D76E", Description: "AI is addressing review feedback"},
	{Name: ReadyToMerge, Color: "50C878", Description: "All reviews passed, ready to merge"},
	{Name: NoDark, Color: "CCCCCC", Description: "Human-only task — skipped by godark agents"},
}

// All returns all PR lifecycle label strings. Useful for bulk operations such
// as ensuring labels exist in the repository before applying them.
func All() []string {
	names := make([]string, len(Specs))
	for i, s := range Specs {
		names[i] = s.Name
	}
	return names
}
