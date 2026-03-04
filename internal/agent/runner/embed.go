// Package runner provides the embedded Python agent runner script.
package runner

import "embed"

// FS holds the embedded agent_runner.py file.
//
//go:embed agent_runner.py
var FS embed.FS
