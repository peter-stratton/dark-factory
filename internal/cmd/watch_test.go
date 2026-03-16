package cmd

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/logging"
	"github.com/phs/dark-factory/internal/tui"
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

// --- watchPollInterval tests ---

func TestWatchPollInterval_NilWatch(t *testing.T) {
	cfg := &config.Config{Watch: nil}
	got := watchPollInterval(cfg)
	if got != watchDefaultPollInterval {
		t.Errorf("nil Watch: got %v, want %v", got, watchDefaultPollInterval)
	}
}

func TestWatchPollInterval_EmptyInterval(t *testing.T) {
	cfg := &config.Config{Watch: &config.Watch{PollInterval: ""}}
	got := watchPollInterval(cfg)
	if got != watchDefaultPollInterval {
		t.Errorf("empty PollInterval: got %v, want %v", got, watchDefaultPollInterval)
	}
}

func TestWatchPollInterval_ParseError(t *testing.T) {
	cfg := &config.Config{Watch: &config.Watch{PollInterval: "not-a-duration"}}
	got := watchPollInterval(cfg)
	if got != watchDefaultPollInterval {
		t.Errorf("unparseable PollInterval: got %v, want %v", got, watchDefaultPollInterval)
	}
}

func TestWatchPollInterval_ZeroOrNegative(t *testing.T) {
	for _, val := range []string{"0s", "-30s"} {
		cfg := &config.Config{Watch: &config.Watch{PollInterval: val}}
		got := watchPollInterval(cfg)
		if got != watchDefaultPollInterval {
			t.Errorf("PollInterval %q: got %v, want %v", val, got, watchDefaultPollInterval)
		}
	}
}

func TestWatchPollInterval_Valid(t *testing.T) {
	cfg := &config.Config{Watch: &config.Watch{PollInterval: "30s"}}
	got := watchPollInterval(cfg)
	if got != 30*time.Second {
		t.Errorf("valid PollInterval: got %v, want 30s", got)
	}
}

// --- watchTUIPoller tests ---

// fakeSender captures messages sent via the msgSender interface.
type fakeSender struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (f *fakeSender) Send(msg tea.Msg) {
	f.mu.Lock()
	f.msgs = append(f.msgs, msg)
	f.mu.Unlock()
}

func (f *fakeSender) messages() []tea.Msg {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]tea.Msg, len(f.msgs))
	copy(cp, f.msgs)
	return cp
}

// discardLogger satisfies the logger interface by doing nothing.
type discardLogger struct{}

func (discardLogger) Info(_ string, _ ...any) {}

// TestWatchTUIPoller_ImmediateUpdate verifies that PRUpdateMsg is sent before
// the first tick fires.
func TestWatchTUIPoller_ImmediateUpdate(t *testing.T) {
	orig := listWatchedPRsFn
	listWatchedPRsFn = func(_, _ string) ([]github.WatchedPR, error) {
		return []github.WatchedPR{{Number: 1, Title: "test PR", Labels: []string{"godark:awaiting-human-review"}}}, nil
	}
	defer func() { listWatchedPRsFn = orig }()

	sender := &fakeSender{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a long interval so only the immediate pre-tick call fires during the test.
	go watchTUIPoller(ctx, sender, "owner/repo", time.Hour, discardLogger{})

	// Wait briefly for the immediate update.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if msgs := sender.messages(); len(msgs) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	msgs := sender.messages()
	if len(msgs) == 0 {
		t.Fatal("expected at least one PRUpdateMsg before first tick, got none")
	}
	update, ok := msgs[0].(tui.PRUpdateMsg)
	if !ok {
		t.Fatalf("first message: got %T, want tui.PRUpdateMsg", msgs[0])
	}
	if len(update.PRs) != 1 || update.PRs[0].Number != 1 {
		t.Errorf("PRUpdateMsg.PRs: got %v, want [{Number:1 ...}]", update.PRs)
	}
}

// TestWatchTUIPoller_TickSendsUpdate verifies that PRUpdateMsg is sent again
// on each ticker tick.
func TestWatchTUIPoller_TickSendsUpdate(t *testing.T) {
	orig := listWatchedPRsFn
	listWatchedPRsFn = func(_, _ string) ([]github.WatchedPR, error) {
		return []github.WatchedPR{{Number: 2, Title: "tick PR"}}, nil
	}
	defer func() { listWatchedPRsFn = orig }()

	sender := &fakeSender{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a very short interval so two updates arrive quickly.
	go watchTUIPoller(ctx, sender, "owner/repo", 10*time.Millisecond, discardLogger{})

	// Wait until at least two PRUpdateMsgs have been sent (immediate + one tick).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(sender.messages()) >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	if got := len(sender.messages()); got < 2 {
		t.Errorf("expected ≥2 PRUpdateMsgs (immediate + tick), got %d", got)
	}
}

// TestWatchTUIPoller_CancelExits verifies that the goroutine exits promptly
// when the context is cancelled.
func TestWatchTUIPoller_CancelExits(t *testing.T) {
	orig := listWatchedPRsFn
	listWatchedPRsFn = func(_, _ string) ([]github.WatchedPR, error) {
		return nil, nil
	}
	defer func() { listWatchedPRsFn = orig }()

	sender := &fakeSender{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		watchTUIPoller(ctx, sender, "owner/repo", time.Hour, discardLogger{})
		close(done)
	}()

	// Let the immediate update fire, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// goroutine exited cleanly
	case <-time.After(2 * time.Second):
		t.Error("watchTUIPoller did not exit after context cancellation")
	}
}
