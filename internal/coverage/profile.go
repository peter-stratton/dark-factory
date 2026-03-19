package coverage

import (
	"strconv"
	"strings"
)

// ProfileBlock represents a single block from a Go coverage profile.
type ProfileBlock struct {
	File      string // relative file path (module prefix stripped)
	StartLine int
	EndLine   int
	Stmts     int
	Count     int // execution count; 0 means uncovered
}

// ParseProfile parses Go coverage profile data (the output of -coverprofile).
// modulePath is used to strip the module prefix from file paths, e.g.
// "github.com/phs/dark-factory/internal/foo/bar.go" becomes "internal/foo/bar.go".
//
// Profile format: "file:startLine.startCol,endLine.endCol stmts count"
func ParseProfile(data string, modulePath string) []ProfileBlock {
	if data == "" {
		return nil
	}

	prefix := modulePath + "/"
	var blocks []ProfileBlock

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}

		// Split "file:startLine.startCol,endLine.endCol stmts count"
		// into "file:startLine.startCol,endLine.endCol" and "stmts count".
		spaceIdx := strings.LastIndex(line, " ")
		if spaceIdx < 0 {
			continue
		}
		countStr := line[spaceIdx+1:]
		rest := line[:spaceIdx]

		spaceIdx2 := strings.LastIndex(rest, " ")
		if spaceIdx2 < 0 {
			continue
		}
		stmtsStr := rest[spaceIdx2+1:]
		fileRange := rest[:spaceIdx2]

		// Split file:startLine.startCol,endLine.endCol
		colonIdx := strings.Index(fileRange, ":")
		if colonIdx < 0 {
			continue
		}
		file := fileRange[:colonIdx]
		rangePart := fileRange[colonIdx+1:]

		// Strip module prefix.
		file = strings.TrimPrefix(file, prefix)

		// Parse "startLine.startCol,endLine.endCol"
		commaIdx := strings.Index(rangePart, ",")
		if commaIdx < 0 {
			continue
		}
		startPart := rangePart[:commaIdx]
		endPart := rangePart[commaIdx+1:]

		startLine := parseDotInt(startPart)
		endLine := parseDotInt(endPart)
		if startLine == 0 || endLine == 0 {
			continue
		}

		stmts, _ := strconv.Atoi(stmtsStr)
		count, _ := strconv.Atoi(countStr)

		blocks = append(blocks, ProfileBlock{
			File:      file,
			StartLine: startLine,
			EndLine:   endLine,
			Stmts:     stmts,
			Count:     count,
		})
	}

	return blocks
}

// parseDotInt parses the integer before the dot in "N.M" (e.g. "10.5" -> 10).
func parseDotInt(s string) int {
	dotIdx := strings.Index(s, ".")
	if dotIdx < 0 {
		n, _ := strconv.Atoi(s)
		return n
	}
	n, _ := strconv.Atoi(s[:dotIdx])
	return n
}
