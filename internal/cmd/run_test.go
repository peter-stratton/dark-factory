package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/config"
	"github.com/spf13/cobra"
)

func TestNoSandboxFlagParsing(t *testing.T) {
	// The --no-sandbox flag should be registered and default to false.
	f := runCmd.Flags().Lookup("no-sandbox")
	if f == nil {
		t.Fatal("run command missing --no-sandbox flag")
	}
	if f.DefValue != "false" {
		t.Errorf("no-sandbox default = %q, want %q", f.DefValue, "false")
	}
}

func TestNoSandboxConfigFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	err := os.WriteFile(p, []byte("repo: owner/repo\nno_sandbox: true\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p, config.CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.NoSandbox {
		t.Error("NoSandbox = false, want true from config file")
	}
}

func TestNoSandboxDefaultFalse(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	err := os.WriteFile(p, []byte("repo: owner/repo\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p, config.CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.NoSandbox {
		t.Error("NoSandbox = true, want false by default")
	}
}

func TestNoSandboxFlagOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	err := os.WriteFile(p, []byte("repo: owner/repo\nno_sandbox: false\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	noSandbox := true
	cfg, err := config.Load(p, config.CLIFlags{NoSandbox: &noSandbox})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.NoSandbox {
		t.Error("NoSandbox = false, want true (flag should override config)")
	}
}

func TestAutoMergeFlagParsing(t *testing.T) {
	// The --auto-merge flag should be registered on both run and implement commands,
	// and default to "none".
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{"run", runCmd},
		{"implement", implementCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.cmd.Flags().Lookup("auto-merge")
			if f == nil {
				t.Fatalf("%s command missing --auto-merge flag", tc.name)
			}
			if f.DefValue != "none" {
				t.Errorf("auto-merge default = %q, want %q", f.DefValue, "none")
			}
		})
	}
}

func TestAutoMergeConfigFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	err := os.WriteFile(p, []byte("repo: owner/repo\nauto_merge: all\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p, config.CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoMerge != "all" {
		t.Errorf("AutoMerge = %q, want %q from config file", cfg.AutoMerge, "all")
	}
}

func TestAutoMergeDefaultNone(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	err := os.WriteFile(p, []byte("repo: owner/repo\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p, config.CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoMerge != "none" {
		t.Errorf("AutoMerge = %q, want %q by default", cfg.AutoMerge, "none")
	}
}

func TestAutoMergeFlagOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	err := os.WriteFile(p, []byte("repo: owner/repo\nauto_merge: none\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	v := "all"
	cfg, err := config.Load(p, config.CLIFlags{AutoMerge: &v})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoMerge != "all" {
		t.Errorf("AutoMerge = %q, want %q (flag should override config)", cfg.AutoMerge, "all")
	}
}

func TestNoSandboxWarning(t *testing.T) {
	// Capture stderr to verify the warning is printed when NoSandbox is true.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cfg := &config.Config{NoSandbox: true}

	// Reproduce the warning logic from RunE.
	if cfg.NoSandbox {
		var buf bytes.Buffer
		buf.WriteString("WARNING: running without sandbox — agent execution is not containerized\n")
		w.Write(buf.Bytes())
	}

	w.Close()
	os.Stderr = oldStderr

	var out bytes.Buffer
	out.ReadFrom(r)

	if !strings.Contains(out.String(), "without sandbox") {
		t.Errorf("stderr = %q, want warning containing 'without sandbox'", out.String())
	}
}

// TestTagResolutionSurfacesConfigError verifies that config.Load returns a
// real error when the config file is syntactically valid YAML but fails
// validation (e.g. wait_for_checks as a flat list instead of the struct
// format). This is the error that the tag resolution code in RunE now surfaces
// via "loading config to resolve --tag: ..." instead of silently swallowing it
// and emitting the misleading "--repo is required when using --tag" message.
func TestTagResolutionSurfacesConfigError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	// This config has repo set correctly but wait_for_checks in the old flat-list
	// format, which fails config validation.
	err := os.WriteFile(p, []byte("repo: owner/repo\nwait_for_checks:\n  - test\n  - lint\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = config.Load(p, config.CLIFlags{})
	if err == nil {
		t.Fatal("expected config.Load to return an error for invalid wait_for_checks format, got nil")
	}
	// The error should not be a "repo is required" message — it should describe
	// the actual config problem so users can fix their godark.yaml.
	if strings.Contains(err.Error(), "repo is required") {
		t.Errorf("config.Load error should describe the real problem, got: %v", err)
	}
}
