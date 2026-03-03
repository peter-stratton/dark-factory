package vet

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFindingFields(t *testing.T) {
	f := Finding{
		Severity: Error,
		Message:  "missing section",
		Location: "#42",
	}
	if f.Severity != Error {
		t.Errorf("Severity = %v, want Error", f.Severity)
	}
	if f.Message != "missing section" {
		t.Errorf("Message = %q, want %q", f.Message, "missing section")
	}
	if f.Location != "#42" {
		t.Errorf("Location = %q, want %q", f.Location, "#42")
	}
}

func TestReportExitCode_WithErrors(t *testing.T) {
	r := &Report{}
	r.Add(Finding{Severity: Warning, Message: "warn"})
	r.Add(Finding{Severity: Error, Message: "err"})
	if got := r.ExitCode(); got != 1 {
		t.Errorf("ExitCode() = %d, want 1", got)
	}
}

func TestReportExitCode_OnlyWarnings(t *testing.T) {
	r := &Report{}
	r.Add(Finding{Severity: Warning, Message: "warn"})
	if got := r.ExitCode(); got != 0 {
		t.Errorf("ExitCode() = %d, want 0", got)
	}
}

func TestReportExitCode_Empty(t *testing.T) {
	r := &Report{}
	if got := r.ExitCode(); got != 0 {
		t.Errorf("ExitCode() = %d, want 0", got)
	}
}

func TestReportPrint_Empty(t *testing.T) {
	r := &Report{}
	var buf bytes.Buffer
	r.Print(&buf)
	if got := buf.String(); !strings.Contains(got, "No issues found.") {
		t.Errorf("Print() = %q, want 'No issues found.'", got)
	}
}

func TestReportPrint_WithFindings(t *testing.T) {
	r := &Report{}
	r.Add(Finding{Severity: Error, Message: "bad thing", Location: "#1"})
	r.Add(Finding{Severity: Warning, Message: "meh thing", Location: "#2"})

	var buf bytes.Buffer
	r.Print(&buf)
	out := buf.String()

	if !strings.Contains(out, "error") {
		t.Error("output should contain 'error'")
	}
	if !strings.Contains(out, "warning") {
		t.Error("output should contain 'warning'")
	}
	if !strings.Contains(out, "bad thing") {
		t.Error("output should contain finding message")
	}
	if !strings.Contains(out, "1 error(s), 1 warning(s), 0 info(s)") {
		t.Errorf("output missing summary line, got:\n%s", out)
	}
}

func TestReportPrintJSON_Valid(t *testing.T) {
	r := &Report{}
	r.Add(Finding{Severity: Error, Message: "problem", Location: "#5"})

	var buf bytes.Buffer
	if err := r.PrintJSON(&buf); err != nil {
		t.Fatalf("PrintJSON() error: %v", err)
	}

	var parsed struct {
		Findings []struct {
			Severity string `json:"Severity"`
			Message  string `json:"Message"`
			Location string `json:"Location"`
		} `json:"findings"`
		Summary struct {
			Errors   int `json:"errors"`
			Warnings int `json:"warnings"`
			Infos    int `json:"infos"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v\nraw: %s", err, buf.String())
	}
	if len(parsed.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(parsed.Findings))
	}
	if parsed.Findings[0].Severity != "error" {
		t.Errorf("Severity = %q, want %q", parsed.Findings[0].Severity, "error")
	}
	if parsed.Summary.Errors != 1 {
		t.Errorf("Summary.Errors = %d, want 1", parsed.Summary.Errors)
	}
}

func TestReportPrintJSON_EmptyFindings(t *testing.T) {
	r := &Report{}

	var buf bytes.Buffer
	if err := r.PrintJSON(&buf); err != nil {
		t.Fatalf("PrintJSON() error: %v", err)
	}

	// Must be [] not null
	if !strings.Contains(buf.String(), `"findings": []`) {
		t.Errorf("empty findings should be [] not null, got:\n%s", buf.String())
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{Error, "error"},
		{Warning, "warning"},
		{Info, "info"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", int(tt.s), got, tt.want)
		}
	}
}
