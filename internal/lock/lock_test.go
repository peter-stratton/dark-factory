package lock

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/github"
)

// newTestLocker creates a Locker with a temp-dir lock file path for isolation.
func newTestLocker(t *testing.T) *Locker {
	t.Helper()
	return &Locker{
		repo:         "owner/repo",
		label:        "godark-in-progress",
		lockFilePath: filepath.Join(t.TempDir(), "lock.json"),
		logger:       slog.Default(),
	}
}

// stubCommandRunner replaces github.CommandRunner for the duration of the test.
func stubCommandRunner(t *testing.T, fn func(string, ...string) ([]byte, error)) {
	t.Helper()
	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })
	github.CommandRunner = fn
}

// emptyJSON returns an empty JSON array (no issues with the lock label).
func emptyJSON() []byte { return []byte(`[]`) }

// numbersJSON returns a JSON array of {"number":n} objects.
func numbersJSON(nums []int) []byte {
	type row struct {
		Number int `json:"number"`
	}
	rows := make([]row, len(nums))
	for i, n := range nums {
		rows[i] = row{Number: n}
	}
	b, _ := json.Marshal(rows)
	return b
}

func TestIsLocked_NoLock(t *testing.T) {
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		return emptyJSON(), nil
	})

	l := newTestLocker(t)
	locked, info, err := l.IsLocked()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if locked {
		t.Error("expected not locked")
	}
	if info != nil {
		t.Error("expected nil RunInfo when not locked")
	}
}

func TestIsLocked_WithLockLabel(t *testing.T) {
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		return numbersJSON([]int{1, 2}), nil
	})

	l := newTestLocker(t)
	locked, _, err := l.IsLocked()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !locked {
		t.Error("expected locked when issues have lock label")
	}
}

func TestIsLocked_ReturnsRunInfoFromFile(t *testing.T) {
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		return numbersJSON([]int{5}), nil
	})

	l := newTestLocker(t)

	// Pre-write a lock file.
	info := RunInfo{Host: "test-host", PID: 42, Issues: []int{5}}
	data, _ := json.MarshalIndent(info, "", "  ")
	os.WriteFile(l.lockFilePath, data, 0o644)

	locked, got, err := l.IsLocked()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !locked {
		t.Fatal("expected locked")
	}
	if got == nil {
		t.Fatal("expected non-nil RunInfo")
	}
	if got.Host != "test-host" || got.PID != 42 {
		t.Errorf("RunInfo = %+v, want host=test-host pid=42", got)
	}
}

func TestAcquire_WhenUnlocked(t *testing.T) {
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		// All calls return success (empty for issue list, ok for create/edit).
		return emptyJSON(), nil
	})

	l := newTestLocker(t)
	if err := l.Acquire([]int{1, 2}, false); err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}

	// Lock file should be written.
	if _, err := os.Stat(l.lockFilePath); err != nil {
		t.Errorf("expected lock file to exist after Acquire: %v", err)
	}

	// Verify lock file content.
	data, _ := os.ReadFile(l.lockFilePath)
	var info RunInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("parsing lock file: %v", err)
	}
	if len(info.Issues) != 2 || info.Issues[0] != 1 || info.Issues[1] != 2 {
		t.Errorf("lock file issues = %v, want [1 2]", info.Issues)
	}
	if info.Repo != "owner/repo" {
		t.Errorf("lock file repo = %q, want %q", info.Repo, "owner/repo")
	}
}

func TestAcquire_BlocksWhenLocked(t *testing.T) {
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		return numbersJSON([]int{3}), nil
	})

	l := newTestLocker(t)
	err := l.Acquire([]int{1}, false)
	if err == nil {
		t.Fatal("expected error when another instance holds the lock")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force, got: %v", err)
	}
	if !strings.Contains(err.Error(), "godark unlock") {
		t.Errorf("error should mention 'godark unlock', got: %v", err)
	}
}

func TestAcquire_BlocksWithRunInfo(t *testing.T) {
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		return numbersJSON([]int{3}), nil
	})

	l := newTestLocker(t)

	// Write a lock file so the error message includes metadata.
	info := RunInfo{Host: "other-host", PID: 9999}
	data, _ := json.MarshalIndent(info, "", "  ")
	os.WriteFile(l.lockFilePath, data, 0o644)

	err := l.Acquire([]int{1}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "other-host") {
		t.Errorf("error should mention the lock holder's host, got: %v", err)
	}
}

func TestAcquire_ForceOverridesLock(t *testing.T) {
	callCount := 0
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		callCount++
		// First call (IsLocked → FindIssuesWithLabel): report locked.
		// All subsequent calls: return empty/ok.
		if callCount == 1 {
			return numbersJSON([]int{3}), nil
		}
		return emptyJSON(), nil
	})

	l := newTestLocker(t)
	if err := l.Acquire([]int{1}, true); err != nil {
		t.Fatalf("Acquire(force=true) error: %v", err)
	}
}

func TestAcquire_FailsWhenAllLabelsFail(t *testing.T) {
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--add-label" {
				return nil, fmt.Errorf("permission denied")
			}
		}
		// FindIssuesWithLabel and EnsureLabel calls succeed with empty list.
		return emptyJSON(), nil
	})

	l := newTestLocker(t)
	err := l.Acquire([]int{1, 2}, false)
	if err == nil {
		t.Fatal("expected error when all label applications fail")
	}
}

func TestRelease_RemovesLabelAndLockFile(t *testing.T) {
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte{}, nil
	})

	l := newTestLocker(t)
	// Write a lock file first.
	os.WriteFile(l.lockFilePath, []byte(`{}`), 0o644)

	if err := l.Release([]int{1, 2}); err != nil {
		t.Fatalf("Release() error: %v", err)
	}
	if _, err := os.Stat(l.lockFilePath); !os.IsNotExist(err) {
		t.Error("expected lock file to be deleted after Release")
	}
}

func TestRelease_NoLockFile(t *testing.T) {
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte{}, nil
	})

	l := newTestLocker(t)
	// No lock file written — Release should succeed without error.
	if err := l.Release([]int{1}); err != nil {
		t.Fatalf("Release() error when no lock file: %v", err)
	}
}

func TestReleaseAll_RemovesAllLockedIssues(t *testing.T) {
	callCount := 0
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			// First call: FindIssuesWithLabel returns 2 locked issues.
			return numbersJSON([]int{7, 8}), nil
		}
		return []byte{}, nil
	})

	l := newTestLocker(t)
	count, err := l.ReleaseAll()
	if err != nil {
		t.Fatalf("ReleaseAll() error: %v", err)
	}
	if count != 2 {
		t.Errorf("ReleaseAll() = %d issues, want 2", count)
	}
}

func TestReleaseAll_NothingLocked(t *testing.T) {
	stubCommandRunner(t, func(name string, args ...string) ([]byte, error) {
		return emptyJSON(), nil
	})

	l := newTestLocker(t)
	count, err := l.ReleaseAll()
	if err != nil {
		t.Fatalf("ReleaseAll() error: %v", err)
	}
	if count != 0 {
		t.Errorf("ReleaseAll() = %d, want 0", count)
	}
}

func TestEnsureLabelExists(t *testing.T) {
	var createCalled bool
	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "create" {
				createCalled = true
				return []byte(`{}`), nil
			}
		}
		return emptyJSON(), nil
	}

	if err := EnsureLabelExists("owner/repo", "custom-label"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createCalled {
		t.Error("expected label create to be called")
	}
}

func TestNew_UsesProvidedLabel(t *testing.T) {
	l := New("owner/repo", "custom-label", slog.Default())
	if l.label != "custom-label" {
		t.Errorf("label = %q, want %q", l.label, "custom-label")
	}
}

func TestEnsureLabelExists_UsesProvidedLabel(t *testing.T) {
	var capturedLabel string
	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		for i, a := range args {
			if a == "--search" && i+1 < len(args) {
				capturedLabel = args[i+1]
			}
			if a == "create" {
				return []byte(`{}`), nil
			}
		}
		return emptyJSON(), nil
	}

	if err := EnsureLabelExists("owner/repo", "my-lock-label"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedLabel != "my-lock-label" {
		t.Errorf("label forwarded to GitHub API = %q, want %q", capturedLabel, "my-lock-label")
	}
}
