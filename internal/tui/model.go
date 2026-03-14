package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// autoMerge holds the feature and rollup branch names used for auto-merge.
// A nil pointer means auto-merge is not configured.
type autoMerge struct {
	feature string
	rollup  string
}

// Model is the root Bubble Tea model for the dark-factory TUI.
//
// It holds run metadata displayed in the header chrome, aggregate issue counts
// displayed in the summary bar, and terminal dimensions for layout calculations.
// Dynamic issue-table content is added in a subsequent issue (#443).
type Model struct {
	// Run metadata — populated via Update messages.
	repo         string
	milestone    string
	timestamp    string
	baseBranch   string
	mergeFeature string
	mergeRollup  string
	autoMerge    *autoMerge

	// Aggregate counts — updated as issues progress.
	merged    int
	inReview  int
	queued    int
	failed    int
	totalCost float64

	// Terminal dimensions.
	width  int
	height int
}

// Compile-time assertion that Model implements tea.Model.
var _ tea.Model = Model{}

// Init implements tea.Model. No initial command is needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Handles window-size messages and will handle
// progress messages once they are defined in issue #443.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// View implements tea.Model. Composes the header, a placeholder for the issue
// table (added in #443), and the summary bar.
func (m Model) View() string {
	header := renderHeader(m)
	summary := renderSummary(m)
	// Issue table placeholder — replaced in #443.
	return header + "\n\n" + summary + "\n"
}

// New returns a Model pre-populated with run metadata.
//
// mergeFeature and mergeRollup may be empty strings when auto-merge is not
// configured. baseBranch may be empty when using the repository default.
func New(repo, milestone, timestamp, baseBranch, mergeFeature, mergeRollup string) Model {
	m := Model{
		repo:         repo,
		milestone:    milestone,
		timestamp:    timestamp,
		baseBranch:   baseBranch,
		mergeFeature: mergeFeature,
		mergeRollup:  mergeRollup,
	}
	if mergeFeature != "" || mergeRollup != "" {
		m.autoMerge = &autoMerge{
			feature: mergeFeature,
			rollup:  mergeRollup,
		}
	}
	return m
}
