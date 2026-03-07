package agent

import (
	"bytes"
	"context"
)

// Check defines a single verification command to run.
type Check struct {
	Name    string // "build", "lint", or "test"
	Command string // shell command to execute
}

// CheckResult holds the outcome of a single verification check.
type CheckResult struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Output   string `json:"output"`   // combined stdout+stderr, truncated
	ExitCode int    `json:"exit_code"`
}

// VerifyResult holds the outcome of running all verification checks.
type VerifyResult struct {
	Checks    []CheckResult `json:"checks"`
	AllPassed bool          `json:"all_passed"`
}

// CommandRunner executes a shell command and returns stdout, stderr,
// exit code, and any execution error.
type CommandRunner func(ctx context.Context, command string) (stdout, stderr []byte, exitCode int, err error)

const verifyOutputLimit = 4096

// RunVerify executes each check in sequence using the provided runner.
// It stops at the first failure unless all checks are requested.
// Returns a VerifyResult with outcomes for all checks that were run.
func RunVerify(ctx context.Context, checks []Check, run CommandRunner) VerifyResult {
	results := []CheckResult{}

	for _, check := range checks {
		if ctx.Err() != nil {
			return VerifyResult{Checks: results, AllPassed: false}
		}

		if check.Command == "" {
			continue
		}

		stdout, stderr, exitCode, err := run(ctx, check.Command)

		combined := bytes.Join([][]byte{stdout, stderr}, nil)
		output := truncateVerifyOutput(combined)

		passed := exitCode == 0 && err == nil
		results = append(results, CheckResult{
			Name:     check.Name,
			Passed:   passed,
			Output:   output,
			ExitCode: exitCode,
		})

		if !passed {
			return VerifyResult{Checks: results, AllPassed: false}
		}
	}

	return VerifyResult{Checks: results, AllPassed: true}
}

// truncateVerifyOutput keeps the last verifyOutputLimit bytes of output.
func truncateVerifyOutput(b []byte) string {
	if len(b) <= verifyOutputLimit {
		return string(b)
	}
	return string(b[len(b)-verifyOutputLimit:])
}
