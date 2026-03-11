package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// checksPollInterval is the delay between successive gh pr checks polls.
// Exposed as a variable so tests can override it without waiting 30 seconds.
var checksPollInterval = 30 * time.Second

// CheckFailure records a CI check that did not pass.
type CheckFailure struct {
	Name   string
	Output string // summary from gh pr checks
}

// WaitForChecks polls GitHub PR checks until all required checks pass,
// any required check fails, or the timeout expires.
// Returns the names of failed checks and their output, or nil if all passed.
func WaitForChecks(ctx context.Context, repo string, prNum int,
	required []string, timeout time.Duration, logger *slog.Logger,
) ([]CheckFailure, error) {
	if logger == nil {
		logger = slog.Default()
	}

	deadline := time.Now().Add(timeout)

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("CI checks timed out after %s", timeout)
		}

		failures, done, err := pollChecks(repo, prNum, required, logger)
		if err != nil {
			return nil, fmt.Errorf("polling PR checks: %w", err)
		}
		if done {
			return failures, nil
		}

		logger.Info("waiting for CI checks", "pr_number", prNum, "poll_interval", checksPollInterval)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(checksPollInterval):
		}
	}
}

// checkEntry is used to parse gh pr checks JSON output.
// gh pr checks --json exposes: name, state, bucket (not "conclusion").
// state values: SUCCESS, FAILURE, PENDING, QUEUED, etc.
// bucket values: pass, fail, pending, skipping.
type checkEntry struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Bucket string `json:"bucket"`
}

// pollChecks queries GitHub PR checks and returns any failed required checks,
// whether a final result is available, and any error.
// Returns (failures, true, nil) when all required checks have completed.
// Returns (nil, false, nil) when some required checks are still pending.
// Returns (nil, false, err) on command or parse errors.
func pollChecks(repo string, prNum int, required []string, logger *slog.Logger) ([]CheckFailure, bool, error) {
	out, err := GuardRunner("gh", "pr", "checks", strconv.Itoa(prNum),
		"--repo", repo,
		"--json", "name,state,bucket",
	)
	if err != nil {
		// gh pr checks exits with code 1 when no checks are configured for the
		// branch (e.g. CI workflow only runs on main). Detect this by checking
		// for empty output; a genuine command error (network, auth, etc.) will
		// also produce non-empty stderr.
		if strings.TrimSpace(string(out)) == "" {
			if len(required) > 0 {
				logger.Warn("no checks reported by gh pr checks; treating as passed",
					"pr_number", prNum, "required", required)
			}
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("gh pr checks: %w", err)
	}

	var checks []checkEntry
	if err := json.Unmarshal(out, &checks); err != nil {
		return nil, false, fmt.Errorf("parsing checks JSON: %w", err)
	}

	// Index checks by name for O(1) lookup.
	byName := make(map[string]checkEntry, len(checks))
	for _, c := range checks {
		byName[c.Name] = c
	}

	var failures []CheckFailure
	pendingCount := 0

	for _, name := range required {
		c, ok := byName[name]
		if !ok {
			// Required check not yet present — treat as still pending.
			logger.Info("required check not yet present, waiting", "check", name)
			pendingCount++
			continue
		}

		bucket := strings.ToLower(c.Bucket)

		switch bucket {
		case "pass", "skipping":
			logger.Info("required check passed", "check", name, "state", c.State)
		case "fail":
			logger.Warn("required check failed", "check", name, "state", c.State)
			failures = append(failures, CheckFailure{
				Name:   name,
				Output: fmt.Sprintf("Check %q failed (state %q)", name, c.State),
			})
		default:
			// pending or unknown — keep waiting.
			logger.Info("required check still running", "check", name, "bucket", c.Bucket, "state", c.State)
			pendingCount++
		}
	}

	// Fail fast: if any required check has failed, return immediately without
	// waiting for remaining pending checks.
	if len(failures) > 0 {
		return failures, true, nil
	}
	if pendingCount > 0 {
		return nil, false, nil
	}
	// All required checks completed successfully.
	return nil, true, nil
}

// formatCheckFailures formats a slice of CheckFailure values into a string
// suitable for feeding to the VerifyFix agent as error context.
func formatCheckFailures(failures []CheckFailure) string {
	var sb strings.Builder
	for i, f := range failures {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString("CI Check: ")
		sb.WriteString(f.Name)
		sb.WriteString("\n")
		sb.WriteString(f.Output)
	}
	return sb.String()
}
