package actions

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/discovery"
)

// ScanNetwork discovers devices via mDNS
func ScanNetwork() tea.Cmd {
	return func() tea.Msg {
		results, err := discovery.ScanCommonServices(5 * time.Second)
		if err != nil {
			return ScanResultMsg{Output: fmt.Sprintf("Scan Error: %v", err)}
		}

		if len(results) == 0 {
			return ScanResultMsg{Output: "No devices found."}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d device(s):\n\n", len(results))
		for _, r := range results {
			ip := "unknown"
			if len(r.IPs) > 0 {
				ip = r.IPs[0]
			}
			fmt.Fprintf(&sb, "â€¢ %s (%s)\n", r.HostName, ip)
			fmt.Fprintf(&sb, "  Service: %s\n", r.Instance)
		}
		return ScanResultMsg{Output: sb.String()}
	}
}
