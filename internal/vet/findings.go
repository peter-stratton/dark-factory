package vet

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// Severity indicates how serious a finding is.
type Severity int

const (
	Info    Severity = iota
	Warning Severity = iota
	Error   Severity = iota
)

func (s Severity) String() string {
	switch s {
	case Info:
		return "info"
	case Warning:
		return "warning"
	case Error:
		return "error"
	default:
		return fmt.Sprintf("severity(%d)", int(s))
	}
}

// MarshalJSON encodes a Severity as its string representation.
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// Finding represents a single validation issue.
type Finding struct {
	Severity Severity
	Message  string
	Location string
}

// Report collects findings from a validation run.
type Report struct {
	findings []Finding
}

// Add appends a finding to the report.
func (r *Report) Add(f Finding) {
	r.findings = append(r.findings, f)
}

// Findings returns a copy of all findings.
func (r *Report) Findings() []Finding {
	out := make([]Finding, len(r.findings))
	copy(out, r.findings)
	return out
}

// ExitCode returns 1 if any finding has Error severity, 0 otherwise.
func (r *Report) ExitCode() int {
	for _, f := range r.findings {
		if f.Severity == Error {
			return 1
		}
	}
	return 0
}

// Print writes a human-readable table of findings to w.
// If there are no findings, it prints "No issues found."
func (r *Report) Print(w io.Writer) {
	if len(r.findings) == 0 {
		fmt.Fprintln(w, "No issues found.")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, f := range r.findings {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", f.Severity, f.Location, f.Message)
	}
	tw.Flush()

	errors, warnings, infos := r.countBySeverity()
	fmt.Fprintf(w, "\n%d error(s), %d warning(s), %d info(s)\n", errors, warnings, infos)
}

// PrintJSON writes the report as JSON to w.
func (r *Report) PrintJSON(w io.Writer) error {
	findings := r.findings
	if findings == nil {
		findings = []Finding{}
	}

	errors, warnings, infos := r.countBySeverity()

	out := struct {
		Findings []Finding `json:"findings"`
		Summary  struct {
			Errors   int `json:"errors"`
			Warnings int `json:"warnings"`
			Infos    int `json:"infos"`
		} `json:"summary"`
	}{
		Findings: findings,
	}
	out.Summary.Errors = errors
	out.Summary.Warnings = warnings
	out.Summary.Infos = infos

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func (r *Report) countBySeverity() (errors, warnings, infos int) {
	for _, f := range r.findings {
		switch f.Severity {
		case Error:
			errors++
		case Warning:
			warnings++
		case Info:
			infos++
		}
	}
	return
}
