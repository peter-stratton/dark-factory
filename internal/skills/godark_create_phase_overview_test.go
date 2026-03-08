package skills_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/skills"
)

func readCreatePhaseOverviewSkill(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(skills.SkillFiles, "godark-create-phase-overview/SKILL.md")
	if err != nil {
		t.Fatalf("reading godark-create-phase-overview/SKILL.md: %v", err)
	}
	return string(data)
}

func TestCreatePhaseOverviewSkillEmbedded(t *testing.T) {
	_, err := fs.ReadFile(skills.SkillFiles, "godark-create-phase-overview/SKILL.md")
	if err != nil {
		t.Errorf("godark-create-phase-overview not embedded: %v", err)
	}
}

func TestCreatePhaseOverviewSkillFrontmatterName(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "name: godark-create-phase-overview") {
		t.Error("frontmatter missing 'name: godark-create-phase-overview'")
	}
}

func TestCreatePhaseOverviewSkillFrontmatterDescription(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "description:") {
		t.Error("frontmatter missing 'description' field")
	}
}

func TestCreatePhaseOverviewSkillFrontmatterArgumentHint(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "argument-hint:") {
		t.Error("frontmatter missing 'argument-hint' field")
	}
}

func TestCreatePhaseOverviewSkillFrontmatterDisableModelInvocation(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "disable-model-invocation: true") {
		t.Error("frontmatter missing 'disable-model-invocation: true'")
	}
}

func TestCreatePhaseOverviewSkillReadsRoadmap(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	if !strings.Contains(content, "ROADMAP.md") {
		t.Error("skill does not reference ROADMAP.md")
	}
}

func TestCreatePhaseOverviewSkillReadsPlanningDoc(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	if !strings.Contains(content, "docs/planning/") {
		t.Error("skill does not reference docs/planning/ for planning docs")
	}
}

func TestCreatePhaseOverviewSkillExploresCodebase(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	if !strings.Contains(content, "internal/cmd/") {
		t.Error("skill does not instruct agent to explore internal/cmd/")
	}
}

func TestCreatePhaseOverviewSkillWritesToOverviewsDir(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	if !strings.Contains(content, "docs/phase-overviews/") {
		t.Error("skill does not specify docs/phase-overviews/ as output directory")
	}
}

func TestCreatePhaseOverviewSkillWarnsOnIncompletePhase(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	hasWarn := strings.Contains(content, "incomplete") || strings.Contains(content, "not marked")
	if !hasWarn {
		t.Error("skill does not warn when phase is not marked complete")
	}
}

func TestCreatePhaseOverviewSkillRequiresRealWorldExamples(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	if !strings.Contains(content, "Example") {
		t.Error("skill does not require real-world examples for each feature")
	}
}

func TestCreatePhaseOverviewSkillGroundsInCodebase(t *testing.T) {
	content := readCreatePhaseOverviewSkill(t)
	hasGrounded := strings.Contains(content, "grounded") || strings.Contains(content, "actual codebase")
	if !hasGrounded {
		t.Error("skill does not emphasize grounding examples in actual codebase")
	}
}
