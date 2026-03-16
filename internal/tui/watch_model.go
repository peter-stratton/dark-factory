package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// WatchPRRow holds display state for a single PR in the watch table.
type WatchPRRow struct {
	Number int
	Title  string
	Labels []string
}

// PollTickMsg is sent periodically to trigger a new poll cycle.
type PollTickMsg struct {
	Time time.Time
}

// PRUpdateMsg carries a refreshed list of watched PRs.
type PRUpdateMsg struct {
	PRs []WatchPRRow
}

// ActivityMsg appends a new entry to the activity log.
type ActivityMsg struct {
	Text string
}

// WatchDoneMsg is sent when the watch loop has stopped. The TUI stays on
// screen so the user can review the log; pressing q/esc/ctrl+c exits.
type WatchDoneMsg struct{}

// WatchModel is the Bubble Tea model for the watch command TUI.
type WatchModel struct {
	// Run metadata.
	repo     string
	interval time.Duration
	cancelFn func()

	// Poll state.
	lastPoll time.Time
	prs      []WatchPRRow

	// Activity log — last 10 entries kept.
	activity []string

	// Run lifecycle.
	done       bool
	cancelling bool

	// Terminal dimensions.
	width  int
	height int
}

// Compile-time assertion that WatchModel implements tea.Model.
var _ tea.Model = WatchModel{}

// NewWatchModel returns a WatchModel pre-populated with run metadata.
//
// cancelFn is called when the user presses ctrl+c to stop the watch loop.
// It may be nil (cancellation disabled).
func NewWatchModel(repo string, interval time.Duration, cancelFn func()) WatchModel {
	return WatchModel{
		repo:     repo,
		interval: interval,
		cancelFn: cancelFn,
	}
}

// Init implements tea.Model. Returns nil — all PR updates are driven by
// PRUpdateMsg messages sent from the external watchTUIPoller goroutine in cmd.
// There is no internal tick; keeping one would schedule no-op messages for
// the entire lifetime of the program.
func (m WatchModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Handles window-size messages and all watch
// message types.
func (m WatchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case PollTickMsg:
		// No-op: the internal timer was removed. PollTickMsg may still be
		// sent by external callers; handled here to prevent unmatched messages.

	case PRUpdateMsg:
		m.prs = msg.PRs
		m.lastPoll = time.Now()

	case ActivityMsg:
		m.activity = append(m.activity, msg.Text)
		if len(m.activity) > 10 {
			m.activity = m.activity[len(m.activity)-10:]
		}

	case WatchDoneMsg:
		m.done = true
	}

	return m, nil
}

// View implements tea.Model. Composes the header, PR table, activity log,
// and footer.
func (m WatchModel) View() string {
	return renderWatchView(m)
}

// handleKey processes key presses. During an active watch, ctrl+c cancels.
// Once done, q/esc/ctrl+c exit the TUI.
func (m WatchModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.done {
			return m, tea.Quit
		}
		if !m.cancelling && m.cancelFn != nil {
			m.cancelling = true
			m.cancelFn()
		}
	case "q", "esc":
		if m.done {
			return m, tea.Quit
		}
	}
	return m, nil
}
