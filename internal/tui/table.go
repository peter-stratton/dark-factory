package tui

import (
	"fmt"
	"strings"
	"time"

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
	traceID     string
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
	markerQueuedStyle        = lipgloss.NewStyle().Foreground(colorMuted)
	markerCompletedStyle     = lipgloss.NewStyle().Foreground(colorGreen)
	markerReviewStyle        = lipgloss.NewStyle().Foreground(colorYellow)
	markerAwaitingMergeStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0E7C7B", Dark: "#2DD4BF"})
	markerFailedStyle        = lipgloss.NewStyle().Foreground(colorRed)
	markerJudgeStyle         = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D4760A", Dark: "#FF8C00"})
	rowNumberStyle       = lipgloss.NewStyle().Foreground(colorMuted)
	rowTitleStyle = lipgloss.NewStyle().Foreground(colorBright)
	traceStyle   = lipgloss.NewStyle().Faint(true)
)

// formatCountdown formats the remaining duration until t as a compact string
// suitable for a TUI badge, e.g. "1h23m", "45m30s", or "12s".
// Returns "" if t is zero or already in the past.
func formatCountdown(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	diff := time.Until(t)
	if diff <= 0 {
		return ""
	}
	total := int(diff.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// renderTable composes all issue rows into a single styled string.
//
// The spin argument supplies the current spinner frame used for in-progress
// rows (status == "" with a stage set). width controls title truncation.
// rateLimitedUntil, when non-nil, causes in-progress rows to display a
// "RATE-LIMITED <countdown>" badge instead of their stage badge.
func renderTable(rows []issueRow, spin spinner.Model, width int, rateLimitedUntil *time.Time) string {
	if len(rows) == 0 {
		return ""
	}

	var out string
	for i, row := range rows {
		out += renderRow(row, spin, width, rateLimitedUntil)
		if i < len(rows)-1 {
			out += "\n"
		}
	}
	return out
}

// renderRow renders a single issue row with the badge right-aligned to a
// consistent column so badges form a clean vertical edge.
// rateLimitedUntil, when non-nil, causes in-progress rows to display a
// "RATE-LIMITED <countdown>" badge instead of their stage badge.
func renderRow(row issueRow, spin spinner.Model, width int, rateLimitedUntil *time.Time) string {
	marker := markerFor(row, spin)
	num := rowNumberStyle.Render(fmt.Sprintf("#%d", row.number))

	// Reserve space for badge (~22 chars with padding) and gutters.
	badgeReserved := 22
	titleWidth := width - 22 - badgeReserved
	if titleWidth < 10 {
		titleWidth = 10
	}
	title := rowTitleStyle.Render(truncate(row.title, titleWidth))

	badge := badgeFor(row, rateLimitedUntil)

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

	if row.traceID != "" {
		short := row.traceID
		if len(short) > 8 {
			short = short[:8]
		}
		line += " " + traceStyle.Render(short)
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
//   - in-progress + rate-limited   → purple "RATE-LIMITED 1h23m"
//   - in-progress (stage set)      → muted  stage name (e.g. "implement")
//   - queued (no status, no stage) → muted  "QUEUED"
func badgeFor(row issueRow, rateLimitedUntil *time.Time) string {
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
		if row.stage == "awaiting-merge" {
			return badgeAwaitingMergeStyle.Render("AWAITING MERGE")
		}
		if row.stage == "rate-limited" || (rateLimitedUntil != nil && row.stage != "") {
			label := "RATE-LIMITED"
			if rateLimitedUntil != nil {
				if cd := formatCountdown(*rateLimitedUntil); cd != "" {
					label = "RATE-LIMITED " + cd
				}
			}
			return badgeRateLimitedStyle.Render(label)
		}
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
		if row.stage == "awaiting-merge" {
			return markerAwaitingMergeStyle.Render(markerReview)
		}
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
