package quality

import (
	"testing"
	"time"
)

func TestCheckCostFloor(t *testing.T) {
	tests := []struct {
		name      string
		costUSD   float64
		threshold float64
		wantCode  string // empty means expect nil
	}{
		{
			name:      "low cost flagged",
			costUSD:   0.02,
			threshold: 0.10,
			wantCode:  "low_cost",
		},
		{
			name:      "cost above threshold",
			costUSD:   0.50,
			threshold: 0.10,
			wantCode:  "",
		},
		{
			name:      "cost equal to threshold",
			costUSD:   0.10,
			threshold: 0.10,
			wantCode:  "",
		},
		{
			name:      "cost check disabled",
			costUSD:   0.02,
			threshold: 0.0,
			wantCode:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckCostFloor(tt.costUSD, tt.threshold)
			if tt.wantCode == "" {
				if got != nil {
					t.Errorf("expected nil, got flag with code %q", got.Code)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected flag with code %q, got nil", tt.wantCode)
			}
			if got.Code != tt.wantCode {
				t.Errorf("got code %q, want %q", got.Code, tt.wantCode)
			}
			if got.Message == "" {
				t.Error("expected non-empty message")
			}
		})
	}
}

func TestCheckDuration(t *testing.T) {
	tests := []struct {
		name      string
		duration  time.Duration
		threshold time.Duration
		wantCode  string // empty means expect nil
	}{
		{
			name:      "short duration flagged",
			duration:  30 * time.Second,
			threshold: 60 * time.Second,
			wantCode:  "short_duration",
		},
		{
			name:      "duration above threshold",
			duration:  90 * time.Second,
			threshold: 60 * time.Second,
			wantCode:  "",
		},
		{
			name:      "duration equal to threshold",
			duration:  60 * time.Second,
			threshold: 60 * time.Second,
			wantCode:  "",
		},
		{
			name:      "duration check disabled",
			duration:  5 * time.Second,
			threshold: 0,
			wantCode:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckDuration(tt.duration, tt.threshold)
			if tt.wantCode == "" {
				if got != nil {
					t.Errorf("expected nil, got flag with code %q", got.Code)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected flag with code %q, got nil", tt.wantCode)
			}
			if got.Code != tt.wantCode {
				t.Errorf("got code %q, want %q", got.Code, tt.wantCode)
			}
			if got.Message == "" {
				t.Error("expected non-empty message")
			}
		})
	}
}

func TestCheckToolTrace(t *testing.T) {
	tests := []struct {
		name        string
		trace       []string
		testCommand string
		wantCodes   []string
	}{
		{
			name:        "no diff read",
			trace:       []string{"Bash: go build ./...", "Bash: go test ./..."},
			testCommand: "go test",
			wantCodes:   []string{"no_diff_read"},
		},
		{
			name:        "no tests run",
			trace:       []string{"Read: main.go", "gh pr diff 42"},
			testCommand: "go test",
			wantCodes:   []string{"no_tests_run"},
		},
		{
			name:        "normal trace with Read and go test",
			trace:       []string{"Read: main.go", "Bash: go test ./..."},
			testCommand: "go test",
			wantCodes:   nil,
		},
		{
			name:        "normal trace with gh pr diff and npm test",
			trace:       []string{"Bash: gh pr diff 42", "Bash: npm test"},
			testCommand: "npm test",
			wantCodes:   nil,
		},
		{
			name:        "empty trace produces both flags",
			trace:       []string{},
			testCommand: "go test",
			wantCodes:   []string{"no_diff_read", "no_tests_run"},
		},
		{
			name:        "nil trace produces both flags",
			trace:       nil,
			testCommand: "go test",
			wantCodes:   []string{"no_diff_read", "no_tests_run"},
		},
		{
			name:        "empty test command skips test check",
			trace:       []string{"Read: main.go"},
			testCommand: "",
			wantCodes:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckToolTrace(tt.trace, tt.testCommand)
			gotCodes := flagCodes(got)
			if !codesEqual(gotCodes, tt.wantCodes) {
				t.Errorf("got flags %v, want %v", gotCodes, tt.wantCodes)
			}
		})
	}
}

func TestCheckReviewTestExecution(t *testing.T) {
	reviewDir := "tests/review"
	testCmd := "go test ./tests/review/..."

	tests := []struct {
		name            string
		trace           []string
		reviewDir       string
		testCommand     string
		hasScenarioSpec bool
		wantCodes       []string
	}{
		{
			name:            "tests written and run",
			trace:           []string{"Write: tests/review/foo_test.go", "Bash: go test ./tests/review/..."},
			reviewDir:       reviewDir,
			testCommand:     testCmd,
			hasScenarioSpec: true,
			wantCodes:       nil,
		},
		{
			name:            "neither written nor run",
			trace:           []string{},
			reviewDir:       reviewDir,
			testCommand:     testCmd,
			hasScenarioSpec: true,
			wantCodes:       []string{"no_review_tests_written", "no_review_tests_run"},
		},
		{
			name:            "written but not run",
			trace:           []string{"Write: tests/review/foo_test.go"},
			reviewDir:       reviewDir,
			testCommand:     testCmd,
			hasScenarioSpec: true,
			wantCodes:       []string{"no_review_tests_run"},
		},
		{
			name:            "run but not written",
			trace:           []string{"Bash: go test ./tests/review/..."},
			reviewDir:       reviewDir,
			testCommand:     testCmd,
			hasScenarioSpec: true,
			wantCodes:       []string{"no_review_tests_written"},
		},
		{
			name:            "write to different dir not counted",
			trace:           []string{"Write: src/foo.go", "Bash: go test ./tests/review/..."},
			reviewDir:       reviewDir,
			testCommand:     testCmd,
			hasScenarioSpec: true,
			wantCodes:       []string{"no_review_tests_written"},
		},
		{
			name:            "no scenario spec skips all checks",
			trace:           []string{},
			reviewDir:       reviewDir,
			testCommand:     testCmd,
			hasScenarioSpec: false,
			wantCodes:       nil,
		},
		{
			name:            "no scenario spec skips checks even when tests not written",
			trace:           []string{"Read: main.go"},
			reviewDir:       reviewDir,
			testCommand:     testCmd,
			hasScenarioSpec: false,
			wantCodes:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckReviewTestExecution(tt.trace, tt.reviewDir, tt.testCommand, tt.hasScenarioSpec)
			gotCodes := flagCodes(got)
			if !codesEqual(gotCodes, tt.wantCodes) {
				t.Errorf("got flags %v, want %v", gotCodes, tt.wantCodes)
			}
		})
	}
}

// flagCodes extracts Code fields from a slice of flags, or nil if slice is empty.
func flagCodes(flags []Flag) []string {
	if len(flags) == 0 {
		return nil
	}
	codes := make([]string, len(flags))
	for i, f := range flags {
		codes[i] = f.Code
	}
	return codes
}

// codesEqual compares two string slices (both nil/empty treated as equal).
func codesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
