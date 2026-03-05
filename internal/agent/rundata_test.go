package agent

import (
	"testing"
	"time"
)

func TestResultToStep_TimingFields(t *testing.T) {
	started := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	finished := time.Date(2026, 3, 5, 10, 0, 30, 0, time.UTC) // 30 seconds later

	r := &Result{
		ResultText: "Implementation complete",
		StartedAt:  started,
		FinishedAt: finished,
	}

	step := ResultToStep(r)

	if step.StartedAt != started {
		t.Errorf("StartedAt = %v, want %v", step.StartedAt, started)
	}
	if step.FinishedAt != finished {
		t.Errorf("FinishedAt = %v, want %v", step.FinishedAt, finished)
	}
	if step.DurationSeconds != 30.0 {
		t.Errorf("DurationSeconds = %v, want 30.0", step.DurationSeconds)
	}
}

func TestResultToStep_ZeroTimingProducesZeroDuration(t *testing.T) {
	r := &Result{ResultText: "done"}

	step := ResultToStep(r)

	if step.DurationSeconds != 0 {
		t.Errorf("DurationSeconds = %v, want 0 when timing is zero", step.DurationSeconds)
	}
}

func TestResultToStep_OutputFromResultText(t *testing.T) {
	r := &Result{
		ResultText: "agent output here",
		StartedAt:  time.Now(),
		FinishedAt: time.Now(),
	}

	step := ResultToStep(r)

	if step.Output != "agent output here" {
		t.Errorf("Output = %q, want %q", step.Output, "agent output here")
	}
}

func TestResultToStep_DurationSeconds_Positive(t *testing.T) {
	started := time.Now()
	finished := started.Add(5 * time.Second)

	r := &Result{
		StartedAt:  started,
		FinishedAt: finished,
	}

	step := ResultToStep(r)

	if step.DurationSeconds <= 0 {
		t.Errorf("DurationSeconds = %v, want positive", step.DurationSeconds)
	}
}
