package sandbox

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

const validStatsJSON = `{"memory_stats":{"max_usage":104857600},"cpu_stats":{"cpu_usage":{"total_usage":500000000}}}`

// validStatsJSONCgroupsV2 simulates Docker stats on cgroups v2 where
// max_usage is absent and only usage is reported.
const validStatsJSONCgroupsV2 = `{"memory_stats":{"usage":83886080},"cpu_stats":{"cpu_usage":{"total_usage":400000000}}}`

// saveRunners saves the current CommandRunner, SplitRunner,
// CommandRunnerWithContext, LogFollower, and StatsInterval values and returns
// a restore function.
func saveRunners() func() {
	origCmd := CommandRunner
	origSplit := SplitRunner
	origCtx := CommandRunnerWithContext
	origLog := LogFollower
	origInterval := StatsInterval
	return func() {
		CommandRunner = origCmd
		SplitRunner = origSplit
		CommandRunnerWithContext = origCtx
		LogFollower = origLog
		StatsInterval = origInterval
	}
}

func TestRunContainerSuccess(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	var mu sync.Mutex
	var calls []string
	appendCall := func(call string) {
		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()
	}
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		appendCall(name + " " + strings.Join(args, " "))
		if args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		appendCall(name + " " + strings.Join(args, " "))
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		appendCall(name + " " + strings.Join(args, " "))
		return []byte("hello\n"), []byte{}, nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"echo", "hello"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "hello\n")
	}
	if result.TimedOut {
		t.Error("TimedOut should be false")
	}

	// Verify docker rm -f was called (cleanup).
	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, c := range calls {
		if strings.Contains(c, "rm -f") {
			found = true
			break
		}
	}
	if !found {
		t.Error("docker rm -f was not called for cleanup")
	}
}

func TestRunContainerFailedCommand(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("1\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return []byte{}, []byte("error output\n"), nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"false"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
}

func TestRunContainerStderrCapture(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return []byte("out\n"), []byte("err\n"), nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"sh", "-c", "echo out; echo err >&2"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stderr != "err\n" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "err\n")
	}
	if result.Stdout != "out\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "out\n")
	}
}

func TestRunContainerEnvironmentVariables(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	var createArgs string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			createArgs = strings.Join(args, " ")
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	_, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"env"},
		Env:   map[string]string{"FOO": "bar"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(createArgs, "-e FOO=bar") {
		t.Errorf("docker create missing -e FOO=bar, got: %s", createArgs)
	}
}

func TestRunContainerMount(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	var createArgs string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			createArgs = strings.Join(args, " ")
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	_, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"ls"},
		Mount: "/host/path:/container/path",
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(createArgs, "-v /host/path:/container/path") {
		t.Errorf("docker create missing -v mount, got: %s", createArgs)
	}
}

func TestRunContainerDockerSocketMountEnabled(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	var createArgs string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			createArgs = strings.Join(args, " ")
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	_, err := RunContainer(context.Background(), RunOpts{
		Image:             "test:latest",
		Cmd:               []string{"docker", "ps"},
		MountDockerSocket: true,
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(createArgs, "-v /var/run/docker.sock:/var/run/docker.sock") {
		t.Errorf("docker create missing socket mount, got: %s", createArgs)
	}
}

func TestRunContainerDockerSocketMountDisabled(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	var createArgs string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			createArgs = strings.Join(args, " ")
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	_, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"ls"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(createArgs, "docker.sock") {
		t.Errorf("docker create should not include socket mount, got: %s", createArgs)
	}
}

func TestRunContainerBothMounts(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	var createArgs string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			createArgs = strings.Join(args, " ")
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	_, err := RunContainer(context.Background(), RunOpts{
		Image:             "test:latest",
		Cmd:               []string{"ls"},
		Mount:             "/host/path:/container/path",
		MountDockerSocket: true,
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(createArgs, "-v /host/path:/container/path") {
		t.Errorf("docker create missing workspace mount, got: %s", createArgs)
	}
	if !strings.Contains(createArgs, "-v /var/run/docker.sock:/var/run/docker.sock") {
		t.Errorf("docker create missing socket mount, got: %s", createArgs)
	}
}

func TestRunContainerTimeout(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	var stopCalled bool
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		if len(args) > 0 && args[0] == "stop" {
			stopCalled = true
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// Simulate docker wait blocking until context is cancelled.
		<-ctx.Done()
		return nil, ctx.Err()
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image:   "test:latest",
		Cmd:     []string{"sleep", "999"},
		Timeout: 50 * time.Millisecond,
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.TimedOut {
		t.Error("TimedOut should be true")
	}
	if !stopCalled {
		t.Error("docker stop was not called on timeout")
	}
}

func TestRunContainerCleanupOnStartFailure(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	var rmCalled bool
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		if len(args) > 0 && args[0] == "start" {
			return []byte("error"), fmt.Errorf("start failed")
		}
		if len(args) > 0 && args[0] == "rm" {
			rmCalled = true
		}
		return []byte{}, nil
	}

	_, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"echo", "hello"},
	}, slog.Default())
	if err == nil {
		t.Fatal("expected error from docker start failure")
	}
	if !rmCalled {
		t.Error("docker rm was not called after start failure")
	}
}

func TestRunContainerCleanupOnContextCancel(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	var stopCalled, rmCalled bool
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		if len(args) > 0 && args[0] == "stop" {
			stopCalled = true
		}
		if len(args) > 0 && args[0] == "rm" {
			rmCalled = true
		}
		return []byte{}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	CommandRunnerWithContext = func(c context.Context, name string, args ...string) ([]byte, error) {
		cancel() // Simulate external cancellation.
		<-c.Done()
		return nil, c.Err()
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	result, err := RunContainer(ctx, RunOpts{
		Image: "test:latest",
		Cmd:   []string{"sleep", "999"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.TimedOut {
		t.Error("TimedOut should be true when context is cancelled")
	}
	if !stopCalled {
		t.Error("docker stop was not called on context cancel")
	}
	if !rmCalled {
		t.Error("docker rm was not called on context cancel")
	}
}

func TestRunContainerInspectParsed(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		if name == "curl" {
			return []byte(validStatsJSON), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"true"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PeakMemoryBytes != 104857600 {
		t.Errorf("PeakMemoryBytes = %d, want 104857600", result.PeakMemoryBytes)
	}
	if result.CPUNanoseconds != 500000000 {
		t.Errorf("CPUNanoseconds = %d, want 500000000", result.CPUNanoseconds)
	}
}

func TestRunContainerStatsCgroupsV2(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		if name == "curl" {
			return []byte(validStatsJSONCgroupsV2), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"true"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PeakMemoryBytes != 83886080 {
		t.Errorf("PeakMemoryBytes = %d, want 83886080 (cgroups v2 usage fallback)", result.PeakMemoryBytes)
	}
	if result.CPUNanoseconds != 400000000 {
		t.Errorf("CPUNanoseconds = %d, want 400000000", result.CPUNanoseconds)
	}
}

func TestRunContainerInspectParseFailure(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		if name == "curl" {
			return []byte("not valid json {{{"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"true"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PeakMemoryBytes != 0 {
		t.Errorf("PeakMemoryBytes = %d, want 0 on parse failure", result.PeakMemoryBytes)
	}
	if result.CPUNanoseconds != 0 {
		t.Errorf("CPUNanoseconds = %d, want 0 on parse failure", result.CPUNanoseconds)
	}
}

func TestRunContainerInspectCommandFailure(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		if name == "curl" {
			return nil, fmt.Errorf("stats api failed")
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"true"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PeakMemoryBytes != 0 {
		t.Errorf("PeakMemoryBytes = %d, want 0 on inspect command failure", result.PeakMemoryBytes)
	}
	if result.CPUNanoseconds != 0 {
		t.Errorf("CPUNanoseconds = %d, want 0 on inspect command failure", result.CPUNanoseconds)
	}
}

func TestRunContainerInspectAfterTimeout(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		if name == "curl" {
			return []byte(validStatsJSON), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image:   "test:latest",
		Cmd:     []string{"sleep", "999"},
		Timeout: 50 * time.Millisecond,
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.TimedOut {
		t.Error("TimedOut should be true")
	}
	if result.PeakMemoryBytes != 104857600 {
		t.Errorf("PeakMemoryBytes = %d, want 104857600 after timeout", result.PeakMemoryBytes)
	}
}

// makeLogFollower returns a LogFollower stub that writes lines to a pipe and
// returns immediately. The caller controls what lines are emitted.
func makeLogFollower(lines []string) func(ctx context.Context, name string) (io.ReadCloser, func() error, error) {
	return func(ctx context.Context, name string) (io.ReadCloser, func() error, error) {
		pr, pw := io.Pipe()
		go func() {
			for _, line := range lines {
				_, _ = fmt.Fprintln(pw, line)
			}
			pw.Close()
		}()
		return pr, func() error { return nil }, nil
	}
}

func TestLogCallbackReceivesLines(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	want := []string{"line one", "line two", "line three"}
	LogFollower = makeLogFollower(want)

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return []byte("out\n"), []byte{}, nil
	}

	var got []string
	_, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"echo", "hello"},
		LogCallback: func(line string) {
			got = append(got, line)
		},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("LogCallback received no lines")
	}
	for i, w := range want {
		found := false
		for _, g := range got {
			if strings.Contains(g, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("want[%d] = %q not found in received lines %v", i, w, got)
		}
	}
}

func TestNilLogCallbackNoOp(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	// LogFollower must not be called when LogCallback is nil.
	LogFollower = func(ctx context.Context, name string) (io.ReadCloser, func() error, error) {
		t.Error("LogFollower called with nil LogCallback")
		return nil, nil, fmt.Errorf("should not be called")
	}

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return []byte("out\n"), []byte{}, nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image: "test:latest",
		Cmd:   []string{"true"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "out\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "out\n")
	}
}

func TestLogCallbackGoroutineExitsOnCancel(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	// LogFollower blocks until ctx is cancelled, then returns EOF.
	LogFollower = func(ctx context.Context, name string) (io.ReadCloser, func() error, error) {
		pr, pw := io.Pipe()
		go func() {
			<-ctx.Done()
			pw.Close()
		}()
		return pr, func() error { return nil }, nil
	}

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	CommandRunnerWithContext = func(c context.Context, name string, args ...string) ([]byte, error) {
		cancel()
		<-c.Done()
		return nil, c.Err()
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	result, err := RunContainer(ctx, RunOpts{
		Image:       "test:latest",
		Cmd:         []string{"sleep", "999"},
		LogCallback: func(line string) {},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// RunContainer must return (goroutines have exited) — if it hangs the
	// test runner's timeout will catch it. Just verify the result.
	if !result.TimedOut {
		t.Error("TimedOut should be true when context is cancelled")
	}
}

func TestLogCallbackRunResultStillPopulated(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	LogFollower = makeLogFollower([]string{"streaming line"})

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		return []byte("final stdout\n"), []byte("final stderr\n"), nil
	}

	result, err := RunContainer(context.Background(), RunOpts{
		Image:       "test:latest",
		Cmd:         []string{"echo", "hello"},
		LogCallback: func(line string) {},
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "final stdout\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "final stdout\n")
	}
	if result.Stderr != "final stderr\n" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "final stderr\n")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}
