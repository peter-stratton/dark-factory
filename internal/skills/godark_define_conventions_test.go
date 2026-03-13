package skills_test

import (
	"strings"
	"testing"
)

func TestDefineConventionsSkillFrontmatterName(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "name:") {
		t.Error("frontmatter missing 'name' field")
	}
}

func TestDefineConventionsSkillFrontmatterDescription(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "description:") {
		t.Error("frontmatter missing 'description' field")
	}
}

func TestDefineConventionsSkillFrontmatterArgumentHint(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "argument-hint:") {
		t.Error("frontmatter missing 'argument-hint' field")
	}
}

func TestDefineConventionsSkillFrontmatterDisableModelInvocation(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "disable-model-invocation: true") {
		t.Error("frontmatter missing 'disable-model-invocation: true'")
	}
}

func TestDefineConventionsSkillStepsDescribeSourceFileAnalysis(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	if !strings.Contains(content, "source file") && !strings.Contains(content, "source files") {
		t.Error("steps do not describe reading source files for existing projects")
	}
}

func TestDefineConventionsSkillStepsIdentifyPatterns(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	if !strings.Contains(content, "pattern") && !strings.Contains(content, "Pattern") {
		t.Error("steps do not describe identifying patterns in existing code")
	}
}

func TestDefineConventionsSkillStepsDescribeAgentFriendlinessExplicitOverImplicit(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	if !strings.Contains(content, "explicit") || !strings.Contains(content, "implicit") {
		t.Error("steps do not describe explicit over implicit principle for agent-friendliness")
	}
}

func TestDefineConventionsSkillStepsDescribeAgentFriendlinessLocalOverGlobal(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	if !strings.Contains(content, "local") || !strings.Contains(content, "global") {
		t.Error("steps do not describe local over global principle for agent-friendliness")
	}
}

func TestDefineConventionsSkillStepsFlagCodeGeneration(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	if !strings.Contains(content, "code generation") && !strings.Contains(content, "generators") {
		t.Error("steps do not flag heavy code generation as an anti-pattern")
	}
}

func TestDefineConventionsSkillStepsFlagConventionOverConfiguration(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	if !strings.Contains(content, "convention-over-configuration") && !strings.Contains(content, "Convention-over-configuration") {
		t.Error("steps do not flag convention-over-configuration magic as an anti-pattern")
	}
}

func TestDefineConventionsSkillStepsSuggestRoadmapForInconsistencies(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	if !strings.Contains(content, "/godark-create-roadmap") {
		t.Error("steps do not suggest running /godark-create-roadmap for inconsistent conventions")
	}
}

func TestDefineConventionsSkillStepsWriteConventionsMD(t *testing.T) {
	content := readSkill(t, "godark-define-conventions")
	if !strings.Contains(content, "docs/conventions.md") {
		t.Error("steps do not include writing docs/conventions.md")
	}
}
