package agent

import "testing"

// --- ParseReviewResult verdict tests ---

func TestParseReviewResult_Approved(t *testing.T) {
	stdout := "some output\nAGENT_RESULT=APPROVED\nmore output"
	got := ParseReviewResult(stdout)
	if got != "APPROVED" {
		t.Errorf("ParseReviewResult() = %q, want %q", got, "APPROVED")
	}
}

func TestParseReviewResult_ChangesRequested(t *testing.T) {
	stdout := "output\nAGENT_RESULT=CHANGES_REQUESTED\n"
	got := ParseReviewResult(stdout)
	if got != "CHANGES_REQUESTED" {
		t.Errorf("ParseReviewResult() = %q, want %q", got, "CHANGES_REQUESTED")
	}
}

func TestParseReviewResult_NotFound(t *testing.T) {
	stdout := "just some random output\nno result here"
	got := ParseReviewResult(stdout)
	if got != "" {
		t.Errorf("ParseReviewResult() = %q, want empty", got)
	}
}

func TestParseReviewResult_FirstMatchWins(t *testing.T) {
	stdout := "AGENT_RESULT=APPROVED\nAGENT_RESULT=CHANGES_REQUESTED\n"
	got := ParseReviewResult(stdout)
	if got != "APPROVED" {
		t.Errorf("ParseReviewResult() = %q, want %q (first match should win)", got, "APPROVED")
	}
}

func TestParseReviewResult_WhitespaceHandling(t *testing.T) {
	stdout := "  AGENT_RESULT=APPROVED  \n"
	got := ParseReviewResult(stdout)
	if got != "APPROVED" {
		t.Errorf("ParseReviewResult() = %q, want %q", got, "APPROVED")
	}
}

func TestParseReviewResult_ColonFormat(t *testing.T) {
	stdout := "AGENT RESULT: APPROVED\n"
	got := ParseReviewResult(stdout)
	if got != "APPROVED" {
		t.Errorf("ParseReviewResult() = %q, want %q", got, "APPROVED")
	}
}

func TestParseReviewResult_ColonFormatChanges(t *testing.T) {
	stdout := "AGENT RESULT: CHANGES_REQUESTED\n"
	got := ParseReviewResult(stdout)
	if got != "CHANGES_REQUESTED" {
		t.Errorf("ParseReviewResult() = %q, want %q", got, "CHANGES_REQUESTED")
	}
}

func TestParseReviewResult_CaseInsensitive(t *testing.T) {
	stdout := "Agent Result: Approved\n"
	got := ParseReviewResult(stdout)
	if got != "APPROVED" {
		t.Errorf("ParseReviewResult() = %q, want %q", got, "APPROVED")
	}
}

func TestParseReviewResult_OldFormatNotMatched(t *testing.T) {
	stdout := "REVIEW_RESULT=APPROVED\n"
	got := ParseReviewResult(stdout)
	if got != "" {
		t.Errorf("ParseReviewResult() = %q, want %q (old REVIEW_RESULT format must not match)", got, "")
	}
}

// --- ParseQualityResult verdict tests ---

func TestParseQualityResult_Approved(t *testing.T) {
	got := ParseQualityResult("some output\nAGENT_RESULT=APPROVED\nmore output")
	if got != "APPROVED" {
		t.Errorf("ParseQualityResult() = %q, want %q", got, "APPROVED")
	}
}

func TestParseQualityResult_ChangesRequested(t *testing.T) {
	got := ParseQualityResult("AGENT_RESULT=CHANGES_REQUESTED\n")
	if got != "CHANGES_REQUESTED" {
		t.Errorf("ParseQualityResult() = %q, want %q", got, "CHANGES_REQUESTED")
	}
}

func TestParseQualityResult_NoMatch(t *testing.T) {
	got := ParseQualityResult("no sentinel here")
	if got != "" {
		t.Errorf("ParseQualityResult() = %q, want %q", got, "")
	}
}

func TestParseQualityResult_OldFormatNotMatched(t *testing.T) {
	got := ParseQualityResult("QUALITY_RESULT=APPROVED\n")
	if got != "" {
		t.Errorf("ParseQualityResult() = %q, want %q (old QUALITY_RESULT format must not match)", got, "")
	}
}
