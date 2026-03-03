package github

import (
	"encoding/json"
	"fmt"
	"testing"
)

// fakeGH returns a CommandRunner that produces canned JSON output.
func fakeGH(issues []ghIssue) func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		out, err := json.Marshal(issues)
		if err != nil {
			return nil, fmt.Errorf("test marshal: %w", err)
		}
		return out, nil
	}
}

func TestPrioritySorting(t *testing.T) {
	raw := []ghIssue{
		{Number: 4, Title: "unlabeled", Labels: []label{}},
		{Number: 3, Title: "p3", Labels: []label{{Name: "p3"}}},
		{Number: 1, Title: "p1", Labels: []label{{Name: "p1"}}},
		{Number: 2, Title: "p2", Labels: []label{{Name: "p2"}}},
	}

	orig := CommandRunner
	CommandRunner = fakeGH(raw)
	defer func() { CommandRunner = orig }()

	issues, err := FetchMilestoneIssues("owner/repo", "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"p1", "p2", "p3", ""}
	for i, iss := range issues {
		if iss.Priority != want[i] {
			t.Errorf("index %d: got priority %q, want %q", i, iss.Priority, want[i])
		}
	}
}

func TestNumberSortingWithinTier(t *testing.T) {
	raw := []ghIssue{
		{Number: 10, Title: "second p1", Labels: []label{{Name: "p1"}}},
		{Number: 5, Title: "first p1", Labels: []label{{Name: "p1"}}},
		{Number: 20, Title: "p2", Labels: []label{{Name: "p2"}}},
	}

	orig := CommandRunner
	CommandRunner = fakeGH(raw)
	defer func() { CommandRunner = orig }()

	issues, err := FetchMilestoneIssues("owner/repo", "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if issues[0].Number != 5 || issues[1].Number != 10 {
		t.Errorf("p1 issues not sorted by number: got %d, %d", issues[0].Number, issues[1].Number)
	}
	if issues[2].Number != 20 {
		t.Errorf("expected p2 issue last, got number %d", issues[2].Number)
	}
}

func TestBodyIncluded(t *testing.T) {
	raw := []ghIssue{
		{Number: 1, Title: "has body", Body: "This is the full body text.", Labels: []label{}},
	}

	orig := CommandRunner
	CommandRunner = fakeGH(raw)
	defer func() { CommandRunner = orig }()

	issues, err := FetchMilestoneIssues("owner/repo", "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if issues[0].Body != "This is the full body text." {
		t.Errorf("body not preserved: got %q", issues[0].Body)
	}
}

func TestLabelParsing(t *testing.T) {
	raw := []ghIssue{
		{Number: 1, Title: "multi-label", Labels: []label{{Name: "p1"}, {Name: "bug"}, {Name: "phase-1"}}},
	}

	orig := CommandRunner
	CommandRunner = fakeGH(raw)
	defer func() { CommandRunner = orig }()

	issues, err := FetchMilestoneIssues("owner/repo", "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(issues[0].Labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(issues[0].Labels))
	}
	want := []string{"p1", "bug", "phase-1"}
	for i, l := range issues[0].Labels {
		if l != want[i] {
			t.Errorf("label %d: got %q, want %q", i, l, want[i])
		}
	}
}

func TestEmptyMilestone(t *testing.T) {
	orig := CommandRunner
	CommandRunner = fakeGH([]ghIssue{})
	defer func() { CommandRunner = orig }()

	issues, err := FetchMilestoneIssues("owner/repo", "empty-milestone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("expected empty slice, got %d issues", len(issues))
	}
}
