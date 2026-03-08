package dialogue

import "testing"

const fullImplComment = `## Implementation Notes

### Approach
Used a line-by-line scanner.

### Key Decisions
- Chose bufio.Scanner for simplicity.

### Known Limitations
- Does not handle deeply nested headers.

### Architecture
Placed in domain layer with no external dependencies.
`

const fullReviewComment = `## Review Notes

### Approved
- Clean implementation.

### Changes Requested
- Add more tests.

### Architecture Compliance
Layer boundaries respected.
`

func TestParseImplementationNotes_Full(t *testing.T) {
	t.Helper()
	got := ParseImplementationNotes(fullImplComment)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Approach != "Used a line-by-line scanner." {
		t.Errorf("Approach = %q, want %q", got.Approach, "Used a line-by-line scanner.")
	}
	if got.KeyDecisions != "- Chose bufio.Scanner for simplicity." {
		t.Errorf("KeyDecisions = %q, want %q", got.KeyDecisions, "- Chose bufio.Scanner for simplicity.")
	}
	if got.KnownLimitations != "- Does not handle deeply nested headers." {
		t.Errorf("KnownLimitations = %q, want %q", got.KnownLimitations, "- Does not handle deeply nested headers.")
	}
	if got.Architecture != "Placed in domain layer with no external dependencies." {
		t.Errorf("Architecture = %q, want %q", got.Architecture, "Placed in domain layer with no external dependencies.")
	}
	if got.Raw != fullImplComment {
		t.Errorf("Raw does not match original comment text")
	}
}

func TestParseImplementationNotes_Partial(t *testing.T) {
	body := `## Implementation Notes

### Approach
Simple approach.

### Key Decisions
Some decisions.
`
	got := ParseImplementationNotes(body)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Approach != "Simple approach." {
		t.Errorf("Approach = %q, want %q", got.Approach, "Simple approach.")
	}
	if got.KeyDecisions != "Some decisions." {
		t.Errorf("KeyDecisions = %q, want %q", got.KeyDecisions, "Some decisions.")
	}
	if got.KnownLimitations != "" {
		t.Errorf("KnownLimitations = %q, want empty string", got.KnownLimitations)
	}
	if got.Architecture != "" {
		t.Errorf("Architecture = %q, want empty string", got.Architecture)
	}
}

func TestParseReviewNotes_Full(t *testing.T) {
	got := ParseReviewNotes(fullReviewComment)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Approved != "- Clean implementation." {
		t.Errorf("Approved = %q, want %q", got.Approved, "- Clean implementation.")
	}
	if got.ChangesRequested != "- Add more tests." {
		t.Errorf("ChangesRequested = %q, want %q", got.ChangesRequested, "- Add more tests.")
	}
	if got.ArchitectureCompliance != "Layer boundaries respected." {
		t.Errorf("ArchitectureCompliance = %q, want %q", got.ArchitectureCompliance, "Layer boundaries respected.")
	}
	if got.Raw != fullReviewComment {
		t.Errorf("Raw does not match original comment text")
	}
}

func TestParseImplementationNotes_NotANotesComment(t *testing.T) {
	body := "This is just a regular PR comment with no special headers."
	got := ParseImplementationNotes(body)
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestParseReviewNotes_NotANotesComment(t *testing.T) {
	body := "LGTM! Great work."
	got := ParseReviewNotes(body)
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestParseComments_MultipleComments(t *testing.T) {
	impl1 := `## Implementation Notes

### Approach
First implementation.

### Key Decisions
Decision A.

### Known Limitations
None.

### Architecture
Domain layer.
`
	review1 := `## Review Notes

### Approved
Looks good.

### Changes Requested
Fix the typo.

### Architecture Compliance
Compliant.
`
	impl2 := `## Implementation Notes

### Approach
Second implementation after review.

### Key Decisions
Decision B.

### Known Limitations
None known.

### Architecture
Same domain layer.
`
	bodies := []string{impl1, review1, impl2}
	implNotes, reviewNotes, _ := ParseComments(bodies)

	if len(implNotes) != 2 {
		t.Fatalf("len(implNotes) = %d, want 2", len(implNotes))
	}
	if len(reviewNotes) != 1 {
		t.Fatalf("len(reviewNotes) = %d, want 1", len(reviewNotes))
	}

	// Verify order is preserved.
	if implNotes[0].Approach != "First implementation." {
		t.Errorf("implNotes[0].Approach = %q, want %q", implNotes[0].Approach, "First implementation.")
	}
	if implNotes[1].Approach != "Second implementation after review." {
		t.Errorf("implNotes[1].Approach = %q, want %q", implNotes[1].Approach, "Second implementation after review.")
	}
	if reviewNotes[0].Approved != "Looks good." {
		t.Errorf("reviewNotes[0].Approved = %q, want %q", reviewNotes[0].Approved, "Looks good.")
	}
}

func TestParseImplementationNotes_WhitespaceHandling(t *testing.T) {
	body := `## Implementation Notes

### Approach

   Indented and padded approach.

### Key Decisions

  Some decisions with leading spaces.

`
	got := ParseImplementationNotes(body)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Approach != "Indented and padded approach." {
		t.Errorf("Approach = %q, want %q", got.Approach, "Indented and padded approach.")
	}
	if got.KeyDecisions != "Some decisions with leading spaces." {
		t.Errorf("KeyDecisions = %q, want %q", got.KeyDecisions, "Some decisions with leading spaces.")
	}
}

func TestParseImplementationNotes_RawPreserved(t *testing.T) {
	body := "## Implementation Notes\n\n### Approach\nDone.\n"
	got := ParseImplementationNotes(body)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Raw != body {
		t.Errorf("Raw = %q, want %q", got.Raw, body)
	}
}

func TestParseReviewNotes_RawPreserved(t *testing.T) {
	body := "## Review Notes\n\n### Approved\nAll good.\n"
	got := ParseReviewNotes(body)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Raw != body {
		t.Errorf("Raw = %q, want %q", got.Raw, body)
	}
}

func TestParseComments_Empty(t *testing.T) {
	implNotes, reviewNotes, _ := ParseComments([]string{})
	if len(implNotes) != 0 {
		t.Errorf("len(implNotes) = %d, want 0", len(implNotes))
	}
	if len(reviewNotes) != 0 {
		t.Errorf("len(reviewNotes) = %d, want 0", len(reviewNotes))
	}
}

func TestParseComments_NoMatchingComments(t *testing.T) {
	bodies := []string{"Just a regular comment.", "Another comment."}
	implNotes, reviewNotes, _ := ParseComments(bodies)
	if len(implNotes) != 0 {
		t.Errorf("len(implNotes) = %d, want 0", len(implNotes))
	}
	if len(reviewNotes) != 0 {
		t.Errorf("len(reviewNotes) = %d, want 0", len(reviewNotes))
	}
}

// ParseCommentsInOrder tests

const qualityComment = `## Quality Review Notes

### Issues Found
- Missing error handling.

### Changes Requested
Yes
`

func implComment(approach string) string {
	return "## Implementation Notes\n\n### Approach\n" + approach + "\n\n### Key Decisions\nDecision.\n\n### Known Limitations\nNone.\n\n### Architecture\nDomain.\n"
}

func reviewComment(approved string) string {
	return "## Review Notes\n\n### Approved\n" + approved + "\n\n### Changes Requested\n\n### Architecture Compliance\nGood.\n"
}

func checkEntry(t *testing.T, got Entry, wantRole string, wantRound int, wantBody string) {
	t.Helper()
	if got.Role != wantRole {
		t.Errorf("Role = %q, want %q", got.Role, wantRole)
	}
	if got.Round != wantRound {
		t.Errorf("Round = %d, want %d", got.Round, wantRound)
	}
	if got.Body != wantBody {
		t.Errorf("Body = %q, want %q", got.Body, wantBody)
	}
}

func TestParseCommentsInOrder_Empty(t *testing.T) {
	entries := ParseCommentsInOrder([]string{})
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseCommentsInOrder_NoRetries(t *testing.T) {
	// Impl, Quality (pass), Functional (pass) → 3 entries in order
	impl := implComment("First implementation.")
	quality := qualityComment
	review := reviewComment("Approved.")
	entries := ParseCommentsInOrder([]string{impl, quality, review})

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	checkEntry(t, entries[0], "implementer", 1, impl)
	checkEntry(t, entries[1], "quality_reviewer", 1, quality)
	checkEntry(t, entries[2], "reviewer", 1, review)
}

func TestParseCommentsInOrder_QualityRetry(t *testing.T) {
	// Impl, Quality (fail), Impl (fix), Quality (pass), Functional (pass)
	impl1 := implComment("First implementation.")
	impl2 := implComment("Fixed implementation.")
	quality1 := qualityComment
	quality2 := qualityComment
	review1 := reviewComment("Approved.")

	entries := ParseCommentsInOrder([]string{impl1, quality1, impl2, quality2, review1})

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
	checkEntry(t, entries[0], "implementer", 1, impl1)
	checkEntry(t, entries[1], "quality_reviewer", 1, quality1)
	checkEntry(t, entries[2], "implementer", 2, impl2)
	checkEntry(t, entries[3], "quality_reviewer", 2, quality2)
	checkEntry(t, entries[4], "reviewer", 1, review1)
}

func TestParseCommentsInOrder_MultipleRetries(t *testing.T) {
	// Impl, Quality (fail), Impl, Quality (fail), Impl, Quality (pass), Functional (pass)
	impl1 := implComment("First.")
	impl2 := implComment("Second.")
	impl3 := implComment("Third.")
	quality1 := qualityComment
	quality2 := qualityComment
	quality3 := qualityComment
	review1 := reviewComment("Approved.")

	entries := ParseCommentsInOrder([]string{impl1, quality1, impl2, quality2, impl3, quality3, review1})

	if len(entries) != 7 {
		t.Fatalf("expected 7 entries, got %d", len(entries))
	}
	checkEntry(t, entries[0], "implementer", 1, impl1)
	checkEntry(t, entries[1], "quality_reviewer", 1, quality1)
	checkEntry(t, entries[2], "implementer", 2, impl2)
	checkEntry(t, entries[3], "quality_reviewer", 2, quality2)
	checkEntry(t, entries[4], "implementer", 3, impl3)
	checkEntry(t, entries[5], "quality_reviewer", 3, quality3)
	checkEntry(t, entries[6], "reviewer", 1, review1)
}

func TestParseCommentsInOrder_MissingQualityReview(t *testing.T) {
	// No quality reviewer configured → Impl, Functional only
	impl := implComment("Only implementation.")
	review := reviewComment("Approved.")
	entries := ParseCommentsInOrder([]string{impl, review})

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	checkEntry(t, entries[0], "implementer", 1, impl)
	checkEntry(t, entries[1], "reviewer", 1, review)
}

func TestParseCommentsInOrder_RoundNumbers(t *testing.T) {
	// Two implementer entries → rounds 1 and 2; single functional → round 1
	impl1 := implComment("Round one.")
	impl2 := implComment("Round two.")
	quality := qualityComment
	review := reviewComment("Approved.")

	entries := ParseCommentsInOrder([]string{impl1, quality, impl2, review})

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].Round != 1 {
		t.Errorf("entries[0].Round = %d, want 1", entries[0].Round)
	}
	if entries[2].Round != 2 {
		t.Errorf("entries[2].Round = %d, want 2", entries[2].Round)
	}
	if entries[3].Round != 1 {
		t.Errorf("entries[3].Round = %d, want 1", entries[3].Round)
	}
}

func TestParseCommentsInOrder_NonStructuredCommentsIgnored(t *testing.T) {
	impl := implComment("Implementation.")
	entries := ParseCommentsInOrder([]string{"Just a comment.", impl, "Another comment."})

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	checkEntry(t, entries[0], "implementer", 1, impl)
}
