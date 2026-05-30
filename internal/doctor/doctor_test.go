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

func TestRun_AllPass(t *testing.T) {
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

func TestChecks_NoGolangciLintCheck(t *testing.T) {
	// golangci-lint check is never added (host toolchain checks removed).
	checks := Checks(Opts{LintCommand: "golangci-lint run ./..."})
	for _, c := range checks {
		if c.Name == "golangci-lint installed" {
			t.Error("unexpected golangci-lint check")
		}
	}
}

func TestChecks_DockerAlwaysIncluded(t *testing.T) {
	// Docker check must appear regardless of opts.
	checks := Checks(Opts{})
	found := false
	for _, c := range checks {
		if c.Name == "Docker daemon running" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Docker daemon running' check to always be present")
	}
}

func TestChecks_NoHostToolchainChecks(t *testing.T) {
	// Runtime toolchain and Python 3 checks must never appear.
	checks := Checks(Opts{Runtime: "go"})
	for _, c := range checks {
		if strings.Contains(c.Name, "toolchain") {
			t.Errorf("unexpected toolchain check: %s", c.Name)
		}
		if c.Name == "Python 3 available" {
			t.Error("unexpected Python 3 check")
		}
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

// --- Runtime mismatch and multi-runtime checks ---

func TestChecks_RuntimeMatch_NoExtraCheck(t *testing.T) {
	checks := Checks(Opts{Runtime: "go", ConfiguredRuntime: "go"})
	for _, c := range checks {
		if strings.Contains(c.Name, "runtime matches") {
			t.Errorf("unexpected runtime mismatch check when runtimes match: %s", c.Name)
		}
	}
}

func TestChecks_RuntimeMismatch_AddsFailingCheck(t *testing.T) {
	checks := Checks(Opts{Runtime: "go", ConfiguredRuntime: "python"})
	var found *Check
	for _, c := range checks {
		if c.Name == "Configured runtime matches repo files" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected runtime mismatch check to be added")
	}
	if found.run() {
		t.Error("expected runtime mismatch check to fail")
	}
}

func TestChecks_RuntimeMismatch_FixMentionsBoth(t *testing.T) {
	checks := Checks(Opts{Runtime: "go", ConfiguredRuntime: "python"})
	for _, c := range checks {
		if c.Name != "Configured runtime matches repo files" {
			continue
		}
		if !strings.Contains(c.Fix, "python") {
			t.Errorf("Fix should mention configured runtime 'python': %s", c.Fix)
		}
		if !strings.Contains(c.Fix, "go") {
			t.Errorf("Fix should mention detected runtime 'go': %s", c.Fix)
		}
		return
	}
	t.Fatal("expected runtime mismatch check to be present")
}

func TestChecks_RuntimeMismatch_SkippedWhenConfiguredEmpty(t *testing.T) {
	// When godark.yaml has no runtime.name, we can't compare. Skip the check.
	checks := Checks(Opts{Runtime: "go", ConfiguredRuntime: ""})
	for _, c := range checks {
		if c.Name == "Configured runtime matches repo files" {
			t.Error("expected mismatch check to be skipped when ConfiguredRuntime is empty")
		}
	}
}

func TestChecks_RuntimeMismatch_SkippedWhenDetectedEmpty(t *testing.T) {
	// When no marker files are found, we can't tell what the project is. Skip.
	checks := Checks(Opts{Runtime: "", ConfiguredRuntime: "go"})
	for _, c := range checks {
		if c.Name == "Configured runtime matches repo files" {
			t.Error("expected mismatch check to be skipped when Runtime is empty")
		}
	}
}

func TestChecks_SingleRuntime_NoMultiRuntimeCheck(t *testing.T) {
	checks := Checks(Opts{DetectedRuntimes: []string{"go"}})
	for _, c := range checks {
		if c.Name == "Multi-runtime repo has modules: configured" {
			t.Error("expected multi-runtime check to be skipped for single runtime")
		}
	}
}

func TestChecks_MultiRuntime_NoModules_AddsFailingCheck(t *testing.T) {
	checks := Checks(Opts{
		DetectedRuntimes:  []string{"go", "python"},
		ModulesConfigured: false,
	})
	var found *Check
	for _, c := range checks {
		if c.Name == "Multi-runtime repo has modules: configured" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected multi-runtime check to be added")
	}
	if found.run() {
		t.Error("expected multi-runtime check to fail when modules: is missing")
	}
	if !strings.Contains(found.Fix, "go") || !strings.Contains(found.Fix, "python") {
		t.Errorf("Fix should mention both detected runtimes: %s", found.Fix)
	}
}

func TestChecks_MultiRuntime_WithModules_NoCheck(t *testing.T) {
	checks := Checks(Opts{
		DetectedRuntimes:  []string{"go", "python"},
		ModulesConfigured: true,
	})
	for _, c := range checks {
		if c.Name == "Multi-runtime repo has modules: configured" {
			t.Error("expected multi-runtime check to be skipped when ModulesConfigured is true")
		}
	}
}

func TestChecks_GoRuntimeWithoutVersion_AddsFailingCheck(t *testing.T) {
	checks := Checks(Opts{ConfiguredRuntime: "go", ConfiguredRuntimeVersion: ""})
	var found *Check
	for _, c := range checks {
		if c.Name == "Go runtime has a version" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected 'Go runtime has a version' check to be added")
	}
	if found.run() {
		t.Error("expected the check to fail when version is empty")
	}
	if !strings.Contains(found.Fix, "runtime.version") {
		t.Errorf("Fix should mention runtime.version: %s", found.Fix)
	}
}

func TestChecks_GoRuntimeWithVersion_NoCheck(t *testing.T) {
	checks := Checks(Opts{ConfiguredRuntime: "go", ConfiguredRuntimeVersion: "1.22"})
	for _, c := range checks {
		if c.Name == "Go runtime has a version" {
			t.Error("expected 'Go runtime has a version' check to be skipped when version is set")
		}
	}
}

func TestChecks_NonGoRuntime_NoVersionCheck(t *testing.T) {
	// Only runtime.name=go triggers the version requirement; other runtimes
	// either default the version (flutter) or have no validation (node, rust,
	// python).
	for _, name := range []string{"python", "node", "rust", "flutter"} {
		t.Run(name, func(t *testing.T) {
			checks := Checks(Opts{ConfiguredRuntime: name, ConfiguredRuntimeVersion: ""})
			for _, c := range checks {
				if c.Name == "Go runtime has a version" {
					t.Errorf("expected version check to be skipped for runtime=%q", name)
				}
			}
		})
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
