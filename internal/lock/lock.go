// Package lock provides a distributed run lock using GitHub labels as the
// visible signal and a local JSON file for stale-lock detection metadata.
//
// When a godark run starts, it applies the lock label to all issues it will
// process. Other instances see the label and refuse to start. When the run
// finishes (or is interrupted), the label is removed.
//
// If godark crashes mid-run, the label persists. Use --force on the next run
// or `godark unlock` to clear stale locks.
package lock

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/label"
)

const (
	// LockLabelColor is the hex color (without #) for the coordination label.
	LockLabelColor = "FF6B35"

	// LockLabelDescription describes the label's purpose.
	LockLabelDescription = "godark run in progress — do not start another instance"

	// defaultLockFilePath is where local run metadata is stored.
	defaultLockFilePath = ".godark/lock.json"
)

// RunInfo records who holds the lock and when it was acquired.
// It is written to the local lock file for stale-lock detection.
type RunInfo struct {
	StartedAt time.Time `json:"started_at"`
	Host      string    `json:"host"`
	PID       int       `json:"pid"`
	Repo      string    `json:"repo"`
	Issues    []int     `json:"issues"`
}

// Locker manages a distributed run lock using GitHub labels as the visible
// signal and a local JSON file for stale-lock detection metadata.
type Locker struct {
	repo         string
	label        string
	lockFilePath string
	logger       *slog.Logger
}

// New returns a Locker for the given repo using the default lock label and file path.
func New(repo string, logger *slog.Logger) *Locker {
	return &Locker{
		repo:         repo,
		label:        label.InProgress,
		lockFilePath: defaultLockFilePath,
		logger:       logger,
	}
}

// Acquire checks for an existing lock and, if none is held, applies the lock
// label to issueNumbers and writes the local lock file.
//
// If force is true, any existing lock is released before acquiring so the
// caller can override a stale lock left by a crashed instance.
func (l *Locker) Acquire(issueNumbers []int, force bool) error {
	locked, info, err := l.IsLocked()
	if err != nil {
		return fmt.Errorf("checking for existing lock: %w", err)
	}

	if locked {
		if !force {
			return lockConflictError(l.label, info)
		}
		l.logger.Warn("force flag set — clearing existing run lock before acquiring")
		existing, err := github.FindIssuesWithLabel(l.repo, l.label)
		if err != nil {
			return fmt.Errorf("finding existing locked issues: %w", err)
		}
		if err := l.releaseFromIssues(existing); err != nil {
			return fmt.Errorf("clearing existing lock: %w", err)
		}
	}

	// Ensure the label exists in the repo (creates it if missing).
	if err := github.EnsureLabel(l.repo, l.label, LockLabelColor, LockLabelDescription); err != nil {
		return fmt.Errorf("ensuring lock label exists: %w", err)
	}

	// Apply the lock label to all issues being processed.
	var successCount int
	for _, n := range issueNumbers {
		if err := github.AddIssueLabel(l.repo, n, l.label); err != nil {
			l.logger.Warn("failed to apply lock label to issue", "issue", n, "error", err)
		} else {
			successCount++
		}
	}
	if len(issueNumbers) > 0 && successCount == 0 {
		return fmt.Errorf("failed to apply lock label to any issue: lock not acquired")
	}

	// Write local metadata for stale-lock detection.
	if err := l.writeLockFile(issueNumbers); err != nil {
		l.logger.Warn("failed to write lock file", "error", err)
	}

	l.logger.Info("run lock acquired", "label", l.label, "issues", issueNumbers)
	return nil
}

// Release removes the lock label from issueNumbers and deletes the local lock file.
// Called via defer at the end of a run.
func (l *Locker) Release(issueNumbers []int) error {
	if err := l.releaseFromIssues(issueNumbers); err != nil {
		return err
	}
	if err := l.deleteLockFile(); err != nil {
		l.logger.Warn("failed to delete lock file", "error", err)
	}
	l.logger.Info("run lock released")
	return nil
}

// ReleaseAll finds all open issues with the lock label in the repo and removes
// the label from each of them. It also deletes the local lock file.
// Used by `godark unlock` for manual cleanup of stale locks.
// Returns the number of issues found with the lock label.
func (l *Locker) ReleaseAll() (int, error) {
	issues, err := github.FindAllIssuesWithLabel(l.repo, l.label)
	if err != nil {
		return 0, fmt.Errorf("finding locked issues: %w", err)
	}
	if err := l.releaseFromIssues(issues); err != nil {
		return len(issues), err
	}
	if err := l.deleteLockFile(); err != nil {
		l.logger.Warn("failed to delete lock file", "error", err)
	}
	return len(issues), nil
}

// IsLocked reports whether a lock is currently held by checking for open issues
// with the lock label. The RunInfo from the local lock file is returned when
// available, providing context for stale-lock detection.
func (l *Locker) IsLocked() (bool, *RunInfo, error) {
	issues, err := github.FindIssuesWithLabel(l.repo, l.label)
	if err != nil {
		return false, nil, fmt.Errorf("querying lock label: %w", err)
	}
	if len(issues) == 0 {
		return false, nil, nil
	}
	info, _ := l.readLockFile() // best-effort; ignore error
	return true, info, nil
}

// EnsureLabelExists creates the lock label in the repo if it doesn't exist.
// Intended to be called by `godark init` to pre-create the label.
func EnsureLabelExists(repo string) error {
	return github.EnsureLabel(repo, label.InProgress, LockLabelColor, LockLabelDescription)
}

func (l *Locker) releaseFromIssues(issueNumbers []int) error {
	var firstErr error
	for _, n := range issueNumbers {
		if err := github.RemoveIssueLabel(l.repo, n, l.label); err != nil {
			l.logger.Warn("failed to remove lock label from issue", "issue", n, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (l *Locker) writeLockFile(issueNumbers []int) error {
	host, _ := os.Hostname()
	info := RunInfo{
		StartedAt: time.Now().UTC(),
		Host:      host,
		PID:       os.Getpid(),
		Repo:      l.repo,
		Issues:    issueNumbers,
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding lock file: %w", err)
	}
	dir := filepath.Dir(l.lockFilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating lock dir: %w", err)
	}
	return os.WriteFile(l.lockFilePath, data, 0o644)
}

func (l *Locker) readLockFile() (*RunInfo, error) {
	data, err := os.ReadFile(l.lockFilePath)
	if err != nil {
		return nil, err
	}
	var info RunInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parsing lock file: %w", err)
	}
	return &info, nil
}

func (l *Locker) deleteLockFile() error {
	if err := os.Remove(l.lockFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing lock file: %w", err)
	}
	return nil
}

func lockConflictError(label string, info *RunInfo) error {
	if info != nil {
		return fmt.Errorf(
			"another godark instance is running (started %s on %s, PID %d) — "+
				"use --force to override or `godark unlock` to clear stale locks",
			info.StartedAt.Format(time.RFC3339),
			info.Host,
			info.PID,
		)
	}
	return fmt.Errorf(
		"another godark instance is running (label %q found on open issues) — "+
			"use --force to override or `godark unlock` to clear stale locks",
		label,
	)
}
