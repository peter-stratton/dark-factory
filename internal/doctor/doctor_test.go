package doctor

import (
	"bytes"
	"errors"
	"strings"
	"testing"
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
	checks := Checks("go")
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
	checks := Checks("") // no runtime check
	passed := Run(&buf, checks)

	if passed {
		t.Error("expected Run to return false when checks fail")
	}
	out := buf.String()
	if !strings.Contains(out, "[FAIL] Docker daemon running") {
		t.Errorf("expected Docker failure in output:\n%s", out)
	}
	if !strings.Contains(out, "[FAIL] ANTHROPIC_API_KEY set") {
		t.Errorf("expected API key failure in output:\n%s", out)
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
	checks := Checks("go")
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
		checks := Checks(rt)
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
	checks := Checks("")
	// Without a runtime: 5 checks (4 base + python3).
	if len(checks) != 5 {
		t.Errorf("expected 5 checks without runtime, got %d", len(checks))
	}
}

func TestChecks_UnknownRuntime(t *testing.T) {
	checks := Checks("cobol")
	// Unknown runtime falls back to 5 checks (no toolchain check added).
	if len(checks) != 5 {
		t.Errorf("expected 5 checks for unknown runtime, got %d", len(checks))
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
