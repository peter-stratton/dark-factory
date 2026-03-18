package vet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/github"
)

func writePlanDoc(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRoadmap_ValidDoc(t *testing.T) {
	dir := t.TempDir()
	writePlanDoc(t, dir, "phase-2.md", `# Phase 2 Planning

## Issue #14: Vet scaffold

Details here.

## Issue #15: Vet issues

Details here.
`)

	issues := []github.Issue{
		{Number: 14, Labels: []string{"phase-2"}},
		{Number: 15, Labels: []string{"phase-2"}},
	}
	allNums := map[int]bool{14: true, 15: true}

	r := ValidateRoadmap(dir, issues, "Phase 2", allNums)
	if len(r.Findings()) != 0 {
		t.Errorf("expected 0 findings, got %d: %+v", len(r.Findings()), r.Findings())
	}
}

func TestValidateRoadmap_PhantomRef(t *testing.T) {
	dir := t.TempDir()
	writePlanDoc(t, dir, "plan.md", `## Issue #999: Does not exist
`)

	r := ValidateRoadmap(dir, nil, "Phase 2", map[int]bool{1: true})
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Warning && contains(f.Message, "phantom") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about phantom issue ref")
	}
}

func TestValidateRoadmap_OrphanedIssue(t *testing.T) {
	dir := t.TempDir()
	writePlanDoc(t, dir, "plan.md", `## Issue #14: Covered
`)

	issues := []github.Issue{
		{Number: 14, Labels: []string{"phase-2"}},
		{Number: 15, Labels: []string{"phase-2"}}, // not in planning doc
	}
	allNums := map[int]bool{14: true, 15: true}

	r := ValidateRoadmap(dir, issues, "Phase 2", allNums)
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Warning && f.Location == "#15" && contains(f.Message, "orphaned") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about orphaned issue #15")
	}
}

func TestValidateRoadmap_LabelMismatch(t *testing.T) {
	dir := t.TempDir()
	writePlanDoc(t, dir, "plan.md", `## Issue #14: Something
`)

	issues := []github.Issue{
		{Number: 14, Labels: []string{"phase-1"}}, // wrong phase label
	}
	allNums := map[int]bool{14: true}

	r := ValidateRoadmap(dir, issues, "Phase 2", allNums)
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Error && contains(f.Message, "phase-1") && contains(f.Message, "Phase 2") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about label mismatch")
	}
}

func TestValidateRoadmap_LabelMatchesCompoundMilestone(t *testing.T) {
	dir := t.TempDir()
	writePlanDoc(t, dir, "plan.md", `## Issue #22: Something
`)

	issues := []github.Issue{
		{Number: 22, Labels: []string{"phase-2"}}, // should match "Phase 2: Vault Reader"
	}
	allNums := map[int]bool{22: true}

	r := ValidateRoadmap(dir, issues, "Phase 2: Vault Reader + Foundation", allNums)
	for _, f := range r.Findings() {
		if f.Severity == Error && contains(f.Message, "label") {
			t.Errorf("unexpected label mismatch error: %+v", f)
		}
	}
}

func TestValidateRoadmap_MultipleDocsCoverIssue(t *testing.T) {
	dir := t.TempDir()
	writePlanDoc(t, dir, "doc1.md", `## Issue #14: Part 1
`)
	writePlanDoc(t, dir, "doc2.md", `## Issue #15: Part 2
`)

	issues := []github.Issue{
		{Number: 14, Labels: []string{"phase-2"}},
		{Number: 15, Labels: []string{"phase-2"}},
	}
	allNums := map[int]bool{14: true, 15: true}

	r := ValidateRoadmap(dir, issues, "Phase 2", allNums)
	for _, f := range r.Findings() {
		if contains(f.Message, "orphaned") {
			t.Errorf("unexpected orphan warning: %+v", f)
		}
	}
}

func TestValidateRoadmap_NoPlanningDocs(t *testing.T) {
	dir := t.TempDir()

	r := ValidateRoadmap(dir, nil, "Phase 2", nil)
	found := false
	for _, f := range r.Findings() {
		if f.Severity == Warning && contains(f.Message, "no planning docs found") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about no planning docs")
	}
}
