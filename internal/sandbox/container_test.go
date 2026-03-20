package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

const validStatsJSON = `{"memory_stats":{"max_usage":104857600},"cpu_stats":{"cpu_usage":{"total_usage":500000000}}}`

// saveRunners saves the current CommandRunner, SplitRunner,
// CommandRunnerWithContext, and StatsInterval values and returns a restore
// function.
func saveRunners() func() {
	origCmd := CommandRunner
	origSplit := SplitRunner
	origCtx := CommandRunnerWithContext
	origInterval := StatsInterval
	return func() {
		CommandRunner = origCmd
		SplitRunner = origSplit
		CommandRunnerWithContext = origCtx
		StatsInterval = origInterval
	}
}

func TestRunContainerSuccess(t *testing.T) {
	defer saveRunners()()
	StatsInterval = time.Millisecond

	var calls []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		call := name + " " + strings.Join(args, " ")
		calls = append(calls, call)
		if args[0] == "create" {
			return []byte("abc123\n"), nil
		}
		return []byte{}, nil
	}
	CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return []byte("0\n"), nil
	}
	SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
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
