package github

import (
	"fmt"
	"testing"
)

func TestFetchPRCommentBodies_Success(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"body":"PR description","comments":[{"body":"First comment"},{"body":"Second comment"}]}`), nil
	}

	bodies, err := FetchPRCommentBodies("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodies) != 3 {
		t.Fatalf("expected 3 bodies, got %d", len(bodies))
	}
	if bodies[0] != "PR description" {
		t.Errorf("bodies[0]: got %q, want %q", bodies[0], "PR description")
	}
	if bodies[1] != "First comment" {
		t.Errorf("bodies[1]: got %q, want %q", bodies[1], "First comment")
	}
	if bodies[2] != "Second comment" {
		t.Errorf("bodies[2]: got %q, want %q", bodies[2], "Second comment")
	}
}

func TestFetchPRCommentBodies_EmptyPRBody(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"body":"","comments":[{"body":"A comment"}]}`), nil
	}

	bodies, err := FetchPRCommentBodies("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("expected 1 body (empty PR body excluded), got %d", len(bodies))
	}
	if bodies[0] != "A comment" {
		t.Errorf("bodies[0]: got %q, want %q", bodies[0], "A comment")
	}
}

func TestFetchPRCommentBodies_CLIError(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exit status 1")
	}

	_, err := FetchPRCommentBodies("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchPRCommentBodies_PRBodySkippedWhenCommentHasImplNotes(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"body":"## Summary\n\n## Implementation Notes\n\nDuplicate in PR body","comments":[{"body":"## Implementation Notes\n\nActual comment"},{"body":"## Quality Review Notes\n\nLGTM"}]}`), nil
	}

	bodies, err := FetchPRCommentBodies("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 bodies (PR body excluded), got %d", len(bodies))
	}
	if bodies[0] != "## Implementation Notes\n\nActual comment" {
		t.Errorf("bodies[0]: got %q, want Implementation Notes comment", bodies[0])
	}
}

func TestFetchPRCommentBodies_PRBodyIncludedWhenNoCommentHasImplNotes(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"body":"## Implementation Notes\n\nOnly in PR body","comments":[{"body":"## Quality Review Notes\n\nLGTM"}]}`), nil
	}

	bodies, err := FetchPRCommentBodies("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 bodies (PR body included as fallback), got %d", len(bodies))
	}
	if bodies[0] != "## Implementation Notes\n\nOnly in PR body" {
		t.Errorf("bodies[0]: got %q, want PR body with Implementation Notes", bodies[0])
	}
}

func TestDeleteLastPRCommentWithHeader_DeletesMatchingComment(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var deletedPath string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		// List call
		if args[1] == "repos/owner/repo/issues/42/comments" {
			return []byte(`[{"id":100,"body":"## Implementation Notes\nfoo"},{"id":200,"body":"## Review Notes\nfirst"},{"id":300,"body":"## Review Notes\nsecond"}]`), nil
		}
		// Delete call: gh api --method DELETE <path>
		if len(args) >= 4 && args[2] == "DELETE" {
			deletedPath = args[3]
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected call: %v", args)
	}

	err := DeleteLastPRCommentWithHeader("owner/repo", 42, "## Review Notes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedPath != "repos/owner/repo/issues/comments/300" {
		t.Errorf("deleted wrong comment: got %q, want path ending with /300", deletedPath)
	}
}

func TestDeleteLastPRCommentWithHeader_NoMatch(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`[{"id":100,"body":"## Implementation Notes\nfoo"}]`), nil
	}

	err := DeleteLastPRCommentWithHeader("owner/repo", 42, "## Review Notes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchPRCommentBodies_MalformedJSON(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`not valid json`), nil
	}

	_, err := FetchPRCommentBodies("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}
