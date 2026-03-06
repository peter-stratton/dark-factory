package cmd

import (
	"testing"

	"github.com/phs/dark-factory/internal/github"
)

func TestFetchPRCommentBodiesFnDefault(t *testing.T) {
	// The package-level variable must default to the real GitHub implementation
	// so that dialogue fetching works in production runs.
	if fetchPRCommentBodiesFn == nil {
		t.Fatal("fetchPRCommentBodiesFn is nil, want github.FetchPRCommentBodies")
	}

	// Verify it's the same function by checking it uses github.CommandRunner
	// (indirectly: stub CommandRunner and confirm the variable calls through).
	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })

	called := false
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		called = true
		return []byte("[]"), nil
	}

	// Call the default function — it will invoke github.CommandRunner.
	fetchPRCommentBodiesFn("owner/repo", 1) //nolint:errcheck // best-effort; ignore error
	if !called {
		t.Error("fetchPRCommentBodiesFn did not delegate to github.CommandRunner, want github.FetchPRCommentBodies")
	}
}

func TestFetchPRCommentBodiesFnReplaceable(t *testing.T) {
	// Confirm the variable can be replaced for testing (testability seam).
	orig := fetchPRCommentBodiesFn
	t.Cleanup(func() { fetchPRCommentBodiesFn = orig })

	called := false
	fetchPRCommentBodiesFn = func(repo string, prNum int) ([]string, error) {
		called = true
		return []string{"body1", "body2"}, nil
	}

	bodies, err := fetchPRCommentBodiesFn("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("replaced fetchPRCommentBodiesFn was not called")
	}
	if len(bodies) != 2 {
		t.Errorf("got %d bodies, want 2", len(bodies))
	}
}

func TestImplementCmd_PunchlistFlagRegistered(t *testing.T) {
	f := implementCmd.Flags().Lookup("punchlist")
	if f == nil {
		t.Fatal("implement command missing --punchlist flag")
	}
	if f.DefValue != "" {
		t.Errorf("punchlist default = %q, want %q", f.DefValue, "")
	}
}
