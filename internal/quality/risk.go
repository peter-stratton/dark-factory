package quality

import (
	"fmt"
	"strings"
)

const (
	defaultMaxLines = 200
	defaultMaxFiles = 10
)

// RiskInput holds all data needed to classify the risk of a PR for auto-merge.
type RiskInput struct {
	LinesChanged   int
	FilesChanged   int
	ChangedFiles   []string // file paths in the PR diff
	ProtectedPaths []string // from config
	FixCycles      int      // verify fix attempts used
	QualityFlags   []Flag   // from review quality checks
}

// RiskAssessment is the result of ClassifyRisk.
type RiskAssessment struct {
	IsLowRisk bool       `json:"is_low_risk"`
	Gates     []RiskGate `json:"gates"`
}

// RiskGate records the outcome of a single risk gate evaluation.
type RiskGate struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

// ClassifyRisk evaluates a PR against all risk gates and returns a RiskAssessment.
// A PR is low-risk only when ALL gates pass.
//
// If maxLines or maxFiles are zero, the defaults (200 and 10 respectively) are used.
func ClassifyRisk(input RiskInput, maxLines, maxFiles int) RiskAssessment {
	if maxLines == 0 {
		maxLines = defaultMaxLines
	}
	if maxFiles == 0 {
		maxFiles = defaultMaxFiles
	}

	gates := []RiskGate{
		evalLinesGate(input.LinesChanged, maxLines),
		evalFilesGate(input.FilesChanged, maxFiles),
		evalProtectedPathsGate(input.ChangedFiles, input.ProtectedPaths),
		evalFixCyclesGate(input.FixCycles),
		evalQualityFlagsGate(input.QualityFlags),
	}

	isLowRisk := true
	for _, g := range gates {
		if !g.Passed {
			isLowRisk = false
			break
		}
	}

	return RiskAssessment{
		IsLowRisk: isLowRisk,
		Gates:     gates,
	}
}

func evalLinesGate(linesChanged, maxLines int) RiskGate {
	passed := linesChanged <= maxLines
	detail := fmt.Sprintf("%d lines changed (max %d)", linesChanged, maxLines)
	return RiskGate{Name: "max_lines", Passed: passed, Detail: detail}
}

func evalFilesGate(filesChanged, maxFiles int) RiskGate {
	passed := filesChanged <= maxFiles
	detail := fmt.Sprintf("%d files changed (max %d)", filesChanged, maxFiles)
	return RiskGate{Name: "max_files", Passed: passed, Detail: detail}
}

func evalProtectedPathsGate(changedFiles, protectedPaths []string) RiskGate {
	for _, f := range changedFiles {
		for _, p := range protectedPaths {
			if strings.HasPrefix(f, p) {
				return RiskGate{
					Name:   "protected_paths",
					Passed: false,
					Detail: fmt.Sprintf("file %q matches protected path %q", f, p),
				}
			}
		}
	}
	return RiskGate{Name: "protected_paths", Passed: true, Detail: "no protected paths touched"}
}

func evalFixCyclesGate(fixCycles int) RiskGate {
	passed := fixCycles == 0
	detail := fmt.Sprintf("%d fix cycle(s) used", fixCycles)
	return RiskGate{Name: "no_fix_cycles", Passed: passed, Detail: detail}
}

func evalQualityFlagsGate(flags []Flag) RiskGate {
	if len(flags) == 0 {
		return RiskGate{Name: "no_quality_flags", Passed: true, Detail: "no quality flags raised"}
	}
	codes := make([]string, len(flags))
	for i, f := range flags {
		codes[i] = f.Code
	}
	return RiskGate{
		Name:   "no_quality_flags",
		Passed: false,
		Detail: fmt.Sprintf("%d quality flag(s) raised: %s", len(flags), strings.Join(codes, ", ")),
	}
}
