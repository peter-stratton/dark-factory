package vet

import (
	"fmt"
	"sort"
	"strings"

	"github.com/phs/dark-factory/internal/harness/layers"
)

// ValidateArchitecture checks a layer Definition for structural correctness.
// It detects cycles in the dependency graph and reports all affected layers.
func ValidateArchitecture(def *layers.Definition) *Report {
	r := &Report{}
	detectCycles(def, r)
	return r
}

// detectCycles uses recursive DFS with gray/black coloring to find all cycles
// in the layer dependency graph. Each unique cycle is reported once as an
// Error finding. The cycle display includes the full path and the closing edge
// (e.g. "A → B → A").
func detectCycles(def *layers.Definition, r *Report) {
	adj := make(map[string][]string, len(def.Layers))
	for _, l := range def.Layers {
		adj[l.Name] = l.MayDependOn
	}

	// 0 = unvisited (white), 1 = in-progress (gray), 2 = done (black)
	color := make(map[string]int, len(def.Layers))
	reported := make(map[string]bool)

	var dfs func(node string, path []string)
	dfs = func(node string, path []string) {
		color[node] = 1
		path = append(path, node)

		for _, dep := range adj[node] {
			switch color[dep] {
			case 1:
				// Back edge: dep is already in the current DFS path — cycle found.
				cycleStart := -1
				for i, p := range path {
					if p == dep {
						cycleStart = i
						break
					}
				}
				cycle := path[cycleStart:]
				key := canonicalCycleKey(cycle)
				if !reported[key] {
					reported[key] = true
					cycleStr := strings.Join(cycle, " → ") + " → " + dep
					r.Add(Finding{
						Severity: Error,
						Location: "architecture",
						Message:  fmt.Sprintf("cycle detected: %s", cycleStr),
					})
				}
			case 0:
				dfs(dep, path)
			}
		}

		color[node] = 2
	}

	// Visit all nodes in definition order for deterministic output.
	for _, l := range def.Layers {
		if color[l.Name] == 0 {
			dfs(l.Name, nil)
		}
	}
}

// canonicalCycleKey returns a sorted, comma-joined key used to deduplicate
// equivalent cycles found via different DFS starting points.
func canonicalCycleKey(cycle []string) string {
	sorted := make([]string, len(cycle))
	copy(sorted, cycle)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}
