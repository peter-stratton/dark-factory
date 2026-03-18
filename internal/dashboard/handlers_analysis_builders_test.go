package dashboard

import (
	"testing"

	"github.com/peter-stratton/dark-factory/internal/analysis"
)

func TestBuildOverviewMetrics(t *testing.T) {
	report := analysis.Report{
		RunCount:   10,
		IssueCount: 50,
		Outcomes: map[string]int{
			"implemented": 30,
			"failed":      20,
		},
		FirstPassSuccessRate: 0.6,
		CostStats: analysis.CostStats{
			TotalUSD: 12.50,
		},
		AvgCostPerSuccessUSD: 0.40,
		WastedCostUSD:        2.00,
		TimeoutRate:          0.10,
	}

	got := buildOverviewMetrics(report)

	if got.TotalRuns != 10 {
		t.Errorf("TotalRuns = %d, want 10", got.TotalRuns)
	}
	if got.TotalIssues != 50 {
		t.Errorf("TotalIssues = %d, want 50", got.TotalIssues)
	}
	wantSuccessRate := 30.0 / 50.0
	if got.SuccessRate != wantSuccessRate {
		t.Errorf("SuccessRate = %f, want %f", got.SuccessRate, wantSuccessRate)
	}
	if got.FirstPassRate != 0.6 {
		t.Errorf("FirstPassRate = %f, want 0.6", got.FirstPassRate)
	}
	if got.TotalCostUSD != 12.50 {
		t.Errorf("TotalCostUSD = %f, want 12.50", got.TotalCostUSD)
	}
	if got.AvgCostPerSuccessUSD != 0.40 {
		t.Errorf("AvgCostPerSuccessUSD = %f, want 0.40", got.AvgCostPerSuccessUSD)
	}
	if got.WastedCostUSD != 2.00 {
		t.Errorf("WastedCostUSD = %f, want 2.00", got.WastedCostUSD)
	}
	if got.TimeoutRate != 0.10 {
		t.Errorf("TimeoutRate = %f, want 0.10", got.TimeoutRate)
	}
}

func TestBuildOverviewMetrics_ZeroIssues(t *testing.T) {
	report := analysis.Report{
		RunCount:   0,
		IssueCount: 0,
	}

	got := buildOverviewMetrics(report)

	if got.SuccessRate != 0 {
		t.Errorf("SuccessRate = %f, want 0 when IssueCount is 0", got.SuccessRate)
	}
}

func TestBuildFailureReasonRows_Sorted(t *testing.T) {
	report := analysis.Report{
		IssueCount: 100,
		FailureReasons: map[string]int{
			"verify":      5,
			"exhaustion":  20,
			"timeout":     10,
			"error":       15,
		},
	}

	rows := buildFailureReasonRows(report)

	if len(rows) != 4 {
		t.Fatalf("len(rows) = %d, want 4", len(rows))
	}

	// Sorted by count descending.
	if rows[0].Reason != "exhaustion" || rows[0].Count != 20 {
		t.Errorf("rows[0] = {%s, %d}, want {exhaustion, 20}", rows[0].Reason, rows[0].Count)
	}
	if rows[1].Reason != "error" || rows[1].Count != 15 {
		t.Errorf("rows[1] = {%s, %d}, want {error, 15}", rows[1].Reason, rows[1].Count)
	}
	if rows[2].Reason != "timeout" || rows[2].Count != 10 {
		t.Errorf("rows[2] = {%s, %d}, want {timeout, 10}", rows[2].Reason, rows[2].Count)
	}
	if rows[3].Reason != "verify" || rows[3].Count != 5 {
		t.Errorf("rows[3] = {%s, %d}, want {verify, 5}", rows[3].Reason, rows[3].Count)
	}

	// Check percentage computation.
	wantPct := 20.0 / 100.0 * 100
	if rows[0].Percent != wantPct {
		t.Errorf("rows[0].Percent = %f, want %f", rows[0].Percent, wantPct)
	}
}

func TestBuildFailureReasonRows_EmptyIsSlice(t *testing.T) {
	// When FailureReasons is nil (zero Report), result must be an empty slice, not nil.
	report := analysis.Report{}

	rows := buildFailureReasonRows(report)

	if rows == nil {
		t.Error("buildFailureReasonRows returned nil, want empty slice")
	}
	if len(rows) != 0 {
		t.Errorf("len(rows) = %d, want 0", len(rows))
	}
}

func TestBuildFailureReasonRows_EmptyMap(t *testing.T) {
	// When FailureReasons is an initialized but empty map, result must be an empty slice.
	report := analysis.Report{
		FailureReasons: make(map[string]int),
	}

	rows := buildFailureReasonRows(report)

	if rows == nil {
		t.Error("buildFailureReasonRows returned nil, want empty slice")
	}
	if len(rows) != 0 {
		t.Errorf("len(rows) = %d, want 0", len(rows))
	}
}

func TestBuildRepoRows_Enriched(t *testing.T) {
	report := analysis.Report{
		RepoStats: map[string]analysis.RepoSummary{
			"org/repo": {
				Total:              10,
				Implemented:        8,
				Failed:             2,
				SuccessRate:        0.8,
				FirstPassRate:      0.5,
				AvgCostPerIssueUSD: 0.25,
			},
		},
	}

	rows := buildRepoRows(report)

	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Repo != "org/repo" {
		t.Errorf("Repo = %q, want %q", row.Repo, "org/repo")
	}
	if row.FirstPassRate != 0.5 {
		t.Errorf("FirstPassRate = %f, want 0.5", row.FirstPassRate)
	}
	if row.AvgCostPerIssueUSD != 0.25 {
		t.Errorf("AvgCostPerIssueUSD = %f, want 0.25", row.AvgCostPerIssueUSD)
	}
	if row.SuccessPct != 80.0 {
		t.Errorf("SuccessPct = %f, want 80.0", row.SuccessPct)
	}
}
