package runner_test

import (
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/agent/runner"
)

func TestAgentRunnerPyIsEmbedded(t *testing.T) {
	content, err := runner.FS.ReadFile("agent_runner.py")
	if err != nil {
		t.Fatalf("ReadFile(agent_runner.py): %v", err)
	}
	if len(content) == 0 {
		t.Fatal("agent_runner.py is empty")
	}
}

func TestAgentRunnerPyImportsClaude(t *testing.T) {
	content, err := runner.FS.ReadFile("agent_runner.py")
	if err != nil {
		t.Fatalf("ReadFile(agent_runner.py): %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "claude_agent_sdk") {
		t.Error("agent_runner.py does not import claude_agent_sdk")
	}
}

func TestAgentRunnerPyReadsGodarkPrompt(t *testing.T) {
	content, err := runner.FS.ReadFile("agent_runner.py")
	if err != nil {
		t.Fatalf("ReadFile(agent_runner.py): %v", err)
	}
	if !strings.Contains(string(content), "GODARK_PROMPT") {
		t.Error("agent_runner.py does not read GODARK_PROMPT from environment")
	}
}
