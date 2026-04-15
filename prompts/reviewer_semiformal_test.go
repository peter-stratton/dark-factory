package prompts

import (
	"strings"
	"testing"
)

func TestReviewerSemiformalContainsSecurityTrace(t *testing.T) {
	data, err := FS.ReadFile("reviewer_semiformal.txt")
	if err != nil {
		t.Fatalf("reading reviewer_semiformal.txt: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "### SECURITY TRACE") {
		t.Error("reviewer_semiformal.txt missing SECURITY TRACE section")
	}
}
