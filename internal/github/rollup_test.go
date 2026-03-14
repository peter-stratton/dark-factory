package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestParsePRNumber(t *testing.T) {
	tests := []struct {
		url     string
		want    int
		wantErr bool
	}{
		{"https://github.com/owner/repo/pull/42", 42, false},
		{"https://github.com/owner/repo/pull/42/", 42, false},
		{"https://github.com/owner/repo/pull/1000", 1000, false},
		{"https://github.com/owner/repo/pull/abc", 0, true},
		{"no-slash", 0, true},
	}

	for _, tt := range tests {
		got, err := parsePRNumber(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("parsePRNumber(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parsePRNumber(%q) = %d, want %d", tt.url, got, tt.want)
		}
	}
}

func TestCreateRollupPR_Success(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("https://github.com/owner/repo/pull/77\n"), nil
	}

	prNum, prURL, err := CreateRollupPR("owner/repo", "feature-branch", "main", "chore: rollup", "body text")
	if err != nil {
		t.Fatalf("CreateRollupPR() error = %v", err)
	}
	if prNum != 77 {
		t.Errorf("CreateRollupPR() prNum = %d, want 77", prNum)
	}
	if prURL != "https://github.com/owner/repo/pull/77" {
		t.Errorf("CreateRollupPR() prURL = %q, want URL", prURL)
	}
}

func TestCreateRollupPR_CommandError(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, errFake
	}

	_, _, err := CreateRollupPR("owner/repo", "feature-branch", "main", "title", "body")
	if err == nil {
		t.Fatal("expected error when command fails")
	}
}

func TestMergeRollupPR_Success(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	var capturedArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte(""), nil
	}

	if err := MergeRollupPR("owner/repo", 42); err != nil {
		t.Fatalf("MergeRollupPR() error = %v", err)
	}

	// Verify gh pr merge was called with expected args.
	found := false
	for _, a := range capturedArgs {
		if a == "--squash" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --squash flag in args, got %v", capturedArgs)
	}
}

func TestMergeRollupPR_CommandError(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, errFake
	}

	if err := MergeRollupPR("owner/repo", 42); err == nil {
		t.Fatal("expected error when command fails")
	}
}

// TestUpsertRollupPR_NoExistingPR verifies that UpsertRollupPR falls through
// to CreateRollupPR when no open PR exists for the head→base branch pair.
func TestUpsertRollupPR_NoExistingPR(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })

	var calls []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		call := strings.Join(append([]string{name}, args...), " ")
		calls = append(calls, call)

		if name == "gh" && len(args) > 0 && args[0] == "pr" && args[1] == "list" {
			// No existing PR.
			return []byte("[]"), nil
		}
		if name == "gh" && len(args) > 0 && args[0] == "pr" && args[1] == "create" {
			return []byte("https://github.com/owner/repo/pull/100\n"), nil
		}
		return nil, fmt.Errorf("unexpected command: %s", call)
	}

	prNum, prURL, err := UpsertRollupPR("owner/repo", "feature-branch", "main", "chore: rollup", "## Implemented issues\n\n- #1 Issue one\n")
	if err != nil {
		t.Fatalf("UpsertRollupPR() error = %v", err)
	}
	if prNum != 100 {
		t.Errorf("UpsertRollupPR() prNum = %d, want 100", prNum)
	}
	if prURL != "https://github.com/owner/repo/pull/100" {
		t.Errorf("UpsertRollupPR() prURL = %q", prURL)
	}
}

// TestUpsertRollupPR_ExistingPR verifies that UpsertRollupPR updates the body
// of an existing open PR without duplicating already-listed issues.
func TestUpsertRollupPR_ExistingPR(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })

	existingBody := "## Implemented issues\n\n- #1 Issue one\n- #2 Issue two\n"
	var editedBody string

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if name != "gh" {
			return nil, fmt.Errorf("unexpected command: %s", name)
		}
		switch args[1] {
		case "list":
			prs := []rollupPRLookup{{Number: 55, URL: "https://github.com/owner/repo/pull/55"}}
			out, _ := json.Marshal(prs)
			return out, nil
		case "view":
			body, _ := json.Marshal(struct {
				Body string `json:"body"`
			}{Body: existingBody})
			return body, nil
		case "edit":
			// Capture the body passed to gh pr edit.
			for i, a := range args {
				if a == "--body" && i+1 < len(args) {
					editedBody = args[i+1]
				}
			}
			return []byte(""), nil
		default:
			return nil, fmt.Errorf("unexpected gh subcommand: %s", args[1])
		}
	}

	newBody := "## Implemented issues\n\n- #2 Issue two\n- #3 Issue three\n"
	prNum, prURL, err := UpsertRollupPR("owner/repo", "feature-branch", "main", "chore: rollup", newBody)
	if err != nil {
		t.Fatalf("UpsertRollupPR() error = %v", err)
	}
	if prNum != 55 {
		t.Errorf("UpsertRollupPR() prNum = %d, want 55", prNum)
	}
	if prURL != "https://github.com/owner/repo/pull/55" {
		t.Errorf("UpsertRollupPR() prURL = %q", prURL)
	}

	// #2 should not be duplicated; #3 should be appended.
	if strings.Count(editedBody, "- #2") != 1 {
		t.Errorf("issue #2 duplicated in merged body:\n%s", editedBody)
	}
	if !strings.Contains(editedBody, "- #3 Issue three") {
		t.Errorf("expected #3 in merged body, got:\n%s", editedBody)
	}
}

// TestUpsertRollupPR_NoDuplicatesWhenAllPresent verifies that no edit is made
// when all new issues are already present in the existing PR body.
func TestUpsertRollupPR_NoDuplicatesWhenAllPresent(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })

	existingBody := "## Implemented issues\n\n- #1 Issue one\n- #2 Issue two\n"
	editCalled := false

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if name != "gh" {
			return nil, fmt.Errorf("unexpected command: %s", name)
		}
		switch args[1] {
		case "list":
			prs := []rollupPRLookup{{Number: 55, URL: "https://github.com/owner/repo/pull/55"}}
			out, _ := json.Marshal(prs)
			return out, nil
		case "view":
			body, _ := json.Marshal(struct {
				Body string `json:"body"`
			}{Body: existingBody})
			return body, nil
		case "edit":
			editCalled = true
			return []byte(""), nil
		default:
			return nil, fmt.Errorf("unexpected gh subcommand: %s", args[1])
		}
	}

	// newBody contains only issues already in existingBody.
	newBody := "## Implemented issues\n\n- #1 Issue one\n- #2 Issue two\n"
	_, _, err := UpsertRollupPR("owner/repo", "feature-branch", "main", "chore: rollup", newBody)
	if err != nil {
		t.Fatalf("UpsertRollupPR() error = %v", err)
	}
	if editCalled {
		t.Error("expected gh pr edit NOT to be called when no new issues to add")
	}
}

// TestParseListedIssueNumbers verifies that issue numbers are extracted correctly.
func TestParseListedIssueNumbers(t *testing.T) {
	body := "## Implemented issues\n\n- #1 Foo\n- #42 Bar\n- #100 Baz\n"
	got := parseListedIssueNumbers(body)

	for _, n := range []int{1, 42, 100} {
		if !got[n] {
			t.Errorf("expected issue #%d in parsed set", n)
		}
	}
	if len(got) != 3 {
		t.Errorf("expected 3 issues, got %d", len(got))
	}
}

// TestMergeRollupBodies_AppendsNewIssues verifies that new issues are appended.
func TestMergeRollupBodies_AppendsNewIssues(t *testing.T) {
	existing := "## Implemented issues\n\n- #1 First\n"
	newBody := "## Implemented issues\n\n- #1 First\n- #2 Second\n"

	merged := mergeRollupBodies(existing, newBody)

	if strings.Count(merged, "- #1") != 1 {
		t.Errorf("issue #1 duplicated in merged body:\n%s", merged)
	}
	if !strings.Contains(merged, "- #2 Second") {
		t.Errorf("expected #2 in merged body, got:\n%s", merged)
	}
}

// TestMergeRollupBodies_NoChangeWhenAllPresent verifies that existingBody is
// returned unchanged when all new issues are already listed.
func TestMergeRollupBodies_NoChangeWhenAllPresent(t *testing.T) {
	existing := "## Implemented issues\n\n- #1 First\n- #2 Second\n"
	newBody := "## Implemented issues\n\n- #1 First\n"

	merged := mergeRollupBodies(existing, newBody)
	if merged != existing {
		t.Errorf("expected unchanged body, got:\n%s", merged)
	}
}

// errFake is a sentinel error for tests.
var errFake = &fakeError{"fake command error"}

type fakeError struct{ msg string }

func (e *fakeError) Error() string { return e.msg }
