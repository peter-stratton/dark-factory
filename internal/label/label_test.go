package label

import "testing"

func TestTransition(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		// Valid transitions from issue spec
		{"first label", "", AwaitingHumanReview, true},
		{"auto-merge first label", "", ReadyToMerge, true},
		{"human requested changes", AwaitingHumanReview, FixingReviewFeedback, true},
		{"loop back", FixingReviewFeedback, AwaitingHumanReview, true},
		{"approval", AwaitingHumanReview, ReadyToMerge, true},
		{"clear from ready", ReadyToMerge, "", true},
		{"clear from awaiting", AwaitingHumanReview, "", true},
		{"clear from fixing", FixingReviewFeedback, "", true},

		// Invalid transitions from issue spec
		{"invalid skip", ReadyToMerge, FixingReviewFeedback, false},

		// Additional invalid transitions
		{"ready to awaiting", ReadyToMerge, AwaitingHumanReview, false},
		{"fixing to ready", FixingReviewFeedback, ReadyToMerge, false},
		{"unknown from", "godark:unknown", AwaitingHumanReview, false},
		{"self transition awaiting", AwaitingHumanReview, AwaitingHumanReview, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Transition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("Transition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestAll(t *testing.T) {
	all := All()
	if len(all) != 4 {
		t.Errorf("All() returned %d labels, want 4", len(all))
	}

	want := map[string]bool{
		AwaitingHumanReview:  true,
		FixingReviewFeedback: true,
		ReadyToMerge:         true,
		NoDark:               true,
	}
	for _, label := range all {
		if !want[label] {
			t.Errorf("All() contains unexpected label %q", label)
		}
		delete(want, label)
	}
	for label := range want {
		t.Errorf("All() missing label %q", label)
	}
}
