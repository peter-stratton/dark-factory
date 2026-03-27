package specdelta

import (
	"strings"
	"testing"
)

func makeSpec(setup, cases string) string {
	return "# Scenario: Feature X\n\nRelates to: Issue #10\n\n## Setup\n" + setup + "\n\n## Cases\n" + cases
}

func TestDiff_AllAdded(t *testing.T) {
	after := makeSpec(
		"- Repository with issues",
		"### Happy path\nExpected outcome 1\n\n### Edge case\nEdge description\n\n### Error case\nError description\n",
	)

	d := Diff("", after)

	if len(d.AddedCases) != 3 {
		t.Errorf("AddedCases: got %d, want 3", len(d.AddedCases))
	}
	if len(d.RemovedCases) != 0 {
		t.Errorf("RemovedCases: got %d, want 0", len(d.RemovedCases))
	}
	if len(d.ChangedCases) != 0 {
		t.Errorf("ChangedCases: got %d, want 0", len(d.ChangedCases))
	}
	if !contains(d.AddedCases, "Happy path") {
		t.Error("AddedCases should contain 'Happy path'")
	}
	if !contains(d.AddedCases, "Edge case") {
		t.Error("AddedCases should contain 'Edge case'")
	}
	if !contains(d.AddedCases, "Error case") {
		t.Error("AddedCases should contain 'Error case'")
	}
}

func TestDiff_AllRemoved(t *testing.T) {
	before := makeSpec(
		"- Repository with issues",
		"### Happy path\nExpected outcome 1\n\n### Edge case\nEdge description\n",
	)

	d := Diff(before, "")

	if len(d.RemovedCases) != 2 {
		t.Errorf("RemovedCases: got %d, want 2", len(d.RemovedCases))
	}
	if len(d.AddedCases) != 0 {
		t.Errorf("AddedCases: got %d, want 0", len(d.AddedCases))
	}
	if !contains(d.RemovedCases, "Happy path") {
		t.Error("RemovedCases should contain 'Happy path'")
	}
	if !contains(d.RemovedCases, "Edge case") {
		t.Error("RemovedCases should contain 'Edge case'")
	}
}

func TestDiff_NoChange(t *testing.T) {
	spec := makeSpec(
		"- Repository with issues",
		"### Happy path\nExpected outcome 1\n\n### Edge case\nEdge description\n",
	)

	d := Diff(spec, spec)

	if !IsEmpty(d) {
		t.Errorf("expected empty delta, got: added=%v removed=%v changed=%v setupChanged=%v",
			d.AddedCases, d.RemovedCases, d.ChangedCases, d.SetupChanged)
	}
}

func TestDiff_MixedChanges(t *testing.T) {
	before := makeSpec(
		"- Repository with issues",
		"### A\nOriginal A\n\n### B\nOriginal B\n\n### C\nOriginal C\n",
	)
	after := makeSpec(
		"- Repository with issues",
		"### B\nModified B\n\n### C\nOriginal C\n\n### D\nNew D\n",
	)

	d := Diff(before, after)

	if len(d.RemovedCases) != 1 || d.RemovedCases[0] != "A" {
		t.Errorf("RemovedCases: got %v, want [A]", d.RemovedCases)
	}
	if len(d.AddedCases) != 1 || d.AddedCases[0] != "D" {
		t.Errorf("AddedCases: got %v, want [D]", d.AddedCases)
	}
	if len(d.ChangedCases) != 1 || d.ChangedCases[0].Name != "B" {
		t.Errorf("ChangedCases: got %v, want [{B ...}]", d.ChangedCases)
	}
	if d.ChangedCases[0].Before == d.ChangedCases[0].After {
		t.Error("ChangedCases[0].Before should differ from After")
	}
	if d.SetupChanged {
		t.Error("SetupChanged should be false")
	}
}

func TestDiff_SetupChanged(t *testing.T) {
	cases := "### Happy path\nExpected outcome 1\n"
	before := makeSpec("- Repository with issues", cases)
	after := makeSpec("- Repository with issues\n- Additional setup step", cases)

	d := Diff(before, after)

	if !d.SetupChanged {
		t.Error("SetupChanged should be true")
	}
	if len(d.AddedCases) != 0 || len(d.RemovedCases) != 0 || len(d.ChangedCases) != 0 {
		t.Errorf("no case changes expected, got: added=%v removed=%v changed=%v",
			d.AddedCases, d.RemovedCases, d.ChangedCases)
	}
}

func TestDiff_ChangedCasesContent(t *testing.T) {
	before := makeSpec(
		"- Setup",
		"### My case\nBefore content\n",
	)
	after := makeSpec(
		"- Setup",
		"### My case\nAfter content\n",
	)

	d := Diff(before, after)

	if len(d.ChangedCases) != 1 {
		t.Fatalf("ChangedCases: got %d, want 1", len(d.ChangedCases))
	}
	cc := d.ChangedCases[0]
	if cc.Name != "My case" {
		t.Errorf("Name: got %q, want %q", cc.Name, "My case")
	}
	if !strings.Contains(cc.Before, "Before content") {
		t.Errorf("Before should contain 'Before content', got %q", cc.Before)
	}
	if !strings.Contains(cc.After, "After content") {
		t.Errorf("After should contain 'After content', got %q", cc.After)
	}
}

func TestFormat_AddedCases(t *testing.T) {
	d := Delta{
		AddedCases: []string{"Happy path", "Edge case"},
	}
	out := Format(d)
	if !strings.Contains(out, "### Added") {
		t.Error("Format output should contain '### Added'")
	}
	if !strings.Contains(out, "Happy path") {
		t.Error("Format output should contain 'Happy path'")
	}
	if !strings.Contains(out, "Edge case") {
		t.Error("Format output should contain 'Edge case'")
	}
}

func TestFormat_EmptyDelta(t *testing.T) {
	out := Format(Delta{})
	if out != "" {
		t.Errorf("Format of empty delta should return \"\", got %q", out)
	}
}

func TestFormat_RemovedCases(t *testing.T) {
	d := Delta{
		RemovedCases: []string{"Old case"},
	}
	out := Format(d)
	if !strings.Contains(out, "### Removed") {
		t.Error("Format output should contain '### Removed'")
	}
	if !strings.Contains(out, "Old case") {
		t.Error("Format output should contain 'Old case'")
	}
}

func TestFormat_ChangedCases(t *testing.T) {
	d := Delta{
		ChangedCases: []CaseChange{
			{Name: "My case", Before: "old content", After: "new content"},
		},
	}
	out := Format(d)
	if !strings.Contains(out, "### Changed") {
		t.Error("Format output should contain '### Changed'")
	}
	if !strings.Contains(out, "My case") {
		t.Error("Format output should contain 'My case'")
	}
}

func TestFormat_SetupChanged(t *testing.T) {
	d := Delta{SetupChanged: true}
	out := Format(d)
	if !strings.Contains(out, "Setup") {
		t.Error("Format output should mention Setup change")
	}
}

func TestIsEmpty(t *testing.T) {
	if !IsEmpty(Delta{}) {
		t.Error("empty Delta should be IsEmpty")
	}
	if IsEmpty(Delta{AddedCases: []string{"x"}}) {
		t.Error("Delta with AddedCases should not be IsEmpty")
	}
	if IsEmpty(Delta{RemovedCases: []string{"x"}}) {
		t.Error("Delta with RemovedCases should not be IsEmpty")
	}
	if IsEmpty(Delta{ChangedCases: []CaseChange{{Name: "x"}}}) {
		t.Error("Delta with ChangedCases should not be IsEmpty")
	}
	if IsEmpty(Delta{SetupChanged: true}) {
		t.Error("Delta with SetupChanged should not be IsEmpty")
	}
}

// contains checks if a string slice contains the given value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
