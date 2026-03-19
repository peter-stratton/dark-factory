package coverage

import (
	"regexp"
	"strconv"
	"strings"
)

// LineRange represents an inclusive range of line numbers.
type LineRange struct {
	Start int
	End   int
}

// diffFileHeader matches unified diff file headers like "+++ b/internal/foo/bar.go".
var diffFileHeader = regexp.MustCompile(`^\+\+\+ b/(.+)$`)

// diffHunkHeader matches unified diff hunk headers like "@@ -0,0 +10,5 @@".
// With --unified=0, the range is typically "+start,count" or "+start" (count=1).
var diffHunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// ParseUnifiedDiff parses the output of "git diff --unified=0" and returns
// the added line ranges per file. Test files (*_test.go) are filtered out.
func ParseUnifiedDiff(output string) map[string][]LineRange {
	if output == "" {
		return nil
	}

	result := make(map[string][]LineRange)
	var currentFile string

	for _, line := range strings.Split(output, "\n") {
		if m := diffFileHeader.FindStringSubmatch(line); m != nil {
			currentFile = m[1]
			// Skip test files.
			if strings.HasSuffix(currentFile, "_test.go") {
				currentFile = ""
			}
			continue
		}

		if currentFile == "" {
			continue
		}

		if m := diffHunkHeader.FindStringSubmatch(line); m != nil {
			start, _ := strconv.Atoi(m[1])
			count := 1
			if m[2] != "" {
				count, _ = strconv.Atoi(m[2])
			}
			// A count of 0 means no lines were added (pure deletion). Skip.
			if count == 0 {
				continue
			}
			result[currentFile] = append(result[currentFile], LineRange{
				Start: start,
				End:   start + count - 1,
			})
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}
