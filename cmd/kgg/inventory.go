package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"

	"charm.land/lipgloss/v2"
	"github.com/DannyStrelok/kuargogo/internal/inventory"
	"github.com/spf13/cobra"
)

var visualMode bool

var inventoryCmd = &cobra.Command{
	Use:   "inventory",
	Short: "List all managed nodes",
	Run: func(cmd *cobra.Command, args []string) {
		if visualMode {
			renderVisualInventory()
			return
		}

		entries := inventory.GetInventory()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		if _, err := fmt.Fprintln(w, "STATUS\tNAME\tIP\tROLE\tARCH\tPOSITION"); err != nil {
			log.Printf("Warning: failed to write header: %v", err)
		}
		for _, e := range entries {
			status := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("OFFLINE")
			if e.IsOnline {
				status = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("ONLINE")
			}

			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				status, e.Name, e.IP, e.Role, e.Arch, e.Position); err != nil {
				log.Printf("Warning: failed to write node: %v", err)
			}
		}
		if err := w.Flush(); err != nil {
			log.Printf("Warning: failed to flush output: %v", err)
		}
	},
}

func init() {
	inventoryCmd.Flags().BoolVarP(&visualMode, "visual", "v", false, "Show visual rack representation")
	rootCmd.AddCommand(inventoryCmd)
}

func renderVisualInventory() {
	// Styles
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Width(30).
		Align(lipgloss.Center)

	onlineBoxStyle := boxStyle.BorderForeground(lipgloss.Color("2"))
	offlineBoxStyle := boxStyle.BorderForeground(lipgloss.Color("1"))

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	nodeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	// Buckets
	leftNodes := []string{}
	centerNodes := []string{}
	rightNodes := []string{}

	// Flags to track column status
	leftOnline := false
	centerOnline := false
	rightOnline := false

	entries := inventory.GetInventory()
	for _, e := range entries {
		statusIndicator := "🔴"
		if e.IsOnline {
			statusIndicator = "🟢"
		}
		entry := fmt.Sprintf("%s %s\n(%s)", statusIndicator, e.Name, e.IP)

		switch strings.ToLower(e.Position) {
		case "left":
			leftNodes = append(leftNodes, entry)
			if e.IsOnline {
				leftOnline = true
			}
		case "right":
			rightNodes = append(rightNodes, entry)
			if e.IsOnline {
				rightOnline = true
			}
		default: // center
			centerNodes = append(centerNodes, entry)
			if e.IsOnline {
				centerOnline = true
			}
		}
	}

	// Render Columns
	renderColumn := func(title string, nodes []string, isOnline bool) string {
		content := titleStyle.Render(title) + "\n\n"
		if len(nodes) == 0 {
			content += nodeStyle.Render("Empty Slot")
		} else {
			content += nodeStyle.Render(strings.Join(nodes, "\n\n"))
		}

		style := offlineBoxStyle
		if isOnline {
			style = onlineBoxStyle
		}
		return style.Render(content)
	}

	leftCol := renderColumn("HP ProDesk (Left)", leftNodes, leftOnline)
	centerCol := renderColumn("Lenovo Cluster (Center)", centerNodes, centerOnline)
	rightCol := renderColumn("Control Plane (Right)", rightNodes, rightOnline)

	// Combine
	rack := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, centerCol, rightCol)
	if _, err := fmt.Fprintln(os.Stdout, rack); err != nil {
		log.Printf("Warning: failed to print rack: %v", err)
	}
}
