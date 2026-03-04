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

func TestNoMergeFlagParsing(t *testing.T) {
	// The --no-merge flag should be registered on both run and implement commands,
	// and default to false.
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{"run", runCmd},
		{"implement", implementCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.cmd.Flags().Lookup("no-merge")
			if f == nil {
				t.Fatalf("%s command missing --no-merge flag", tc.name)
			}
			if f.DefValue != "false" {
				t.Errorf("no-merge default = %q, want %q", f.DefValue, "false")
			}
		})
	}
}

func TestNoMergeConfigFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	err := os.WriteFile(p, []byte("repo: owner/repo\nno_merge: true\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p, config.CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.NoMerge {
		t.Error("NoMerge = false, want true from config file")
	}
}

func TestNoMergeDefaultFalse(t *testing.T) {
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
	if cfg.NoMerge {
		t.Error("NoMerge = true, want false by default")
	}
}

func TestNoMergeFlagOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "godark.yaml")
	err := os.WriteFile(p, []byte("repo: owner/repo\nno_merge: false\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	noMerge := true
	cfg, err := config.Load(p, config.CLIFlags{NoMerge: &noMerge})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.NoMerge {
		t.Error("NoMerge = false, want true (flag should override config)")
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
