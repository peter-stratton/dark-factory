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

// TestAgentRunnerPyReviewerDisallowsWrite verifies that the reviewer role
// configuration in agent_runner.py hard-denies Write and Edit tools via
// disallowed_tools.
func TestAgentRunnerPyReviewerDisallowsWrite(t *testing.T) {
	content, err := runner.FS.ReadFile("agent_runner.py")
	if err != nil {
		t.Fatalf("ReadFile(agent_runner.py): %v", err)
	}
	s := string(content)
	if !strings.Contains(s, `"reviewer"`) {
		t.Error("agent_runner.py does not define reviewer role")
	}
	if !strings.Contains(s, "disallowed_tools") {
		t.Error("agent_runner.py does not use disallowed_tools")
	}
	// Verify that Write and Edit appear in the disallowed section associated with reviewer.
	reviewerIdx := strings.Index(s, `"reviewer"`)
	if reviewerIdx < 0 {
		t.Fatal("reviewer role not found in agent_runner.py")
	}
	reviewerSection := s[reviewerIdx:]
	// The next role entry starts a new quoted key; find the boundary.
	nextRoleIdx := strings.Index(reviewerSection[1:], `"implementer"`)
	if nextRoleIdx < 0 {
		nextRoleIdx = len(reviewerSection)
	} else {
		nextRoleIdx++ // account for the offset
	}
	reviewerSection = reviewerSection[:nextRoleIdx]
	if !strings.Contains(reviewerSection, `"Write"`) {
		t.Error("reviewer disallowed_tools does not include Write")
	}
	if !strings.Contains(reviewerSection, `"Edit"`) {
		t.Error("reviewer disallowed_tools does not include Edit")
	}
}

// TestAgentRunnerPyReconRolePermissions verifies that the recon role
// configuration in agent_runner.py allows Read, Glob, Grep and hard-denies
// Write, Edit, Bash via disallowed_tools.
func TestAgentRunnerPyReconRolePermissions(t *testing.T) {
	content, err := runner.FS.ReadFile("agent_runner.py")
	if err != nil {
		t.Fatalf("ReadFile(agent_runner.py): %v", err)
	}
	s := string(content)

	reconIdx := strings.Index(s, `"recon"`)
	if reconIdx < 0 {
		t.Fatal("recon role not found in agent_runner.py")
	}

	// Find the recon section up to the next role entry (closing brace of dict).
	reconSection := s[reconIdx:]
	// The next top-level key after "recon" would be another quoted role name or the closing '}'.
	// We look for the pattern `},` or `}` followed by a newline and `}` to bound the section.
	// A simple approach: find the next `},` after the opening of the recon entry.
	nextEntryIdx := strings.Index(reconSection[1:], `},`)
	if nextEntryIdx < 0 {
		// Last entry in dict — bound by `}`
		nextEntryIdx = strings.Index(reconSection[1:], `}`)
	}
	if nextEntryIdx >= 0 {
		reconSection = reconSection[:nextEntryIdx+2]
	}

	// Verify allowed tools.
	for _, tool := range []string{`"Read"`, `"Glob"`, `"Grep"`} {
		if !strings.Contains(reconSection, tool) {
			t.Errorf("recon allowed_tools does not include %s", tool)
		}
	}

	// Verify disallowed tools.
	if !strings.Contains(reconSection, "disallowed_tools") {
		t.Error("recon role does not define disallowed_tools")
	}
	for _, tool := range []string{`"Write"`, `"Edit"`, `"Bash"`} {
		if !strings.Contains(reconSection, tool) {
			t.Errorf("recon disallowed_tools does not include %s", tool)
		}
	}
}

// TestAgentRunnerPyUnknownRoleExits verifies that agent_runner.py contains
// logic to exit with a non-zero code when GODARK_ROLE is not a known role.
func TestAgentRunnerPyUnknownRoleExits(t *testing.T) {
	content, err := runner.FS.ReadFile("agent_runner.py")
	if err != nil {
		t.Fatalf("ReadFile(agent_runner.py): %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "GODARK_ROLE") {
		t.Error("agent_runner.py does not read GODARK_ROLE from environment")
	}
	if !strings.Contains(s, "sys.exit(1)") {
		t.Error("agent_runner.py does not call sys.exit(1) for unknown roles")
	}
	if !strings.Contains(s, "unknown GODARK_ROLE") {
		t.Error("agent_runner.py does not print a descriptive error for unknown roles")
	}
}
