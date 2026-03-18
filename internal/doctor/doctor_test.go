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

func TestRun_AllPass(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() {
		CommandRunner = orig
		EnvLookup = origEnv
	}()

	CommandRunner = stubRunner(
		"docker info",
		"gh --version",
		"gh auth",
		"go version",
		"python3 --version",
	)
	EnvLookup = func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-test"
		}
		return ""
	}

	var buf bytes.Buffer
	checks := Checks("go", "", false)
	passed := Run(&buf, checks)

	if !passed {
		t.Errorf("expected all checks to pass, output:\n%s", buf.String())
	}
	out := buf.String()
	if strings.Contains(out, "[FAIL]") {
		t.Errorf("unexpected FAIL in output:\n%s", out)
	}
	// All 6 checks (docker, gh install, gh auth, api key, go toolchain, python3)
	if c := strings.Count(out, "[PASS]"); c != 6 {
		t.Errorf("expected 6 PASS lines, got %d:\n%s", c, out)
	}
}

func TestRun_AllPass_OAuthToken(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() {
		CommandRunner = orig
		EnvLookup = origEnv
	}()

	CommandRunner = stubRunner(
		"docker info",
		"gh --version",
		"gh auth",
		"go version",
		"python3 --version",
	)
	EnvLookup = func(key string) string {
		if key == "CLAUDE_CODE_OAUTH_TOKEN" {
			return "oauth-test"
		}
		return ""
	}

	var buf bytes.Buffer
	checks := Checks("go", "", false)
	passed := Run(&buf, checks)

	if !passed {
		t.Errorf("expected all checks to pass with OAuth token, output:\n%s", buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "[PASS] Anthropic auth token set") {
		t.Errorf("expected auth token PASS in output:\n%s", out)
	}
}

func TestRun_SomeFail(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() {
		CommandRunner = orig
		EnvLookup = origEnv
	}()

	// Docker fails, gh fails, python3 passes.
	CommandRunner = stubRunner(
		"gh --version",
		"gh auth",
		"python3 --version",
	)
	EnvLookup = func(key string) string { return "" } // API key missing

	var buf bytes.Buffer
	checks := Checks("", "", false) // no runtime check, no lint command
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
	if !strings.Contains(out, "[PASS] gh CLI installed") {
		t.Errorf("expected gh CLI pass in output:\n%s", out)
	}
	if !strings.Contains(out, "Fix:") {
		t.Errorf("expected Fix hint in output:\n%s", out)
	}
}

func TestRun_NoShortCircuit(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() {
		CommandRunner = orig
		EnvLookup = origEnv
	}()

	// Everything fails.
	CommandRunner = stubRunner()
	EnvLookup = func(key string) string { return "" }

	var buf bytes.Buffer
	checks := Checks("go", "", false)
	passed := Run(&buf, checks)

	if passed {
		t.Error("expected Run to return false")
	}
	out := buf.String()
	// All 6 checks should still appear even if the first one fails.
	if c := strings.Count(out, "[FAIL]"); c != 6 {
		t.Errorf("expected 6 FAIL lines (no short-circuit), got %d:\n%s", c, out)
	}
}

func TestChecks_WithRuntime(t *testing.T) {
	runtimes := []string{"go", "flutter", "node", "rust", "elixir", "python"}
	for _, rt := range runtimes {
		checks := Checks(rt, "", false)
		// With a runtime we expect 6 checks (4 base + runtime toolchain + python3).
		if len(checks) != 6 {
			t.Errorf("runtime=%s: expected 6 checks, got %d", rt, len(checks))
		}
		// Verify the runtime check name contains the runtime name.
		found := false
		for _, c := range checks {
			if strings.Contains(c.Name, rt+" toolchain") {
				found = true
			}
		}
		if !found {
			t.Errorf("runtime=%s: no toolchain check found in check names", rt)
		}
	}
}

func TestChecks_NoRuntime(t *testing.T) {
	checks := Checks("", "", false)
	// Without a runtime: 5 checks (4 base + python3).
	if len(checks) != 5 {
		t.Errorf("expected 5 checks without runtime, got %d", len(checks))
	}
}

func TestChecks_UnknownRuntime(t *testing.T) {
	checks := Checks("cobol", "", false)
	// Unknown runtime falls back to 5 checks (no toolchain check added).
	if len(checks) != 5 {
		t.Errorf("expected 5 checks for unknown runtime, got %d", len(checks))
	}
}

func TestChecks_GolangciLint_Included(t *testing.T) {
	checks := Checks("", "golangci-lint run ./...", false)
	// With golangci-lint in lint_command: 6 checks (5 base + golangci-lint).
	if len(checks) != 6 {
		t.Errorf("expected 6 checks with golangci-lint lint_command, got %d", len(checks))
	}
	found := false
	for _, c := range checks {
		if c.Name == "golangci-lint installed" {
			found = true
		}
	}
	if !found {
		t.Error("expected golangci-lint check to be included")
	}
}

func TestChecks_GolangciLint_NotIncluded(t *testing.T) {
	checks := Checks("", "staticcheck ./...", false)
	// Without golangci-lint in lint_command: 5 checks (no golangci-lint check).
	if len(checks) != 5 {
		t.Errorf("expected 5 checks without golangci-lint lint_command, got %d", len(checks))
	}
}

func TestRun_GolangciLint_Pass(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() {
		CommandRunner = orig
		EnvLookup = origEnv
	}()

	CommandRunner = stubRunner(
		"docker info",
		"gh --version",
		"gh auth",
		"python3 --version",
		"golangci-lint --version",
	)
	EnvLookup = func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-test"
		}
		return ""
	}

	var buf bytes.Buffer
	checks := Checks("", "golangci-lint run ./...", false)
	passed := Run(&buf, checks)

	if !passed {
		t.Errorf("expected all checks to pass, output:\n%s", buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "[PASS] golangci-lint installed") {
		t.Errorf("expected golangci-lint PASS in output:\n%s", out)
	}
}

func TestRun_GolangciLint_Fail(t *testing.T) {
	orig := CommandRunner
	origEnv := EnvLookup
	defer func() {
		CommandRunner = orig
		EnvLookup = origEnv
	}()

	// golangci-lint is not in the pass set — it will fail.
	CommandRunner = stubRunner(
		"docker info",
		"gh --version",
		"gh auth",
		"python3 --version",
	)
	EnvLookup = func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-test"
		}
		return ""
	}

	var buf bytes.Buffer
	checks := Checks("", "golangci-lint run ./...", false)
	passed := Run(&buf, checks)

	if passed {
		t.Error("expected Run to return false when golangci-lint is missing")
	}
	out := buf.String()
	if !strings.Contains(out, "[FAIL] golangci-lint installed") {
		t.Errorf("expected golangci-lint FAIL in output:\n%s", out)
	}
	if !strings.Contains(out, "brew install golangci-lint") {
		t.Errorf("expected brew install hint in fix message:\n%s", out)
	}
}

func TestChecks_ComposeConfigured_AddsTwo(t *testing.T) {
	checksWithout := Checks("", "", false)
	checksWith := Checks("", "", true)
	if len(checksWith) != len(checksWithout)+2 {
		t.Errorf("expected 2 extra checks when composeConfigured=true, got %d without and %d with",
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
		t.Error("expected 'Docker socket accessible' check when composeConfigured=true")
	}
	if !foundCompose {
		t.Error("expected 'docker compose CLI available' check when composeConfigured=true")
	}
}

func TestChecks_NoCompose_SkipsChecks(t *testing.T) {
	checks := Checks("", "", false)
	for _, c := range checks {
		if c.Name == "Docker socket accessible" {
			t.Error("expected 'Docker socket accessible' to be absent when composeConfigured=false")
		}
		if c.Name == "docker compose CLI available" {
			t.Error("expected 'docker compose CLI available' to be absent when composeConfigured=false")
		}
	}
}

func TestRun_Compose_SocketExists(t *testing.T) {
	orig := CommandRunner
	origStat := SocketStat
	origEnv := EnvLookup
	defer func() {
		CommandRunner = orig
		SocketStat = origStat
		EnvLookup = origEnv
	}()

	CommandRunner = stubRunner(
		"docker info",
		"gh --version",
		"gh auth",
		"python3 --version",
		"docker compose",
	)
	SocketStat = func(path string) error { return nil }
	EnvLookup = func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-test"
		}
		return ""
	}

	var buf bytes.Buffer
	checks := Checks("", "", true)
	passed := Run(&buf, checks)

	if !passed {
		t.Errorf("expected all checks to pass, output:\n%s", buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "[PASS] Docker socket accessible") {
		t.Errorf("expected socket PASS in output:\n%s", out)
	}
	if !strings.Contains(out, "[PASS] docker compose CLI available") {
		t.Errorf("expected compose CLI PASS in output:\n%s", out)
	}
}

func TestRun_Compose_SocketMissing(t *testing.T) {
	orig := CommandRunner
	origStat := SocketStat
	origEnv := EnvLookup
	defer func() {
		CommandRunner = orig
		SocketStat = origStat
		EnvLookup = origEnv
	}()

	CommandRunner = stubRunner(
		"docker info",
		"gh --version",
		"gh auth",
		"python3 --version",
		"docker compose",
	)
	SocketStat = func(path string) error { return os.ErrNotExist }
	EnvLookup = func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-test"
		}
		return ""
	}

	var buf bytes.Buffer
	checks := Checks("", "", true)
	passed := Run(&buf, checks)

	if passed {
		t.Error("expected Run to return false when socket is missing")
	}
	out := buf.String()
	if !strings.Contains(out, "[FAIL] Docker socket accessible") {
		t.Errorf("expected socket FAIL in output:\n%s", out)
	}
	if !strings.Contains(out, "/var/run/docker.sock") {
		t.Errorf("expected fix message to mention /var/run/docker.sock:\n%s", out)
	}
}

func TestRun_Compose_CLIMissing(t *testing.T) {
	orig := CommandRunner
	origStat := SocketStat
	origEnv := EnvLookup
	defer func() {
		CommandRunner = orig
		SocketStat = origStat
		EnvLookup = origEnv
	}()

	// docker compose fails, socket passes.
	CommandRunner = stubRunner(
		"docker info",
		"gh --version",
		"gh auth",
		"python3 --version",
	)
	SocketStat = func(path string) error { return nil }
	EnvLookup = func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-test"
		}
		return ""
	}

	var buf bytes.Buffer
	checks := Checks("", "", true)
	passed := Run(&buf, checks)

	if passed {
		t.Error("expected Run to return false when docker compose CLI is missing")
	}
	out := buf.String()
	if !strings.Contains(out, "[FAIL] docker compose CLI available") {
		t.Errorf("expected compose CLI FAIL in output:\n%s", out)
	}
	if !strings.Contains(out, "docker compose") {
		t.Errorf("expected fix message to mention docker compose:\n%s", out)
	}
}

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

	// A check that blocks indefinitely.
	block := make(chan struct{})
	checks := []*Check{
		{
			Name: "blocking check",
			Fix:  "unblock it",
			run: func() bool {
				<-block // never returns
				return true
			},
		},
	}

	var buf bytes.Buffer
	passed := Run(&buf, checks)

	close(block) // allow goroutine to exit after test

	if passed {
		t.Error("expected Run to return false on timeout")
	}
	out := buf.String()
	if !strings.Contains(out, "[FAIL] blocking check") {
		t.Errorf("expected blocking check to be reported as FAIL, got:\n%s", out)
	}
}
