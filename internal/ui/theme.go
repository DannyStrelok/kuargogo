package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// KGGTheme implements the engine.Theme interface for the kuargogo application.
type KGGTheme struct {
	isDark bool
}

func (t *KGGTheme) SetDark(dark bool) {
	t.isDark = dark
}

func (t KGGTheme) PrimaryColor() color.Color {
	return lipgloss.Color("#C9D1D9") // Primary Text
}

func (t KGGTheme) SecondaryColor() color.Color {
	return lipgloss.Color("#8B949E") // Secondary Text
}

func (t KGGTheme) AccentColor() color.Color {
	return lipgloss.Color("#58A6FF") // DevOps Blue
}

func (t KGGTheme) SurfaceColor() color.Color {
	return lipgloss.Color("#161B22") // Panels
}

func (t KGGTheme) BorderColor() color.Color {
	return lipgloss.Color("#30363D") // Borders
}

func (t KGGTheme) BackgroundColor() color.Color {
	return lipgloss.Color("#0D1117") // Deep Dark Background
}

func (t KGGTheme) ErrorColor() color.Color {
	return lipgloss.Color("#F85149")
}

func (t KGGTheme) SuccessColor() color.Color {
	return lipgloss.Color("#2EA043")
}

func (t KGGTheme) WarningColor() color.Color {
	return lipgloss.Color("#D29922")
}

func (t KGGTheme) InfoColor() color.Color {
	return lipgloss.Color("#58A6FF")
}

func (t KGGTheme) TextColor() color.Color {
	return t.PrimaryColor()
}

func (t KGGTheme) MutedColor() color.Color {
	return lipgloss.Color("#6E7681")
}

func (t KGGTheme) Banner() string {
	return `
   █  █ █  █ █▀▀█ █▀▀█ █▀▀█ █▀▀█ █▀▀█ █▀▀█
   █▄▄▀ █  █ █▄▄█ █▄▄▀ █ ▄▄ █  █ █ ▄▄ █  █
   █  █ ▀▄▄▀ █  █ █  █ █▄▄█ ▀▄▄▀ █▄▄█ ▀▄▄▀`
}

func (t KGGTheme) BannerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.AccentColor()).Bold(true)
}

func (t KGGTheme) SubtitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.MutedColor()).Italic(true)
}

func (t KGGTheme) BreadcrumbStyle() lipgloss.Style {
	// Violet variant for navigation as proposed
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
}

func (t KGGTheme) DocStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Padding(1, 4).
		Foreground(t.TextColor())
}
