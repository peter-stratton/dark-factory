package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GuardRunner executes a command and returns its combined output.
// Replaceable for testing.
var GuardRunner = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// FindPR returns the PR number for the given branch in the given repo,
// or 0 if no PR is found.
func FindPR(repo, branch string) (int, error) {
	out, err := GuardRunner("gh", "pr", "view", branch, "--repo", repo, "--json", "number")
	if err != nil {
		// gh pr view exits non-zero when no PR exists
		return 0, nil
	}

	var result struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return 0, fmt.Errorf("parsing PR JSON: %w", err)
	}
	return result.Number, nil
}

// EnsureClosesRef checks whether the PR body contains "Closes #N".
// If missing, it appends the reference via gh pr edit.
func EnsureClosesRef(repo string, prNum, issueNum int) error {
	out, err := GuardRunner("gh", "pr", "view", strconv.Itoa(prNum), "--repo", repo, "--json", "body")
	if err != nil {
		return fmt.Errorf("reading PR body: %w", err)
	}

	var pr struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return fmt.Errorf("parsing PR body: %w", err)
	}

	ref := fmt.Sprintf("Closes #%d", issueNum)
	if strings.Contains(pr.Body, ref) {
		return nil
	}

	// Also check for "Fixes #N" variant
	fixRef := fmt.Sprintf("Fixes #%d", issueNum)
	if strings.Contains(pr.Body, fixRef) {
		return nil
	}

	newBody := pr.Body + "\n\n" + ref
	_, err = GuardRunner("gh", "pr", "edit", strconv.Itoa(prNum), "--repo", repo, "--body", newBody)
	if err != nil {
		return fmt.Errorf("editing PR body: %w", err)
	}
	return nil
}

// CheckProtectedDrift returns the subset of protectedPaths that have been
// modified between baseSHA and HEAD.
func CheckProtectedDrift(baseSHA string, protectedPaths []string) ([]string, error) {
	out, err := GuardRunner("git", "diff", "--name-only", baseSHA+"...HEAD")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	changed := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			changed[line] = true
		}
	}

	var touched []string
	for _, p := range protectedPaths {
		if changed[p] {
			touched = append(touched, p)
		}
	}
	return touched, nil
}

// ClosePR closes the given PR with a comment explaining the reason.
func ClosePR(repo string, prNum int, reason string) error {
	_, err := GuardRunner("gh", "pr", "close", strconv.Itoa(prNum), "--repo", repo, "--comment", reason)
	if err != nil {
		return fmt.Errorf("closing PR #%d: %w", prNum, err)
	}
	return nil
}

// ParseReviewResult scans the agent stdout for a REVIEW_RESULT line and
// returns "APPROVED", "CHANGES_REQUESTED", or "" if not found.
func ParseReviewResult(stdout string) string {
	upper := strings.ToUpper(stdout)
	for _, line := range strings.Split(upper, "\n") {
		line = strings.TrimSpace(line)
		// Accept both "REVIEW_RESULT=APPROVED" and "REVIEW RESULT: APPROVED" etc.
		if strings.Contains(line, "APPROVED") && strings.Contains(line, "REVIEW") && strings.Contains(line, "RESULT") {
			if strings.Contains(line, "CHANGES") {
				return "CHANGES_REQUESTED"
			}
			return "APPROVED"
		}
		if strings.Contains(line, "CHANGES_REQUESTED") && strings.Contains(line, "REVIEW") {
			return "CHANGES_REQUESTED"
		}
	}
	return ""
}

// HasScenarioSpec returns true if any .md file in scenarioDir contains a
// reference to the given issue number (e.g. "#5").
func HasScenarioSpec(scenarioDir string, issueNum int) bool {
	ref := fmt.Sprintf("#%d", issueNum)

	entries, err := os.ReadDir(scenarioDir)
	if err != nil {
		return false
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(scenarioDir, e.Name()))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), ref) {
			return true
		}
	}
	return false
}

// WarnMissingScenario checks whether any scenario spec file in scenarioDir
// references the given issue number. If not, it posts a warning comment on
// the PR.
func WarnMissingScenario(repo string, prNum, issueNum int, scenarioDir string, logger *slog.Logger) error {
	if HasScenarioSpec(scenarioDir, issueNum) {
		return nil
	}

	if _, err := os.ReadDir(scenarioDir); err != nil && os.IsNotExist(err) {
		logger.Warn("scenario directory not found", "dir", scenarioDir)
	}

	comment := fmt.Sprintf("Warning: no scenario spec found referencing issue #%d in %s. Consider creating one with `godark create-scenario %d`.", issueNum, scenarioDir, issueNum)
	_, err := GuardRunner("gh", "pr", "comment", strconv.Itoa(prNum), "--repo", repo, "--body", comment)
	if err != nil {
		return fmt.Errorf("posting scenario warning: %w", err)
	}

	logger.Warn("no scenario spec found for issue", "issue_number", issueNum)
	return nil
}

// LabelPR adds a label to the given PR.
func LabelPR(repo string, prNum int, label string) error {
	_, err := GuardRunner("gh", "pr", "edit", strconv.Itoa(prNum), "--repo", repo, "--add-label", label)
	if err != nil {
		return fmt.Errorf("labeling PR #%d with %q: %w", prNum, label, err)
	}
	return nil
}
