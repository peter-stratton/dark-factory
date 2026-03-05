package vet

import (
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/harness/layers"
)

// parseDef is a test helper that parses a JSON layer definition, failing on error.
func parseDef(t *testing.T, json string) *layers.Definition {
	t.Helper()
	def, err := layers.Parse(strings.NewReader(json))
	if err != nil {
		t.Fatalf("parse layers: %v", err)
	}
	return def
}

func TestValidateArchitecture_NoCycles(t *testing.T) {
	def := parseDef(t, `{
		"layers": [
			{"name": "A", "dir": "a/", "imports": []},
			{"name": "B", "dir": "b/", "imports": ["A"]}
		]
	}`)
	report := ValidateArchitecture(def)
	if got := report.ExitCode(); got != 0 {
		t.Errorf("expected exit code 0 for valid DAG, got %d; findings: %v", got, report.Findings())
	}
}

func TestValidateArchitecture_SimpleCycle(t *testing.T) {
	// A→B→A
	def := parseDef(t, `{
		"layers": [
			{"name": "A", "dir": "a/", "imports": ["B"]},
			{"name": "B", "dir": "b/", "imports": ["A"]}
		]
	}`)
	report := ValidateArchitecture(def)
	if report.ExitCode() == 0 {
		t.Fatal("expected non-zero exit code for cycle A→B→A")
	}
	findings := report.Findings()
	if len(findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	combined := findings[0].Message + " " + findings[0].Location
	if !strings.Contains(combined, "A") || !strings.Contains(combined, "B") {
		t.Errorf("expected finding to name both A and B, got: %q", combined)
	}
}

func TestValidateArchitecture_TransitiveCycle(t *testing.T) {
	// A→B→C→A
	def := parseDef(t, `{
		"layers": [
			{"name": "A", "dir": "a/", "imports": ["B"]},
			{"name": "B", "dir": "b/", "imports": ["C"]},
			{"name": "C", "dir": "c/", "imports": ["A"]}
		]
	}`)
	report := ValidateArchitecture(def)
	if report.ExitCode() == 0 {
		t.Fatal("expected non-zero exit code for cycle A→B→C→A")
	}
	findings := report.Findings()
	if len(findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	combined := findings[0].Message + " " + findings[0].Location
	for _, layer := range []string{"A", "B", "C"} {
		if !strings.Contains(combined, layer) {
			t.Errorf("expected finding to name layer %q, got: %q", layer, combined)
		}
	}
}

func TestValidateArchitecture_SelfImport(t *testing.T) {
	// A imports A
	def := parseDef(t, `{
		"layers": [
			{"name": "A", "dir": "a/", "imports": ["A"]}
		]
	}`)
	report := ValidateArchitecture(def)
	if report.ExitCode() == 0 {
		t.Fatal("expected non-zero exit code for self-import A→A")
	}
	findings := report.Findings()
	if len(findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	if !strings.Contains(findings[0].Message, "A") {
		t.Errorf("expected finding to name layer A, got: %q", findings[0].Message)
	}
}

func TestValidateArchitecture_CleanArchitecture(t *testing.T) {
	// Well-structured 5-layer DAG — no cycles.
	def := parseDef(t, `{
		"layers": [
			{"name": "types",        "dir": "internal/types/",        "imports": []},
			{"name": "config",       "dir": "internal/config/",       "imports": ["types"]},
			{"name": "deps",         "dir": "internal/deps/",         "imports": ["types", "config"]},
			{"name": "github",       "dir": "internal/github/",       "imports": ["types"]},
			{"name": "orchestrator", "dir": "internal/orchestrator/", "imports": ["types", "config", "deps", "github"]}
		]
	}`)
	report := ValidateArchitecture(def)
	if got := report.ExitCode(); got != 0 {
		t.Errorf("expected exit code 0 for clean architecture, got %d; findings: %v", got, report.Findings())
	}
}

func TestValidateArchitecture_NoDuplicateCycleReports(t *testing.T) {
	// A→B→A: should produce exactly one finding, not two.
	def := parseDef(t, `{
		"layers": [
			{"name": "A", "dir": "a/", "imports": ["B"]},
			{"name": "B", "dir": "b/", "imports": ["A"]}
		]
	}`)
	report := ValidateArchitecture(def)
	findings := report.Findings()
	if len(findings) != 1 {
		t.Errorf("expected exactly 1 finding for A→B→A, got %d: %v", len(findings), findings)
	}
}

func TestValidateArchitecture_DisconnectedCycles(t *testing.T) {
	// Two independent cycles: A→B→A and C→D→C.
	def := parseDef(t, `{
		"layers": [
			{"name": "A", "dir": "a/", "imports": ["B"]},
			{"name": "B", "dir": "b/", "imports": ["A"]},
			{"name": "C", "dir": "c/", "imports": ["D"]},
			{"name": "D", "dir": "d/", "imports": ["C"]}
		]
	}`)
	report := ValidateArchitecture(def)
	if report.ExitCode() == 0 {
		t.Fatal("expected non-zero exit code for two independent cycles")
	}
	findings := report.Findings()
	if len(findings) != 2 {
		t.Errorf("expected 2 findings for two independent cycles, got %d: %v", len(findings), findings)
	}
}
