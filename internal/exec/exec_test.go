package exec

import (
	"strings"
	"testing"
)

func TestDefaultRunsCommand(t *testing.T) {
	out, err := Default("echo", "hello")
	if err != nil {
		t.Fatalf("Default: unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("Default: got %q, want output containing %q", string(out), "hello")
	}
}

func TestCommandRunnerFuncAssignable(t *testing.T) {
	var runner CommandRunnerFunc = func(name string, args ...string) ([]byte, error) {
		return []byte("fake"), nil
	}
	out, err := runner("anything")
	if err != nil {
		t.Fatalf("runner: unexpected error: %v", err)
	}
	if string(out) != "fake" {
		t.Errorf("runner: got %q, want %q", string(out), "fake")
	}
}
