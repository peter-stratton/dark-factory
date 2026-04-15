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

func TestCheckSemiformalConsistency(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantCode string // empty means expect nil
	}{
		{
			name:     "no formal conclusion",
			output:   "some review output without the marker",
			wantCode: "",
		},
		{
			name:     "clean approval",
			output:   "FORMAL CONCLUSION\nAll checks SATISFIED\nAGENT_RESULT=APPROVED",
			wantCode: "",
		},
		{
			name:     "NOT SATISFIED with APPROVED",
			output:   "FORMAL CONCLUSION\nAC1: Verdict: NOT SATISFIED\nAGENT_RESULT=APPROVED",
			wantCode: "semiformal_inconsistency",
		},
		{
			name:     "BROKEN with APPROVED",
			output:   "FORMAL CONCLUSION\nStatus: BROKEN\nAGENT_RESULT=APPROVED",
			wantCode: "semiformal_inconsistency",
		},
		{
			name:     "HIGH risk with APPROVED",
			output:   "FORMAL CONCLUSION\nRisk: HIGH\nAGENT_RESULT=APPROVED",
			wantCode: "semiformal_inconsistency",
		},
		{
			name:     "NOT SATISFIED with CHANGES_REQUESTED",
			output:   "FORMAL CONCLUSION\nAC1: Verdict: NOT SATISFIED\nAGENT_RESULT=CHANGES_REQUESTED",
			wantCode: "",
		},
		{
			name:     "BROKEN with CHANGES_REQUESTED",
			output:   "FORMAL CONCLUSION\nStatus: BROKEN\nAGENT_RESULT=CHANGES_REQUESTED",
			wantCode: "",
		},
		{
			name:     "FLAGGED with APPROVED",
			output:   "FORMAL CONCLUSION\nSECURITY TRACE\nauth.go: FLAGGED (hardcoded API key)\nAGENT_RESULT=APPROVED",
			wantCode: "semiformal_inconsistency",
		},
		{
			name:     "CLEAR with APPROVED",
			output:   "FORMAL CONCLUSION\nSECURITY TRACE\nauth.go: CLEAR\nAGENT_RESULT=APPROVED",
			wantCode: "",
		},
		{
			name:     "FLAGGED with CHANGES_REQUESTED",
			output:   "FORMAL CONCLUSION\nSECURITY TRACE\nauth.go: FLAGGED (missing auth)\nAGENT_RESULT=CHANGES_REQUESTED",
			wantCode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckSemiformalConsistency(tt.output)
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
		name         string
		trace        []string
		testCommand  string
		checkTestRun bool
		wantCodes    []string
	}{
		{
			name:         "no diff read",
			trace:        []string{"Bash: go build ./...", "Bash: go test ./..."},
			testCommand:  "go test",
			checkTestRun: true,
			wantCodes:    []string{"no_diff_read"},
		},
		{
			name:         "no tests run",
			trace:        []string{"Read: main.go", "gh pr diff 42"},
			testCommand:  "go test",
			checkTestRun: true,
			wantCodes:    []string{"no_tests_run"},
		},
		{
			name:         "normal trace with Read and go test",
			trace:        []string{"Read: main.go", "Bash: go test ./..."},
			testCommand:  "go test",
			checkTestRun: true,
			wantCodes:    nil,
		},
		{
			name:         "normal trace with gh pr diff and npm test",
			trace:        []string{"Bash: gh pr diff 42", "Bash: npm test"},
			testCommand:  "npm test",
			checkTestRun: true,
			wantCodes:    nil,
		},
		{
			name:         "empty trace produces both flags",
			trace:        []string{},
			testCommand:  "go test",
			checkTestRun: true,
			wantCodes:    []string{"no_diff_read", "no_tests_run"},
		},
		{
			name:         "nil trace produces both flags",
			trace:        nil,
			testCommand:  "go test",
			checkTestRun: true,
			wantCodes:    []string{"no_diff_read", "no_tests_run"},
		},
		{
			name:         "empty test command skips test check",
			trace:        []string{"Read: main.go"},
			testCommand:  "",
			checkTestRun: true,
			wantCodes:    nil,
		},
		{
			name:         "checkTestRun false skips test check",
			trace:        []string{"Read: main.go"},
			testCommand:  "go test",
			checkTestRun: false,
			wantCodes:    nil,
		},
		{
			name:         "checkTestRun false with empty trace only flags diff",
			trace:        nil,
			testCommand:  "go test",
			checkTestRun: false,
			wantCodes:    []string{"no_diff_read"},
		},
		{
			name:         "agent uses absolute path instead of cd prefix",
			trace:        []string{"Read: main.go", "Bash: cd /workspace/service && go test -v ./... 2>&1"},
			testCommand:  "cd service && go test ./...",
			checkTestRun: true,
			wantCodes:    nil,
		},
		{
			name:         "agent adds extra flags to test command",
			trace:        []string{"Read: main.go", "Bash: go test -v -timeout 30s -tags integration ./... 2>&1"},
			testCommand:  "go test ./...",
			checkTestRun: true,
			wantCodes:    nil,
		},
		{
			name:         "agent runs pytest with extra flags",
			trace:        []string{"Read: main.py", "Bash: pytest -xvs tests/ --tb=short 2>&1"},
			testCommand:  "pytest tests/",
			checkTestRun: true,
			wantCodes:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckToolTrace(tt.trace, tt.testCommand, tt.checkTestRun)
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
			name:            "bash write to review dir counts",
			trace:           []string{"Bash: cat > tests/review/issue42_test.go << 'EOF'", "Bash: go test ./tests/review/..."},
			reviewDir:       reviewDir,
			testCommand:     testCmd,
			hasScenarioSpec: true,
			wantCodes:       nil,
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

func TestTestRunnerSignature(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{
			name:    "go test with cd prefix",
			command: "cd service && go test ./...",
			want:    []string{"go", "test"},
		},
		{
			name:    "npm test",
			command: "npm test",
			want:    []string{"npm", "test"},
		},
		{
			name:    "pytest with dir target",
			command: "pytest tests/",
			want:    []string{"pytest"},
		},
		{
			name:    "cargo test",
			command: "cargo test",
			want:    []string{"cargo", "test"},
		},
		{
			name:    "dart test with dir",
			command: "dart test test/",
			want:    []string{"dart", "test"},
		},
		{
			name:    "python -m pytest",
			command: "cd src && python -m pytest",
			want:    []string{"python", "-m", "pytest"},
		},
		{
			name:    "go test with relative path",
			command: "go test ./internal/...",
			want:    []string{"go", "test"},
		},
		{
			name:    "empty command",
			command: "",
			want:    nil,
		},
		{
			name:    "flutter test",
			command: "flutter test",
			want:    []string{"flutter", "test"},
		},
		{
			name:    "command with redirect",
			command: "go test ./... 2>&1",
			want:    []string{"go", "test"},
		},
		{
			name:    "command with pipe",
			command: "go test ./... | head -20",
			want:    []string{"go", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testRunnerSignature(tt.command)
			if !codesEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesSignature(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		sig   []string
		want  bool
	}{
		{
			name:  "exact match",
			entry: "Bash: go test ./...",
			sig:   []string{"go", "test"},
			want:  true,
		},
		{
			name:  "absolute path with flags",
			entry: "Bash: cd /workspace/service && go test -v -timeout 30s ./... 2>&1",
			sig:   []string{"go", "test"},
			want:  true,
		},
		{
			name:  "npm test with extra args",
			entry: "Bash: npm test -- --coverage 2>&1",
			sig:   []string{"npm", "test"},
			want:  true,
		},
		{
			name:  "pytest with flags",
			entry: "Bash: pytest -xvs tests/ 2>&1",
			sig:   []string{"pytest"},
			want:  true,
		},
		{
			name:  "non-bash entry ignored",
			entry: "Read: go test output.txt",
			sig:   []string{"go", "test"},
			want:  false,
		},
		{
			name:  "partial match not enough",
			entry: "Bash: go build ./...",
			sig:   []string{"go", "test"},
			want:  false,
		},
		{
			name:  "empty signature",
			entry: "Bash: go test ./...",
			sig:   nil,
			want:  false,
		},
		{
			name:  "python -m pytest",
			entry: "Bash: python -m pytest tests/ -v 2>&1",
			sig:   []string{"python", "-m", "pytest"},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesSignature(tt.entry, tt.sig)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
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
