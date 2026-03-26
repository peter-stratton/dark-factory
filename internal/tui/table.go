package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// issueRow holds all display state for a single issue in the table.
type issueRow struct {
	number      int
	title       string
	status      string
	stage       string
	prNumber    int
	retries     int
	errMsg      string
	judgeReason string // e.g. "Killed: idle 300s" — set by JudgeInterventionMsg
}

// Status marker constants.
const (
	markerQueued    = "○"
	markerCompleted = "■"
	markerReview    = "●"
	markerFailed    = "✕"
	markerJudge     = "⚖"
)

// Lipgloss styles for status markers.
var (
	markerQueuedStyle    = lipgloss.NewStyle().Foreground(colorMuted)
	markerCompletedStyle = lipgloss.NewStyle().Foreground(colorGreen)
	markerReviewStyle    = lipgloss.NewStyle().Foreground(colorYellow)
	markerFailedStyle    = lipgloss.NewStyle().Foreground(colorRed)
	markerJudgeStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D4760A", Dark: "#FF8C00"})
	rowNumberStyle       = lipgloss.NewStyle().Foreground(colorMuted)
	rowTitleStyle = lipgloss.NewStyle().Foreground(colorBright)
	rowErrStyle   = lipgloss.NewStyle().Foreground(colorRed)
)

// renderTable composes all issue rows into a single styled string.
//
// The spin argument supplies the current spinner frame used for in-progress
// rows (status == "" with a stage set). width controls title truncation.
func renderTable(rows []issueRow, spin spinner.Model, width int) string {
	if len(rows) == 0 {
		return ""
	}

	var out string
	for i, row := range rows {
		out += renderRow(row, spin, width)
		if i < len(rows)-1 {
			out += "\n"
		}
	}
	return out
}

// renderRow renders a single issue row with the badge right-aligned to a
// consistent column so badges form a clean vertical edge.
func renderRow(row issueRow, spin spinner.Model, width int) string {
	marker := markerFor(row, spin)
	num := rowNumberStyle.Render(fmt.Sprintf("#%d", row.number))

	// Reserve space for badge (~12 chars with padding) and gutters.
	badgeReserved := 14
	titleWidth := width - 22 - badgeReserved
	if titleWidth < 10 {
		titleWidth = 10
	}
	title := rowTitleStyle.Render(truncate(row.title, titleWidth))

	badge := badgeFor(row)

	left := marker + " " + num + " " + title
	leftWidth := lipgloss.Width(left)

	// Pad between title and badge so badges align at a consistent column.
	badgeCol := width - badgeReserved
	if badgeCol < leftWidth+2 {
		badgeCol = leftWidth + 2
	}
	gap := badgeCol - leftWidth
	if gap < 2 {
		gap = 2
	}

	line := left + strings.Repeat(" ", gap) + badge

	if row.judgeReason != "" {
		line += " " + markerJudgeStyle.Render(row.judgeReason)
	} else if row.errMsg != "" {
		line += " " + rowErrStyle.Render(row.errMsg)
	}

	return line
}

// badgeFor returns a styled status badge for a row, or "" if the row is
// still in-progress or queued without a terminal status.
//
// Badge mapping:
//   - "implemented"/"merged"       → green  "MERGED"
//   - "ready-to-merge"             → yellow "REVIEW"
//   - "needs-human-review"         → yellow "REVIEW"
//   - "failed"                     → red    "FAILED"
//   - in-progress (stage set)      → muted  stage name (e.g. "implement")
//   - queued (no status, no stage) → muted  "QUEUED"
func badgeFor(row issueRow) string {
	switch row.status {
	case "implemented", "merged":
		return badgeMergedStyle.Render("MERGED")
	case "ready-to-merge", "needs-human-review":
		return badgeReviewStyle.Render("REVIEW")
	case "failed":
		if row.judgeReason != "" {
			return badgeJudgeStyle.Render("JUDGE")
		}
		return badgeFailedStyle.Render("FAILED")
	default:
		if row.stage != "" {
			return badgeInProgressStyle.Render(strings.ToUpper(row.stage))
		}
		return badgeQueuedStyle.Render("QUEUED")
	}
}

// markerFor returns the styled status marker for a row.
//
// State machine:
//   - terminal status "implemented"/"merged" → green ■
//   - terminal status "ready-to-merge"       → yellow ●
//   - terminal status "needs-human-review"   → yellow ●
//   - terminal status "failed"               → red ✕
//   - in-progress (no status, stage set)     → spinner frame
//   - queued (no status, no stage)           → dim ○
func markerFor(row issueRow, spin spinner.Model) string {
	switch row.status {
	case "implemented", "merged":
		return markerCompletedStyle.Render(markerCompleted)
	case "ready-to-merge", "needs-human-review":
		return markerReviewStyle.Render(markerReview)
	case "failed":
		if row.judgeReason != "" {
			return markerJudgeStyle.Render(markerJudge)
		}
		return markerFailedStyle.Render(markerFailed)
	default:
		// No terminal status: distinguish in-progress (stage set) from queued.
		if row.stage != "" {
			return spin.View()
		}
		return markerQueuedStyle.Render(markerQueued)
	}
}

// truncate shortens s to at most maxWidth runes, replacing the last rune with
// "…" if truncation occurs.
func truncate(s string, maxWidth int) string {
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth <= 0 {
		return "…"
	}
	return string(runes[:maxWidth-1]) + "…"
}
