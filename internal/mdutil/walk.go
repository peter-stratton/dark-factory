package mdutil

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// WalkMarkdownFiles walks dir recursively and calls fn for each .md file found.
// Directories and non-.md files are skipped. Returns the first error from fn
// or from the walk itself.
func WalkMarkdownFiles(dir string, fn func(path string) error) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		return fn(path)
	})
}
