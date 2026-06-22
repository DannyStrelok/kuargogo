package models

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
	"github.com/DannyStrelok/kuargogo/internal/ui/menu/actions"
)

// ListModel delegates generic list behavior and styling.
type MenuModel struct {
	BaseModel
	list        list.Model
	currentNode MenuNode
	breadcrumbs []string
}

func NewMenuModel(node MenuNode, crumbs []string) *MenuModel {
	// We'll update delegate styles in ApplyTheme once we have the theme
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Title = node.Title
	l.SetShowTitle(false) // We render our own header

	m := &MenuModel{
		list:        l,
		currentNode: node,
		breadcrumbs: crumbs,
	}
	m.SetTitle(node.Title)
	m.updateListItems()
	return m
}

func (m *MenuModel) MouseMode() tea.MouseMode {
	return tea.MouseModeCellMotion
}

func (m *MenuModel) ApplyTheme(t engine.Theme) {
	m.theme = t

	// Update list delegate styles for a 'terminal moderna' look
	d := list.NewDefaultDelegate()

	// Use Accent color for selected items instead of default pink
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.
		Foreground(t.AccentColor()).
		BorderLeft(true).
		BorderForeground(t.AccentColor())

	d.Styles.SelectedDesc = d.Styles.SelectedDesc.
		Foreground(t.AccentColor()).
		BorderLeft(true).
		BorderForeground(t.AccentColor())

	d.Styles.NormalTitle = d.Styles.NormalTitle.Foreground(t.PrimaryColor())
	d.Styles.NormalDesc = d.Styles.NormalDesc.Foreground(t.MutedColor())

	m.list.SetDelegate(d)
}

func (m *MenuModel) Init() tea.Cmd {
	return nil
}

func (m *MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.MarkReady()
		// msg is already adjusted by the engine for the header/footer
		m.list.SetWidth(msg.Width)
		// Subtract the context header in MenuModel (approx 3 lines) to avoid overflow
		m.list.SetHeight(msg.Height - 3)
		return m, nil

	case engine.ConfigReloadedMsg, engine.ResumeMsg:
		m.updateListItems()
		return m, nil

	case actions.ContextSwitchedMsg:
		m.updateListItems()
		return m, engine.PopToRoot()

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "enter" {
			selected := m.list.SelectedItem()
			if selected == nil {
				return m, nil
			}
			it := selected.(menuItem)

			// Action?
			if it.node.Action != nil {
				// Execute the action - it returns a tea.Cmd (usually a Push)
				return m, it.node.Action()
			}

			// Submenu?
			if len(it.node.Children) > 0 || it.node.DynamicChildren != nil {
				sub := NewMenuModel(it.node, nil) // crumbs managed by engine now
				// We need to PUSH onto engine stack.
				return m, engine.Push(sub)
			}
		}

		// Back navigation
		if msg.String() == "esc" || msg.String() == "q" {
			// Engine stack will handle quitting if it's the root pop.
			return m, engine.Pop()
		}
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *MenuModel) View() tea.View {
	return tea.NewView(m.list.View())
}

// Helpers
type menuItem struct {
	node MenuNode
}

func (i menuItem) Title() string       { return i.node.Title }
func (i menuItem) Description() string { return i.node.Description }
func (i menuItem) FilterValue() string { return i.node.Title }

func (m *MenuModel) updateListItems() {
	items := []list.Item{}
	for _, child := range m.currentNode.Children {
		items = append(items, menuItem{node: child})
	}
	if m.currentNode.DynamicChildren != nil {
		for _, child := range m.currentNode.DynamicChildren() {
			items = append(items, menuItem{node: child})
		}
	}
	m.list.SetItems(items)
}
