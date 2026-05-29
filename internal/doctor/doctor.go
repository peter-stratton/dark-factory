package doctor

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	goexec "github.com/peter-stratton/dark-factory/internal/exec"
)

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner goexec.CommandRunnerFunc = goexec.Default

// EnvLookup looks up an environment variable.
// Replaceable for testing.
var EnvLookup = os.Getenv

// SocketStat reports whether the path exists and is accessible.
// Replaceable for testing.
var SocketStat = func(path string) error {
	_, err := os.Stat(path)
	return err
}

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

// Opts controls which checks are included.
type Opts struct {
	Runtime           string   // detected runtime name (e.g. "go")
	ConfiguredRuntime string   // runtime.name from godark.yaml (e.g. "python")
	DetectedRuntimes  []string // every runtime whose marker file is present in the repo
	ModulesConfigured bool     // a non-empty modules: block is present in godark.yaml
	LintCommand       string   // configured lint command
	ComposeConfigured bool     // docker_compose block is present
	OAuthTokenEnv     string   // host env var name for Claude OAuth token (default CLAUDE_CODE_OAUTH_TOKEN)
}

// Checks returns the full ordered list of pre-flight checks.
//
// Checks always include Docker, gh CLI, and an Anthropic auth token.
// Compose checks are conditional on ComposeConfigured.
func Checks(opts Opts) []*Check {
	var checks []*Check

	checks = append(checks, &Check{
		Name: "Docker daemon running",
		Fix:  "Start Docker Desktop or the Docker daemon (e.g. `sudo systemctl start docker`).",
		run: func() bool {
			_, err := CommandRunner("docker", "info")
			return err == nil
		},
	})

	checks = append(checks,
		&Check{
			Name: "gh CLI installed",
			Fix:  "Install the GitHub CLI: https://cli.github.com",
			run: func() bool {
				_, err := CommandRunner("gh", "--version")
				return err == nil
			},
		},
		&Check{
			Name: "gh CLI authenticated",
			Fix:  "Run `gh auth login` to authenticate.",
			run: func() bool {
				_, err := CommandRunner("gh", "auth", "status")
				return err == nil
			},
		},
		func() *Check {
			oauthEnv := opts.OAuthTokenEnv
			if oauthEnv == "" {
				oauthEnv = "CLAUDE_CODE_OAUTH_TOKEN"
			}
			return &Check{
				Name: "Anthropic auth token set",
				Fix:  fmt.Sprintf("Export ANTHROPIC_API_KEY or %s in your shell profile or pass it in the environment.", oauthEnv),
				run: func() bool {
					return EnvLookup("ANTHROPIC_API_KEY") != "" || EnvLookup(oauthEnv) != ""
				},
			}
		}(),
	)

	if opts.ConfiguredRuntime != "" && opts.Runtime != "" && opts.ConfiguredRuntime != opts.Runtime {
		configured := opts.ConfiguredRuntime
		detected := opts.Runtime
		checks = append(checks, &Check{
			Name: "Configured runtime matches repo files",
			Fix: fmt.Sprintf(
				"godark.yaml has runtime.name=%q but the repo contains %s marker files. Update godark.yaml to runtime.name=%q, or configure a modules: block if this is a multi-runtime repo.",
				configured, detected, detected,
			),
			run: func() bool { return false },
		})
	}

	if len(opts.DetectedRuntimes) > 1 && !opts.ModulesConfigured {
		detected := strings.Join(opts.DetectedRuntimes, ", ")
		checks = append(checks, &Check{
			Name: "Multi-runtime repo has modules: configured",
			Fix: fmt.Sprintf(
				"Detected multiple runtimes (%s) but no modules: block in godark.yaml. Add a modules: map keyed by directory so each module has its own build_command and test_command, or remove marker files for runtimes you don't use.",
				detected,
			),
			run: func() bool { return false },
		})
	}

	if opts.ComposeConfigured {
		checks = append(checks, &Check{
			Name: "Docker socket accessible",
			Fix:  "/var/run/docker.sock is not accessible. Ensure the Docker daemon is running on the host and the socket is mounted into the container (e.g. add `-v /var/run/docker.sock:/var/run/docker.sock` to your docker run flags).",
			run: func() bool {
				return SocketStat("/var/run/docker.sock") == nil
			},
		})
		checks = append(checks, &Check{
			Name: "docker compose CLI available",
			Fix:  "The `docker compose` plugin is not installed. Install Docker Desktop or the compose plugin manually: https://docs.docker.com/compose/install/",
			run: func() bool {
				_, err := CommandRunner("docker", "compose", "version")
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
