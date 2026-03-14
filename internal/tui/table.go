package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// issueRow holds all display state for a single issue in the table.
type issueRow struct {
	number   int
	title    string
	status   string
	stage    string
	prNumber int
	retries  int
	errMsg   string
}

// Status marker constants.
const (
	markerQueued    = "○"
	markerCompleted = "■"
	markerReview    = "●"
	markerFailed    = "✕"
)

// Lipgloss styles for status markers.
var (
	markerQueuedStyle    = lipgloss.NewStyle().Foreground(colorMuted)
	markerCompletedStyle = lipgloss.NewStyle().Foreground(colorGreen)
	markerReviewStyle    = lipgloss.NewStyle().Foreground(colorYellow)
	markerFailedStyle    = lipgloss.NewStyle().Foreground(colorRed)
	rowNumberStyle       = lipgloss.NewStyle().Foreground(colorMuted)
	rowTitleStyle        = lipgloss.NewStyle().Foreground(colorBright)
	rowStageStyle        = lipgloss.NewStyle().Foreground(colorMuted)
	rowErrStyle          = lipgloss.NewStyle().Foreground(colorRed)
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

// renderRow renders a single issue row.
func renderRow(row issueRow, spin spinner.Model, width int) string {
	marker := markerFor(row, spin)
	num := rowNumberStyle.Render(fmt.Sprintf("#%d", row.number))

	// Reserve space for: marker(1) + space(1) + "#NNN"(≤6) + space(1) + stage(≤10) + space(1) = ~20
	// Approximate title column width from terminal width.
	titleWidth := width - 22
	if titleWidth < 10 {
		titleWidth = 10
	}
	title := rowTitleStyle.Render(truncate(row.title, titleWidth))

	badge := badgeFor(row)

	line := marker + " " + num + " " + title
	if badge != "" {
		line += "  " + badge
	}

	if row.errMsg != "" {
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
		return badgeFailedStyle.Render("FAILED")
	default:
		if row.stage != "" {
			return rowStageStyle.Render(row.stage)
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
