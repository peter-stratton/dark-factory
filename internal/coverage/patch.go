package coverage

import (
	"fmt"
	"sort"
	"strings"
)

// UncoveredFile groups uncovered line ranges for a single file.
type UncoveredFile struct {
	File   string
	Ranges []LineRange
}

// PatchResult holds the outcome of a patch coverage computation.
type PatchResult struct {
	Percent      float64
	TotalLines   int
	CoveredLines int
	Uncovered    []UncoveredFile
}

// ComputePatchCoverage computes coverage for changed lines only.
// changed maps file paths to added line ranges (from ParseUnifiedDiff).
// blocks is the parsed coverage profile (from ParseProfile).
//
// For each changed line, it is considered covered if any coverage block
// with Count > 0 overlaps that line.
func ComputePatchCoverage(changed map[string][]LineRange, blocks []ProfileBlock) PatchResult {
	if len(changed) == 0 {
		return PatchResult{Percent: 100}
	}

	// Index coverage blocks by file.
	blocksByFile := make(map[string][]ProfileBlock)
	for _, b := range blocks {
		blocksByFile[b.File] = append(blocksByFile[b.File], b)
	}

	var totalLines, coveredLines int
	var uncovered []UncoveredFile

	// Sort file names for deterministic output.
	files := make([]string, 0, len(changed))
	for f := range changed {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, file := range files {
		ranges := changed[file]
		fileBlocks := blocksByFile[file]

		var fileUncoveredRanges []LineRange

		for _, lr := range ranges {
			for line := lr.Start; line <= lr.End; line++ {
				totalLines++
				if isLineCovered(line, fileBlocks) {
					coveredLines++
				} else {
					// Accumulate into ranges.
					fileUncoveredRanges = appendToRanges(fileUncoveredRanges, line)
				}
			}
		}

		if len(fileUncoveredRanges) > 0 {
			uncovered = append(uncovered, UncoveredFile{
				File:   file,
				Ranges: fileUncoveredRanges,
			})
		}
	}

	var pct float64
	if totalLines > 0 {
		pct = float64(coveredLines) / float64(totalLines) * 100
	}

	return PatchResult{
		Percent:      pct,
		TotalLines:   totalLines,
		CoveredLines: coveredLines,
		Uncovered:    uncovered,
	}
}

// isLineCovered checks if any coverage block with Count > 0 includes the given line.
func isLineCovered(line int, blocks []ProfileBlock) bool {
	for _, b := range blocks {
		if b.Count > 0 && line >= b.StartLine && line <= b.EndLine {
			return true
		}
	}
	return false
}

// appendToRanges appends a line to the last range if contiguous, or starts a new range.
func appendToRanges(ranges []LineRange, line int) []LineRange {
	if len(ranges) > 0 && ranges[len(ranges)-1].End == line-1 {
		ranges[len(ranges)-1].End = line
		return ranges
	}
	return append(ranges, LineRange{Start: line, End: line})
}

// FormatResult formats a PatchResult into a human-readable string.
// If the result is below target, the output includes uncovered line details.
func FormatResult(result PatchResult, target int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Patch coverage: %.1f%% (%d/%d changed lines covered), target: %d%%",
		result.Percent, result.CoveredLines, result.TotalLines, target)

	if len(result.Uncovered) > 0 {
		sb.WriteString("\n\nUncovered changed lines:")
		for _, uf := range result.Uncovered {
			for _, r := range uf.Ranges {
				if r.Start == r.End {
					fmt.Fprintf(&sb, "\n  %s:%d", uf.File, r.Start)
				} else {
					fmt.Fprintf(&sb, "\n  %s:%d-%d", uf.File, r.Start, r.End)
				}
			}
		}
		sb.WriteString("\n\nWrite tests that exercise these specific code paths.")
	}

	return sb.String()
}
