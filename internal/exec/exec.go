package exec

import osexec "os/exec"

// CommandRunnerFunc is the shared function type for package-level command
// runner variables. It executes a named command with the given arguments and
// returns the combined output.
type CommandRunnerFunc func(name string, args ...string) ([]byte, error)

// Default is a CommandRunnerFunc that runs the named command and returns its
// combined stdout and stderr output.
var Default CommandRunnerFunc = func(name string, args ...string) ([]byte, error) {
	return osexec.Command(name, args...).CombinedOutput()
}
