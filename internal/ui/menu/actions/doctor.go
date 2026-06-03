package actions

import (
	"bytes"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/deps"
	"github.com/DannyStrelok/kuargogo/internal/doctor"
)

// Doctor runs health diagnostics across all nodes.
// It captures output and formats a table for TUI display.
func Doctor() tea.Cmd {
	return func() tea.Msg {
		// Check dependencies
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v\n\nPlease install Ansible first.", err)}
		}

		var buf bytes.Buffer

		result, err := ansible.RunDoctor(false, nil, &buf)

		if err != nil && result == nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v", err)}
		}

		// Parse metrics from output
		metrics := ansible.ParseDoctorMetrics(buf.String())

		if len(metrics) == 0 {
			return ResultMsg{Output: "No metrics collected. Check that nodes are reachable."}
		}

		// Build formatted table output
		var output strings.Builder
		output.WriteString("🩺 Cluster Health Report\n\n")
		fmt.Fprintf(&output, "%-18s | %-15s | %-6s | %-6s | %-6s | %-6s | %s\n",
			"Node", "IP", "CPU°C", "Disk", "RAM", "Load", "Uptime")
		output.WriteString(strings.Repeat("-", 80))
		output.WriteString("\n")

		for _, m := range metrics {
			fmt.Fprintf(&output, "%-18s | %-15s | %-6s | %-6s | %-6s | %-6s | %s\n",
				m.Host, m.IP, m.CPUTemp+"°", m.Disk, m.Mem, m.Load, m.Uptime)
		}

		fmt.Fprintf(&output, "\n✅ Diagnostics completed in %s", result.Duration.Round(1))

		return ResultMsg{Output: output.String()}
	}
}

// DoctorConfig runs the configuration validation logic (linting)
func DoctorConfig() tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer

		configPassed := doctor.ValidateConfig(&buf)

		if !configPassed {
			return ResultMsg{Output: buf.String() + "\n\n❌ Validation failed. Please check the warnings and errors above."}
		}

		return ResultMsg{Output: buf.String() + "\n\n✅ Configuration is valid!"}
	}
}
