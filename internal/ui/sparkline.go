package ui

import (
	"fmt"
	"math"
	"strings"
	"image/color"

	"charm.land/lipgloss/v2"
)

// Bar block characters from empty to full (8 levels)
var barBlocks = []string{"░", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}

// Color thresholds
var (
	colorGreen  = lipgloss.Color("#22c55e") // 0-60%
	colorYellow = lipgloss.Color("#eab308") // 60-80%
	colorRed    = lipgloss.Color("#ef4444") // 80-100%
	colorDim    = lipgloss.Color("#525252") // Empty part
)

// RenderBar converts a 0-100 percentage into a colored bar.
// width is the number of character cells the bar occupies.
func RenderBar(percent float64, width int) string {
	if math.IsNaN(percent) {
		percent = 0
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	// Determine fill color based on severity
	var fillColor color.Color
	switch {
	case percent >= 80:
		fillColor = colorRed
	case percent >= 60:
		fillColor = colorYellow
	default:
		fillColor = colorGreen
	}

	fillStyle := lipgloss.NewStyle().Foreground(fillColor)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)

	// Calculate how many full blocks and the partial block
	filledCells := percent / 100.0 * float64(width)
	fullBlocks := int(filledCells)
	partialIdx := int((filledCells - float64(fullBlocks)) * 8)

	if fullBlocks > width {
		fullBlocks = width
	}

	var sb strings.Builder

	// Full blocks
	for i := 0; i < fullBlocks && i < width; i++ {
		sb.WriteString(fillStyle.Render("█"))
	}

	// Partial block
	if fullBlocks < width && partialIdx > 0 {
		sb.WriteString(fillStyle.Render(barBlocks[partialIdx]))
		fullBlocks++ // account for partial cell
	}

	// Empty blocks
	for i := fullBlocks; i < width; i++ {
		sb.WriteString(dimStyle.Render("░"))
	}

	return sb.String()
}

// FormatPercent returns a right-aligned percentage string
func FormatPercent(percent float64) string {
	if math.IsNaN(percent) {
		return "  N/A"
	}
	return fmt.Sprintf("%5.1f%%", percent)
}
