package models

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/DannyStrelok/kuargogo/internal/inventory"
	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
)

type InventoryModel struct {
	BaseModel
	spinner spinner.Model
	loading bool
	entries []inventory.NodeEntry
	cursor  int
	width   int
	height  int
}

func NewInventoryModel() *InventoryModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	// Color will be set in View

	m := &InventoryModel{
		spinner: sp,
		loading: true,
		cursor:  0,
	}
	m.SetTitle("Cluster Inventory")
	return m
}

func (m *InventoryModel) MouseMode() tea.MouseMode {
	return tea.MouseModeCellMotion
}

func (m *InventoryModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, FetchInventoryCmd())
}

func (m *InventoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.MarkReady()
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case LoadedInventoryMsg:
		m.loading = false
		m.entries = msg.Entries
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case RefreshDataMsg, engine.ConfigReloadedMsg, engine.ResumeMsg:
		m.loading = true
		m.entries = nil
		return m, FetchInventoryCmd()

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.MouseWheelDown:
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		}
		return m, nil

	case tea.MouseClickMsg:
		// Calculate row index relative to table start
		// \n (1) + Top Border (1) + Header (1) = 3 lines offset
		targetRow := msg.Y - 3
		if targetRow >= 0 && targetRow < len(m.entries) {
			m.cursor = targetRow
		}
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "esc" || msg.String() == "q" {
			return m, engine.Pop()
		}
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case "enter":
			if !m.loading && len(m.entries) > 0 {
				if m.cursor >= 0 && m.cursor < len(m.entries) {
					node := m.entries[m.cursor].Node
					return m, engine.Push(NewMenuModel(BuildNodeOpsMenu(node), nil))
				}
			}
		}
	}
	return m, nil
}

func (m *InventoryModel) View() tea.View {
	if !m.IsReady() {
		return m.LoadingView()
	}

	if m.loading && len(m.entries) == 0 {
		m.spinner.Style = lipgloss.NewStyle().Foreground(m.theme.AccentColor())
		return tea.NewView(fmt.Sprintf("\n  %s Loading inventory...", m.spinner.View()))
	}

	if len(m.entries) == 0 {
		style := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(m.theme.BorderColor()).
			Padding(1, 2).
			Foreground(m.theme.MutedColor())

		msg := "No nodes configured in kuargogo.yaml.\nUse 'Add Node' to get started."
		return tea.NewView("\n" + style.Render(msg))
	}

	// Build Professional Table using the theme's border palette
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(m.theme.BorderColor())).
		Headers("STATUS", "NAME", "IP ADDRESS", "ROLE", "ARCH")

	for i, e := range m.entries {
		status := "● OFF"
		statusStyle := lipgloss.NewStyle().Foreground(m.theme.ErrorColor())
		if e.IsOnline {
			status = "● ON"
			statusStyle = lipgloss.NewStyle().Foreground(m.theme.SuccessColor())
		}

		rowStyle := lipgloss.NewStyle()
		if i == m.cursor {
			rowStyle = rowStyle.
				Background(m.theme.AccentColor()).
				Foreground(m.theme.BackgroundColor()). // Contrast text on accent
				Bold(true)
		}

		t.Row(
			statusStyle.Render(status),
			rowStyle.Render(e.Name),
			rowStyle.Render(e.IP),
			rowStyle.Render(e.Role),
			rowStyle.Render(e.Arch),
		)
	}

	// Full width table
	t.Width(m.width)

	return tea.NewView("\n" + t.Render())
}

// LoadedInventoryMsg contains the results of the background fetch
type LoadedInventoryMsg struct {
	Entries []inventory.NodeEntry
	Err     error
}

// FetchInventoryCmd creates a command to fetch inventory
func FetchInventoryCmd() tea.Cmd {
	return func() tea.Msg {
		entries := inventory.GetInventory()
		return LoadedInventoryMsg{Entries: entries}
	}
}

type RefreshDataMsg struct{}
