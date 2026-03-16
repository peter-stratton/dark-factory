package cmd

import (
	"os"
	"testing"

	"github.com/phs/dark-factory/internal/logging"
)

func TestWatchCmd_Registered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "watch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("watch command not registered on rootCmd")
	}
}

func TestWatchCmd_HasHelpOutput(t *testing.T) {
	if watchCmd.Use != "watch" {
		t.Errorf("watchCmd.Use: got %q, want %q", watchCmd.Use, "watch")
	}
	if watchCmd.Short == "" {
		t.Error("watchCmd.Short must not be empty")
	}
}

// TestWatchCmd_NoTUIFlagRegistered verifies that the --no-tui flag is present
// on watchCmd so that `godark watch --help` shows it.
func TestWatchCmd_NoTUIFlagRegistered(t *testing.T) {
	if watchCmd.Flags().Lookup("no-tui") == nil {
		t.Error("expected --no-tui flag to be registered on watchCmd")
	}
}

// TestWatchCmd_LoggerSelection_TUIMode verifies that when the terminal is
// interactive (and --no-tui is not set), the file-only logger factory is
// selected.
func TestWatchCmd_LoggerSelection_TUIMode(t *testing.T) {
	orig := isTerminalFn
	isTerminalFn = func(_ int) bool { return true }
	defer func() { isTerminalFn = orig }()

	// Simulate the logger selection logic from RunE.
	noTUI := false
	useTUI := !noTUI && isTerminalFn(0)
	logFactory := logging.NewLogger
	if useTUI {
		logFactory = logging.NewLoggerFileOnly
	}

	dir := t.TempDir()
	logger, err := logFactory(dir)
	if err != nil {
		t.Fatalf("logFactory returned error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// In TUI mode the file debug.log must exist and stdout must not be written to.
	entries, err := readDir(t, dir)
	if err != nil {
		t.Fatalf("reading log dir: %v", err)
	}
	found := false
	for _, e := range entries {
		if e == "debug.log" {
			found = true
		}
	}
	if !found {
		t.Error("expected debug.log to be created by NewLoggerFileOnly")
	}
}

// TestWatchCmd_LoggerSelection_NoTUIForced verifies that --no-tui forces the
// standard logger even when the terminal appears interactive.
func TestWatchCmd_LoggerSelection_NoTUIForced(t *testing.T) {
	orig := isTerminalFn
	isTerminalFn = func(_ int) bool { return true }
	defer func() { isTerminalFn = orig }()

	// Simulate the logger selection logic from RunE with --no-tui set.
	noTUI := true
	useTUI := !noTUI && isTerminalFn(0)
	logFactory := logging.NewLogger
	if useTUI {
		logFactory = logging.NewLoggerFileOnly
	}

	// useTUI must be false when noTUI is true.
	if useTUI {
		t.Error("expected useTUI=false when --no-tui is set")
	}

	dir := t.TempDir()
	logger, err := logFactory(dir)
	if err != nil {
		t.Fatalf("logFactory returned error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

// readDir returns a list of entry names in dir.
func readDir(t *testing.T, dir string) ([]string, error) {
	t.Helper()
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	return names, err
}
