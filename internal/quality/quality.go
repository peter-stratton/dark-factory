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
// Returns no_tests_run if the test runner signature is not found in any Bash trace entry.
// If testCommand is empty or checkTestRun is false, the test run check is skipped.
func CheckToolTrace(toolTrace []string, testCommand string, checkTestRun bool) []Flag {
	var flags []Flag

	diffRead := false
	testsRun := !checkTestRun || testCommand == ""

	sig := testRunnerSignature(testCommand)

	for _, entry := range toolTrace {
		if strings.Contains(entry, "Read") || strings.Contains(entry, "gh pr diff") {
			diffRead = true
		}
		if !testsRun && matchesSignature(entry, sig) {
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

	sig := testRunnerSignature(testCommand)

	for _, entry := range toolTrace {
		if !testsWritten && strings.Contains(entry, dir+"/") {
			if strings.Contains(entry, "Write") {
				testsWritten = true
			} else if strings.Contains(entry, "Bash") && isBashWrite(entry, dir) {
				testsWritten = true
			}
		}
		if !testsRun && matchesSignature(entry, sig) {
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

// CheckSemiformalConsistency parses the reviewer's semi-formal analysis output
// and returns a flag when the verdict contradicts the stated traces.
// Returns nil if output does not contain "FORMAL CONCLUSION" (not a semiformal review).
// Returns nil when the verdict is not APPROVED (no contradiction possible).
func CheckSemiformalConsistency(output string) *Flag {
	if !strings.Contains(output, "FORMAL CONCLUSION") {
		return nil
	}
	if !strings.Contains(output, "AGENT_RESULT=APPROVED") {
		return nil
	}
	if strings.Contains(output, "NOT SATISFIED") {
		return &Flag{
			Code:    "semiformal_inconsistency",
			Message: "verdict APPROVED but acceptance trace contains NOT SATISFIED",
		}
	}
	if strings.Contains(output, ": BROKEN") {
		return &Flag{
			Code:    "semiformal_inconsistency",
			Message: "verdict APPROVED but regression trace contains BROKEN",
		}
	}
	if strings.Contains(output, "Risk: HIGH") {
		return &Flag{
			Code:    "semiformal_inconsistency",
			Message: "verdict APPROVED but uncovered paths contain Risk: HIGH",
		}
	}
	return nil
}

// testRunnerSignature extracts the core test runner tokens from a configured
// test command. It strips cd prefixes, directory targets, and shell operators
// to produce the minimal set of tokens that identify the test runner.
//
// Examples:
//
//	"cd service && go test ./..."       -> ["go", "test"]
//	"npm test"                          -> ["npm", "test"]
//	"pytest tests/"                     -> ["pytest"]
//	"cargo test"                        -> ["cargo", "test"]
//	"dart test test/"                   -> ["dart", "test"]
//	"cd src && python -m pytest"        -> ["python", "-m", "pytest"]
func testRunnerSignature(testCommand string) []string {
	if testCommand == "" {
		return nil
	}

	// Take the last segment after && (the actual test command, not cd prefix).
	parts := strings.Split(testCommand, "&&")
	cmd := strings.TrimSpace(parts[len(parts)-1])

	tokens := strings.Fields(cmd)

	// Collect tokens that form the runner invocation, stopping at directory
	// targets, redirects, or pipe operators.
	var sig []string
	for _, tok := range tokens {
		// Skip directory-like arguments (./..., tests/, src/foo).
		if strings.HasPrefix(tok, "./") || strings.HasPrefix(tok, "../") {
			continue
		}
		// Skip if it looks like a path with a trailing slash.
		if strings.HasSuffix(tok, "/") {
			continue
		}
		// Stop at shell operators.
		if tok == "|" || tok == ">" || tok == ">>" || tok == "2>&1" || tok == ";" {
			break
		}
		sig = append(sig, tok)
		// Most test runners are identified by 1-3 tokens. Once we have enough
		// to be distinctive, stop. We keep going for patterns like
		// "python -m pytest" (3 tokens).
		if len(sig) >= 3 {
			break
		}
	}

	return sig
}

// matchesSignature returns true if a tool trace entry contains all tokens from
// the test runner signature. Tokens are matched as substrings of the entry so
// that agents using absolute paths, extra flags, or redirects still match.
func matchesSignature(entry string, sig []string) bool {
	if len(sig) == 0 {
		return false
	}
	// Only check Bash entries - test commands run through the shell.
	if !strings.HasPrefix(entry, "Bash:") && !strings.HasPrefix(entry, "Bash ") {
		return false
	}
	for _, tok := range sig {
		if !strings.Contains(entry, tok) {
			return false
		}
	}
	return true
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
