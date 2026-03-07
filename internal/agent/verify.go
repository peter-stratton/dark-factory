package agent

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/phs/dark-factory/internal/sandbox"
)

// sandboxRunContainer is a testability seam for sandbox.RunContainer.
// Replaceable for testing.
var sandboxRunContainer = sandbox.RunContainer

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

// sandboxCommandRunner returns a CommandRunner that executes verify commands
// inside a Docker container. The container clones the repo, checks out the
// PR branch, and runs the verify command via sh. The verify command is passed
// through the GODARK_VERIFY_CMD environment variable to avoid shell quoting issues.
func sandboxCommandRunner(image, repo, branch string, logger *slog.Logger) CommandRunner {
	return func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		script := fmt.Sprintf(
			"#!/bin/sh\nset -e\ngit clone https://github.com/%s.git /workspace && cd /workspace && git checkout %s && sh -c \"$GODARK_VERIFY_CMD\"\n",
			repo, branch,
		)
		opts := sandbox.RunOpts{
			Image: image,
			Cmd:   []string{"sh", "-c", script},
			Env:   map[string]string{"GODARK_VERIFY_CMD": command},
		}
		result, err := sandboxRunContainer(ctx, opts, logger)
		if err != nil {
			return nil, nil, 1, fmt.Errorf("running verify container: %w", err)
		}
		return []byte(result.Stdout), []byte(result.Stderr), result.ExitCode, nil
	}
}

// formatVerifyErrors formats the failed checks from a VerifyResult as a
// human-readable string suitable for inclusion in the verify_fix prompt.
// Each failed check is rendered as "=== <name> (exit code N) ===\n<output>\n".
func formatVerifyErrors(result VerifyResult) string {
	var sb strings.Builder
	for _, cr := range result.Checks {
		if cr.Passed {
			continue
		}
		sb.WriteString(fmt.Sprintf("=== %s (exit code %d) ===\n", cr.Name, cr.ExitCode))
		sb.WriteString(cr.Output)
		sb.WriteString("\n")
	}
	return sb.String()
}
