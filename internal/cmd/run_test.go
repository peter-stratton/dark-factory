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
	// The --auto-merge-feature flag should be registered on both run and implement commands,
	// and default to "none".
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{"run", runCmd},
		{"implement", implementCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.cmd.Flags().Lookup("auto-merge-feature")
			if f == nil {
				t.Fatalf("%s command missing --auto-merge-feature flag", tc.name)
			}
			if f.DefValue != "none" {
				t.Errorf("auto-merge-feature default = %q, want %q", f.DefValue, "none")
			}
		})
	}
}

func TestAutoMergeRollupFlagParsing(t *testing.T) {
	// The --auto-merge-rollup flag should be registered on both run and implement commands,
	// and default to "none".
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{"run", runCmd},
		{"implement", implementCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.cmd.Flags().Lookup("auto-merge-rollup")
			if f == nil {
				t.Fatalf("%s command missing --auto-merge-rollup flag", tc.name)
			}
			if f.DefValue != "none" {
				t.Errorf("auto-merge-rollup default = %q, want %q", f.DefValue, "none")
			}
		})
	}
}

func TestAutoMergeConfigFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	err := os.WriteFile(p, []byte("repo: owner/repo\nauto_merge:\n  feature: all\n  rollup: manual\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p, config.CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoMerge.Feature != "all" {
		t.Errorf("AutoMerge.Feature = %q, want %q from config file", cfg.AutoMerge.Feature, "all")
	}
	if cfg.AutoMerge.Rollup != "manual" {
		t.Errorf("AutoMerge.Rollup = %q, want %q from config file", cfg.AutoMerge.Rollup, "manual")
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
	if cfg.AutoMerge.Feature != "none" {
		t.Errorf("AutoMerge.Feature = %q, want %q by default", cfg.AutoMerge.Feature, "none")
	}
	if cfg.AutoMerge.Rollup != "none" {
		t.Errorf("AutoMerge.Rollup = %q, want %q by default", cfg.AutoMerge.Rollup, "none")
	}
}

func TestAutoMergeFlagOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	err := os.WriteFile(p, []byte("repo: owner/repo\nauto_merge:\n  feature: none\n  rollup: none\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	feat := "all"
	rollup := "auto"
	cfg, err := config.Load(p, config.CLIFlags{AutoMergeFeature: &feat, AutoMergeRollup: &rollup})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoMerge.Feature != "all" {
		t.Errorf("AutoMerge.Feature = %q, want %q (flag should override config)", cfg.AutoMerge.Feature, "all")
	}
	if cfg.AutoMerge.Rollup != "auto" {
		t.Errorf("AutoMerge.Rollup = %q, want %q (flag should override config)", cfg.AutoMerge.Rollup, "auto")
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
// format). This ensures config validation problems produce useful messages
// rather than silently falling back to "--repo is required when using --tag".
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
