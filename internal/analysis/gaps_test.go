package analysis

import (
	"math"
	"strings"
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

func TestDetectGapsQualityReviewGap(t *testing.T) {
	// 5 issues with quality review: 1 failed (20%), 4 implemented.
	// 5 issues without quality review: 3 failed (60%), 2 implemented.
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

	var qrGap *PromptGap
	for i := range got {
		if got[i].Finding == "quality reviewer" {
			qrGap = &got[i]
			break
		}
	}

	if qrGap == nil {
		t.Fatal("expected quality reviewer finding, got none")
	}
	if !nearlyEqual(qrGap.FailRateWith, 0.2, 0.001) {
		t.Errorf("FailRateWith = %f, want 0.2", qrGap.FailRateWith)
	}
	if !nearlyEqual(qrGap.FailRateWithout, 0.6, 0.001) {
		t.Errorf("FailRateWithout = %f, want 0.6", qrGap.FailRateWithout)
	}
	if qrGap.SamplesWith != 5 {
		t.Errorf("SamplesWith = %d, want 5", qrGap.SamplesWith)
	}
	if qrGap.SamplesWithout != 5 {
		t.Errorf("SamplesWithout = %d, want 5", qrGap.SamplesWithout)
	}
}

func TestDetectGapsScenarioSpecGap(t *testing.T) {
	// 10 issues with spec gen: 1 failed (10%), 9 implemented.
	// 10 issues without spec gen: 5 failed (50%), 5 implemented.
	var issues []rundata.IssueDetail

	for i := 0; i < 10; i++ {
		status := "implemented"
		if i == 0 {
			status = "failed"
		}
		issues = append(issues, rundata.IssueDetail{
			SpecGenerator: rundata.StepResult{Output: "spec generated"},
			Outcome:       rundata.Outcome{Status: status},
		})
	}

	for i := 0; i < 10; i++ {
		status := "implemented"
		if i < 5 {
			status = "failed"
		}
		issues = append(issues, rundata.IssueDetail{
			Outcome: rundata.Outcome{Status: status},
		})
	}

	runs := []rundata.RunDetail{{Issues: issues}}
	got := DetectGaps(runs)

	var specGap *PromptGap
	for i := range got {
		if got[i].Finding == "scenario specs" {
			specGap = &got[i]
			break
		}
	}

	if specGap == nil {
		t.Fatal("expected scenario specs finding, got none")
	}
	if !nearlyEqual(specGap.FailRateWith, 0.1, 0.001) {
		t.Errorf("FailRateWith = %f, want 0.1", specGap.FailRateWith)
	}
	if !nearlyEqual(specGap.FailRateWithout, 0.5, 0.001) {
		t.Errorf("FailRateWithout = %f, want 0.5", specGap.FailRateWithout)
	}
	if specGap.SamplesWith != 10 {
		t.Errorf("SamplesWith = %d, want 10", specGap.SamplesWith)
	}
	if specGap.SamplesWithout != 10 {
		t.Errorf("SamplesWithout = %d, want 10", specGap.SamplesWithout)
	}
}

func TestDetectGapsExhaustedRetries(t *testing.T) {
	// 2 issues with needs-human-review — finding lists both.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					IssueNumber: 10,
					Outcome:     rundata.Outcome{Status: "needs-human-review", Title: "Issue Alpha"},
				},
				{
					IssueNumber: 20,
					Outcome:     rundata.Outcome{Status: "needs-human-review", Title: "Issue Beta"},
				},
			},
		},
	}

	got := DetectGaps(runs)

	var exGap *PromptGap
	for i := range got {
		if strings.HasPrefix(got[i].Finding, "exhausted retries:") {
			exGap = &got[i]
			break
		}
	}

	if exGap == nil {
		t.Fatal("expected exhausted retries finding, got none")
	}
	if !strings.Contains(exGap.Finding, "#10") {
		t.Errorf("finding %q should contain #10", exGap.Finding)
	}
	if !strings.Contains(exGap.Finding, "#20") {
		t.Errorf("finding %q should contain #20", exGap.Finding)
	}
	if !strings.Contains(exGap.Finding, "Issue Alpha") {
		t.Errorf("finding %q should contain 'Issue Alpha'", exGap.Finding)
	}
	if !strings.Contains(exGap.Finding, "Issue Beta") {
		t.Errorf("finding %q should contain 'Issue Beta'", exGap.Finding)
	}
	if exGap.SamplesWith != 2 {
		t.Errorf("SamplesWith = %d, want 2", exGap.SamplesWith)
	}
}

func TestDetectGapsInsufficientSamples(t *testing.T) {
	// 2 issues with quality review (below minimum of 3), 5 without.
	// The quality reviewer finding should be excluded.
	var issues []rundata.IssueDetail

	for i := 0; i < 2; i++ {
		issues = append(issues, rundata.IssueDetail{
			QualityReview: rundata.StepResult{Output: "reviewed"},
			Outcome:       rundata.Outcome{Status: "failed"},
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
		if g.Finding == "quality reviewer" {
			t.Errorf("quality reviewer finding should be excluded with < %d samples on one side, got %+v", minGapSamples, g)
		}
	}
}

func TestDetectGapsNoGaps(t *testing.T) {
	// All issues have quality review — no "without" group for comparison.
	// No issues have spec gen — no "with" group for comparison.
	// No exhausted retries.
	// Result should be nil.
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

	runs := []rundata.RunDetail{{Issues: issues}}
	got := DetectGaps(runs)

	if got != nil {
		t.Errorf("DetectGaps with identical conditions = %v, want nil", got)
	}
}

func TestDetectGapsSortedByMagnitude(t *testing.T) {
	// Two pairwise findings with different gap sizes — larger gap sorts first.
	var issues []rundata.IssueDetail

	// 5 issues: have quality reviewer AND spec gen, none failed.
	for i := 0; i < 5; i++ {
		issues = append(issues, rundata.IssueDetail{
			QualityReview: rundata.StepResult{Output: "reviewed"},
			SpecGenerator: rundata.StepResult{Output: "spec"},
			Outcome:       rundata.Outcome{Status: "implemented"},
		})
	}

	// 5 issues: no quality reviewer, have spec gen — 1 failed (20%).
	for i := 0; i < 5; i++ {
		status := "implemented"
		if i == 0 {
			status = "failed"
		}
		issues = append(issues, rundata.IssueDetail{
			SpecGenerator: rundata.StepResult{Output: "spec"},
			Outcome:       rundata.Outcome{Status: status},
		})
	}

	// 5 issues: no quality reviewer, no spec gen — 3 failed (60%).
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

	// Should have both findings. Quality reviewer gap:
	//   with = 5 (0 failed), without = 10 (1+3=4 failed → 40%)
	//   gap = |0 - 0.4| = 0.4
	// Scenario specs gap:
	//   with = 10 (1 failed → 10%), without = 5 (3 failed → 60%)
	//   gap = |0.1 - 0.6| = 0.5
	// Scenario specs gap is larger, should sort first.

	if len(got) < 2 {
		t.Fatalf("expected at least 2 findings, got %d: %v", len(got), got)
	}

	di0 := math.Abs(got[0].FailRateWith - got[0].FailRateWithout)
	di1 := math.Abs(got[1].FailRateWith - got[1].FailRateWithout)

	if di0 < di1 {
		t.Errorf("findings not sorted by gap magnitude: gap[0]=%f < gap[1]=%f", di0, di1)
	}
}
