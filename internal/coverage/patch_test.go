package coverage

import (
	"strings"
	"testing"
)

func TestComputePatchCoverage_AllCovered(t *testing.T) {
	changed := map[string][]LineRange{
		"internal/foo/bar.go": {{Start: 10, End: 15}},
	}
	blocks := []ProfileBlock{
		{File: "internal/foo/bar.go", StartLine: 1, EndLine: 20, Stmts: 5, Count: 1},
	}

	result := ComputePatchCoverage(changed, blocks)

	if result.Percent != 100 {
		t.Errorf("Percent = %f, want 100", result.Percent)
	}
	if result.TotalLines != 6 {
		t.Errorf("TotalLines = %d, want 6", result.TotalLines)
	}
	if result.CoveredLines != 6 {
		t.Errorf("CoveredLines = %d, want 6", result.CoveredLines)
	}
	if len(result.Uncovered) != 0 {
		t.Errorf("Uncovered = %v, want empty", result.Uncovered)
	}
}

func TestComputePatchCoverage_PartiallyCovered(t *testing.T) {
	changed := map[string][]LineRange{
		"internal/foo/bar.go": {
			{Start: 10, End: 15}, // 6 lines
			{Start: 20, End: 25}, // 6 lines
		},
	}
	blocks := []ProfileBlock{
		{File: "internal/foo/bar.go", StartLine: 10, EndLine: 15, Stmts: 3, Count: 1}, // covers 10-15
		{File: "internal/foo/bar.go", StartLine: 20, EndLine: 25, Stmts: 3, Count: 0}, // uncovered 20-25
	}

	result := ComputePatchCoverage(changed, blocks)

	if result.TotalLines != 12 {
		t.Errorf("TotalLines = %d, want 12", result.TotalLines)
	}
	if result.CoveredLines != 6 {
		t.Errorf("CoveredLines = %d, want 6", result.CoveredLines)
	}
	if result.Percent != 50 {
		t.Errorf("Percent = %f, want 50", result.Percent)
	}
	if len(result.Uncovered) != 1 {
		t.Fatalf("Uncovered count = %d, want 1", len(result.Uncovered))
	}
	if result.Uncovered[0].File != "internal/foo/bar.go" {
		t.Errorf("Uncovered file = %q, want %q", result.Uncovered[0].File, "internal/foo/bar.go")
	}
}

func TestComputePatchCoverage_NoChangedLines(t *testing.T) {
	result := ComputePatchCoverage(nil, nil)

	if result.Percent != 100 {
		t.Errorf("Percent = %f, want 100 for no changed lines", result.Percent)
	}
}

func TestComputePatchCoverage_NoCoverageData(t *testing.T) {
	changed := map[string][]LineRange{
		"internal/foo/bar.go": {{Start: 10, End: 15}},
	}

	result := ComputePatchCoverage(changed, nil)

	if result.TotalLines != 6 {
		t.Errorf("TotalLines = %d, want 6", result.TotalLines)
	}
	if result.CoveredLines != 0 {
		t.Errorf("CoveredLines = %d, want 0", result.CoveredLines)
	}
	if result.Percent != 0 {
		t.Errorf("Percent = %f, want 0", result.Percent)
	}
}

func TestComputePatchCoverage_MultipleFiles(t *testing.T) {
	changed := map[string][]LineRange{
		"a.go": {{Start: 1, End: 2}},  // 2 lines
		"b.go": {{Start: 10, End: 10}}, // 1 line
	}
	blocks := []ProfileBlock{
		{File: "a.go", StartLine: 1, EndLine: 2, Stmts: 1, Count: 1},
		// b.go has no coverage blocks - uncovered
	}

	result := ComputePatchCoverage(changed, blocks)

	if result.TotalLines != 3 {
		t.Errorf("TotalLines = %d, want 3", result.TotalLines)
	}
	if result.CoveredLines != 2 {
		t.Errorf("CoveredLines = %d, want 2", result.CoveredLines)
	}
}

func TestFormatResult_BelowTarget(t *testing.T) {
	result := PatchResult{
		Percent:      45.0,
		TotalLines:   40,
		CoveredLines: 18,
		Uncovered: []UncoveredFile{
			{
				File: "internal/foo/bar.go",
				Ranges: []LineRange{
					{Start: 23, End: 31},
					{Start: 45, End: 47},
				},
			},
			{
				File: "internal/baz/qux.go",
				Ranges: []LineRange{
					{Start: 10, End: 15},
				},
			},
		},
	}

	got := FormatResult(result, 80)

	if !strings.Contains(got, "45.0%") {
		t.Errorf("FormatResult() missing percentage: %s", got)
	}
	if !strings.Contains(got, "18/40") {
		t.Errorf("FormatResult() missing line counts: %s", got)
	}
	if !strings.Contains(got, "target: 80%") {
		t.Errorf("FormatResult() missing target: %s", got)
	}
	if !strings.Contains(got, "internal/foo/bar.go:23-31") {
		t.Errorf("FormatResult() missing uncovered range: %s", got)
	}
	if !strings.Contains(got, "internal/baz/qux.go:10-15") {
		t.Errorf("FormatResult() missing uncovered range: %s", got)
	}
	if !strings.Contains(got, "Write tests that exercise") {
		t.Errorf("FormatResult() missing action prompt: %s", got)
	}
}

func TestFormatResult_AtTarget(t *testing.T) {
	result := PatchResult{
		Percent:      100,
		TotalLines:   10,
		CoveredLines: 10,
	}

	got := FormatResult(result, 80)

	if !strings.Contains(got, "100.0%") {
		t.Errorf("FormatResult() missing percentage: %s", got)
	}
	if strings.Contains(got, "Uncovered") {
		t.Errorf("FormatResult() should not contain uncovered section when all covered: %s", got)
	}
}

func TestFormatResult_SingleLineUncovered(t *testing.T) {
	result := PatchResult{
		Percent:      50,
		TotalLines:   2,
		CoveredLines: 1,
		Uncovered: []UncoveredFile{
			{
				File:   "foo.go",
				Ranges: []LineRange{{Start: 5, End: 5}},
			},
		},
	}

	got := FormatResult(result, 80)

	// Single line should be formatted as "foo.go:5" not "foo.go:5-5"
	if !strings.Contains(got, "foo.go:5") {
		t.Errorf("FormatResult() missing single line reference: %s", got)
	}
	if strings.Contains(got, "foo.go:5-5") {
		t.Errorf("FormatResult() should not show range for single line: %s", got)
	}
}

func TestAppendToRanges_Contiguous(t *testing.T) {
	ranges := []LineRange{{Start: 5, End: 7}}
	got := appendToRanges(ranges, 8)

	if len(got) != 1 {
		t.Fatalf("appendToRanges() returned %d ranges, want 1", len(got))
	}
	if got[0].End != 8 {
		t.Errorf("End = %d, want 8", got[0].End)
	}
}

func TestAppendToRanges_Gap(t *testing.T) {
	ranges := []LineRange{{Start: 5, End: 7}}
	got := appendToRanges(ranges, 10)

	if len(got) != 2 {
		t.Fatalf("appendToRanges() returned %d ranges, want 2", len(got))
	}
}
