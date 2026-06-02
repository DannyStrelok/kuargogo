package main

import (
	"fmt"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/discovery"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(discoverCmd)
}

var (
	flagMDNS      bool
	flagLLDP      bool
	flagInterface string
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Automatically discover nodes via mDNS and/or LLDP",
	RunE:  runDiscover,
}

func init() {
	discoverCmd.Flags().BoolVar(&flagMDNS, "mdns", true, "Enable mDNS discovery (default true)")
	discoverCmd.Flags().BoolVar(&flagLLDP, "lldp", false, "Enable LLDP discovery (Linux only)")
	discoverCmd.Flags().StringVar(&flagInterface, "interface", "", "Network interface to use for discovery (optional)")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	var discovered []config.Node

	if flagMDNS {
		services, err := discovery.ScanCommonServices(5 * time.Second)
		if err != nil {
			return fmt.Errorf("mDNS discovery failed: %w", err)
		}
		discovered = append(discovered, discovery.MDNSToNodes(services, "root")...)
	}

	if flagLLDP {
		devices, err := discovery.DiscoverLLDP()
		if err != nil {
			return fmt.Errorf("LLDP discovery failed: %w", err)
		}
		for _, d := range devices {
			node := config.Node{
				Name: d.SysName,
				IP:   d.IP,
				MAC:  d.ChassisID,
			}
			discovered = append(discovered, node)
		}
	}

	// Merge with existing nodes
	existing := config.GetConfig().Nodes
	merged := discovery.MergeDiscoveredNodes(existing, discovered)
	config.UpdateNodes(merged)
	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Printf("Discovered %d node(s). Configuration updated.\n", len(discovered))
	return nil
}
