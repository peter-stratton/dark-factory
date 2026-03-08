package quality

import (
	"fmt"
	"strings"
	"time"
)

// Flag represents a quality issue detected in a review run.
type Flag struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CheckCostFloor returns a low_cost flag if costUSD is below threshold.
// Returns nil if threshold is 0 (check disabled) or if cost is at or above threshold.
func CheckCostFloor(costUSD, threshold float64) *Flag {
	if threshold == 0 {
		return nil
	}
	if costUSD < threshold {
		return &Flag{
			Code:    "low_cost",
			Message: fmt.Sprintf("review cost $%.4f is below threshold $%.4f", costUSD, threshold),
		}
	}
	return nil
}

// CheckDuration returns a short_duration flag if duration is below threshold.
// Returns nil if threshold is 0 (check disabled) or if duration is at or above threshold.
func CheckDuration(duration time.Duration, threshold time.Duration) *Flag {
	if threshold == 0 {
		return nil
	}
	if duration < threshold {
		return &Flag{
			Code:    "short_duration",
			Message: fmt.Sprintf("review duration %s is below threshold %s", duration, threshold),
		}
	}
	return nil
}

// CheckToolTrace inspects the tool trace for evidence of a diff read and test run.
// Returns no_diff_read if no Read or "gh pr diff" call is found.
// Returns no_tests_run if testCommand is not found in the trace.
// If testCommand is empty or checkTestRun is false, the test run check is skipped.
func CheckToolTrace(toolTrace []string, testCommand string, checkTestRun bool) []Flag {
	var flags []Flag

	diffRead := false
	testsRun := !checkTestRun || testCommand == ""

	for _, entry := range toolTrace {
		if strings.Contains(entry, "Read") || strings.Contains(entry, "gh pr diff") {
			diffRead = true
		}
		if !testsRun && strings.Contains(entry, testCommand) {
			testsRun = true
		}
	}

	if !diffRead {
		flags = append(flags, Flag{
			Code:    "no_diff_read",
			Message: "no diff read detected in tool trace (expected Read or gh pr diff)",
		})
	}
	if !testsRun {
		flags = append(flags, Flag{
			Code:    "no_tests_run",
			Message: fmt.Sprintf("no test run detected in tool trace (expected %q)", testCommand),
		})
	}

	return flags
}

// CheckReviewTestExecution inspects the tool trace for evidence that review tests
// were written to reviewDir and executed via testCommand.
// Returns no_review_tests_written if no Write to reviewDir is found and hasScenarioSpec is true.
// Returns no_review_tests_run if testCommand is not found in the trace and hasScenarioSpec is true.
// When hasScenarioSpec is false, both checks are skipped (no scenario spec means no tests expected).
func CheckReviewTestExecution(toolTrace []string, reviewDir, testCommand string, hasScenarioSpec bool) []Flag {
	if !hasScenarioSpec {
		return nil
	}

	var flags []Flag

	testsWritten := reviewDir == ""
	testsRun := testCommand == ""

	dir := strings.TrimRight(reviewDir, "/")

	for _, entry := range toolTrace {
		if !testsWritten && strings.Contains(entry, dir+"/") {
			if strings.Contains(entry, "Write") {
				testsWritten = true
			} else if strings.Contains(entry, "Bash") && isBashWrite(entry, dir) {
				testsWritten = true
			}
		}
		if !testsRun && strings.Contains(entry, testCommand) {
			testsRun = true
		}
	}

	if !testsWritten {
		flags = append(flags, Flag{
			Code:    "no_review_tests_written",
			Message: fmt.Sprintf("no test files written to review dir %q in tool trace", reviewDir),
		})
	}
	if !testsRun {
		flags = append(flags, Flag{
			Code:    "no_review_tests_run",
			Message: fmt.Sprintf("test command %q not found in tool trace", testCommand),
		})
	}

	return flags
}

// isBashWrite returns true if entry looks like a Bash command that creates a
// file inside dir (e.g. cat >, tee, heredoc redirects). This catches cases
// where the reviewer uses Bash instead of the Write tool to create test files.
func isBashWrite(entry, dir string) bool {
	// Look for common file-creation patterns followed by the review dir path.
	// The entry format is "Bash: <command>".
	for _, pattern := range []string{"cat >", "cat>>", "tee ", "echo >", "echo>>", "printf >", "printf>>", "> " + dir, ">> " + dir} {
		if strings.Contains(entry, pattern) {
			return true
		}
	}
	// Heredoc with cat writing to the dir.
	if strings.Contains(entry, "cat") && strings.Contains(entry, "<<") {
		return true
	}
	// mkdir for the review dir suggests the agent is about to write files.
	if strings.Contains(entry, "mkdir") && strings.Contains(entry, dir) {
		return true
	}
	return false
}
