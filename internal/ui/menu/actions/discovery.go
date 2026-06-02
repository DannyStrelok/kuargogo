package actions

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/discovery"
)

// DiscoverAndAdd ran the mDNS discovery and automatically merges new nodes into the configuration.
// It matches the behavior of the 'kgg discover' CLI command.
func DiscoverAndAdd() tea.Cmd {
	return func() tea.Msg {
		// 1. Scan for services
		services, err := discovery.ScanCommonServices(5 * time.Second)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Discovery Error: %v", err)}
		}

		if len(services) == 0 {
			return ResultMsg{Output: "No new devices found via mDNS."}
		}

		// 2. Convert to nodes (default user 'root' as in CLI)
		discoveredNodes := discovery.MDNSToNodes(services, "root")

		// 3. Merge with existing inventory
		existing := config.GetConfig().Nodes
		merged := discovery.MergeDiscoveredNodes(existing, discoveredNodes)

		// 4. Save to config
		config.UpdateNodes(merged)
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error saving config: %v", err)}
		}

		return ResultMsg{Output: fmt.Sprintf("✅ Discovery complete!\n\nFound and merged %d devices into your inventory.\nCheck 'Inventory' to see the results.", len(services))}
	}
}
