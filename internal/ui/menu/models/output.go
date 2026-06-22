package models

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"strings"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
	"github.com/DannyStrelok/kuargogo/internal/ui/menu/actions"
)

type OutputModel struct {
	BaseModel
	spinner       spinner.Model
	viewport      viewport.Model
	output        string
	running       bool
	taskCmd       tea.Cmd
	AutoScroll    bool
	progressChan  <-chan string
	startTime     time.Time
	finalDuration time.Duration
}

type timerMsg time.Time

func NewOutputModel(task tea.Cmd) *OutputModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	vp := viewport.New()

	m := &OutputModel{
		spinner:    sp,
		viewport:   vp,
		running:    true,
		taskCmd:    task,
		AutoScroll: true,
	}
	m.viewport.MouseWheelEnabled = true
	m.SetTitle("Process Output")
	return m
}

func (m *OutputModel) WithAutoScroll(v bool) *OutputModel {
	m.AutoScroll = v
	return m
}

func (m *OutputModel) Init() tea.Cmd {
	// Only start the spinner tick initially.
	// The stopwatch will start when the actual task starts (ActionStartedMsg).
	return tea.Batch(m.spinner.Tick, m.taskCmd)
}

func (m *OutputModel) MouseMode() tea.MouseMode {
	// Enable mouse to allow scrollbar clicking and wheel support.
	// NOTE: Users may need to hold Shift to perform native terminal text selection.
	return tea.MouseModeAllMotion
}

func (m *OutputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.MarkReady()
		// Reserve 1 column for scrollbar
		m.viewport.SetWidth(msg.Width - 3)
		m.viewport.SetHeight(msg.Height - 1)
		m.refreshViewport()
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "q":
			actions.StopActiveTunnel()
			return m, engine.Pop()
		}

	case tea.MouseClickMsg:
		// Check if click is in the scrollbar area (right edge)
		if m.IsReady() && msg.X >= m.viewport.Width() {
			trackHeight := m.viewport.Height()
			contentHeight := m.viewport.TotalLineCount()
			if trackHeight > 0 && contentHeight > trackHeight {
				// Map Y click coordinate to proportional content offset
				scrollRatio := float64(msg.Y) / float64(trackHeight)
				targetY := int(scrollRatio * float64(contentHeight))
				m.viewport.SetYOffset(targetY)
				m.AutoScroll = false // Disable autoscroll on manual interaction
			}
			return m, nil
		}

	case spinner.TickMsg:
		if m.running {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case timerMsg:
		if m.running {
			return m, m.tick()
		}
		return m, nil

	case actions.ActionStartedMsg:
		m.running = true
		m.progressChan = msg.ProgressChan
		m.startTime = time.Now()
		return m, tea.Batch(m.tick(), m.waitForProgress())

	case string: // Generic output message
		m.output += msg + "\n"
		m.refreshViewport()
		if m.AutoScroll {
			m.viewport.GotoBottom()
		}
		return m, nil

	case actions.ProgressMsg:
		m.output += msg.Output + "\n"
		m.refreshViewport()
		if m.AutoScroll {
			m.viewport.GotoBottom()
		}
		return m, m.waitForProgress()

	case actions.ProgressFinishedMsg:
		m.running = false
		m.finalDuration = time.Since(m.startTime)
		return m, nil

	case actions.ResultMsg:
		m.running = false
		m.finalDuration = time.Since(m.startTime)
		if msg.Output != "" {
			m.output += "\n" + msg.Output + "\n"
		} else {
			m.output += "\n✅ Done."
		}
		m.refreshViewport()
		return m, nil

	case actions.ContextSwitchedMsg:
		m.running = false
		m.finalDuration = time.Since(m.startTime)
		return m, engine.PopToRoot()

	case error: // Process finished with error
		m.running = false
		m.finalDuration = time.Since(m.startTime)
		m.output += "\n❌ Error: " + msg.Error()
		m.refreshViewport()
		return m, nil

	case bool: // Process finished successfully
		m.running = false
		m.finalDuration = time.Since(m.startTime)
		m.output += "\n✅ Finished successfully."
		m.refreshViewport()
		return m, nil
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *OutputModel) View() tea.View {
	if !m.IsReady() {
		return m.LoadingView()
	}

	var header string
	var d time.Duration
	if m.running {
		d = time.Since(m.startTime)
	} else {
		d = m.finalDuration
	}

	// Format duration as MM:SS or SSs
	var elapsed string
	if d.Minutes() >= 1 {
		elapsed = fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
	} else {
		elapsed = fmt.Sprintf("%ds", int(d.Seconds()))
	}

	if m.running && m.output == "" {
		if m.theme != nil {
			m.spinner.Style = lipgloss.NewStyle().Foreground(m.theme.AccentColor())
		} else {
			// Fallback for safety during initialization/navigation edge cases
			m.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
		}
		return tea.NewView(fmt.Sprintf("%s Executing task on cluster...", m.spinner.View()))
	}

	if m.running {
		header = fmt.Sprintf("%s Processing... [%s]", m.spinner.View(), elapsed)
	} else {
		header = fmt.Sprintf("Process Finished in %s", elapsed)
	}

	headerStyle := lipgloss.NewStyle().Padding(0, 1)
	content := lipgloss.JoinHorizontal(lipgloss.Top,
		m.viewport.View(),
		m.renderScrollbar(),
	)

	return tea.NewView(headerStyle.Render(header) + "\n" + content)
}

func (m *OutputModel) refreshViewport() {
	wrapped := lipgloss.NewStyle().Width(m.viewport.Width()).Render(m.output)
	m.viewport.SetContent(wrapped)
}

func (m *OutputModel) renderScrollbar() string {
	trackHeight := m.viewport.Height()
	if trackHeight <= 0 {
		return ""
	}

	contentHeight := m.viewport.TotalLineCount()
	if contentHeight <= trackHeight {
		// Content fits, show empty track or handle gracefully
		trackStyle := lipgloss.NewStyle().Foreground(m.theme.BorderColor())
		return strings.Repeat(trackStyle.Render("┃")+"\n", trackHeight-1) + trackStyle.Render("┃")
	}

	// Calculate proportional thumb height
	thumbHeight := int(float64(trackHeight) * float64(trackHeight) / float64(contentHeight))
	if thumbHeight < 1 {
		thumbHeight = 1
	}

	// Calculate offset mapping YOffset [0...Total-Height] to [0...trackHeight-thumbHeight]
	maxScroll := contentHeight - trackHeight
	if maxScroll <= 0 {
		maxScroll = 1
	}
	scrollRatio := float64(m.viewport.YOffset()) / float64(maxScroll)
	thumbStart := int(scrollRatio * float64(trackHeight-thumbHeight))

	trackStyle := lipgloss.NewStyle().Foreground(m.theme.BorderColor())
	thumbStyle := lipgloss.NewStyle().Foreground(m.theme.AccentColor())

	var sb strings.Builder
	for i := 0; i < trackHeight; i++ {
		if i >= thumbStart && i < thumbStart+thumbHeight {
			sb.WriteString(thumbStyle.Render("█"))
		} else {
			sb.WriteString(trackStyle.Render("┃"))
		}
		if i < trackHeight-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
func (m *OutputModel) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return timerMsg(t)
	})
}

func (m *OutputModel) waitForProgress() tea.Cmd {
	return func() tea.Msg {
		if m.progressChan == nil {
			return nil
		}
		out, ok := <-m.progressChan
		if !ok {
			return actions.ProgressFinishedMsg{}
		}
		return actions.ProgressMsg{Output: out}
	}
}
