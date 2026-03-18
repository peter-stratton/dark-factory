package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	darkexec "github.com/peter-stratton/dark-factory/internal/exec"
)

// Issue represents a GitHub issue with the fields needed for orchestration.
type Issue struct {
	Number   int
	Title    string
	Body     string
	Labels   []string
	Priority string // "p1", "p2", "p3", or "" for unlabeled
}

// ghIssue maps the JSON fields returned by gh issue list.
type ghIssue struct {
	Number int     `json:"number"`
	Title  string  `json:"title"`
	Body   string  `json:"body"`
	Labels []label `json:"labels"`
}

type label struct {
	Name string `json:"name"`
}

// priorityRank returns a sort key for priority labels.
// Lower rank = higher priority. Unlabeled issues sort last.
var priorityRank = map[string]int{
	"p1": 0,
	"p2": 1,
	"p3": 2,
	"":   3,
}

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner darkexec.CommandRunnerFunc = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// ResolveMilestoneByTag fetches all milestones from the repo and returns the
// title of the one whose label form matches the given tag. For example, tag
// "phase-3" matches milestone "Phase 3: Analysis Commands" because
// milestoneToLabel("Phase 3: Analysis Commands") == "phase-3".
func ResolveMilestoneByTag(repo, tag string) (string, error) {
	out, err := CommandRunner("gh", "api",
		fmt.Sprintf("repos/%s/milestones", repo),
		"--paginate",
		"--jq", ".[].title",
	)
	if err != nil {
		return "", fmt.Errorf("fetching milestones: %w", err)
	}

	normalizedTag := strings.ToLower(strings.TrimSpace(tag))
	for _, line := range strings.Split(string(out), "\n") {
		title := strings.TrimSpace(line)
		if title == "" {
			continue
		}
		label := milestoneToLabel(title)
		if label == normalizedTag {
			return title, nil
		}
	}
	return "", fmt.Errorf("no milestone found matching tag %q", tag)
}

// milestoneToLabel converts a milestone title to its label form.
// "Phase 2" and "Phase 2: Vault Reader + Foundation" both become "phase-2".
func milestoneToLabel(title string) string {
	re := regexp.MustCompile(`(?i)^phase\s+(\d+)`)
	if m := re.FindStringSubmatch(title); m != nil {
		return "phase-" + m[1]
	}
	return strings.ReplaceAll(strings.ToLower(title), " ", "-")
}

// FetchMilestoneIssues returns open issues for the given milestone in the
// given repo, sorted by priority label (p1 → p2 → p3 → unlabeled) then by
// issue number ascending within each tier.
func FetchMilestoneIssues(repo, milestone string) ([]Issue, error) {
	out, err := CommandRunner("gh", "issue", "list",
		"--repo", repo,
		"--milestone", milestone,
		"--state", "open",
		"--json", "number,title,body,labels",
		"--limit", "200",
	)
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w", err)
	}

	var raw []ghIssue
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}

	issues := make([]Issue, len(raw))
	for i, r := range raw {
		labels := make([]string, len(r.Labels))
		for j, l := range r.Labels {
			labels[j] = l.Name
		}
		issues[i] = Issue{
			Number:   r.Number,
			Title:    r.Title,
			Body:     r.Body,
			Labels:   labels,
			Priority: extractPriority(labels),
		}
	}

	sortIssues(issues)
	return issues, nil
}

// extractPriority finds the first p1/p2/p3 label and returns it.
// Returns "" if no priority label is found.
func extractPriority(labels []string) string {
	for _, l := range labels {
		lower := strings.ToLower(l)
		if lower == "p1" || lower == "p2" || lower == "p3" {
			return lower
		}
	}
	return ""
}

// CloseIssue closes a GitHub issue by number. This is needed when PRs merge
// into a non-default branch, since GitHub's "Closes #N" keyword only
// auto-closes issues on merge to the default branch.
func CloseIssue(repo string, number int) error {
	_, err := CommandRunner("gh", "issue", "close",
		fmt.Sprintf("%d", number),
		"--repo", repo,
	)
	if err != nil {
		return fmt.Errorf("gh issue close #%d: %w", number, err)
	}
	return nil
}

// FetchClosedIssueNumbers returns the issue numbers of all closed issues in
// the given repo. This is used to build the closed-set for dependency resolution.
func FetchClosedIssueNumbers(repo string) ([]int, error) {
	return fetchNumbersByState(repo, "closed")
}

// FetchAllIssueNumbers returns a set of all issue numbers (open and closed)
// in the given repo. Useful for validating cross-references.
func FetchAllIssueNumbers(repo string) (map[int]bool, error) {
	open, err := fetchNumbersByState(repo, "open")
	if err != nil {
		return nil, err
	}
	closed, err := fetchNumbersByState(repo, "closed")
	if err != nil {
		return nil, err
	}

	all := make(map[int]bool, len(open)+len(closed))
	for _, n := range open {
		all[n] = true
	}
	for _, n := range closed {
		all[n] = true
	}
	return all, nil
}

// fetchNumbersByState fetches issue numbers filtered by state (open/closed).
func fetchNumbersByState(repo, state string) ([]int, error) {
	out, err := CommandRunner("gh", "issue", "list",
		"--repo", repo,
		"--state", state,
		"--json", "number",
		"--limit", "500",
	)
	if err != nil {
		return nil, fmt.Errorf("gh issue list (%s): %w", state, err)
	}

	var raw []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}

	numbers := make([]int, len(raw))
	for i, r := range raw {
		numbers[i] = r.Number
	}
	return numbers, nil
}

// FetchIssue returns a single GitHub issue by number.
func FetchIssue(repo string, number int) (Issue, error) {
	out, err := CommandRunner("gh", "issue", "view",
		fmt.Sprintf("%d", number),
		"--repo", repo,
		"--json", "number,title,body,labels",
	)
	if err != nil {
		return Issue{}, fmt.Errorf("gh issue view: %w", err)
	}

	var raw ghIssue
	if err := json.Unmarshal(out, &raw); err != nil {
		return Issue{}, fmt.Errorf("parsing gh output: %w", err)
	}

	labels := make([]string, len(raw.Labels))
	for i, l := range raw.Labels {
		labels[i] = l.Name
	}

	return Issue{
		Number:   raw.Number,
		Title:    raw.Title,
		Body:     raw.Body,
		Labels:   labels,
		Priority: extractPriority(labels),
	}, nil
}

// sortIssues sorts by priority rank ascending, then by issue number ascending.
func sortIssues(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool {
		ri := priorityRank[issues[i].Priority]
		rj := priorityRank[issues[j].Priority]
		if ri != rj {
			return ri < rj
		}
		return issues[i].Number < issues[j].Number
	})
}
