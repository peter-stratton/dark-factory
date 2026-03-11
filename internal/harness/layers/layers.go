// Package layers parses layer definitions from architecture.json and returns
// a directed graph of layer dependencies.
package layers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Layer represents a single architectural layer with its paths and dependencies.
type Layer struct {
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	Paths           []string `json:"paths"`
	MayDependOn     []string `json:"may_depend_on"`
	MustNotDependOn []string `json:"must_not_depend_on,omitempty"`
}

// Definition holds the full set of layers parsed from architecture.json.
type Definition struct {
	Layers []Layer `json:"layers"`
}

// Parse decodes a layer definition from r. Returns an error on invalid JSON,
// empty layers slice, duplicate names, empty name/paths, or may_depend_on /
// must_not_depend_on referencing non-existent layers. Cycle detection is left
// to the vet package.
func Parse(r io.Reader) (*Definition, error) {
	var def Definition
	if err := json.NewDecoder(r).Decode(&def); err != nil {
		return nil, fmt.Errorf("layers: decode JSON: %w", err)
	}

	if len(def.Layers) == 0 {
		return nil, fmt.Errorf("layers: definition must contain at least one layer")
	}

	// First pass: validate names and paths, build name set.
	names := make(map[string]struct{}, len(def.Layers))
	for i, l := range def.Layers {
		if l.Name == "" {
			return nil, fmt.Errorf("layers: layer[%d] has empty name", i)
		}
		if len(l.Paths) == 0 {
			return nil, fmt.Errorf("layers: layer %q has no paths", l.Name)
		}
		if _, dup := names[l.Name]; dup {
			return nil, fmt.Errorf("layers: duplicate layer name %q", l.Name)
		}
		names[l.Name] = struct{}{}
	}

	// Second pass: validate may_depend_on and must_not_depend_on reference known layers.
	for _, l := range def.Layers {
		for _, dep := range l.MayDependOn {
			if _, ok := names[dep]; !ok {
				return nil, fmt.Errorf("layers: layer %q may_depend_on unknown layer %q", l.Name, dep)
			}
		}
		for _, dep := range l.MustNotDependOn {
			if _, ok := names[dep]; !ok {
				return nil, fmt.Errorf("layers: layer %q must_not_depend_on unknown layer %q", l.Name, dep)
			}
		}
	}

	return &def, nil
}

// ParseFile opens the file at path and calls Parse.
func ParseFile(path string) (*Definition, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("layers: open %s: %w", path, err)
	}
	defer f.Close()

	return Parse(f)
}
