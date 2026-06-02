package actions

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/network/factory"
)

// NetworkStatus returns the result of the Status command
func NetworkStatus() tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		mgr, err := factory.NewManager(cfg.Network, cfg.NetworkLayout)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error initializing network manager: %v", err)}
		}

		status, err := mgr.GetStatus()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error getting status: %v", err)}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Switch: %s (%s)\n", status.Hostname, status.Model)
		fmt.Fprintf(&sb, "IP: %s\n", status.IP)
		fmt.Fprintf(&sb, "Uptime: %s\n\n", status.Uptime)
		sb.WriteString("Ports:\n")
		for _, p := range status.Ports {
			state := "DOWN"
			if p.IsUp {
				state = fmt.Sprintf("UP (%s)", p.Speed)
			}
			macInfo := ""
			if p.ConnectedMAC != "" {
				macInfo = fmt.Sprintf(" -> %s", p.ConnectedMAC)
			}
			fmt.Fprintf(&sb, " [%s] %s: %s%s\n", p.ID, p.Name, state, macInfo)
		}
		return ResultMsg{Output: sb.String()}
	}
}

// NetworkApply applies the configuration
func NetworkApply() tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		mgr, err := factory.NewManager(cfg.Network, cfg.NetworkLayout)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error: %v", err)}
		}
		if err := mgr.ApplyConfig(cfg.NetworkLayout); err != nil {
			return ResultMsg{Output: fmt.Sprintf("Failed to apply config: %v", err)}
		}
		return ResultMsg{Output: "✅ Configuration applied successfully."}
	}
}

// NetworkValidate runs the validation check
func NetworkValidate() tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		mgr, err := factory.NewManager(cfg.Network, cfg.NetworkLayout)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error: %v", err)}
		}
		violations, err := mgr.ValidatePhysicalConnections(cfg.Nodes)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Validation error: %v", err)}
		}
		if len(violations) > 0 {
			var sb strings.Builder
			sb.WriteString("⚠️  Violations Found:\n")
			for _, v := range violations {
				fmt.Fprintf(&sb, " - %s\n", v)
			}
			return ResultMsg{Output: sb.String()}
		}
		return ResultMsg{Output: "✅ All connections match the inventory."}
	}
}

// NetworkReboot reboots the switch
func NetworkReboot() tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		mgr, err := factory.NewManager(cfg.Network, cfg.NetworkLayout)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error: %v", err)}
		}
		if err := mgr.Reboot(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("Failed to reboot: %v", err)}
		}
		return ResultMsg{Output: "✅ Switch is rebooting."}
	}
}

// NetworkMap returns the visual port map
func NetworkMap() tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		mgr, err := factory.NewManager(cfg.Network, cfg.NetworkLayout)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error: %v", err)}
		}

		netMap, err := mgr.GetNetworkMap(cfg.Nodes)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Failed to map network: %v", err)}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# Network Map: %s\n\n", netMap.Hostname)

		// Simple text table for now, or use lipgloss for styling if preferred style guide allows
		// Using a markdown-like table format for Glamour renderer compatibility if we use it,
		// but simple text is standard for this TUI OutputModel.

		sb.WriteString("| Port | State | Speed | Device | IP | Role |\n")
		sb.WriteString("|---|---|---|---|---|---|\n")

		for _, p := range netMap.Ports {
			state := "🔴 DOWN"
			if p.IsUp {
				state = "🟢 UP"
			}

			devName := ""
			if p.NodeName != "" {
				devName = fmt.Sprintf("**%s**", p.NodeName)
			} else if p.ConnectedMAC != "" {
				devName = fmt.Sprintf("`%s`", p.ConnectedMAC)
			}

			speed := string(p.Speed)
			if !p.IsUp {
				speed = "-"
			}

			fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s | %s |\n",
				p.PortID, state, speed, devName, p.NodeIP, p.NodeRole)
		}

		return ResultMsg{Output: sb.String()}
	}
}
