package actions

import "context"

// ============================================================================
// Message Types
// ============================================================================

// ResultMsg is returned when an async action completes
type ResultMsg struct {
	Output        string
	RepairActions []string // Optional: List of kgg commands suggested by AI
}

// ProgressMsg is sent when partial output is available during an async action
type ProgressMsg struct {
	Output string
}

// ActionStartedMsg is sent when an async action starts streaming output
type ActionStartedMsg struct {
	ProgressChan <-chan string
}

// ProgressFinishedMsg is sent when the progress channel is closed
type ProgressFinishedMsg struct{}

// ScanResultMsg is returned when network scan completes
type ScanResultMsg struct {
	Output string
}

// DashboardNodeSnapshot holds fetched metrics for a single node
type DashboardNodeSnapshot struct {
	Name   string
	CPU    float64
	Memory float64
	Disk   float64
	Error  string
}

// DashboardMsg carries the fetched metrics for the dashboard
type DashboardMsg struct {
	Nodes []DashboardNodeSnapshot
}

// ============================================================================
// Helpers
// ============================================================================

var activeTunnelCancel context.CancelFunc

// RegisterTunnel stores the cancel function for the active tunnel.
func RegisterTunnel(cancel context.CancelFunc) {
	if activeTunnelCancel != nil {
		activeTunnelCancel()
	}
	activeTunnelCancel = cancel
}

// StopActiveTunnel kills the active SSH tunnel if one exists.
func StopActiveTunnel() {
	if activeTunnelCancel != nil {
		activeTunnelCancel()
		activeTunnelCancel = nil
	}
}
