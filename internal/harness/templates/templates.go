// Package templates provides embedded document templates for harness scaffolding.
// The templates are written by godark init and godark new into target projects.
package templates

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteIfNotExists writes the embedded file at srcPath to destPath.
// Creates parent directories as needed. Returns true if the file was
// written, false if it already existed. Returns an error if the write fails.
func WriteIfNotExists(srcPath, destPath string) (written bool, err error) {
	_, err = os.Stat(destPath)
	if err == nil {
		// File already exists; skip.
		return false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat %s: %w", destPath, err)
	}

	data, err := FS.ReadFile(srcPath)
	if err != nil {
		return false, fmt.Errorf("reading embedded file %s: %w", srcPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return false, fmt.Errorf("creating parent directories for %s: %w", destPath, err)
	}

	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return false, fmt.Errorf("writing %s: %w", destPath, err)
	}

	return true, nil
}
