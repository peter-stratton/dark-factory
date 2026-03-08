package github

import (
	"fmt"
	"testing"
)

func TestListPRsWithLabel_ReturnsList(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`[{"number":5,"headRefName":"feature-branch"},{"number":8,"headRefName":"other-branch"}]`), nil
	}
	defer func() { CommandRunner = orig }()

	prs, err := ListPRsWithLabel("owner/repo", "godark:awaiting-human-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(prs))
	}
	if prs[0].Number != 5 {
		t.Errorf("prs[0].Number: got %d, want 5", prs[0].Number)
	}
	if prs[0].HeadRefName != "feature-branch" {
		t.Errorf("prs[0].HeadRefName: got %q, want %q", prs[0].HeadRefName, "feature-branch")
	}
	if prs[1].Number != 8 {
		t.Errorf("prs[1].Number: got %d, want 8", prs[1].Number)
	}
}

func TestListPRsWithLabel_Empty(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	}
	defer func() { CommandRunner = orig }()

	prs, err := ListPRsWithLabel("owner/repo", "godark:awaiting-human-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected empty, got %v", prs)
	}
}

func TestListPRsWithLabel_CLIError(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exit status 1")
	}
	defer func() { CommandRunner = orig }()

	_, err := ListPRsWithLabel("owner/repo", "godark:awaiting-human-review")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListPRsWithLabel_MalformedJSON(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`not json`), nil
	}
	defer func() { CommandRunner = orig }()

	_, err := ListPRsWithLabel("owner/repo", "godark:awaiting-human-review")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestFetchPRReviews_ReturnsReviews(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`[
			{"id":101,"state":"CHANGES_REQUESTED","body":"Please fix X","user":{"login":"alice"}},
			{"id":102,"state":"APPROVED","body":"LGTM","user":{"login":"bob"}}
		]`), nil
	}
	defer func() { CommandRunner = orig }()

	reviews, err := FetchPRReviews("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("expected 2 reviews, got %d", len(reviews))
	}
	if reviews[0].ID != 101 {
		t.Errorf("reviews[0].ID: got %d, want 101", reviews[0].ID)
	}
	if reviews[0].State != "CHANGES_REQUESTED" {
		t.Errorf("reviews[0].State: got %q, want %q", reviews[0].State, "CHANGES_REQUESTED")
	}
	if reviews[0].Body != "Please fix X" {
		t.Errorf("reviews[0].Body: got %q, want %q", reviews[0].Body, "Please fix X")
	}
	if reviews[0].Author != "alice" {
		t.Errorf("reviews[0].Author: got %q, want %q", reviews[0].Author, "alice")
	}
	if reviews[1].State != "APPROVED" {
		t.Errorf("reviews[1].State: got %q, want %q", reviews[1].State, "APPROVED")
	}
}

func TestFetchPRReviews_Empty(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	}
	defer func() { CommandRunner = orig }()

	reviews, err := FetchPRReviews("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reviews) != 0 {
		t.Errorf("expected empty, got %v", reviews)
	}
}

func TestFetchPRReviews_CLIError(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh api error")
	}
	defer func() { CommandRunner = orig }()

	_, err := FetchPRReviews("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchPRReviews_MalformedJSON(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`not valid json`), nil
	}
	defer func() { CommandRunner = orig }()

	_, err := FetchPRReviews("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestFetchReviewComments_ReturnsBodies(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`[
			{"body":"Fix the error handling here."},
			{"body":"This method name is unclear."}
		]`), nil
	}
	defer func() { CommandRunner = orig }()

	bodies, err := FetchReviewComments("owner/repo", 42, 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 bodies, got %d", len(bodies))
	}
	if bodies[0] != "Fix the error handling here." {
		t.Errorf("bodies[0]: got %q, want %q", bodies[0], "Fix the error handling here.")
	}
	if bodies[1] != "This method name is unclear." {
		t.Errorf("bodies[1]: got %q, want %q", bodies[1], "This method name is unclear.")
	}
}

func TestFetchReviewComments_Empty(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	}
	defer func() { CommandRunner = orig }()

	bodies, err := FetchReviewComments("owner/repo", 42, 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodies) != 0 {
		t.Errorf("expected empty, got %v", bodies)
	}
}

func TestFetchReviewComments_SkipsEmptyBodies(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`[{"body":"real comment"},{"body":""},{"body":"another comment"}]`), nil
	}
	defer func() { CommandRunner = orig }()

	bodies, err := FetchReviewComments("owner/repo", 42, 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 bodies (empty skipped), got %d", len(bodies))
	}
}

func TestFetchReviewComments_CLIError(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh api error")
	}
	defer func() { CommandRunner = orig }()

	_, err := FetchReviewComments("owner/repo", 42, 101)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchReviewComments_MalformedJSON(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`not valid json`), nil
	}
	defer func() { CommandRunner = orig }()

	_, err := FetchReviewComments("owner/repo", 42, 101)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}
