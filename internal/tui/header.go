package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// sep is the middle-dot separator used between metadata segments.
const sep = " \xc2\xb7 "

// renderHeader returns the two-line header chrome for a given model.
//
// Line 1: "godark" logo left-aligned, repo right-aligned.
// Line 2: milestone · timestamp [· base: baseBranch] [· auto-merge: feature=X rollup=Y]
//
// The base-branch and auto-merge segments are omitted when empty/nil,
// mirroring the conditional logic in run-detail.html line 75.
func renderHeader(m Model) string {
	line1 := renderHeaderLine1(m)
	line2 := renderHeaderLine2(m)
	return line1 + "\n" + line2
}

// renderHeaderLine1 builds the logo ↔ repo line.
func renderHeaderLine1(m Model) string {
	logo := headerLogoStyle.Render("godark")
	repo := headerRepoStyle.Render(m.repo)

	if m.width <= 0 {
		return logo + "  " + repo
	}

	// Calculate the available gap width between logo and repo.
	logoWidth := lipgloss.Width(logo)
	repoWidth := lipgloss.Width(repo)
	gap := m.width - logoWidth - repoWidth
	if gap < 1 {
		gap = 1
	}
	return logo + strings.Repeat(" ", gap) + repo
}

// renderHeaderLine2 builds the metadata subtitle line.
func renderHeaderLine2(m Model) string {
	styledSep := headerSepStyle.Render(sep)

	parts := []string{
		headerValueStyle.Render(m.milestone),
		headerValueStyle.Render(m.timestamp),
	}

	if m.baseBranch != "" {
		parts = append(parts,
			headerLabelStyle.Render("base: ")+headerValueStyle.Render(m.baseBranch),
		)
	}

	if m.autoMerge != nil {
		parts = append(parts,
			headerLabelStyle.Render("auto-merge: ")+
				headerValueStyle.Render("feature="+m.autoMerge.feature+" rollup="+m.autoMerge.rollup),
		)
	}

	return strings.Join(parts, styledSep)
}
