package sandbox

import (
	"strings"
	"testing"
)

func mustCloneScript(t *testing.T, repo, branch, workDir string) string {
	t.Helper()
	script, err := CloneScript(repo, branch, workDir)
	if err != nil {
		t.Fatalf("CloneScript(%q, %q, %q) returned error: %v", repo, branch, workDir, err)
	}
	return script
}

func TestCloneScript_ContainsCloneCommand(t *testing.T) {
	script := mustCloneScript(t, "owner/repo", "", "/workspace")
	if !strings.Contains(script, "git clone https://github.com/owner/repo.git /workspace") {
		t.Fatalf("expected clone command, got:\n%s", script)
	}
}

func TestCloneScript_WithBranch(t *testing.T) {
	script := mustCloneScript(t, "owner/repo", "feature-branch", "/workspace")
	if !strings.Contains(script, "git checkout feature-branch") {
		t.Fatalf("expected git checkout, got:\n%s", script)
	}
}

func TestCloneScript_NoBranch(t *testing.T) {
	script := mustCloneScript(t, "owner/repo", "", "/workspace")
	if strings.Contains(script, "git checkout") {
		t.Fatalf("unexpected git checkout in:\n%s", script)
	}
}

func TestCloneScript_GitConfig(t *testing.T) {
	script := mustCloneScript(t, "owner/repo", "", "/workspace")
	if !strings.Contains(script, `git config --global user.name`) {
		t.Fatalf("missing user.name config in:\n%s", script)
	}
	if !strings.Contains(script, `git config --global user.email`) {
		t.Fatalf("missing user.email config in:\n%s", script)
	}
	// Default values used when env vars are unset.
	if !strings.Contains(script, "dark-factory") {
		t.Fatalf("missing default author name in:\n%s", script)
	}
}

func TestCloneScript_AuthSetup(t *testing.T) {
	script := mustCloneScript(t, "owner/repo", "", "/workspace")
	if !strings.Contains(script, "gh auth setup-git") {
		t.Fatalf("missing gh auth setup-git in:\n%s", script)
	}
}

func TestCloneScript_EmptyRepo(t *testing.T) {
	_, err := CloneScript("", "", "/workspace")
	if err == nil {
		t.Fatal("expected error for empty repo, got nil")
	}
}

func TestEntrypointScript_CombinesCloneAndAgent(t *testing.T) {
	clone := mustCloneScript(t, "owner/repo", "main", "/workspace")
	ep := EntrypointScript(clone, "claude --run")

	if !strings.Contains(ep, "git clone") {
		t.Fatal("entrypoint missing clone step")
	}
	if !strings.Contains(ep, "claude --run") {
		t.Fatal("entrypoint missing agent command")
	}
}

func TestEntrypointScript_SetE(t *testing.T) {
	ep := EntrypointScript("", "echo hi")
	if !strings.Contains(ep, "set -e") {
		t.Fatalf("missing set -e in:\n%s", ep)
	}
}

func TestCloneScript_NoTokenInURL(t *testing.T) {
	script := mustCloneScript(t, "owner/repo", "", "/workspace")
	if strings.Contains(script, "$GH_TOKEN") || strings.Contains(script, "GH_TOKEN") {
		t.Fatalf("token leaked into clone URL:\n%s", script)
	}
}
