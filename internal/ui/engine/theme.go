package engine

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme encapsulates the core colors and styles used by the generic UI Engine.
// This allows the Engine to be agnostic to the specific branding of the CLI.
type Theme interface {
	// Base Colors
	PrimaryColor() color.Color
	SecondaryColor() color.Color
	AccentColor() color.Color
	SurfaceColor() color.Color
	BorderColor() color.Color
	BackgroundColor() color.Color
	ErrorColor() color.Color
	SuccessColor() color.Color
	WarningColor() color.Color
	InfoColor() color.Color
	TextColor() color.Color
	MutedColor() color.Color

	// Core UI Elements
	Banner() string
	BannerStyle() lipgloss.Style
	SubtitleStyle() lipgloss.Style
	BreadcrumbStyle() lipgloss.Style
	DocStyle() lipgloss.Style

	// Reactive overrides
	SetDark(bool)
}
