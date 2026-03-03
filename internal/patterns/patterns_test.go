package patterns

import "testing"

func TestIssueRef_MatchesRefs(t *testing.T) {
	matches := IssueRef.FindAllStringSubmatch("See #1, #42, and #100", -1)
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}
	want := []string{"1", "42", "100"}
	for i, m := range matches {
		if m[1] != want[i] {
			t.Errorf("match[%d] = %q, want %q", i, m[1], want[i])
		}
	}
}

func TestIssueRef_NoMatch(t *testing.T) {
	matches := IssueRef.FindAllString("no issue refs here", -1)
	if len(matches) != 0 {
		t.Errorf("expected no matches, got %v", matches)
	}
}

func TestPhase_MatchesPhaseN(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Phase 1", "1"},
		{"phase 2: Something", "2"},
		{"PHASE 10", "10"},
	}
	for _, tt := range tests {
		m := Phase.FindStringSubmatch(tt.input)
		if m == nil || m[1] != tt.want {
			t.Errorf("Phase.FindStringSubmatch(%q) = %v, want [_, %q]", tt.input, m, tt.want)
		}
	}
}

func TestPhase_NoMatch(t *testing.T) {
	inputs := []string{"not a phase", "Phased approach", "1 Phase"}
	for _, s := range inputs {
		if Phase.MatchString(s) {
			t.Errorf("Phase should not match %q", s)
		}
	}
}
