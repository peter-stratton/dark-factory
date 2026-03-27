package vet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/github"
)

const validSpec = `# Scenario: Feature X

Relates to: Issue #10

## Setup
- Repository with issues

## Cases

### Happy path
Description of the happy path.
- GIVEN a valid repository
- WHEN the feature runs
- THEN the expected outcome is observed
`

func writeSpec(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateScenarios_ValidSpec(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "feature-x.md", validSpec)

	issues := []github.Issue{{Number: 10}}
	allNums := map[int]bool{10: true}

	r := ValidateScenarios(dir, issues, allNums)
	if len(r.Findings()) != 0 {
		t.Errorf("expected 0 findings, got %d: %+v", len(r.Findings()), r.Findings())
	}
}

func TestValidateScenarios_MissingSetup(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "no-setup.md", `# Scenario: No Setup

Relates to: Issue #10

## Cases

### Case 1
- GIVEN a state
- WHEN an action
- THEN a result
`)

	r := ValidateScenarios(dir, nil, map[int]bool{10: true})
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && f.Message == "missing ## Setup section" {
			found = true
		}
	}
	if !found {
		t.Error("expected error about missing Setup section")
	}
}

func TestValidateScenarios_MissingCases(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "no-cases.md", `# Scenario: No Cases

Relates to: Issue #10

## Setup
- Stuff
`)

	r := ValidateScenarios(dir, nil, map[int]bool{10: true})
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && f.Message == "missing ## Cases section" {
			found = true
		}
	}
	if !found {
		t.Error("expected error about missing Cases section")
	}
}

func TestValidateScenarios_CaseWithoutOutcomes(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "no-outcomes.md", `# Scenario: No Outcomes

Relates to: Issue #10

## Setup
- Stuff

## Cases

### Case with no bullets
Just some text, no bullets.
`)

	r := ValidateScenarios(dir, nil, map[int]bool{10: true})
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && contains(f.Message, "missing GIVEN clause") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about case missing GIVEN clause")
	}
}

func TestValidateScenarios_MissingRelatesTo(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "no-relates.md", `# Scenario: No Relates

## Setup
- Stuff

## Cases

### Case 1
- GIVEN a state
- WHEN an action
- THEN a result
`)

	r := ValidateScenarios(dir, nil, nil)
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && f.Message == "missing Relates to: Issue #N line" {
			found = true
		}
	}
	if !found {
		t.Error("expected error about missing Relates to line")
	}
}

func TestValidateScenarios_MalformedRelatesTo(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "malformed.md", `# Scenario: Malformed

Relates to: 5

## Setup
- Stuff

## Cases

### Case 1
- GIVEN a state
- WHEN an action
- THEN a result
`)

	r := ValidateScenarios(dir, nil, nil)
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && contains(f.Message, "malformed Relates to") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about malformed Relates to line")
	}
}

func TestValidateScenarios_NonExistentIssueRef(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "bad-ref.md", `# Scenario: Bad Ref

Relates to: Issue #999

## Setup
- Stuff

## Cases

### Case 1
- GIVEN a state
- WHEN an action
- THEN a result
`)

	r := ValidateScenarios(dir, nil, map[int]bool{1: true})
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Warning && contains(f.Message, "non-existent issue #999") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about non-existent issue ref")
	}
}

func TestValidateScenarios_IssueMissingCoverage(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "covers-10.md", validSpec)

	issues := []github.Issue{
		{Number: 10},
		{Number: 11}, // not covered by any spec
	}

	r := ValidateScenarios(dir, issues, map[int]bool{10: true, 11: true})
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Warning && f.Location == "#11" && contains(f.Message, "no matching scenario spec") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about uncovered issue #11")
	}
}

func TestValidateScenarios_MultipleRelatesTo(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "multi.md", `# Scenario: Multi

Relates to: Issue #10, Issue #11

## Setup
- Stuff

## Cases

### Case 1
- GIVEN a state
- WHEN an action
- THEN a result
`)

	issues := []github.Issue{
		{Number: 10},
		{Number: 11},
	}

	r := ValidateScenarios(dir, issues, map[int]bool{10: true, 11: true})
	// Both issues should be covered — no coverage warnings
	for _, f := range r.Findings() {
		if contains(f.Message, "no matching scenario spec") {
			t.Errorf("unexpected coverage warning: %+v", f)
		}
	}
}

func TestValidateScenarios_RecursesIntoSubdirs(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "phase-1")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, sub, "feature-x.md", validSpec)

	issues := []github.Issue{{Number: 10}}
	allNums := map[int]bool{10: true}

	r := ValidateScenarios(dir, issues, allNums)
	if len(r.Findings()) != 0 {
		t.Errorf("expected 0 findings, got %d: %+v", len(r.Findings()), r.Findings())
	}
}

func TestValidateScenarios_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	issues := []github.Issue{{Number: 10}}

	r := ValidateScenarios(dir, issues, map[int]bool{10: true})
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Warning && f.Location == "#10" {
			found = true
		}
	}
	if !found {
		t.Error("expected coverage warning for empty dir")
	}
}

func TestValidateCaseOutcomes_ValidGWT(t *testing.T) {
	r := &Report{}
	casesBody := `### Happy path
- GIVEN a valid repository
- WHEN the feature runs
- THEN the expected outcome is observed
`
	validateCaseOutcomes(r, "test.md", casesBody)
	if len(r.Findings()) != 0 {
		t.Errorf("expected 0 findings, got %d: %+v", len(r.Findings()), r.Findings())
	}
}

func TestValidateCaseOutcomes_MissingGiven(t *testing.T) {
	r := &Report{}
	casesBody := `### Case A
- WHEN an action occurs
- THEN a result is seen
`
	validateCaseOutcomes(r, "test.md", casesBody)
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && contains(f.Message, "missing GIVEN clause") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about missing GIVEN clause")
	}
	for _, f := range r.Findings() {
		if contains(f.Message, "missing WHEN clause") || contains(f.Message, "missing THEN clause") {
			t.Errorf("unexpected finding: %+v", f)
		}
	}
}

func TestValidateCaseOutcomes_MissingWhen(t *testing.T) {
	r := &Report{}
	casesBody := `### Case B
- GIVEN a precondition
- THEN a result is seen
`
	validateCaseOutcomes(r, "test.md", casesBody)
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && contains(f.Message, "missing WHEN clause") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about missing WHEN clause")
	}
	for _, f := range r.Findings() {
		if contains(f.Message, "missing GIVEN clause") || contains(f.Message, "missing THEN clause") {
			t.Errorf("unexpected finding: %+v", f)
		}
	}
}

func TestValidateCaseOutcomes_MissingThen(t *testing.T) {
	r := &Report{}
	casesBody := `### Case C
- GIVEN a precondition
- WHEN an action occurs
`
	validateCaseOutcomes(r, "test.md", casesBody)
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && contains(f.Message, "missing THEN clause") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about missing THEN clause")
	}
	for _, f := range r.Findings() {
		if contains(f.Message, "missing GIVEN clause") || contains(f.Message, "missing WHEN clause") {
			t.Errorf("unexpected finding: %+v", f)
		}
	}
}

func TestValidateCaseOutcomes_CaseInsensitive(t *testing.T) {
	r := &Report{}
	casesBody := `### Case D
- given a precondition
- When an action occurs
- THEN a result is seen
`
	validateCaseOutcomes(r, "test.md", casesBody)
	if len(r.Findings()) != 0 {
		t.Errorf("expected 0 findings with mixed-case GWT, got %d: %+v", len(r.Findings()), r.Findings())
	}
}

func TestValidateCaseOutcomes_MultipleClauses(t *testing.T) {
	r := &Report{}
	casesBody := `### Case E
- GIVEN first precondition
- GIVEN second precondition
- WHEN an action occurs
- THEN first result
- THEN second result
- THEN third result
`
	validateCaseOutcomes(r, "test.md", casesBody)
	if len(r.Findings()) != 0 {
		t.Errorf("expected 0 findings with multiple GWT clauses, got %d: %+v", len(r.Findings()), r.Findings())
	}
}
