package skills_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/skills"
)

func TestConfigureProjectSkillEmbedded(t *testing.T) {
	_, err := fs.ReadFile(skills.SkillFiles, "godark-configure-project/SKILL.md")
	if err != nil {
		t.Errorf("godark-configure-project not embedded: %v", err)
	}
}

func TestConfigureProjectSkillFrontmatterName(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "name: godark-configure-project") {
		t.Error("frontmatter missing 'name: godark-configure-project'")
	}
}

func TestConfigureProjectSkillFrontmatterDescription(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "description:") {
		t.Error("frontmatter missing 'description' field")
	}
}

func TestConfigureProjectSkillFrontmatterArgumentHint(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "argument-hint:") {
		t.Error("frontmatter missing 'argument-hint' field")
	}
}

func TestConfigureProjectSkillFrontmatterDisableModelInvocation(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "disable-model-invocation: true") {
		t.Error("frontmatter missing 'disable-model-invocation: true'")
	}
}

func TestConfigureProjectSkillDetectsMultipleModules(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	hasGoMod := strings.Contains(content, "go.mod")
	hasPackageJSON := strings.Contains(content, "package.json")
	hasPubspec := strings.Contains(content, "pubspec.yaml")
	if !hasGoMod || !hasPackageJSON || !hasPubspec {
		t.Error("skill does not describe detecting multi-module projects (go.mod, package.json, pubspec.yaml)")
	}
}

func TestConfigureProjectSkillSuggestsModulesConfig(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, "modules:") {
		t.Error("skill does not suggest modules: config for multi-module projects")
	}
}

func TestConfigureProjectSkillModulesIncludeDependsOn(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, "depends_on") {
		t.Error("skill does not mention depends_on for module dependency ordering")
	}
}

func TestConfigureProjectSkillDetectsCodegenConfigs(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	hasSqlc := strings.Contains(content, "sqlc")
	hasGqlgen := strings.Contains(content, "gqlgen")
	if !hasSqlc || !hasGqlgen {
		t.Error("skill does not describe detecting codegen config files (sqlc, gqlgen)")
	}
}

func TestConfigureProjectSkillDetectsProtoFiles(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, ".proto") {
		t.Error("skill does not describe detecting .proto files for protobuf codegen")
	}
}

func TestConfigureProjectSkillDetectsMakefileGenerateTargets(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, "Makefile") {
		t.Error("skill does not describe detecting Makefile generate targets")
	}
}

func TestConfigureProjectSkillSuggestsGenerateCommand(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, "generate_command") {
		t.Error("skill does not suggest generate_command for codegen")
	}
}

func TestConfigureProjectSkillSuggestsGeneratedPaths(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, "generated_paths") {
		t.Error("skill does not suggest generated_paths for codegen output directories")
	}
}

func TestConfigureProjectSkillDetectsEnvExample(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, ".env.example") {
		t.Error("skill does not describe detecting .env.example for required_env")
	}
}

func TestConfigureProjectSkillDetectsComposeSecrets(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	hasCompose := strings.Contains(content, "docker-compose") || strings.Contains(content, "compose.yml")
	hasSecrets := strings.Contains(content, "secrets:")
	if !hasCompose || !hasSecrets {
		t.Error("skill does not describe detecting secrets in Docker Compose files for required_env")
	}
}

func TestConfigureProjectSkillSuggestsRequiredEnv(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, "required_env") {
		t.Error("skill does not suggest required_env field")
	}
}

func TestConfigureProjectSkillDetectsCIWorkflows(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, ".github/workflows") {
		t.Error("skill does not describe detecting CI workflow files in .github/workflows/")
	}
}

func TestConfigureProjectSkillSuggestsWaitForChecks(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, "wait_for_checks") {
		t.Error("skill does not suggest wait_for_checks from CI workflow job names")
	}
}

func TestConfigureProjectSkillDetectsDockerCompose(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	hasCompose := strings.Contains(content, "docker-compose.yml") || strings.Contains(content, "compose.yml")
	if !hasCompose {
		t.Error("skill does not describe detecting Docker Compose files")
	}
}

func TestConfigureProjectSkillNotesNoSandbox(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	if !strings.Contains(content, "no_sandbox") {
		t.Error("skill does not note that no_sandbox: true may be needed for Docker Compose integration tests")
	}
}

func TestConfigureProjectSkillMergesExistingConfig(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	hasMerge := strings.Contains(content, "merge") || strings.Contains(content, "overwrite")
	if !hasMerge {
		t.Error("skill does not describe merging into existing godark.yaml without overwriting existing fields")
	}
}

func TestConfigureProjectSkillRequiresUserConfirmation(t *testing.T) {
	content := readSkill(t, "godark-configure-project")
	hasConfirm := strings.Contains(content, "confirm") || strings.Contains(content, "approve")
	if !hasConfirm {
		t.Error("skill does not require user confirmation before writing")
	}
}
