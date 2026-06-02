package engine

import (
	"charm.land/lipgloss/v2"
)

// ConfirmationDialog is a small state-helper for rendering a Yes/No modal.
// It is managed by the Engine to intercept exit requests.
type ConfirmationDialog struct {
	Question string
	Yes      bool // Current selection
}

func (d *ConfirmationDialog) Toggle() {
	d.Yes = !d.Yes
}

// View computes the styled modal box string.
func (d ConfirmationDialog) View(theme Theme) string {
	accent := theme.AccentColor()
	surface := theme.SurfaceColor()
	bg := theme.BackgroundColor()
	muted := theme.MutedColor()

	// Title / Question
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.PrimaryColor()).
		Bold(true).
		MarginBottom(1).
		Align(lipgloss.Center)

	title := titleStyle.Width(40).Render(d.Question)

	// Buttons
	yesStyle := lipgloss.NewStyle().
		Padding(0, 3).
		MarginRight(2)

	noStyle := lipgloss.NewStyle().
		Padding(0, 3)

	if d.Yes {
		yesStyle = yesStyle.
			Background(accent).
			Foreground(bg).
			Bold(true)
		noStyle = noStyle.
			Background(surface).
			Foreground(muted)
	} else {
		yesStyle = yesStyle.
			Background(surface).
			Foreground(muted)
		noStyle = noStyle.
			Background(accent).
			Foreground(bg).
			Bold(true)
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center,
		yesStyle.Render("Yes"),
		noStyle.Render("No"),
	)

	// Combine into modal box using professional NormalBorder and Surface background
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.BorderColor()).
		Background(theme.SurfaceColor()).
		Padding(1, 4).
		Width(50).
		Align(lipgloss.Center)

	return boxStyle.Render(lipgloss.JoinVertical(lipgloss.Center, title, buttons))
}
