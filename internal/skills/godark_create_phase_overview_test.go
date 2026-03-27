package skills_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/skills"
)

func TestCreatePhaseOverviewSkillEmbedded(t *testing.T) {
	_, err := fs.ReadFile(skills.SkillFiles, "godark-create-phase-overview/SKILL.md")
	if err != nil {
		t.Errorf("godark-create-phase-overview not embedded: %v", err)
	}
}

func TestCreatePhaseOverviewSkillFrontmatterName(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "name: godark-create-phase-overview") {
		t.Error("frontmatter missing 'name: godark-create-phase-overview'")
	}
}

func TestCreatePhaseOverviewSkillFrontmatterDescription(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "description:") {
		t.Error("frontmatter missing 'description' field")
	}
}

func TestCreatePhaseOverviewSkillFrontmatterArgumentHint(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "argument-hint:") {
		t.Error("frontmatter missing 'argument-hint' field")
	}
}

func TestCreatePhaseOverviewSkillFrontmatterDisableModelInvocation(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "disable-model-invocation: true") {
		t.Error("frontmatter missing 'disable-model-invocation: true'")
	}
}

func TestCreatePhaseOverviewSkillReadsRoadmap(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	if !strings.Contains(content, "docs/roadmap/") {
		t.Error("skill does not reference docs/roadmap/")
	}
}

func TestCreatePhaseOverviewSkillReadsPlanningDoc(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	if !strings.Contains(content, "docs/planning/") {
		t.Error("skill does not reference docs/planning/ for planning docs")
	}
}

func TestCreatePhaseOverviewSkillExploresCodebase(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	if !strings.Contains(content, "docs/architecture.json") {
		t.Error("skill does not instruct agent to use docs/architecture.json for codebase exploration")
	}
}

func TestCreatePhaseOverviewSkillWritesToOverviewsDir(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	if !strings.Contains(content, "docs/phase-overviews/") {
		t.Error("skill does not specify docs/phase-overviews/ as output directory")
	}
}

func TestCreatePhaseOverviewSkillWarnsOnIncompletePhase(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	hasWarn := strings.Contains(content, "incomplete") || strings.Contains(content, "not marked")
	if !hasWarn {
		t.Error("skill does not warn when phase is not marked complete")
	}
}

func TestCreatePhaseOverviewSkillRequiresRealWorldExamples(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	if !strings.Contains(content, "Example") {
		t.Error("skill does not require real-world examples for each feature")
	}
}

func TestCreatePhaseOverviewSkillGroundsInCodebase(t *testing.T) {
	content := readSkill(t, "godark-create-phase-overview")
	hasGrounded := strings.Contains(content, "grounded") || strings.Contains(content, "actual codebase")
	if !hasGrounded {
		t.Error("skill does not emphasize grounding examples in actual codebase")
	}
}
