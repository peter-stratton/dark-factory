package deps

import (
	"testing"

	"github.com/phs/dark-factory/internal/github"
)

func TestParseDeps_SimpleBlockedBy(t *testing.T) {
	body := "**Blocked by**: #1 (CLI scaffold)"
	deps := ParseDeps(body)
	if len(deps) != 1 || deps[0] != 1 {
		t.Errorf("ParseDeps = %v, want [1]", deps)
	}
}

func TestParseDeps_MultipleDeps(t *testing.T) {
	body := "Depends on: #3, #5"
	deps := ParseDeps(body)
	if len(deps) != 2 || deps[0] != 3 || deps[1] != 5 {
		t.Errorf("ParseDeps = %v, want [3 5]", deps)
	}
}

func TestParseDeps_CaseInsensitive(t *testing.T) {
	body := "BLOCKED BY: #4"
	deps := ParseDeps(body)
	if len(deps) != 1 || deps[0] != 4 {
		t.Errorf("ParseDeps = %v, want [4]", deps)
	}
}

func TestParseDeps_MarkdownBold(t *testing.T) {
	body := "**Depends on**: #2"
	deps := ParseDeps(body)
	if len(deps) != 1 || deps[0] != 2 {
		t.Errorf("ParseDeps = %v, want [2]", deps)
	}
}

func TestParseDeps_ParentheticalDescription(t *testing.T) {
	body := "Blocked by: #1 (CLI scaffold)"
	deps := ParseDeps(body)
	if len(deps) != 1 || deps[0] != 1 {
		t.Errorf("ParseDeps = %v, want [1]", deps)
	}
}

func TestParseDeps_NoDeps(t *testing.T) {
	body := "This issue has no dependency declarations.\nJust a normal issue body."
	deps := ParseDeps(body)
	if len(deps) != 0 {
		t.Errorf("ParseDeps = %v, want []", deps)
	}
}

func TestParseDeps_MultiLine(t *testing.T) {
	body := "## Description\nSome text\n**Blocked by**: #1\nMore text\nDepends on: #3, #5"
	deps := ParseDeps(body)
	if len(deps) != 3 || deps[0] != 1 || deps[1] != 3 || deps[2] != 5 {
		t.Errorf("ParseDeps = %v, want [1 3 5]", deps)
	}
}

func TestFilterUnblocked_OpenDependency(t *testing.T) {
	issues := []github.Issue{
		{Number: 2, Body: "**Blocked by**: #1"},
	}
	closed := ClosedSet([]int{}) // #1 is open
	result := FilterUnblocked(issues, closed)
	if len(result) != 0 {
		t.Errorf("got %d issues, want 0 (dep #1 is open)", len(result))
	}
}

func TestFilterUnblocked_ClosedDependency(t *testing.T) {
	issues := []github.Issue{
		{Number: 2, Body: "**Blocked by**: #1"},
	}
	closed := ClosedSet([]int{1}) // #1 is closed
	result := FilterUnblocked(issues, closed)
	if len(result) != 1 || result[0].Number != 2 {
		t.Errorf("got %v, want issue #2", result)
	}
}

func TestFilterUnblocked_MultipleDepsPartiallyOpen(t *testing.T) {
	issues := []github.Issue{
		{Number: 6, Body: "Depends on: #3, #5"},
	}
	closed := ClosedSet([]int{3}) // #3 closed, #5 open
	result := FilterUnblocked(issues, closed)
	if len(result) != 0 {
		t.Errorf("got %d issues, want 0 (#5 still open)", len(result))
	}
}

func TestFilterUnblocked_AllDepsClosed(t *testing.T) {
	issues := []github.Issue{
		{Number: 6, Body: "Depends on: #3, #5"},
	}
	closed := ClosedSet([]int{3, 5})
	result := FilterUnblocked(issues, closed)
	if len(result) != 1 || result[0].Number != 6 {
		t.Errorf("got %v, want issue #6", result)
	}
}

func TestFilterUnblocked_NoDeps(t *testing.T) {
	issues := []github.Issue{
		{Number: 7, Body: "No dependencies here."},
	}
	closed := ClosedSet([]int{})
	result := FilterUnblocked(issues, closed)
	if len(result) != 1 || result[0].Number != 7 {
		t.Errorf("got %v, want issue #7 (no deps = always included)", result)
	}
}

func TestFilterUnblocked_PreservesOrder(t *testing.T) {
	issues := []github.Issue{
		{Number: 1, Body: "No deps"},
		{Number: 2, Body: "**Blocked by**: #10"}, // #10 open → excluded
		{Number: 3, Body: "Depends on: #1"},      // #1 closed → included
		{Number: 4, Body: "No deps either"},
	}
	closed := ClosedSet([]int{1})
	result := FilterUnblocked(issues, closed)
	if len(result) != 3 {
		t.Fatalf("got %d issues, want 3", len(result))
	}
	if result[0].Number != 1 || result[1].Number != 3 || result[2].Number != 4 {
		t.Errorf("got [%d %d %d], want [1 3 4]",
			result[0].Number, result[1].Number, result[2].Number)
	}
}

func TestResolve_SameAsFilterUnblocked(t *testing.T) {
	issues := []github.Issue{
		{Number: 1, Body: ""},
		{Number: 2, Body: "**Blocked by**: #1"},
	}
	closed := ClosedSet([]int{1})
	result := Resolve(issues, closed)
	if len(result) != 2 {
		t.Errorf("got %d issues, want 2", len(result))
	}
}

func TestHasDeps(t *testing.T) {
	if !HasDeps("**Blocked by**: #1") {
		t.Error("expected HasDeps to return true for 'Blocked by' line")
	}
	if !HasDeps("Depends on: #3") {
		t.Error("expected HasDeps to return true for 'Depends on' line")
	}
	if HasDeps("No dependencies here") {
		t.Error("expected HasDeps to return false for body without deps")
	}
}

func TestCollectAllDeps(t *testing.T) {
	issues := []github.Issue{
		{Number: 1, Body: "**Blocked by**: #10"},
		{Number: 2, Body: "Depends on: #10, #20"},
		{Number: 3, Body: "No deps"},
	}
	all := CollectAllDeps(issues)
	if len(all) != 2 {
		t.Fatalf("got %d deps, want 2 (unique)", len(all))
	}
	// #10 should appear once despite being in two issues
	seen := make(map[int]bool)
	for _, d := range all {
		seen[d] = true
	}
	if !seen[10] || !seen[20] {
		t.Errorf("got deps %v, want [10 20]", all)
	}
}

func TestParseDeps_MixedCase(t *testing.T) {
	body := "blocked BY: #4"
	deps := ParseDeps(body)
	if len(deps) != 1 || deps[0] != 4 {
		t.Errorf("ParseDeps = %v, want [4]", deps)
	}
}

func TestParseDeps_EmptyBody(t *testing.T) {
	deps := ParseDeps("")
	if len(deps) != 0 {
		t.Errorf("ParseDeps(\"\") = %v, want []", deps)
	}
}

func TestParseDeps_IssueRefWithoutBlockerPrefix(t *testing.T) {
	// #5 appearing in normal text should NOT be treated as a dependency.
	body := "This relates to #5 but is not blocked by anything."
	deps := ParseDeps(body)
	if len(deps) != 0 {
		t.Errorf("ParseDeps = %v, want [] (refs without blocker prefix should be ignored)", deps)
	}
}

func TestParseDeps_LargeIssueNumber(t *testing.T) {
	body := "Blocked by: #999999"
	deps := ParseDeps(body)
	if len(deps) != 1 || deps[0] != 999999 {
		t.Errorf("ParseDeps = %v, want [999999]", deps)
	}
}

func TestResolveWithDeps(t *testing.T) {
	issues := []github.Issue{
		{Number: 1, Body: ""},
		{Number: 2, Body: "**Blocked by**: #1"},
		{Number: 3, Body: "Depends on: #99"},
	}
	closed := ClosedSet([]int{1})
	result := ResolveWithDeps(issues, closed)
	if len(result) != 2 {
		t.Fatalf("got %d issues, want 2", len(result))
	}
	if result[0].Issue.Number != 1 {
		t.Errorf("first issue = #%d, want #1", result[0].Issue.Number)
	}
	if result[1].Issue.Number != 2 {
		t.Errorf("second issue = #%d, want #2", result[1].Issue.Number)
	}
	if len(result[1].Deps) != 1 || result[1].Deps[0] != 1 {
		t.Errorf("issue #2 deps = %v, want [1]", result[1].Deps)
	}
}

func TestClosedSet(t *testing.T) {
	s := ClosedSet([]int{1, 5, 10})
	if !s[1] || !s[5] || !s[10] {
		t.Error("expected all numbers in set")
	}
	if s[2] {
		t.Error("expected 2 to not be in set")
	}
}

func TestClosedSet_Empty(t *testing.T) {
	s := ClosedSet(nil)
	if len(s) != 0 {
		t.Errorf("expected empty set, got %v", s)
	}
}
