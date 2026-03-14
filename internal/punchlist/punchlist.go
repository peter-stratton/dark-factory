package punchlist

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gexec "github.com/phs/dark-factory/internal/exec"
	"github.com/phs/dark-factory/internal/mdutil"
)

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner gexec.CommandRunnerFunc = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// Entry holds the data needed to generate one punchlist item.
type Entry struct {
	IssueNumber     int
	IssueTitle      string
	IssueBody       string
	PRNumber        int
	Repo            string
	ScenarioSpec    string // content of the scenario spec file, if any
	ChangedFiles    []string
	AcceptanceTests []string // LLM-generated acceptance test suggestions
}

// FetchChangedFiles returns the list of files changed in the given PR.
func FetchChangedFiles(repo string, prNum int) ([]string, error) {
	out, err := CommandRunner("gh", "pr", "diff",
		fmt.Sprintf("%d", prNum),
		"--repo", repo,
		"--name-only",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr diff: %w", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// FetchPRDiff returns the full diff for the given PR, truncated to maxLen bytes.
func FetchPRDiff(repo string, prNum int, maxLen int) (string, error) {
	out, err := CommandRunner("gh", "pr", "diff",
		fmt.Sprintf("%d", prNum),
		"--repo", repo,
	)
	if err != nil {
		return "", fmt.Errorf("gh pr diff: %w", err)
	}
	s := string(out)
	if maxLen > 0 && len(s) > maxLen {
		s = s[:maxLen]
	}
	return s, nil
}

// ReadScenarioSpec reads the content of the first scenario spec file in
// scenarioDir that references the given issue number. Returns ("", nil) if
// none found, or ("", err) if the directory cannot be walked.
func ReadScenarioSpec(scenarioDir string, issueNum int) (string, error) {
	ref := fmt.Sprintf("#%d", issueNum)
	var content string
	err := mdutil.WalkMarkdownFiles(scenarioDir, func(path string) error {
		if content != "" {
			return filepath.SkipAll
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), ref) {
			content = string(data)
		}
		return nil
	})
	return content, err
}

// Generate produces a human-readable punchlist from the given entries.
// Returns an empty string if there are no entries.
func Generate(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("=== Manual Testing Punchlist ===\n\n")

	for i, e := range entries {
		fmt.Fprintf(&sb, "%d. Issue #%d: %s (PR #%d)\n", i+1, e.IssueNumber, e.IssueTitle, e.PRNumber)
		fmt.Fprintf(&sb, "   https://github.com/%s/pull/%d\n", e.Repo, e.PRNumber)

		// Verification steps from issue body.
		items := extractVerificationSteps(e.IssueBody)
		if len(items) > 0 {
			sb.WriteString("\n   Verification steps (from issue):\n")
			for _, item := range items {
				fmt.Fprintf(&sb, "   - [ ] %s\n", item)
			}
		}

		// Test cases from scenario spec.
		if e.ScenarioSpec != "" {
			cases := extractScenarioCases(e.ScenarioSpec)
			if len(cases) > 0 {
				sb.WriteString("\n   Scenario test cases:\n")
				for _, c := range cases {
					fmt.Fprintf(&sb, "   - [ ] %s\n", c)
				}
			}
		}

		// LLM-generated acceptance tests.
		if len(e.AcceptanceTests) > 0 {
			sb.WriteString("\n   Suggested acceptance tests:\n")
			for _, t := range e.AcceptanceTests {
				fmt.Fprintf(&sb, "   - [ ] %s\n", t)
			}
		}

		// Changed files.
		if len(e.ChangedFiles) > 0 {
			sb.WriteString("\n   Changed files:\n")
			for _, f := range e.ChangedFiles {
				fmt.Fprintf(&sb, "   - %s\n", f)
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// Write prints the punchlist to stdout and, if path is non-empty, also writes
// it to a file at that path.
func Write(text, path string) error {
	fmt.Print(text)
	if path == "" {
		return nil
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return fmt.Errorf("writing punchlist to %s: %w", path, err)
	}
	return nil
}

// ExtractVerificationSteps returns the verification step strings for the entry.
func (e Entry) ExtractVerificationSteps() []string {
	return extractVerificationSteps(e.IssueBody)
}

// ExtractScenarioCases returns the scenario case names for the entry.
func (e Entry) ExtractScenarioCases() []string {
	return extractScenarioCases(e.ScenarioSpec)
}

// extractPrefixedItem checks each prefix in order and returns the line with the
// first matching prefix stripped. Returns ("", false) if no prefix matches.
func extractPrefixedItem(line string, prefixes ...string) (string, bool) {
	for _, p := range prefixes {
		if strings.HasPrefix(line, p) {
			return strings.TrimPrefix(line, p), true
		}
	}
	return "", false
}

// extractVerificationSteps extracts manual verification items from an issue body.
// It collects checkbox items (- [ ]) from anywhere in the body, plus plain
// bullet points within test/acceptance/verification/cases section headers.
func extractVerificationSteps(body string) []string {
	var items []string
	lines := strings.Split(body, "\n")
	inTestSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect section headers.
		if strings.HasPrefix(trimmed, "#") {
			lower := strings.ToLower(trimmed)
			inTestSection = strings.Contains(lower, "test") ||
				strings.Contains(lower, "accept") ||
				strings.Contains(lower, "verif") ||
				strings.Contains(lower, "manual") ||
				strings.Contains(lower, "cases")
			continue
		}

		// Explicit checkboxes anywhere in the body.
		if item, ok := extractPrefixedItem(trimmed, "- [ ] ", "* [ ] "); ok {
			items = append(items, item)
			continue
		}

		// Bullet points within test/acceptance sections.
		if inTestSection {
			if item, ok := extractPrefixedItem(trimmed, "- ", "* "); ok && item != "" {
				items = append(items, item)
			}
		}
	}
	return items
}

// extractScenarioCases extracts test case names (### headers) from a scenario spec.
func extractScenarioCases(spec string) []string {
	var cases []string
	for _, line := range strings.Split(spec, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			caseName := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			if caseName != "" {
				cases = append(cases, caseName)
			}
		}
	}
	return cases
}
