package vet

import (
	"testing"

	"github.com/peter-stratton/dark-factory/internal/github"
)

func completeIssue(number int) github.Issue {
	return github.Issue{
		Number: number,
		Title:  "Complete issue",
		Labels: []string{"phase-2"},
		Body: `## Acceptance criteria

- [ ] Thing works
- [ ] Other thing works

## Test cases

- **Happy path**: does the right thing
- **Edge case**: handles edge

Blocked by: #1
`,
	}
}

func TestValidateIssues_CompleteIssue(t *testing.T) {
	issues := []github.Issue{completeIssue(10)}
	allNums := map[int]bool{1: true, 10: true}

	r := ValidateIssues(issues, allNums, "phase-2")
	if len(r.Findings()) != 0 {
		t.Errorf("expected 0 findings, got %d: %+v", len(r.Findings()), r.Findings())
	}
}

func TestValidateIssues_MissingAcceptanceCriteria(t *testing.T) {
	iss := github.Issue{
		Number: 5,
		Labels: []string{"phase-2"},
		Body:   "## Test cases\n\n- **Foo**: bar\n",
	}
	r := ValidateIssues([]github.Issue{iss}, nil, "phase-2")
	findings := r.Findings()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Severity != Error {
		t.Errorf("expected Error severity, got %v", findings[0].Severity)
	}
}

func TestValidateIssues_MissingTestCases(t *testing.T) {
	iss := github.Issue{
		Number: 5,
		Labels: []string{"phase-2"},
		Body:   "## Acceptance criteria\n\n- [ ] Done\n",
	}
	r := ValidateIssues([]github.Issue{iss}, nil, "phase-2")
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && f.Message == "missing ## Test cases section" {
			found = true
		}
	}
	if !found {
		t.Error("expected error about missing Test cases section")
	}
}

func TestValidateIssues_EmptyAcceptanceCriteria(t *testing.T) {
	iss := github.Issue{
		Number: 5,
		Labels: []string{"phase-2"},
		Body:   "## Acceptance criteria\n\nSome text but no checkboxes\n\n## Test cases\n\n- **Foo**: bar\n",
	}
	r := ValidateIssues([]github.Issue{iss}, nil, "phase-2")
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && f.Message == "## Acceptance criteria has no checkbox items (- [ ])" {
			found = true
		}
	}
	if !found {
		t.Error("expected error about empty acceptance criteria")
	}
}

func TestValidateIssues_EmptyTestCases(t *testing.T) {
	iss := github.Issue{
		Number: 5,
		Labels: []string{"phase-2"},
		Body:   "## Acceptance criteria\n\n- [ ] Done\n\n## Test cases\n\nSome text but no entries\n",
	}
	r := ValidateIssues([]github.Issue{iss}, nil, "phase-2")
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && f.Message == "## Test cases has no - **Name**: entries" {
			found = true
		}
	}
	if !found {
		t.Error("expected error about empty test cases")
	}
}

func TestValidateIssues_MalformedBlocker(t *testing.T) {
	iss := github.Issue{
		Number: 5,
		Labels: []string{"phase-2"},
		Body:   "## Acceptance criteria\n\n- [ ] Done\n\n## Test cases\n\n- **Foo**: bar\n\nBlocked by #1\n",
	}
	r := ValidateIssues([]github.Issue{iss}, map[int]bool{1: true}, "phase-2")
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Warning && f.Message != "" {
			if contains(f.Message, "malformed blocker") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected warning about malformed blocker notation")
	}
}

func TestValidateIssues_NonExistentBlockerRef(t *testing.T) {
	iss := github.Issue{
		Number: 5,
		Labels: []string{"phase-2"},
		Body:   "## Acceptance criteria\n\n- [ ] Done\n\n## Test cases\n\n- **Foo**: bar\n\nBlocked by: #999\n",
	}
	r := ValidateIssues([]github.Issue{iss}, map[int]bool{5: true}, "phase-2")
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Warning && contains(f.Message, "non-existent issue #999") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about non-existent blocker ref")
	}
}

func TestValidateIssues_MissingPhaseLabel(t *testing.T) {
	iss := github.Issue{
		Number: 5,
		Labels: []string{"bug"},
		Body:   "## Acceptance criteria\n\n- [ ] Done\n\n## Test cases\n\n- **Foo**: bar\n",
	}
	r := ValidateIssues([]github.Issue{iss}, nil, "phase-2")
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Warning && contains(f.Message, "missing phase label") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about missing phase label")
	}
}

func TestValidateIssues_MultipleIssues(t *testing.T) {
	issues := []github.Issue{
		{Number: 1, Labels: []string{"phase-2"}, Body: "no sections"},
		{Number: 2, Labels: []string{"phase-2"}, Body: "no sections either"},
	}
	r := ValidateIssues(issues, nil, "phase-2")
	// Each issue missing AC + TC = 2 errors each = 4 total
	if len(r.Findings()) != 4 {
		t.Errorf("expected 4 findings, got %d: %+v", len(r.Findings()), r.Findings())
	}
}

func TestValidateIssues_LocationFormat(t *testing.T) {
	iss := github.Issue{
		Number: 42,
		Labels: []string{"phase-2"},
		Body:   "no sections",
	}
	r := ValidateIssues([]github.Issue{iss}, nil, "phase-2")
	for _, f := range r.Findings() {
		if f.Location != "#42" {
			t.Errorf("Location = %q, want #42", f.Location)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
