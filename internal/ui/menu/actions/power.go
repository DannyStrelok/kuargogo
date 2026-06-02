package actions

import (
	"bytes"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/hardware"
	"github.com/DannyStrelok/kuargogo/internal/provision"
)

// -- Core Execution Logic (Shared between single and bulk actions) --

func executePowerOn(n config.Node) (string, error) {
	if n.MAC == "" {
		return "", fmt.Errorf("no MAC address configured for WoL")
	}
	if config.IsDryRun() {
		return fmt.Sprintf("[DRY-RUN] Sending Magic Packet to %s (%s)", n.Name, n.MAC), nil
	}
	err := hardware.WakeOnLAN(n.MAC, n.IP)
	if err != nil {
		return "", fmt.Errorf("WoL Error: %w", err)
	}
	return "✅ Magic Packet Sent Successfully.", nil
}

func executePowerControl(n config.Node, action provision.PowerAction) (string, error) {
	if config.IsDryRun() {
		return fmt.Sprintf("[DRY-RUN] Executing %s", action), nil
	}

	cfg := config.GetConfig()
	kp, err := cfg.SSH.ExpandedKeyPath()
	if err != nil {
		return "", fmt.Errorf("error expanding key path: %w", err)
	}
	user := n.User
	if user == "" {
		user = "kgg-admin"
	}

	executor, err := provision.NewExecutor(user, kp, false)
	if err != nil {
		return "", fmt.Errorf("executor error: %w", err)
	}

	var stderrBuf bytes.Buffer
	executor.Stdout = io.Discard
	executor.Stderr = &stderrBuf

	out, err := executor.RemotePowerControl(n.IP, cfg.SSH.Port, action)
	if err != nil {
		return "", fmt.Errorf("execution error: %v\n%s\n%s", err, out, stderrBuf.String())
	}
	return out, nil
}

// -- BubbleTea UI Actions --

// PowerOnAction sends a Wake-on-LAN magic packet to the node
func PowerOnAction(n config.Node) tea.Cmd {
	return func() tea.Msg {
		out, err := executePowerOn(n)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v", err)}
		}
		return ResultMsg{Output: out}
	}
}

// PowerControl performs power actions (off, reboot) via SSH
func PowerControl(n config.Node, action provision.PowerAction) tea.Cmd {
	return func() tea.Msg {
		out, err := executePowerControl(n, action)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v", err)}
		}
		return ResultMsg{Output: out}
	}
}

// BulkPowerOnAction sends Wake-on-LAN magic packets to multiple nodes
func BulkPowerOnAction(nodes []config.Node) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		fmt.Fprintln(&buf, "⚡ Bulk Power On (WoL) Report:")
		fmt.Fprintln(&buf, "================================")

		for _, n := range nodes {
			out, err := executePowerOn(n)
			if err != nil {
				fmt.Fprintf(&buf, "❌ %-15s: %v\n", n.Name, err)
			} else {
				fmt.Fprintf(&buf, "✅ %-15s: %s\n", n.Name, out)
			}
		}

		return ResultMsg{Output: buf.String()}
	}
}

// BulkPowerControl performs power actions (off, reboot) on multiple nodes via SSH
func BulkPowerControl(nodes []config.Node, action provision.PowerAction) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		actionTitle := "Reboot"
		if action == provision.PowerOff {
			actionTitle = "Shutdown"
		}
		fmt.Fprintf(&buf, "⚡ Bulk %s Report:\n", actionTitle)
		fmt.Fprintln(&buf, "================================")

		for _, n := range nodes {
			out, err := executePowerControl(n, action)
			if err != nil {
				fmt.Fprintf(&buf, "❌ %-15s: %v\n", n.Name, err)
			} else {
				fmt.Fprintf(&buf, "✅ %-15s: %s\n", n.Name, out)
			}
		}

		return ResultMsg{Output: buf.String()}
	}
}
