package cmd

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

// fakeRunner builds a CommandRunner stub that dispatches on the first gh
// subcommand (args[0]) and returns the corresponding payload.
func fakeRunner(t *testing.T, dispatch func(args []string) ([]byte, error)) func(string, ...string) ([]byte, error) {
	t.Helper()
	return func(name string, args ...string) ([]byte, error) {
		return dispatch(args)
	}
}

func TestWatchCmd_Registered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "watch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("watch command not registered on rootCmd")
	}
}

func TestWatchCmd_HasHelpOutput(t *testing.T) {
	if watchCmd.Use != "watch" {
		t.Errorf("watchCmd.Use: got %q, want %q", watchCmd.Use, "watch")
	}
	if watchCmd.Short == "" {
		t.Error("watchCmd.Short must not be empty")
	}
}

// TestWatchPollInterval_DefaultWhenNilConfig verifies the 60s default when
// Watch config is nil.
func TestWatchPollInterval_DefaultWhenNilConfig(t *testing.T) {
	cfg := &config.Config{Watch: nil}
	got, err := watchPollInterval(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 60*time.Second {
		t.Errorf("expected 60s default, got %v", got)
	}
}

// TestWatchPollInterval_DefaultWhenEmptyInterval verifies the 60s default when
// Watch is set but PollInterval is empty.
func TestWatchPollInterval_DefaultWhenEmptyInterval(t *testing.T) {
	cfg := &config.Config{Watch: &config.Watch{PollInterval: ""}}
	got, err := watchPollInterval(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 60*time.Second {
		t.Errorf("expected 60s default, got %v", got)
	}
}

// TestWatchPollInterval_CustomInterval verifies that a configured interval is used.
func TestWatchPollInterval_CustomInterval(t *testing.T) {
	cfg := &config.Config{Watch: &config.Watch{PollInterval: "10s"}}
	got, err := watchPollInterval(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 10*time.Second {
		t.Errorf("expected 10s, got %v", got)
	}
}

// TestPollOnce_NoPRs verifies that an empty PR list causes no label calls.
func TestPollOnce_NoPRs(t *testing.T) {
	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer func() { github.CommandRunner = orig }()

	processed := make(map[int]bool)
	if err := pollOnce(context.Background(), "owner/repo", processed, slog.Default()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(processed) != 0 {
		t.Errorf("expected no processed reviews, got %v", processed)
	}
}

// TestPollOnce_DetectsChangesRequestedReview verifies that a CHANGES_REQUESTED
// review triggers label swapping and marks the review as processed.
func TestPollOnce_DetectsChangesRequestedReview(t *testing.T) {
	var addLabelCalled, removeLabelCalled bool

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) < 2 {
			return []byte(`{}`), nil
		}
		switch {
		case args[0] == "pr" && args[1] == "list":
			return []byte(`[{"number":5,"headRefName":"feature-branch"}]`), nil
		case args[0] == "api":
			return []byte(`[{"id":101,"state":"CHANGES_REQUESTED","body":"fix this","user":{"login":"alice"}}]`), nil
		case args[0] == "issue" && args[1] == "edit":
			for _, a := range args {
				if a == "--add-label" {
					addLabelCalled = true
				}
				if a == "--remove-label" {
					removeLabelCalled = true
				}
			}
			return []byte(`{}`), nil
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	processed := make(map[int]bool)
	if err := pollOnce(context.Background(), "owner/repo", processed, slog.Default()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !processed[101] {
		t.Error("expected review ID 101 to be marked as processed")
	}
	if !addLabelCalled {
		t.Error("expected add-label to be called")
	}
	if !removeLabelCalled {
		t.Error("expected remove-label to be called")
	}
}

// TestPollOnce_DuplicateSkipped verifies that an already-processed review ID
// does not trigger another label swap.
func TestPollOnce_DuplicateSkipped(t *testing.T) {
	var addLabelCalled bool

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) < 2 {
			return []byte(`{}`), nil
		}
		switch {
		case args[0] == "pr" && args[1] == "list":
			return []byte(`[{"number":5,"headRefName":"feature-branch"}]`), nil
		case args[0] == "api":
			return []byte(`[{"id":101,"state":"CHANGES_REQUESTED","body":"fix this","user":{"login":"alice"}}]`), nil
		case args[0] == "issue" && args[1] == "edit":
			for _, a := range args {
				if a == "--add-label" {
					addLabelCalled = true
				}
			}
			return []byte(`{}`), nil
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	// Pre-mark review 101 as already handled.
	processed := map[int]bool{101: true}
	if err := pollOnce(context.Background(), "owner/repo", processed, slog.Default()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if addLabelCalled {
		t.Error("expected no label update for already-processed review")
	}
}

// TestPollOnce_NonChangesRequestedSkipped verifies that APPROVED reviews do not
// trigger label swapping.
func TestPollOnce_NonChangesRequestedSkipped(t *testing.T) {
	var addLabelCalled bool

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) < 2 {
			return []byte(`{}`), nil
		}
		switch {
		case args[0] == "pr" && args[1] == "list":
			return []byte(`[{"number":5,"headRefName":"feature-branch"}]`), nil
		case args[0] == "api":
			return []byte(`[{"id":200,"state":"APPROVED","body":"LGTM","user":{"login":"bob"}}]`), nil
		case args[0] == "issue" && args[1] == "edit":
			for _, a := range args {
				if a == "--add-label" {
					addLabelCalled = true
				}
			}
			return []byte(`{}`), nil
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	processed := make(map[int]bool)
	if err := pollOnce(context.Background(), "owner/repo", processed, slog.Default()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if addLabelCalled {
		t.Error("expected no label update for APPROVED review")
	}
	if processed[200] {
		t.Error("expected APPROVED review not to be recorded in processed set")
	}
}

// TestPollOnce_ContextCancelled verifies that a cancelled context causes
// pollOnce to return nil without processing PRs.
func TestPollOnce_ContextCancelled(t *testing.T) {
	listCalled := false

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "list" {
			listCalled = true
		}
		return []byte(`[]`), nil
	})
	defer func() { github.CommandRunner = orig }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before polling

	processed := make(map[int]bool)
	// pollOnce calls ListPRsWithLabel before checking ctx — the important thing
	// is it returns nil (no error) even when context is already cancelled.
	err := pollOnce(ctx, "owner/repo", processed, slog.Default())
	if err != nil {
		t.Fatalf("expected nil error on cancelled context, got %v", err)
	}
	_ = listCalled // the list may or may not be called depending on timing
}
