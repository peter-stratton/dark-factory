package agent

import (
	"bytes"
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
// startupGrace controls how long to wait for checks to appear before treating
// absent checks as N/A (not triggered by this PR). Default 60s if zero.
// Returns the names of failed checks and their output, or nil if all passed.
func WaitForChecks(ctx context.Context, repo string, prNum int,
	required []string, timeout time.Duration, startupGrace time.Duration,
	logger *slog.Logger,
) ([]CheckFailure, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if startupGrace == 0 {
		startupGrace = 60 * time.Second
	}

	deadline := time.Now().Add(timeout)
	graceDeadline := time.Now().Add(startupGrace)
	active := make([]string, len(required))
	copy(active, required)

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("CI checks timed out after %s", timeout)
		}

		// After the grace period, drop any required checks that never appeared.
		if time.Now().After(graceDeadline) {
			active = pruneAbsentChecks(ctx, repo, prNum, active, logger)
			if len(active) == 0 {
				logger.Info("no required checks triggered after grace period, proceeding", "pr_number", prNum)
				return nil, nil
			}
			// Only prune once — set graceDeadline far in the future.
			graceDeadline = deadline.Add(time.Hour)
		}

		failures, done, err := pollChecks(repo, prNum, active, logger)
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

// pruneAbsentChecks queries current checks and removes any required checks
// that haven't appeared. Returns the filtered list of checks that are present.
func pruneAbsentChecks(ctx context.Context, repo string, prNum int, required []string, logger *slog.Logger) []string {
	out, err := GuardRunner("gh", "pr", "checks", strconv.Itoa(prNum),
		"--repo", repo,
		"--json", "name",
	)
	if err != nil {
		// gh reports "no checks reported on the '...' branch" as plain text with
		// a non-zero exit when CI is not configured for this PR's target branch.
		// Treat all required checks as absent so they get pruned.
		if strings.Contains(string(out), "no checks reported") {
			logger.Info("no checks reported during prune, treating all required as absent", "pr_number", prNum)
			return nil
		}
		// Other command errors (auth, network) — can't determine what's present, keep all required.
		return required
	}

	var checks []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &checks); err != nil {
		return required
	}

	present := make(map[string]bool, len(checks))
	for _, c := range checks {
		present[c.Name] = true
	}

	var kept []string
	for _, name := range required {
		if present[name] {
			kept = append(kept, name)
		} else {
			logger.Info("required check not triggered after grace period, skipping", "check", name)
		}
	}
	return kept
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
		// gh pr checks exits with code 1 when any check hasn't passed yet,
		// but still returns valid JSON. Try to parse the output as JSON first;
		// only treat it as a command error if parsing fails too.
		if strings.TrimSpace(string(out)) == "" {
			if len(required) > 0 {
				logger.Warn("no checks reported by gh pr checks; treating as passed",
					"pr_number", prNum, "required", required)
			}
			return nil, true, nil
		}
		// Fall through to JSON parse below — exit code 1 with valid JSON
		// just means some checks are pending/failed.
	}

	// CombinedOutput may contain stderr preamble (e.g. auth refresh
	// messages) before the JSON array. Find the first '[' to skip it.
	jsonStart := bytes.IndexByte(out, '[')
	if jsonStart < 0 {
		// gh reports "no checks reported on the '...' branch" as plain text
		// when CI workflows aren't configured to run on this PR's target branch.
		// Treat it as pending so the startup grace period can prune all required
		// checks as absent rather than failing the entire run immediately.
		if strings.Contains(string(out), "no checks reported") {
			logger.Info("no checks reported for PR, waiting for grace period to prune", "pr_number", prNum)
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("parsing checks JSON: no JSON array found in output: %s", truncateForError(out))
	}
	var checks []checkEntry
	if err := json.Unmarshal(out[jsonStart:], &checks); err != nil {
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

// ParseGraceOrDefault parses a duration string, returning 60s if empty.
func ParseGraceOrDefault(s string) time.Duration {
	if s == "" {
		return 60 * time.Second
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 60 * time.Second
	}
	return d
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

// truncateForError returns the first 200 bytes of output for inclusion in
// error messages, avoiding huge dumps in logs.
func truncateForError(out []byte) string {
	if len(out) <= 200 {
		return string(out)
	}
	return string(out[:200]) + "..."
}
