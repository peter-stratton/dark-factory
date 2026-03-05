package agent

import (
	"testing"
)

func TestParseAcceptanceTests_ValidJSON(t *testing.T) {
	input := `["Test one", "Test two", "Test three"]`
	tests := parseAcceptanceTests(input, testLogger(t))
	if len(tests) != 3 {
		t.Fatalf("expected 3 tests, got %d: %v", len(tests), tests)
	}
	if tests[0] != "Test one" {
		t.Errorf("tests[0] = %q, want %q", tests[0], "Test one")
	}
}

func TestParseAcceptanceTests_FencedJSON(t *testing.T) {
	input := "```json\n[\"Test one\", \"Test two\"]\n```"
	tests := parseAcceptanceTests(input, testLogger(t))
	if len(tests) != 2 {
		t.Fatalf("expected 2 tests, got %d: %v", len(tests), tests)
	}
	if tests[0] != "Test one" {
		t.Errorf("tests[0] = %q, want %q", tests[0], "Test one")
	}
}

func TestParseAcceptanceTests_FencedNoLang(t *testing.T) {
	input := "```\n[\"A\", \"B\"]\n```"
	tests := parseAcceptanceTests(input, testLogger(t))
	if len(tests) != 2 {
		t.Fatalf("expected 2 tests, got %d: %v", len(tests), tests)
	}
}

func TestParseAcceptanceTests_InvalidJSON(t *testing.T) {
	input := "This is not JSON at all"
	tests := parseAcceptanceTests(input, testLogger(t))
	if tests != nil {
		t.Errorf("expected nil for invalid JSON, got %v", tests)
	}
}

func TestParseAcceptanceTests_EmptyString(t *testing.T) {
	tests := parseAcceptanceTests("", testLogger(t))
	if tests != nil {
		t.Errorf("expected nil for empty string, got %v", tests)
	}
}

func TestParseAcceptanceTests_CapsAtFive(t *testing.T) {
	input := `["A","B","C","D","E","F","G"]`
	tests := parseAcceptanceTests(input, testLogger(t))
	if len(tests) != 5 {
		t.Fatalf("expected 5 tests (capped), got %d: %v", len(tests), tests)
	}
}

func TestParseAcceptanceTests_EmbeddedInText(t *testing.T) {
	input := "Here are the tests:\n[\"Test one\", \"Test two\"]\nDone."
	tests := parseAcceptanceTests(input, testLogger(t))
	if len(tests) != 2 {
		t.Fatalf("expected 2 tests from embedded JSON, got %d: %v", len(tests), tests)
	}
}

func TestParseAcceptanceTests_EmptyArray(t *testing.T) {
	input := `[]`
	tests := parseAcceptanceTests(input, testLogger(t))
	if len(tests) != 0 {
		t.Errorf("expected 0 tests for empty array, got %d", len(tests))
	}
}
