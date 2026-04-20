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

	if step.StartedAt == nil || *step.StartedAt != started {
		t.Errorf("StartedAt = %v, want %v", step.StartedAt, started)
	}
	if step.FinishedAt == nil || *step.FinishedAt != finished {
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

func TestResultToStep_ToolTracePropagated(t *testing.T) {
	trace := []string{"Read /some/file", "Write /some/output", "Bash go test ./..."}
	r := &Result{
		ResultText: "done",
		ToolTrace:  trace,
	}

	step := ResultToStep(r)

	if len(step.ToolTrace) != len(trace) {
		t.Fatalf("ToolTrace length = %d, want %d", len(step.ToolTrace), len(trace))
	}
	for i, want := range trace {
		if step.ToolTrace[i] != want {
			t.Errorf("ToolTrace[%d] = %q, want %q", i, step.ToolTrace[i], want)
		}
	}
}

func TestResultToStep_NilToolTraceOmitted(t *testing.T) {
	r := &Result{ResultText: "done"}

	step := ResultToStep(r)

	if step.ToolTrace != nil {
		t.Errorf("ToolTrace = %v, want nil when Result.ToolTrace is nil", step.ToolTrace)
	}
}

func TestResultToStep_CopiesPrompt(t *testing.T) {
	r := &Result{
		ResultText: "done",
		Prompt:     "You are an implementer agent.",
	}

	step := ResultToStep(r)

	if step.Prompt != "You are an implementer agent." {
		t.Errorf("Prompt = %q, want %q", step.Prompt, "You are an implementer agent.")
	}
}

func TestResultToStep_EmptyPrompt(t *testing.T) {
	r := &Result{ResultText: "done"}

	step := ResultToStep(r)

	if step.Prompt != "" {
		t.Errorf("Prompt = %q, want empty string", step.Prompt)
	}
}

func TestResultToStep_PromptHashPopulatedFromPrompt(t *testing.T) {
	r := &Result{
		ResultText: "done",
		Prompt:     "You are an implementer agent.",
	}

	step := ResultToStep(r)

	if len(step.PromptHash) != 64 {
		t.Fatalf("PromptHash length = %d, want 64 hex chars", len(step.PromptHash))
	}
	step2 := ResultToStep(r)
	if step2.PromptHash != step.PromptHash {
		t.Errorf("PromptHash not deterministic: %q vs %q", step.PromptHash, step2.PromptHash)
	}
	r.Prompt = "You are a reviewer agent."
	step3 := ResultToStep(r)
	if step3.PromptHash == step.PromptHash {
		t.Errorf("PromptHash did not change when prompt changed")
	}
}

func TestResultToStep_EmptyPromptProducesEmptyHash(t *testing.T) {
	r := &Result{ResultText: "done"}

	step := ResultToStep(r)

	if step.PromptHash != "" {
		t.Errorf("PromptHash = %q, want empty when Prompt is empty", step.PromptHash)
	}
}

func TestResultToStep_PopulatesTranscriptFromStdout(t *testing.T) {
	r := &Result{
		ResultText: "done",
		Stdout: `{"type":"system","subtype":"init"}` + "\n" +
			`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}` + "\n" +
			`{"type":"result","session_id":"s","result":"done"}`,
	}

	step := ResultToStep(r)

	if len(step.Transcript) == 0 {
		t.Fatal("expected Transcript to be populated from Stdout")
	}
}

func TestResultToStep_NilTranscriptWhenStdoutHasNoRelevantEvents(t *testing.T) {
	r := &Result{
		ResultText: "done",
		Stdout:     `{"type":"result","session_id":"s","result":"done"}`,
	}

	step := ResultToStep(r)

	if step.Transcript != nil {
		t.Errorf("Transcript = %v, want nil when Stdout has only result/system events", step.Transcript)
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
