package engine

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/i18n"
)

type EngineModel interface {
	tea.Model
	Title() string
	ShowBreadcrumbs() bool
	ApplyTheme(Theme)
	MouseMode() tea.MouseMode
}

// ============================================================================
// Core Engine (Stack Router)
// ============================================================================

// Engine manages the stack of models, global error handling, and declarative V2 rendering.
type Engine struct {
	name        string
	stack       []EngineModel
	theme       Theme
	breadcrumbs []string
	width       int
	height      int
	errorMode   bool
	errorMsg    error
	exitDialog  *ConfirmationDialog
}

func NewEngine(root EngineModel, theme Theme, name string) Engine {
	root.ApplyTheme(theme)
	return Engine{
		name:        name,
		stack:       []EngineModel{root},
		theme:       theme,
		breadcrumbs: []string{root.Title()},
	}
}

func (e Engine) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor}
	if len(e.stack) > 0 {
		cmds = append(cmds, e.stack[0].Init())
	}
	return tea.Batch(cmds...)
}

func (e Engine) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// 1. Intercept Global Window Size
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		e.width = size.Width
		e.height = size.Height

		// Overwrite the message with adjusted dimensions for the child models
		// This ensures they render within the safe area and don't push the legend off-screen
		msg = e.calculateAdjustedSize()
	}

	// 1.5 Intercept Background Color Detection
	if msg, ok := msg.(tea.BackgroundColorMsg); ok {
		e.theme.SetDark(msg.IsDark())
		for _, m := range e.stack {
			m.ApplyTheme(e.theme)
		}
	}

	// 2. Intercept Global Error Handler
	if err, ok := msg.(ErrorMsg); ok {
		e.errorMode = true
		e.errorMsg = err.Err
		return e, nil
	}

	// 3. Error Mode Navigation overrides
	if e.errorMode {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			if key.String() == "esc" || key.String() == "enter" || len(e.stack) == 0 {
				e.errorMode = false
				e.errorMsg = nil
				// If we have no stack under the error, quit.
				if len(e.stack) == 0 {
					return e, tea.Quit
				}
				return e, nil
			}
		}
		// Absorb all other messages while showing error
		return e, nil
	}

	// 3.5 Intercept Global Exit Modal
	if e.exitDialog != nil {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			switch key.String() {
			case "esc", "q":
				e.exitDialog = nil
				return e, nil
			case "left", "right", "tab":
				e.exitDialog.Toggle()
				return e, nil
			case "enter":
				if e.exitDialog.Yes {
					return e, tea.Quit
				}
				e.exitDialog = nil
				return e, nil
			}
		}
		// Absorb all other messages while showing exit modal
		return e, nil
	}

	// 4. Intercept Navigation Commands
	switch msg := msg.(type) {
	case PushMsg:
		modelToPush := msg.Model
		modelToPush.ApplyTheme(e.theme)
		cmds := []tea.Cmd{modelToPush.Init()}

		if e.width > 0 && e.height > 0 {
			adjustedSize := e.calculateAdjustedSize()
			var sizeCmd tea.Cmd
			var newBase tea.Model
			newBase, sizeCmd = modelToPush.Update(adjustedSize)
			if typed, ok := newBase.(EngineModel); ok {
				modelToPush = typed
			}
			if sizeCmd != nil {
				cmds = append(cmds, sizeCmd)
			}
		}

		newStack := make([]EngineModel, len(e.stack)+1)
		copy(newStack, e.stack)
		newStack[len(e.stack)] = modelToPush
		e.stack = newStack

		e.breadcrumbs = append(e.breadcrumbs, modelToPush.Title())
		return e, tea.Batch(cmds...)

	case PopMsg:
		if len(e.stack) > 1 {
			e.stack = e.stack[:len(e.stack)-1]
			e.breadcrumbs = e.breadcrumbs[:len(e.breadcrumbs)-1]

			top := e.stack[len(e.stack)-1]
			var cmd tea.Cmd
			var newBase tea.Model
			newBase, cmd = top.Update(ResumeMsg{})
			if typed, ok := newBase.(EngineModel); ok {
				e.stack[len(e.stack)-1] = typed
			}
			return e, cmd
		}
		// Trigger exit confirmation at root
		e.exitDialog = &ConfirmationDialog{Question: i18n.T("exit_warn")}
		return e, nil

	case ReplaceMsg:
		if len(e.stack) > 0 {
			modelToReplace := msg.Model
			modelToReplace.ApplyTheme(e.theme)
			cmds := []tea.Cmd{modelToReplace.Init()}

			if e.width > 0 && e.height > 0 {
				adjustedSize := e.calculateAdjustedSize()
				var sizeCmd tea.Cmd
				var newBase tea.Model
				newBase, sizeCmd = modelToReplace.Update(adjustedSize)
				if typed, ok := newBase.(EngineModel); ok {
					modelToReplace = typed
				}
				if sizeCmd != nil {
					cmds = append(cmds, sizeCmd)
				}
			}

			e.stack[len(e.stack)-1] = modelToReplace
			e.breadcrumbs[len(e.breadcrumbs)-1] = modelToReplace.Title()
			return e, tea.Batch(cmds...)
		}

	case PopToRootMsg:
		if len(e.stack) > 1 {
			e.stack = e.stack[:1]
			e.breadcrumbs = e.breadcrumbs[:1]
			top := e.stack[0]
			var cmd tea.Cmd
			var newBase tea.Model
			newBase, cmd = top.Update(ResumeMsg{})
			if typed, ok := newBase.(EngineModel); ok {
				e.stack[0] = typed
			}
			return e, cmd
		}
		return e, nil
	}

	// 5. Pass through to Top Model
	if len(e.stack) > 0 {
		topIdx := len(e.stack) - 1
		top := e.stack[topIdx]

		// Handle coordinate-aware messages (Mouse)
		if _, ok := msg.(tea.MouseMsg); ok {
			offset := e.headerHeight() + 1
			switch m := msg.(type) {
			case tea.MouseClickMsg:
				m.Y -= offset
				msg = m
			case tea.MouseReleaseMsg:
				m.Y -= offset
				msg = m
			case tea.MouseWheelMsg:
				m.Y -= offset
				msg = m
			case tea.MouseMotionMsg:
				m.Y -= offset
				msg = m
			}
		}

		newBase, cmd := top.Update(msg)
		if typed, ok := newBase.(EngineModel); ok {
			e.stack[topIdx] = typed
			// Update breadcrumb title in case it changed dynamically
			e.breadcrumbs[len(e.breadcrumbs)-1] = typed.Title()
		}
		return e, cmd
	}

	return e, nil
}

// View implements the V2 Declarative interface returning a tea.View struct.
func (e Engine) View() tea.View {
	if e.errorMode {
		return e.renderErrorView()
	}

	if len(e.stack) == 0 {
		v := tea.NewView("No view loaded.")
		v.WindowTitle = "Engine Empty"
		return v
	}

	top := e.stack[len(e.stack)-1]

	// 1. Render Background
	childView := top.View()
	content := childView.Content

	if top.ShowBreadcrumbs() {
		header := e.renderBreadcrumbs()
		content = lipgloss.JoinVertical(lipgloss.Left, header, "\n"+content)
	}

	// 2. Wrap in DocStyle
	styledContent := e.theme.DocStyle().Render(content)

	// 3. Render Overlay if needed
	if e.exitDialog != nil {
		modal := e.exitDialog.View(e.theme)

		// 1. Prepare Dimmed Background
		// We use a grayed-out style for everything behind the modal.
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		bgLines := strings.Split(styledContent, "\n")
		for i, line := range bgLines {
			bgLines[i] = dimStyle.Render(line)
		}

		modalLines := strings.Split(modal, "\n")
		mHeight := len(modalLines)
		mWidth := lipgloss.Width(modalLines[0])
		bgHeight := len(bgLines)

		if bgHeight <= mHeight {
			v := tea.NewView(modal)
			v.AltScreen = childView.AltScreen
			v.MouseMode = childView.MouseMode
			return v
		}

		// 2. Stamp Modal
		startX := (e.width - mWidth) / 2
		startY := (bgHeight - mHeight) / 2

		for i, mLine := range modalLines {
			targetY := startY + i
			if targetY >= 0 && targetY < bgHeight {
				bgLine := bgLines[targetY]

				// We want to overlay mLine on bgLines[targetY] at startX.
				// 1. Get left part of background (0 to startX)
				left := ansi.Truncate(bgLine, startX, "")

				// 2. The modal line itself (mLine)

				// 3. Get right part of background (from startX + mWidth to end)
				// TruncateLeft(s, n, tail) removes the first n columns.
				right := ansi.TruncateLeft(bgLine, startX+mWidth, "")

				// Join them: [Left BG] [Modal] [Right BG]
				bgLines[targetY] = left + mLine + right
			}
		}

		overlayed := strings.Join(bgLines, "\n")
		v := tea.NewView(overlayed)
		v.AltScreen = childView.AltScreen
		v.MouseMode = childView.MouseMode
		return v
	}

	// 4. Render the page content without forcing a terminal-wide background fill.
	// This ensures we respect the user's terminal background and transparency.
	fullScreen := lipgloss.NewStyle().
		Foreground(e.theme.TextColor()).
		Render(styledContent)

	finalView := tea.NewView(fullScreen)
	finalView.WindowTitle = e.name + " | " + top.Title()

	// 5. Force AltScreen & Use dynamic MouseMode for a specialized experience
	finalView.AltScreen = true
	finalView.MouseMode = top.MouseMode()

	return finalView
}

// Engine implements EngineModel itself to satisfy interface requirements uniformly
func (e Engine) Title() string {
	if len(e.stack) > 0 {
		return e.stack[len(e.stack)-1].Title()
	}
	return e.name
}

func (e Engine) ShowBreadcrumbs() bool {
	return false // The engine renders its own breadcrumbs around the inner content
}

func (e Engine) showBanner() bool {
	// Show banner only at the root (Main Menu) and if there's enough height
	return len(e.stack) == 1 && e.height >= 22
}

func (e Engine) renderBreadcrumbs() string {
	var header string

	// Show Global Context Indicator at the very top using the theme's Accent color
	ctxStyle := lipgloss.NewStyle().Foreground(e.theme.AccentColor())
	ctxLine := ctxStyle.Render(fmt.Sprintf("  [CTX: %s]", config.GetCurrentContext()))
	syncStatus := e.renderSyncStatus()

	if syncStatus != "" && e.width > 60 {
		// Combine Context and sync status on the same line (top line)
		paddingWidth := e.width - lipgloss.Width(ctxLine) - 4
		if paddingWidth > 0 {
			combined := lipgloss.JoinHorizontal(lipgloss.Top,
				ctxLine,
				lipgloss.PlaceHorizontal(paddingWidth, lipgloss.Right, syncSyncLabel(e.theme)+syncStatusValue(e, syncStatus)),
			)
			header = combined + "\n\n"
		} else {
			header = ctxLine + "\n\n"
		}
	} else {
		header = ctxLine + "\n\n"
	}

	// Show banner only if we are at the root (Main Menu) and have enough vertical space
	if e.showBanner() {
		header += e.theme.BannerStyle().Render(e.theme.Banner()) + "\n"
		header += e.theme.SubtitleStyle().Render("  Homelab Management CLI") + "\n"
	}

	// Start with "Main"
	crumbStr := "Main"
	hasRealCrumbs := false

	// Process stack-based breadcrumbs
	// Skip the first one if it's just the root title "kuargogo" to avoid "Main > kuargogo"
	for i, crumb := range e.breadcrumbs {
		if i == 0 && (crumb == e.name || crumb == "Main Menu") {
			continue
		}
		crumbStr += " > " + crumb
		hasRealCrumbs = true
	}

	// Omit breadcrumbs if we are at the root (Main)
	if !hasRealCrumbs {
		return header
	}

	header += e.theme.BreadcrumbStyle().Render("\n  "+crumbStr) + "\n"
	return header
}

func syncSyncLabel(theme Theme) string {
	return lipgloss.NewStyle().Foreground(theme.MutedColor()).Render("☁ Backup Sync: ")
}

func syncStatusValue(e Engine, status string) string {
	return lipgloss.NewStyle().Foreground(e.theme.AccentColor()).Render(status)
}

func (e Engine) headerHeight() int {
	h := 3 // CTX (2) + Subtitle (1)

	// Add banner height only if visible
	if e.showBanner() {
		h += 7
	}

	// Add breadcrumbs height only if we have sub-navigation
	hasRealCrumbs := false
	for i, crumb := range e.breadcrumbs {
		if i == 0 && (crumb == e.name || crumb == "Main Menu") {
			continue
		}
		hasRealCrumbs = true
		break
	}

	if hasRealCrumbs {
		h += 2 // \n + crumbStr line
	}

	return h
}

func (e Engine) renderSyncStatus() string {
	sync := config.RootConfigGetSync()
	if sync.LastSync == "" {
		return ""
	}

	// Try to parse as RFC3339 for a prettier display
	displayTime := sync.LastSync
	if t, err := time.Parse(time.RFC3339, sync.LastSync); err == nil {
		displayTime = t.Format("2006-01-02 15:04:05")
	}

	return displayTime
}

func (e Engine) renderErrorView() tea.View {
	errStyle := lipgloss.NewStyle().Foreground(e.theme.ErrorColor()).Bold(true)
	msg := errStyle.Render("CRITICAL ERROR") + "\n\n"
	if e.errorMsg != nil {
		msg += e.errorMsg.Error()
	} else {
		msg += "Unknown engine failure."
	}
	msg += "\n\nPress Esc or Enter to dismiss."

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(e.theme.ErrorColor()).
		Padding(1, 4).
		Width(e.width / 2).
		Align(lipgloss.Center)

	centered := lipgloss.Place(e.width, e.height, lipgloss.Center, lipgloss.Center, style.Render(msg))
	v := tea.NewView(centered)
	v.WindowTitle = "Error - " + e.name
	return v
}

func (e Engine) calculateAdjustedSize() tea.WindowSizeMsg {
	// Padding(1, 4) from DocStyle -> 2 vertical (1 top + 1 bottom), 8 horizontal (4 left + 4 right)
	// We add an extra -1 as a safety buffer for terminal window bars to keep the legend visible
	w := e.width - 8
	h := e.height - e.headerHeight() - 2 - 1

	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return tea.WindowSizeMsg{
		Width:  w,
		Height: h,
	}
}
