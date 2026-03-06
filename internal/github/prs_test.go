package github

import (
	"fmt"
	"testing"
)

func TestFetchPRCommentBodies_Success(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"comments":[{"body":"First comment"},{"body":"Second comment"}]}`), nil
	}

	bodies, err := FetchPRCommentBodies("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 bodies, got %d", len(bodies))
	}
	if bodies[0] != "First comment" {
		t.Errorf("bodies[0]: got %q, want %q", bodies[0], "First comment")
	}
	if bodies[1] != "Second comment" {
		t.Errorf("bodies[1]: got %q, want %q", bodies[1], "Second comment")
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
