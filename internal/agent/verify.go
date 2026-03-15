package agent

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/phs/dark-factory/internal/sandbox"
)

// sandboxRunContainer is a testability seam for sandbox.RunContainer.
// Replaceable for testing.
var sandboxRunContainer = sandbox.RunContainer

// verifyFixFn is a testability seam for VerifyFix.
// Replaceable for testing.
var verifyFixFn = VerifyFix

// Check defines a single verification command to run.
type Check struct {
	Name    string // "build", "lint", or "test"
	Command string // shell command to execute
}

// CheckResult holds the outcome of a single verification check.
type CheckResult struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Output   string `json:"output"` // combined stdout+stderr, truncated
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

// RunVerify executes each check in sequence using the provided runner.
// It stops at the first failure unless all checks are requested.
// Returns a VerifyResult with outcomes for all checks that were run.
func RunVerify(ctx context.Context, checks []Check, run CommandRunner, limits config.TruncationLimits) VerifyResult {
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
		output := truncateVerifyOutput(combined, limits.VerifyOutput)

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

// truncateVerifyOutput keeps the last limit bytes of output.
// A limit of 0 means no truncation.
func truncateVerifyOutput(b []byte, limit int) string {
	if limit <= 0 || len(b) <= limit {
		return string(b)
	}
	return string(b[len(b)-limit:])
}

// sandboxCommandRunner returns a CommandRunner that executes verify commands
// inside a Docker container. The container clones the repo, checks out the
// PR branch, and runs the verify command via sh. The verify command is passed
// through the GODARK_VERIFY_CMD environment variable to avoid shell quoting issues.
func sandboxCommandRunner(image, repo, branch string, authEnv map[string]string, logger *slog.Logger) CommandRunner {
	return func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		script := "#!/bin/sh\nset -e\ngh auth setup-git\ngit clone \"https://github.com/${GODARK_REPO}.git\" /workspace && cd /workspace && git checkout \"${GODARK_BRANCH}\" && sh -c \"$GODARK_VERIFY_CMD\"\n"
		env := map[string]string{
			"GODARK_REPO":       repo,
			"GODARK_BRANCH":     branch,
			"GODARK_VERIFY_CMD": command,
		}
		for k, v := range authEnv {
			env[k] = v
		}
		opts := sandbox.RunOpts{
			Image: image,
			Cmd:   []string{"sh", "-c", script},
			Env:   env,
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
		fmt.Fprintf(&sb, "=== %s (exit code %d) ===\n", cr.Name, cr.ExitCode)
		sb.WriteString(cr.Output)
		sb.WriteString("\n")
	}
	return sb.String()
}

// failedCheckNames extracts the names of failed checks from a VerifyResult.
func failedCheckNames(vr VerifyResult) []string {
	var names []string
	for _, cr := range vr.Checks {
		if !cr.Passed {
			names = append(names, cr.Name)
		}
	}
	return names
}

// RunRollupVerify runs the verify pipeline on the rollup branch and invokes
// the implementer agent on failure (same fix cycle as per-issue verify).
// writeResult, if non-nil, is called after each verify attempt to persist
// the result; errors are logged as warnings and do not abort the cycle.
// Returns (true, nil) when verify passes; (false, nil) when all fix attempts
// are exhausted (caller should leave the PR open); non-nil error for
// unexpected failures (agent crashes, context cancellation, etc.).
func RunRollupVerify(
	ctx context.Context,
	prNum int,
	branch string,
	cfg *config.Config,
	prompts *Prompts,
	authEnv map[string]string,
	logger *slog.Logger,
	writeResult func(rundata.VerifyStepResult) error,
) (bool, error) {
	verifyChecks := buildVerifyChecks(cfg)
	if len(verifyChecks) == 0 {
		return true, nil
	}

	var verifyRunner CommandRunner
	if cfg.NoSandbox {
		verifyRunner = newHostRunner()
	} else {
		verifyRunner = sandboxCommandRunner(cfg.Docker.Image, cfg.Repo, branch, authEnv, logger)
	}

	logger.Info("running rollup verify step", "check_count", len(verifyChecks))
	verifyResult := RunVerify(ctx, verifyChecks, verifyRunner, cfg.Truncation)
	if writeResult != nil {
		if err := writeResult(verifyToRundata(verifyResult, 0, false)); err != nil {
			logger.Warn("failed to write rollup verify result", "error", err)
		}
	}

	if verifyResult.AllPassed {
		logger.Info("rollup verify passed")
		return true, nil
	}

	if prompts.VerifyFix == "" || cfg.Verify.MaxFixAttempts == 0 {
		logger.Warn("rollup verify failed (no fix configured)", "failed_checks", failedCheckNames(verifyResult))
		return false, nil
	}

	// Synthetic issue for VerifyFix prompt rendering — rollup has no real issue number.
	rollupIssue := github.Issue{Title: "rollup PR"}
	var sessionID string

	for fixAttempt := 0; fixAttempt < cfg.Verify.MaxFixAttempts; fixAttempt++ {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		verifyErrors := formatVerifyErrors(verifyResult)
		logger.Info("running rollup verify-fix attempt",
			"attempt", fixAttempt+1,
			"max_attempts", cfg.Verify.MaxFixAttempts,
		)

		fixResult, err := verifyFixFn(ctx, rollupIssue, prNum, verifyErrors, sessionID, cfg, prompts, authEnv, logger)
		if err != nil {
			return false, fmt.Errorf("rollup verify-fix agent: %w", err)
		}
		if fixResult.TimedOut {
			return false, fmt.Errorf("rollup verify-fix agent timed out")
		}
		sessionID = fixResult.SessionID

		verifyResult = RunVerify(ctx, verifyChecks, verifyRunner, cfg.Truncation)
		if writeResult != nil {
			if err := writeResult(verifyToRundata(verifyResult, fixAttempt+1, true)); err != nil {
				logger.Warn("failed to write rollup verify result", "error", err)
			}
		}

		if verifyResult.AllPassed {
			logger.Info("rollup verify passed after fix", "attempt", fixAttempt+1)
			return true, nil
		}
	}

	logger.Warn("rollup verify failed after all fix attempts",
		"max_attempts", cfg.Verify.MaxFixAttempts,
		"failed_checks", failedCheckNames(verifyResult),
	)
	return false, nil
}
