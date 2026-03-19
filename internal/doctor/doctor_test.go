package doctor

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// stubRunner returns a CommandRunner stub that succeeds for the listed
// commands (identified by name+args[0] if any) and fails for all others.
func stubRunner(pass ...string) func(string, ...string) ([]byte, error) {
	passSet := make(map[string]bool, len(pass))
	for _, p := range pass {
		passSet[p] = true
	}
	return func(name string, args ...string) ([]byte, error) {
		key := name
		if len(args) > 0 {
			key = name + " " + args[0]
		}
		if passSet[key] {
			return []byte("ok"), nil
		}
		return nil, errors.New("command failed")
	}
}

func apiKeyEnv(key string) string {
	if key == "ANTHROPIC_API_KEY" {
		return "sk-test"
	}
	return ""
}

func oauthEnv(key string) string {
	if key == "CLAUDE_CODE_OAUTH_TOKEN" {
		return "oauth-test"
	}
	return ""
}

func noEnv(_ string) string { return "" }

// --- Sandbox mode (default) ---

func TestRun_Sandbox_AllPass(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() { CommandRunner = orig; EnvLookup = origEnv }()

	CommandRunner = stubRunner("docker info", "gh --version", "gh auth")
	EnvLookup = apiKeyEnv

	var buf bytes.Buffer
	checks := Checks(Opts{})
	passed := Run(&buf, checks)

	if !passed {
		t.Errorf("expected all checks to pass, output:\n%s", buf.String())
	}
	if c := strings.Count(buf.String(), "[PASS]"); c != 4 {
		t.Errorf("expected 4 PASS lines, got %d:\n%s", c, buf.String())
	}
}

func TestRun_Sandbox_OAuthToken(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() { CommandRunner = orig; EnvLookup = origEnv }()

	CommandRunner = stubRunner("docker info", "gh --version", "gh auth")
	EnvLookup = oauthEnv

	var buf bytes.Buffer
	checks := Checks(Opts{})
	passed := Run(&buf, checks)

	if !passed {
		t.Errorf("expected all checks to pass with OAuth token, output:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "[PASS] Anthropic auth token set") {
		t.Errorf("expected auth token PASS in output:\n%s", buf.String())
	}
}

func TestRun_Sandbox_SomeFail(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() { CommandRunner = orig; EnvLookup = origEnv }()

	CommandRunner = stubRunner("gh --version", "gh auth")
	EnvLookup = noEnv

	var buf bytes.Buffer
	checks := Checks(Opts{})
	passed := Run(&buf, checks)

	if passed {
		t.Error("expected Run to return false when checks fail")
	}
	out := buf.String()
	if !strings.Contains(out, "[FAIL] Docker daemon running") {
		t.Errorf("expected Docker failure in output:\n%s", out)
	}
	if !strings.Contains(out, "[FAIL] Anthropic auth token set") {
		t.Errorf("expected auth token failure in output:\n%s", out)
	}
	if !strings.Contains(out, "Fix:") {
		t.Errorf("expected Fix hint in output:\n%s", out)
	}
}

func TestChecks_Sandbox_NoRuntimeOrPython(t *testing.T) {
	// In sandbox mode, runtime and Python checks should NOT appear even
	// when a runtime is detected.
	checks := Checks(Opts{Runtime: "go"})
	for _, c := range checks {
		if strings.Contains(c.Name, "toolchain") {
			t.Errorf("unexpected toolchain check in sandbox mode: %s", c.Name)
		}
		if c.Name == "Python 3 available" {
			t.Error("unexpected Python 3 check in sandbox mode")
		}
	}
	// 4 base checks: docker, gh install, gh auth, auth token.
	if len(checks) != 4 {
		t.Errorf("expected 4 checks in sandbox mode, got %d", len(checks))
	}
}

func TestChecks_Sandbox_NoGolangciLint(t *testing.T) {
	checks := Checks(Opts{LintCommand: "golangci-lint run ./..."})
	for _, c := range checks {
		if c.Name == "golangci-lint installed" {
			t.Error("unexpected golangci-lint check in sandbox mode")
		}
	}
}

// --- No-sandbox mode ---

func TestRun_NoSandbox_AllPass(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() { CommandRunner = orig; EnvLookup = origEnv }()

	CommandRunner = stubRunner("gh --version", "gh auth", "go version", "python3 --version")
	EnvLookup = apiKeyEnv

	var buf bytes.Buffer
	checks := Checks(Opts{Runtime: "go", NoSandbox: true})
	passed := Run(&buf, checks)

	if !passed {
		t.Errorf("expected all checks to pass, output:\n%s", buf.String())
	}
	// 3 base (no docker) + runtime + python = 5
	if c := strings.Count(buf.String(), "[PASS]"); c != 5 {
		t.Errorf("expected 5 PASS lines, got %d:\n%s", c, buf.String())
	}
}

func TestChecks_NoSandbox_NoDocker(t *testing.T) {
	checks := Checks(Opts{NoSandbox: true})
	for _, c := range checks {
		if c.Name == "Docker daemon running" {
			t.Error("unexpected Docker check in no-sandbox mode")
		}
	}
}

func TestChecks_NoSandbox_WithRuntime(t *testing.T) {
	runtimes := []string{"go", "flutter", "node", "rust", "elixir", "python"}
	for _, rt := range runtimes {
		checks := Checks(Opts{Runtime: rt, NoSandbox: true})
		found := false
		for _, c := range checks {
			if strings.Contains(c.Name, rt+" toolchain") {
				found = true
			}
		}
		if !found {
			t.Errorf("runtime=%s: no toolchain check found in no-sandbox mode", rt)
		}
	}
}

func TestChecks_NoSandbox_NoRuntime(t *testing.T) {
	checks := Checks(Opts{NoSandbox: true})
	// 3 base (no docker) + python = 4
	if len(checks) != 4 {
		t.Errorf("expected 4 checks without runtime in no-sandbox mode, got %d", len(checks))
	}
}

func TestChecks_NoSandbox_UnknownRuntime(t *testing.T) {
	checks := Checks(Opts{Runtime: "cobol", NoSandbox: true})
	// Unknown runtime: 3 base + python = 4 (no toolchain check added).
	if len(checks) != 4 {
		t.Errorf("expected 4 checks for unknown runtime in no-sandbox mode, got %d", len(checks))
	}
}

func TestChecks_NoSandbox_GolangciLint(t *testing.T) {
	checks := Checks(Opts{LintCommand: "golangci-lint run ./...", NoSandbox: true})
	found := false
	for _, c := range checks {
		if c.Name == "golangci-lint installed" {
			found = true
		}
	}
	if !found {
		t.Error("expected golangci-lint check in no-sandbox mode")
	}
}

func TestChecks_NoSandbox_NoGolangciLint(t *testing.T) {
	checks := Checks(Opts{LintCommand: "staticcheck ./...", NoSandbox: true})
	for _, c := range checks {
		if c.Name == "golangci-lint installed" {
			t.Error("unexpected golangci-lint check")
		}
	}
}

func TestRun_NoSandbox_GolangciLint_Pass(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() { CommandRunner = orig; EnvLookup = origEnv }()

	CommandRunner = stubRunner("gh --version", "gh auth", "python3 --version", "golangci-lint --version")
	EnvLookup = apiKeyEnv

	var buf bytes.Buffer
	checks := Checks(Opts{LintCommand: "golangci-lint run ./...", NoSandbox: true})
	passed := Run(&buf, checks)

	if !passed {
		t.Errorf("expected all checks to pass, output:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "[PASS] golangci-lint installed") {
		t.Errorf("expected golangci-lint PASS:\n%s", buf.String())
	}
}

func TestRun_NoSandbox_GolangciLint_Fail(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() { CommandRunner = orig; EnvLookup = origEnv }()

	CommandRunner = stubRunner("gh --version", "gh auth", "python3 --version")
	EnvLookup = apiKeyEnv

	var buf bytes.Buffer
	checks := Checks(Opts{LintCommand: "golangci-lint run ./...", NoSandbox: true})
	passed := Run(&buf, checks)

	if passed {
		t.Error("expected Run to return false when golangci-lint is missing")
	}
	out := buf.String()
	if !strings.Contains(out, "[FAIL] golangci-lint installed") {
		t.Errorf("expected golangci-lint FAIL:\n%s", out)
	}
	if !strings.Contains(out, "brew install golangci-lint") {
		t.Errorf("expected brew install hint:\n%s", out)
	}
}

func TestRun_NoSandbox_NoShortCircuit(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() { CommandRunner = orig; EnvLookup = origEnv }()

	CommandRunner = stubRunner()
	EnvLookup = noEnv

	var buf bytes.Buffer
	checks := Checks(Opts{Runtime: "go", NoSandbox: true})
	passed := Run(&buf, checks)

	if passed {
		t.Error("expected Run to return false")
	}
	// 3 base + runtime + python = 5 checks, all should appear.
	if c := strings.Count(buf.String(), "[FAIL]"); c != 5 {
		t.Errorf("expected 5 FAIL lines, got %d:\n%s", c, buf.String())
	}
}

// --- Compose checks ---

func TestChecks_ComposeConfigured_AddsTwo(t *testing.T) {
	checksWithout := Checks(Opts{})
	checksWith := Checks(Opts{ComposeConfigured: true})
	if len(checksWith) != len(checksWithout)+2 {
		t.Errorf("expected 2 extra checks when ComposeConfigured=true, got %d without and %d with",
			len(checksWithout), len(checksWith))
	}
	foundSocket := false
	foundCompose := false
	for _, c := range checksWith {
		if c.Name == "Docker socket accessible" {
			foundSocket = true
		}
		if c.Name == "docker compose CLI available" {
			foundCompose = true
		}
	}
	if !foundSocket {
		t.Error("expected 'Docker socket accessible' check")
	}
	if !foundCompose {
		t.Error("expected 'docker compose CLI available' check")
	}
}

func TestChecks_NoCompose_SkipsChecks(t *testing.T) {
	checks := Checks(Opts{})
	for _, c := range checks {
		if c.Name == "Docker socket accessible" {
			t.Error("unexpected Docker socket check")
		}
		if c.Name == "docker compose CLI available" {
			t.Error("unexpected compose CLI check")
		}
	}
}

func TestRun_Compose_SocketExists(t *testing.T) {
	orig := CommandRunner
	origStat := SocketStat
	origEnv := EnvLookup
	defer func() { CommandRunner = orig; SocketStat = origStat; EnvLookup = origEnv }()

	CommandRunner = stubRunner("docker info", "gh --version", "gh auth", "docker compose")
	SocketStat = func(path string) error { return nil }
	EnvLookup = apiKeyEnv

	var buf bytes.Buffer
	checks := Checks(Opts{ComposeConfigured: true})
	passed := Run(&buf, checks)

	if !passed {
		t.Errorf("expected all checks to pass, output:\n%s", buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "[PASS] Docker socket accessible") {
		t.Errorf("expected socket PASS:\n%s", out)
	}
	if !strings.Contains(out, "[PASS] docker compose CLI available") {
		t.Errorf("expected compose CLI PASS:\n%s", out)
	}
}

func TestRun_Compose_SocketMissing(t *testing.T) {
	orig := CommandRunner
	origStat := SocketStat
	origEnv := EnvLookup
	defer func() { CommandRunner = orig; SocketStat = origStat; EnvLookup = origEnv }()

	CommandRunner = stubRunner("docker info", "gh --version", "gh auth", "docker compose")
	SocketStat = func(path string) error { return os.ErrNotExist }
	EnvLookup = apiKeyEnv

	var buf bytes.Buffer
	checks := Checks(Opts{ComposeConfigured: true})
	passed := Run(&buf, checks)

	if passed {
		t.Error("expected Run to return false when socket is missing")
	}
	out := buf.String()
	if !strings.Contains(out, "[FAIL] Docker socket accessible") {
		t.Errorf("expected socket FAIL:\n%s", out)
	}
}

func TestRun_Compose_CLIMissing(t *testing.T) {
	orig := CommandRunner
	origStat := SocketStat
	origEnv := EnvLookup
	defer func() { CommandRunner = orig; SocketStat = origStat; EnvLookup = origEnv }()

	CommandRunner = stubRunner("docker info", "gh --version", "gh auth")
	SocketStat = func(path string) error { return nil }
	EnvLookup = apiKeyEnv

	var buf bytes.Buffer
	checks := Checks(Opts{ComposeConfigured: true})
	passed := Run(&buf, checks)

	if passed {
		t.Error("expected Run to return false when compose CLI is missing")
	}
	if !strings.Contains(buf.String(), "[FAIL] docker compose CLI available") {
		t.Errorf("expected compose CLI FAIL:\n%s", buf.String())
	}
}

// --- Helpers ---

func TestRuntimeVersionCmd(t *testing.T) {
	cases := []struct {
		runtime string
		wantBin string
	}{
		{"go", "go"},
		{"flutter", "flutter"},
		{"node", "node"},
		{"rust", "rustc"},
		{"elixir", "elixir"},
		{"python", "python3"},
		{"unknown", ""},
	}
	for _, tc := range cases {
		bin, _ := runtimeVersionCmd(tc.runtime)
		if bin != tc.wantBin {
			t.Errorf("runtimeVersionCmd(%q) bin = %q, want %q", tc.runtime, bin, tc.wantBin)
		}
	}
}

func TestRun_Timeout(t *testing.T) {
	origTimeout := CheckTimeout
	defer func() { CheckTimeout = origTimeout }()
	CheckTimeout = 50 * time.Millisecond

	block := make(chan struct{})
	checks := []*Check{
		{
			Name: "blocking check",
			Fix:  "unblock it",
			run: func() bool {
				<-block
				return true
			},
		},
	}

	var buf bytes.Buffer
	passed := Run(&buf, checks)

	close(block)

	if passed {
		t.Error("expected Run to return false on timeout")
	}
	if !strings.Contains(buf.String(), "[FAIL] blocking check") {
		t.Errorf("expected blocking check FAIL:\n%s", buf.String())
	}
}
