package progress

import (
	"bytes"
	"strings"
	"testing"
)

func TestIssueCompleted_Implemented(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.IssueCompleted(42, "add cost tracking", "implemented", 87, 0, "", 0.0, "")
	want := "  #42 add cost tracking \u2014 implemented (PR #87, 0 retries)\n"
	if got := buf.String(); got != want {
		t.Errorf("IssueCompleted implemented\ngot:  %q\nwant: %q", got, want)
	}
}

func TestIssueCompleted_ReadyToMerge(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.IssueCompleted(10, "fix login", "ready-to-merge", 55, 2, "", 0.0, "")
	want := "  #10 fix login \u2014 ready-to-merge (PR #55, 2 retries)\n"
	if got := buf.String(); got != want {
		t.Errorf("IssueCompleted ready-to-merge\ngot:  %q\nwant: %q", got, want)
	}
}

func TestIssueCompleted_NeedsHumanReview(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.IssueCompleted(7, "refactor auth", "needs-human-review", 33, 1, "", 0.0, "")
	want := "  #7 refactor auth \u2014 needs human review (PR #33)\n"
	if got := buf.String(); got != want {
		t.Errorf("IssueCompleted needs-human-review\ngot:  %q\nwant: %q", got, want)
	}
}

func TestIssueCompleted_Failed(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.IssueCompleted(42, "add cost tracking", "failed", 0, 0, "timeout", 0.0, "")
	want := "  #42 add cost tracking \u2014 failed: timeout\n"
	if got := buf.String(); got != want {
		t.Errorf("IssueCompleted failed\ngot:  %q\nwant: %q", got, want)
	}
}

func TestIssueCompleted_ImplementedWithTraceID(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.IssueCompleted(42, "add cost tracking", "implemented", 87, 0, "", 0.0, "abcd1234-5678-90ef")
	got := buf.String()
	if !strings.Contains(got, "[trace:abcd1234]") {
		t.Errorf("IssueCompleted with trace\ngot:  %q\nwant to contain: %q", got, "[trace:abcd1234]")
	}
}

func TestIssueCompleted_FailedWithEmptyTraceID(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.IssueCompleted(42, "add cost tracking", "failed", 0, 0, "timeout", 0.0, "")
	got := buf.String()
	if strings.Contains(got, "[trace:") {
		t.Errorf("IssueCompleted empty trace should not contain trace suffix\ngot: %q", got)
	}
}

func TestRunFinished(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.RunFinished(3, 1, 1, 1, 2)
	want := "\nResults: 3 implemented, 1 ready-to-merge, 1 needs-human-review, 1 failed, 2 skipped (blocked)\n"
	if got := buf.String(); got != want {
		t.Errorf("RunFinished\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWaveStarted(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.WaveStarted(2, 3)
	want := "\n--- Wave 2: 3 newly unblocked issues ---\n"
	if got := buf.String(); got != want {
		t.Errorf("WaveStarted\ngot:  %q\nwant: %q", got, want)
	}
}

func TestAllBlocked(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.AllBlocked(5, 5)
	got := buf.String()
	wantFirst := "All issues are blocked \u2014 nothing to process.\n"
	wantSecond := "Summary: 5 total, 5 blocked, 0 processable\n"
	if !strings.HasPrefix(got, wantFirst) {
		t.Errorf("AllBlocked first line\ngot:  %q\nwant prefix: %q", got, wantFirst)
	}
	if !strings.HasSuffix(got, wantSecond) {
		t.Errorf("AllBlocked second line\ngot:  %q\nwant suffix: %q", got, wantSecond)
	}
}

func TestRollupCreated_Merged(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.RollupCreated(12, "https://github.com/org/repo/pull/12", true)
	got := buf.String()
	wantCreated := "Rollup PR #12 created: https://github.com/org/repo/pull/12\n"
	wantMerged := "Rollup PR #12 merged.\n"
	if !strings.Contains(got, wantCreated) {
		t.Errorf("RollupCreated merged: missing created line\ngot: %q", got)
	}
	if !strings.Contains(got, wantMerged) {
		t.Errorf("RollupCreated merged: missing merged line\ngot: %q", got)
	}
}

func TestRollupCreated_Open(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.RollupCreated(12, "https://github.com/org/repo/pull/12", false)
	got := buf.String()
	wantCreated := "Rollup PR #12 created: https://github.com/org/repo/pull/12\n"
	wantOpen := "Rollup PR #12 is open for review: https://github.com/org/repo/pull/12\n"
	if !strings.Contains(got, wantCreated) {
		t.Errorf("RollupCreated open: missing created line\ngot: %q", got)
	}
	if !strings.Contains(got, wantOpen) {
		t.Errorf("RollupCreated open: missing open line\ngot: %q", got)
	}
}

func TestPunchlistText(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.PunchlistText("- fix the thing\n")
	want := "- fix the thing\n"
	if got := buf.String(); got != want {
		t.Errorf("PunchlistText\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriterInjection(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.WaveStarted(1, 5)
	if buf.Len() == 0 {
		t.Error("expected output in buffer, got none")
	}
}

func TestNoOpMethods(t *testing.T) {
	var buf bytes.Buffer
	r := NewTextReporter(&buf)
	r.RunStarted("org/repo", "v1.0", "2026-03-14", "main", "true", "true", nil)
	r.IssueStarted(1, "some issue")
	r.IssueStageChanged(1, "implementing")
	if buf.Len() != 0 {
		t.Errorf("expected no output from no-op methods, got %q", buf.String())
	}
}

// Verify TextReporter satisfies the ProgressReporter interface at compile time.
var _ ProgressReporter = (*TextReporter)(nil)
