package github

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CreateRollupPR opens a pull request from baseBranch into defaultBranch and
// returns the PR number and URL. The PR is created with the given title and
// body. An error is returned when gh pr create fails or the URL cannot be
// parsed.
func CreateRollupPR(repo, baseBranch, defaultBranch, title, body string) (int, string, error) {
	out, err := CommandRunner("gh", "pr", "create",
		"--repo", repo,
		"--head", baseBranch,
		"--base", defaultBranch,
		"--title", title,
		"--body", body,
	)
	if err != nil {
		return 0, "", fmt.Errorf("gh pr create: %w", err)
	}

	url := strings.TrimSpace(string(out))
	prNum, err := parsePRNumber(url)
	if err != nil {
		return 0, url, fmt.Errorf("parsing PR number from %q: %w", url, err)
	}
	return prNum, url, nil
}

// UpsertRollupPR creates or updates the rollup PR for baseBranch→defaultBranch.
// If an open PR already exists for that branch pair, its body is updated to
// include any new issues not already listed. Returns the PR number and URL.
func UpsertRollupPR(repo, baseBranch, defaultBranch, title, body string) (int, string, error) {
	num, url, found, err := findOpenRollupPR(repo, baseBranch, defaultBranch)
	if err != nil {
		return 0, "", fmt.Errorf("looking up existing rollup PR: %w", err)
	}

	if !found {
		return CreateRollupPR(repo, baseBranch, defaultBranch, title, body)
	}

	// Fetch the existing PR body so we can merge without duplicating issues.
	out, err := CommandRunner("gh", "pr", "view", strconv.Itoa(num),
		"--repo", repo,
		"--json", "body",
	)
	if err != nil {
		return 0, "", fmt.Errorf("fetching rollup PR body: %w", err)
	}
	var prView struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(out, &prView); err != nil {
		return 0, "", fmt.Errorf("parsing rollup PR body: %w", err)
	}

	merged := mergeRollupBodies(prView.Body, body)
	if merged != prView.Body {
		_, err = CommandRunner("gh", "pr", "edit", strconv.Itoa(num),
			"--repo", repo,
			"--body", merged,
		)
		if err != nil {
			return 0, "", fmt.Errorf("updating rollup PR body: %w", err)
		}
	}

	return num, url, nil
}

// MergeFeaturePR merges a feature PR with squash and deletes the branch. An
// error is returned when gh pr merge fails. This is used by the watch command
// when an APPROVED review is detected; it mirrors the merge mechanics used by
// the orchestrator (squash + delete branch).
func MergeFeaturePR(repo string, prNum int) error {
	_, err := CommandRunner("gh", "pr", "merge",
		strconv.Itoa(prNum),
		"--repo", repo,
		"--squash",
		"--delete-branch",
	)
	if err != nil {
		return fmt.Errorf("gh pr merge: %w", err)
	}
	return nil
}

// MergeRollupPR merges the given PR with squash. An error is returned when
// gh pr merge fails.
func MergeRollupPR(repo string, prNum int) error {
	_, err := CommandRunner("gh", "pr", "merge",
		strconv.Itoa(prNum),
		"--repo", repo,
		"--squash",
		"--auto",
	)
	if err != nil {
		return fmt.Errorf("gh pr merge: %w", err)
	}
	return nil
}

// rollupPRLookup holds fields returned by gh pr list used for rollup PR lookup.
type rollupPRLookup struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// findOpenRollupPR checks whether an open PR from head→base already exists in
// repo. Returns the PR number, URL, and found=true when one is found.
func findOpenRollupPR(repo, head, base string) (num int, url string, found bool, err error) {
	out, err := CommandRunner("gh", "pr", "list",
		"--repo", repo,
		"--head", head,
		"--base", base,
		"--state", "open",
		"--json", "number,url",
		"--limit", "1",
	)
	if err != nil {
		return 0, "", false, fmt.Errorf("gh pr list: %w", err)
	}

	var prs []rollupPRLookup
	if err := json.Unmarshal(out, &prs); err != nil {
		return 0, "", false, fmt.Errorf("parsing pr list: %w", err)
	}

	if len(prs) == 0 {
		return 0, "", false, nil
	}
	return prs[0].Number, prs[0].URL, true, nil
}

// issueLineRe matches a rollup body issue line such as "- #123 Some title".
// The (?m) flag enables multiline mode so ^ matches the start of each line
// when the regex is applied to the full body string.
var issueLineRe = regexp.MustCompile(`(?m)^- #(\d+)`)

// issueLineRePerLine is a flag-free variant of issueLineRe for use when
// matching against individual lines already split from a body string.
var issueLineRePerLine = regexp.MustCompile(`^- #(\d+)`)

// mergeRollupBodies appends to existingBody any issue lines from newBody whose
// issue numbers are not already present in existingBody. Returns existingBody
// unchanged when there are no new issues to add.
func mergeRollupBodies(existingBody, newBody string) string {
	existing := parseListedIssueNumbers(existingBody)

	var newLines []string
	for _, line := range strings.Split(newBody, "\n") {
		m := issueLineRePerLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if !existing[n] {
			newLines = append(newLines, line)
		}
	}

	if len(newLines) == 0 {
		return existingBody
	}

	return existingBody + "\n" + strings.Join(newLines, "\n") + "\n"
}

// parseListedIssueNumbers returns a set of issue numbers already listed in a
// rollup PR body.
func parseListedIssueNumbers(body string) map[int]bool {
	result := make(map[int]bool)
	for _, m := range issueLineRe.FindAllStringSubmatch(body, -1) {
		n, err := strconv.Atoi(m[1])
		if err == nil {
			result[n] = true
		}
	}
	return result
}

// parsePRNumber extracts the pull request number from a GitHub PR URL of the
// form https://github.com/{owner}/{repo}/pull/{number}.
func parsePRNumber(url string) (int, error) {
	// Strip trailing slash if present.
	url = strings.TrimRight(url, "/")
	idx := strings.LastIndex(url, "/")
	if idx == -1 {
		return 0, fmt.Errorf("no slash found in URL %q", url)
	}
	numStr := url[idx+1:]
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("non-numeric PR number %q: %w", numStr, err)
	}
	return n, nil
}
