package doctor

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	goexec "github.com/phs/dark-factory/internal/exec"
)

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner goexec.CommandRunnerFunc = goexec.Default

// EnvLookup looks up an environment variable.
// Replaceable for testing.
var EnvLookup = os.Getenv

// CheckTimeout is the per-check deadline enforced by Run.
// Overridable in tests.
var CheckTimeout = 15 * time.Second

// Check represents a single pre-flight check.
type Check struct {
	Name string
	Fix  string
	run  func() bool
}

// runtimeVersionCmd returns the command and args to verify a named runtime is
// available. Returns an empty string if the runtime is unknown.
func runtimeVersionCmd(runtime string) (string, []string) {
	switch runtime {
	case "go":
		return "go", []string{"version"}
	case "flutter":
		return "flutter", []string{"--version"}
	case "node":
		return "node", []string{"--version"}
	case "rust":
		return "rustc", []string{"--version"}
	case "elixir":
		return "elixir", []string{"--version"}
	case "python":
		return "python3", []string{"--version"}
	default:
		return "", nil
	}
}

// Checks returns the full ordered list of pre-flight checks. If runtime is
// non-empty, a toolchain availability check for that runtime is included.
// If lintCommand contains "golangci-lint", a check for that tool is appended.
func Checks(runtime, lintCommand string) []*Check {
	checks := []*Check{
		{
			Name: "Docker daemon running",
			Fix:  "Start Docker Desktop or the Docker daemon (e.g. `sudo systemctl start docker`).",
			run: func() bool {
				_, err := CommandRunner("docker", "info")
				return err == nil
			},
		},
		{
			Name: "gh CLI installed",
			Fix:  "Install the GitHub CLI: https://cli.github.com",
			run: func() bool {
				_, err := CommandRunner("gh", "--version")
				return err == nil
			},
		},
		{
			Name: "gh CLI authenticated",
			Fix:  "Run `gh auth login` to authenticate.",
			run: func() bool {
				_, err := CommandRunner("gh", "auth", "status")
				return err == nil
			},
		},
		{
			Name: "Anthropic auth token set",
			Fix:  "Export ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN in your shell profile or pass it in the environment.",
			run: func() bool {
				return EnvLookup("ANTHROPIC_API_KEY") != "" || EnvLookup("CLAUDE_CODE_OAUTH_TOKEN") != ""
			},
		},
	}

	if runtime != "" {
		bin, args := runtimeVersionCmd(runtime)
		if bin != "" {
			rt := runtime // capture for closure
			b := bin
			a := args
			checks = append(checks, &Check{
				Name: fmt.Sprintf("%s toolchain available", rt),
				Fix:  fmt.Sprintf("Install the %s toolchain and ensure it is on your PATH.", rt),
				run: func() bool {
					_, err := CommandRunner(b, a...)
					return err == nil
				},
			})
		}
	}

	checks = append(checks, &Check{
		Name: "Python 3 available",
		Fix:  "Install Python 3: https://www.python.org/downloads/",
		run: func() bool {
			_, err := CommandRunner("python3", "--version")
			return err == nil
		},
	})

	if strings.Contains(lintCommand, "golangci-lint") {
		checks = append(checks, &Check{
			Name: "golangci-lint installed",
			Fix:  "Install golangci-lint: `brew install golangci-lint` or see https://golangci-lint.run/usage/install/",
			run: func() bool {
				_, err := CommandRunner("golangci-lint", "--version")
				return err == nil
			},
		})
	}

	return checks
}

// Run executes all checks, writes a pass/fail report to w, and returns true
// if every check passed. Each check is run in a goroutine; if it does not
// complete within CheckTimeout it is reported as a failure.
func Run(w io.Writer, checks []*Check) bool {
	allPassed := true
	for _, c := range checks {
		passed := runWithTimeout(c)
		if passed {
			fmt.Fprintf(w, "[PASS] %s\n", c.Name)
		} else {
			allPassed = false
			fmt.Fprintf(w, "[FAIL] %s\n", c.Name)
			// Indent the fix hint under the failure.
			fmt.Fprintf(w, "       Fix: %s\n", strings.TrimSpace(c.Fix))
		}
	}
	return allPassed
}

// runWithTimeout runs a single check in a goroutine and returns its result,
// or false if it exceeds CheckTimeout.
func runWithTimeout(c *Check) bool {
	result := make(chan bool, 1)
	go func() {
		result <- c.run()
	}()
	timer := time.NewTimer(CheckTimeout)
	defer timer.Stop()
	select {
	case passed := <-result:
		return passed
	case <-timer.C:
		return false
	}
}
