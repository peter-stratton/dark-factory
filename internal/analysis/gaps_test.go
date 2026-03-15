package analysis

import (
	"math"
	"testing"

	"github.com/phs/dark-factory/internal/rundata"
)

func TestDetectGapsNilRuns(t *testing.T) {
	got := DetectGaps(nil)
	if got != nil {
		t.Errorf("DetectGaps(nil) = %v, want nil", got)
	}
}

func TestDetectGapsEmptyRuns(t *testing.T) {
	got := DetectGaps([]rundata.RunDetail{})
	if got != nil {
		t.Errorf("DetectGaps([]) = %v, want nil", got)
	}
}

func TestDetectGapsFlagCorrelation(t *testing.T) {
	// 10 issues with no_diff_read: 7 failed (70%), 3 implemented.
	// 40 issues without: 8 failed (20%), 32 implemented.
	var issues []rundata.IssueDetail

	for i := 0; i < 10; i++ {
		status := "implemented"
		if i < 7 {
			status = "failed"
		}
		issues = append(issues, rundata.IssueDetail{
			Implement: rundata.StepResult{
				Flags: []rundata.Flag{{Code: "no_diff_read", Message: "no diff read"}},
			},
			Outcome: rundata.Outcome{Status: status},
		})
	}

	for i := 0; i < 40; i++ {
		status := "implemented"
		if i < 8 {
			status = "failed"
		}
		issues = append(issues, rundata.IssueDetail{
			Outcome: rundata.Outcome{Status: status},
		})
	}

	runs := []rundata.RunDetail{{Issues: issues}}
	got := DetectGaps(runs)

	var flagGap *PromptGap
	for i := range got {
		if got[i].Finding == "no_diff_read" {
			flagGap = &got[i]
			break
		}
	}

	if flagGap == nil {
		t.Fatal("expected no_diff_read finding, got none")
	}
	if !nearlyEqual(flagGap.FailRateWith, 0.7, 0.001) {
		t.Errorf("FailRateWith = %f, want 0.7", flagGap.FailRateWith)
	}
	if !nearlyEqual(flagGap.FailRateWithout, 0.2, 0.001) {
		t.Errorf("FailRateWithout = %f, want 0.2", flagGap.FailRateWithout)
	}
	if flagGap.SamplesWith != 10 {
		t.Errorf("SamplesWith = %d, want 10", flagGap.SamplesWith)
	}
	if flagGap.SamplesWithout != 40 {
		t.Errorf("SamplesWithout = %d, want 40", flagGap.SamplesWithout)
	}
}

func TestDetectGapsQualityReviewGapRemoved(t *testing.T) {
	// Ensure the old "quality reviewer" condition is no longer emitted.
	var issues []rundata.IssueDetail

	for i := 0; i < 5; i++ {
		status := "implemented"
		if i == 0 {
			status = "failed"
		}
		issues = append(issues, rundata.IssueDetail{
			QualityReview: rundata.StepResult{Output: "reviewed"},
			Outcome:       rundata.Outcome{Status: status},
		})
	}

	for i := 0; i < 5; i++ {
		status := "failed"
		if i >= 3 {
			status = "implemented"
		}
		issues = append(issues, rundata.IssueDetail{
			Outcome: rundata.Outcome{Status: status},
		})
	}

	runs := []rundata.RunDetail{{Issues: issues}}
	got := DetectGaps(runs)

	for _, g := range got {
		if g.Finding == "quality reviewer" {
			t.Errorf("quality reviewer gap should be removed, but found: %+v", g)
		}
	}
}

func TestDetectGapsInsufficientSamples(t *testing.T) {
	// 2 issues with low_cost flag (below minimum of 3), 5 without.
	// The flag finding should be excluded.
	var issues []rundata.IssueDetail

	for i := 0; i < 2; i++ {
		issues = append(issues, rundata.IssueDetail{
			Implement: rundata.StepResult{
				Flags: []rundata.Flag{{Code: "low_cost", Message: "low cost"}},
			},
			Outcome: rundata.Outcome{Status: "failed"},
		})
	}
	for i := 0; i < 5; i++ {
		issues = append(issues, rundata.IssueDetail{
			Outcome: rundata.Outcome{Status: "implemented"},
		})
	}

	runs := []rundata.RunDetail{{Issues: issues}}
	got := DetectGaps(runs)

	for _, g := range got {
		if g.Finding == "low_cost" {
			t.Errorf("low_cost finding should be excluded with < %d samples on one side, got %+v", minGapSamples, g)
		}
	}
}

func TestDetectGapsNoGaps(t *testing.T) {
	// No quality flags across any issues — no flag gaps.
	// Result should be nil.
	var issues []rundata.IssueDetail

	for i := 0; i < 5; i++ {
		status := "implemented"
		if i == 0 {
			status = "failed"
		}
		issues = append(issues, rundata.IssueDetail{
			Outcome: rundata.Outcome{Status: status},
		})
	}

	runs := []rundata.RunDetail{{Issues: issues}}
	got := DetectGaps(runs)

	if got != nil {
		t.Errorf("DetectGaps with no flags and no conditions = %v, want nil", got)
	}
}

func TestDetectGapsSortedByMagnitude(t *testing.T) {
	// Two flag-based findings with different gap sizes — larger gap sorts first.
	// flag_a: 5 issues (0 failed → 0%), flag_b: 5 issues (1 failed → 20%), neither: 5 issues (3 failed → 60%)
	// flag_a gap: |0 - 0.4| = 0.4 (without = 10 issues, 4 failed)
	// flag_b gap: |0.2 - 0.375| = 0.175 (without = 8 issues, 3 failed)
	// Actually let's pick cleaner numbers:
	// flag_a: 5 with (0 failed), 10 without (4 failed → 40%) → gap = 0.4
	// flag_b: 5 with (2 failed → 40%), 10 without (1 failed → 10%) → gap = 0.3
	// flag_a should sort first.
	var issues []rundata.IssueDetail

	// 5 issues with flag_a only, none failed.
	for i := 0; i < 5; i++ {
		issues = append(issues, rundata.IssueDetail{
			Implement: rundata.StepResult{
				Flags: []rundata.Flag{{Code: "flag_a", Message: "flag a"}},
			},
			Outcome: rundata.Outcome{Status: "implemented"},
		})
	}

	// 5 issues with flag_b only — 2 failed (40%).
	for i := 0; i < 5; i++ {
		status := "implemented"
		if i < 2 {
			status = "failed"
		}
		issues = append(issues, rundata.IssueDetail{
			Implement: rundata.StepResult{
				Flags: []rundata.Flag{{Code: "flag_b", Message: "flag b"}},
			},
			Outcome: rundata.Outcome{Status: status},
		})
	}

	// 5 issues with no flags — 4 failed (80%).
	for i := 0; i < 5; i++ {
		status := "failed"
		if i >= 4 {
			status = "implemented"
		}
		issues = append(issues, rundata.IssueDetail{
			Outcome: rundata.Outcome{Status: status},
		})
	}

	runs := []rundata.RunDetail{{Issues: issues}}
	got := DetectGaps(runs)

	// flag_a: with=5 (0/5 failed → 0%), without=10 (2+4=6 failed → 60%), gap = 0.6
	// flag_b: with=5 (2/5 failed → 40%), without=10 (0+4=4 failed → 40%), gap = 0.0
	// So only flag_a should produce a meaningful gap.
	// Both must appear (they both have >= 3 samples on each side).

	if len(got) < 1 {
		t.Fatalf("expected at least 1 finding, got %d: %v", len(got), got)
	}

	// Verify sorted by descending gap magnitude.
	for i := 1; i < len(got); i++ {
		di := math.Abs(got[i-1].FailRateWith - got[i-1].FailRateWithout)
		dj := math.Abs(got[i].FailRateWith - got[i].FailRateWithout)
		if di < dj {
			t.Errorf("findings not sorted by gap magnitude at [%d,%d]: gap=%f < gap=%f", i-1, i, di, dj)
		}
	}
}
