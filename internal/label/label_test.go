package label

import (
	"testing"
)

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

func TestNew_DefaultPrefix(t *testing.T) {
	l := New("godark")
	if l.InProgress != InProgress {
		t.Errorf("InProgress = %q, want %q", l.InProgress, InProgress)
	}
	if l.AwaitingHumanReview != AwaitingHumanReview {
		t.Errorf("AwaitingHumanReview = %q, want %q", l.AwaitingHumanReview, AwaitingHumanReview)
	}
	if l.FixingReviewFeedback != FixingReviewFeedback {
		t.Errorf("FixingReviewFeedback = %q, want %q", l.FixingReviewFeedback, FixingReviewFeedback)
	}
	if l.ReadyToMerge != ReadyToMerge {
		t.Errorf("ReadyToMerge = %q, want %q", l.ReadyToMerge, ReadyToMerge)
	}
	if l.NoDark != NoDark {
		t.Errorf("NoDark = %q, want %q", l.NoDark, NoDark)
	}
}

func TestNew_CustomPrefix(t *testing.T) {
	l := New("df")
	if l.InProgress != "df-in-progress" {
		t.Errorf("InProgress = %q, want %q", l.InProgress, "df-in-progress")
	}
	if l.AwaitingHumanReview != "df:awaiting-human-review" {
		t.Errorf("AwaitingHumanReview = %q, want %q", l.AwaitingHumanReview, "df:awaiting-human-review")
	}
	if l.FixingReviewFeedback != "df:fixing-review-feedback" {
		t.Errorf("FixingReviewFeedback = %q, want %q", l.FixingReviewFeedback, "df:fixing-review-feedback")
	}
	if l.ReadyToMerge != "df:ready-to-merge" {
		t.Errorf("ReadyToMerge = %q, want %q", l.ReadyToMerge, "df:ready-to-merge")
	}
	if l.NoDark != "nodark" {
		t.Errorf("NoDark = %q, want %q", l.NoDark, "nodark")
	}
}

func TestLabelsTransition(t *testing.T) {
	l := New("df")
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"first label", "", l.AwaitingHumanReview, true},
		{"auto-merge first label", "", l.ReadyToMerge, true},
		{"human requested changes", l.AwaitingHumanReview, l.FixingReviewFeedback, true},
		{"loop back", l.FixingReviewFeedback, l.AwaitingHumanReview, true},
		{"approval", l.AwaitingHumanReview, l.ReadyToMerge, true},
		{"clear from ready", l.ReadyToMerge, "", true},
		{"clear from awaiting", l.AwaitingHumanReview, "", true},
		{"clear from fixing", l.FixingReviewFeedback, "", true},
		{"invalid skip", l.ReadyToMerge, l.FixingReviewFeedback, false},
		{"fixing to ready", l.FixingReviewFeedback, l.ReadyToMerge, false},
		{"unknown from", "df:unknown", l.AwaitingHumanReview, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := l.Transition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("Transition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestLabelsAll(t *testing.T) {
	l := New("df")
	all := l.All()
	if len(all) != 3 {
		t.Errorf("Labels.All() returned %d labels, want 3", len(all))
	}

	want := map[string]bool{
		"df:awaiting-human-review":  true,
		"df:fixing-review-feedback": true,
		"df:ready-to-merge":         true,
	}
	for _, name := range all {
		if !want[name] {
			t.Errorf("Labels.All() contains unexpected label %q", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("Labels.All() missing label %q", name)
	}
}

func TestLabelsSpecs(t *testing.T) {
	l := New("df")
	if len(l.Specs) != 3 {
		t.Errorf("Labels.Specs has %d entries, want 3", len(l.Specs))
	}
	for _, s := range l.Specs {
		if s.Name == "" {
			t.Error("Labels.Specs contains entry with empty Name")
		}
		if s.Color == "" {
			t.Error("Labels.Specs contains entry with empty Color")
		}
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
