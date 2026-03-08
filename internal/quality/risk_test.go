package quality

import (
	"testing"
)

func TestClassifyRisk_AllPass(t *testing.T) {
	input := RiskInput{
		LinesChanged:   50,
		FilesChanged:   3,
		ChangedFiles:   []string{"internal/foo/bar.go"},
		ProtectedPaths: []string{"CLAUDE.md", "tests/scenarios/"},
		FixCycles:      0,
		QualityFlags:   nil,
	}
	got := ClassifyRisk(input, 200, 10)

	if !got.IsLowRisk {
		t.Error("expected IsLowRisk: true, got false")
	}
	for _, g := range got.Gates {
		if !g.Passed {
			t.Errorf("gate %q unexpectedly failed: %s", g.Name, g.Detail)
		}
	}
}

func TestClassifyRisk_LinesExceeded(t *testing.T) {
	input := RiskInput{
		LinesChanged: 201,
		FilesChanged: 3,
	}
	got := ClassifyRisk(input, 200, 10)

	if got.IsLowRisk {
		t.Error("expected IsLowRisk: false, got true")
	}
	assertGateFailed(t, got.Gates, "max_lines")
}

func TestClassifyRisk_FilesExceeded(t *testing.T) {
	input := RiskInput{
		LinesChanged: 50,
		FilesChanged: 11,
	}
	got := ClassifyRisk(input, 200, 10)

	if got.IsLowRisk {
		t.Error("expected IsLowRisk: false, got true")
	}
	assertGateFailed(t, got.Gates, "max_files")
}

func TestClassifyRisk_ProtectedPathTouched(t *testing.T) {
	input := RiskInput{
		LinesChanged:   10,
		FilesChanged:   1,
		ChangedFiles:   []string{"tests/scenarios/my_scenario.yaml"},
		ProtectedPaths: []string{"tests/scenarios/"},
	}
	got := ClassifyRisk(input, 200, 10)

	if got.IsLowRisk {
		t.Error("expected IsLowRisk: false, got true")
	}
	assertGateFailed(t, got.Gates, "protected_paths")
}

func TestClassifyRisk_FixCyclesUsed(t *testing.T) {
	input := RiskInput{
		LinesChanged: 10,
		FilesChanged: 1,
		FixCycles:    1,
	}
	got := ClassifyRisk(input, 200, 10)

	if got.IsLowRisk {
		t.Error("expected IsLowRisk: false, got true")
	}
	assertGateFailed(t, got.Gates, "no_fix_cycles")
}

func TestClassifyRisk_QualityFlagsRaised(t *testing.T) {
	input := RiskInput{
		LinesChanged: 10,
		FilesChanged: 1,
		QualityFlags: []Flag{{Code: "low_cost", Message: "cost too low"}},
	}
	got := ClassifyRisk(input, 200, 10)

	if got.IsLowRisk {
		t.Error("expected IsLowRisk: false, got true")
	}
	assertGateFailed(t, got.Gates, "no_quality_flags")
}

func TestClassifyRisk_MultipleGatesFail(t *testing.T) {
	input := RiskInput{
		LinesChanged: 201,
		FilesChanged: 11,
	}
	got := ClassifyRisk(input, 200, 10)

	if got.IsLowRisk {
		t.Error("expected IsLowRisk: false, got true")
	}
	assertGateFailed(t, got.Gates, "max_lines")
	assertGateFailed(t, got.Gates, "max_files")
}

func TestClassifyRisk_DefaultThresholds(t *testing.T) {
	// Passing 0 for maxLines and maxFiles should apply defaults (200, 10).
	input := RiskInput{
		LinesChanged: 200,
		FilesChanged: 10,
	}
	got := ClassifyRisk(input, 0, 0)

	if !got.IsLowRisk {
		t.Error("expected IsLowRisk: true with default thresholds, got false")
	}

	// One over the default should fail.
	input.LinesChanged = 201
	got = ClassifyRisk(input, 0, 0)
	if got.IsLowRisk {
		t.Error("expected IsLowRisk: false when lines exceed default 200, got true")
	}
	assertGateFailed(t, got.Gates, "max_lines")
}

func TestClassifyRisk_AllGatesPresent(t *testing.T) {
	input := RiskInput{}
	got := ClassifyRisk(input, 200, 10)

	wantGates := []string{"max_lines", "max_files", "protected_paths", "no_fix_cycles", "no_quality_flags"}
	if len(got.Gates) != len(wantGates) {
		t.Fatalf("expected %d gates, got %d", len(wantGates), len(got.Gates))
	}
	for i, name := range wantGates {
		if got.Gates[i].Name != name {
			t.Errorf("gate[%d]: got name %q, want %q", i, got.Gates[i].Name, name)
		}
	}
}

func TestClassifyRisk_ProtectedPathPrefixMatching(t *testing.T) {
	// Prefix match: "CLAUDE.md" matches protected path "CLAUDE.md"
	input := RiskInput{
		LinesChanged:   5,
		FilesChanged:   1,
		ChangedFiles:   []string{"CLAUDE.md"},
		ProtectedPaths: []string{"CLAUDE.md"},
	}
	got := ClassifyRisk(input, 200, 10)
	if got.IsLowRisk {
		t.Error("expected IsLowRisk: false when exact protected file touched, got true")
	}
	assertGateFailed(t, got.Gates, "protected_paths")

	// Non-matching file should pass.
	input.ChangedFiles = []string{"internal/foo.go"}
	got = ClassifyRisk(input, 200, 10)
	assertGatePassed(t, got.Gates, "protected_paths")
}

// assertGateFailed verifies that the named gate is present and failed.
func assertGateFailed(t *testing.T, gates []RiskGate, name string) {
	t.Helper()
	for _, g := range gates {
		if g.Name == name {
			if g.Passed {
				t.Errorf("gate %q: expected Passed=false, got true (detail: %s)", name, g.Detail)
			}
			return
		}
	}
	t.Errorf("gate %q not found in assessment", name)
}

// assertGatePassed verifies that the named gate is present and passed.
func assertGatePassed(t *testing.T, gates []RiskGate, name string) {
	t.Helper()
	for _, g := range gates {
		if g.Name == name {
			if !g.Passed {
				t.Errorf("gate %q: expected Passed=true, got false (detail: %s)", name, g.Detail)
			}
			return
		}
	}
	t.Errorf("gate %q not found in assessment", name)
}
