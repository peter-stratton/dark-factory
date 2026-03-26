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

func TestFetchAllIssueNumbers(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	callCount := 0
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		callCount++
		// First call = open, second = closed
		if callCount == 1 {
			return []byte(`[{"number":1},{"number":2}]`), nil
		}
		return []byte(`[{"number":3},{"number":4}]`), nil
	}

	all, err := FetchAllIssueNumbers("owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 numbers, got %d", len(all))
	}
	for _, n := range []int{1, 2, 3, 4} {
		if !all[n] {
			t.Errorf("expected %d in result set", n)
		}
	}
}

func TestFetchIssue(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"number":42,"title":"Add widget","body":"Please add a widget.","labels":[{"name":"p2"},{"name":"enhancement"}]}`), nil
	}

	issue, err := FetchIssue("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if issue.Number != 42 {
		t.Errorf("number: got %d, want 42", issue.Number)
	}
	if issue.Title != "Add widget" {
		t.Errorf("title: got %q, want %q", issue.Title, "Add widget")
	}
	if issue.Body != "Please add a widget." {
		t.Errorf("body: got %q", issue.Body)
	}
	if issue.Priority != "p2" {
		t.Errorf("priority: got %q, want %q", issue.Priority, "p2")
	}
	if len(issue.Labels) != 2 {
		t.Fatalf("labels: got %d, want 2", len(issue.Labels))
	}
}

func TestFetchIssueError(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("not found")
	}

	_, err := FetchIssue("owner/repo", 999)
	if err == nil {
		t.Fatal("expected error, got nil")
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

func TestFetchMergedPRIssueNumbers_ClosingKeywords(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		want  []int
	}{
		{
			name: "closes",
			body: "Closes #31",
			want: []int{31},
		},
		{
			name: "fixes",
			body: "Fixes #42",
			want: []int{42},
		},
		{
			name: "resolves",
			body: "Resolves #7",
			want: []int{7},
		},
		{
			name: "multiple",
			body: "Closes #10\nFixes #20\nResolves #30",
			want: []int{10, 20, 30},
		},
		{
			name: "case insensitive",
			body: "CLOSES #99",
			want: []int{99},
		},
		{
			name: "no keywords",
			body: "This PR adds a new feature.",
			want: nil,
		},
		{
			name: "deduplication",
			body: "Closes #5\nCloses #5",
			want: []int{5},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := CommandRunner
			defer func() { CommandRunner = orig }()

			CommandRunner = func(name string, args ...string) ([]byte, error) {
				type prBody struct {
					Body string `json:"body"`
				}
				prs := []prBody{{Body: tc.body}}
				out, err := json.Marshal(prs)
				if err != nil {
					return nil, err
				}
				return out, nil
			}

			got, err := FetchMergedPRIssueNumbers("owner/repo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i, n := range tc.want {
				if got[i] != n {
					t.Errorf("index %d: got %d, want %d", i, got[i], n)
				}
			}
		})
	}
}

func TestFetchMergedPRIssueNumbers_Error(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh: not authenticated")
	}

	_, err := FetchMergedPRIssueNumbers("owner/repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchMergedPRIssueNumbers_Empty(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}

	got, err := FetchMergedPRIssueNumbers("owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}
