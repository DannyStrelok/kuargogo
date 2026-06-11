package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/network"
	"github.com/DannyStrelok/kuargogo/internal/network/factory"
	"github.com/spf13/cobra"
)

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage switch and network logic",
}

var networkApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Enforce kuargogo.yaml network state onto the switch",
	Run: func(cmd *cobra.Command, args []string) {
		mgr, err := getNetworkManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Applying configuration to switch...")
		if err := mgr.ApplyConfig(config.GetConfig().NetworkLayout); err != nil {
			fmt.Printf("Failed to apply config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Configuration applied successfully.")
	},
}

var networkValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check if physical connections match inventory",
	Run: func(cmd *cobra.Command, args []string) {
		mgr, err := getNetworkManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Validating physical connections...")
		violations, err := mgr.ValidatePhysicalConnections(config.GetConfig().Nodes)
		if err != nil {
			fmt.Printf("Validation failed: %v\n", err)
			os.Exit(1)
		}

		if len(violations) > 0 {
			fmt.Println("⚠️  Connection Policy Violations Found:")
			for _, v := range violations {
				fmt.Println(" - " + v)
			}
			os.Exit(1)
		} else {
			fmt.Println("✅ All connections match the inventory.")
		}
	},
}

var networkStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show switch status and ports",
	Run: func(cmd *cobra.Command, args []string) {
		mgr, err := getNetworkManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		status, err := mgr.GetStatus()
		if err != nil {
			fmt.Printf("Failed to get status: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Switch: %s (%s) IP: %s Uptime: %s\n", status.Hostname, status.Model, status.IP, status.Uptime)
		fmt.Println("Ports:")
		for _, p := range status.Ports {
			state := "DOWN"
			if p.IsUp {
				state = fmt.Sprintf("UP (%s)", p.Speed)
			}
			fmt.Printf(" [%s] %s: %s\n", p.ID, p.Name, state)
		}
	},
}

var networkRebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Restart the switch hardware",
	Run: func(cmd *cobra.Command, args []string) {
		mgr, err := getNetworkManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Rebooting switch...")
		if err := mgr.Reboot(); err != nil {
			fmt.Printf("Failed to reboot: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Switch is rebooting.")
	},
}

var networkMapCmd = &cobra.Command{
	Use:   "map",
	Short: "Show a physical map of which node is on which port",
	Run: func(cmd *cobra.Command, args []string) {
		mgr, err := getNetworkManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Querying switch inventory...")
		netMap, err := mgr.GetNetworkMap(config.GetConfig().Nodes)
		if err != nil {
			fmt.Printf("Failed to map network: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Network Map for %s (Model: %s)\n", netMap.Hostname, netMap.Model)
		fmt.Println("─────────────────────────────────────────────────────────────────────────────────────")
		fmt.Printf("%-8s │ %-6s │ %-10s │ %-25s │ %-15s │ %-10s\n", "Port", "State", "Speed", "Device", "IP", "Role")
		fmt.Println("─────────────────────────────────────────────────────────────────────────────────────")

		for _, p := range netMap.Ports {
			stateIcon := "🔴"
			stateText := "DOWN"
			if p.IsUp {
				stateIcon = "🟢"
				stateText = "UP"
			}

			speedStr := "—"
			if p.IsUp {
				speedStr = string(p.Speed)
			}

			devName := "—"
			ip := ""
			role := ""

			if p.NodeName != "" {
				devName = p.NodeName
				ip = p.NodeIP
				role = p.NodeRole
			} else if p.ConnectedMAC != "" {
				devName = fmt.Sprintf("? (%s)", p.ConnectedMAC)
			}

			fmt.Printf("%-8s │ %s %-4s │ %-10s │ %-25s │ %-15s │ %-10s\n",
				p.PortID, stateIcon, stateText, speedStr, devName, ip, role)
		}
		fmt.Println("─────────────────────────────────────────────────────────────────────────────────────")
	},
}

var panicConfirm bool

var networkPanicCmd = &cobra.Command{
	Use:   "panic",
	Short: "Activate homelab network and cluster panic isolation",
	Run: func(cmd *cobra.Command, args []string) {
		if !panicConfirm {
			fmt.Print("🚨 WARNING: You are about to isolate your homelab network and cluster. Confirm? (y/N): ")
			var response string
			fmt.Scanln(&response)
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Panic cancelled.")
				return
			}
		}

		mgr, err := getNetworkManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		if err := mgr.TriggerPanic(os.Stdout, config.GetConfig()); err != nil {
			fmt.Printf("Failed to isolate: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("🚨 Panic isolation sequence completed.")
	},
}

var networkPanicRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore homelab network and cluster from panic isolation",
	Run: func(cmd *cobra.Command, args []string) {
		mgr, err := getNetworkManager()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		if err := mgr.RestorePanic(os.Stdout, config.GetConfig()); err != nil {
			fmt.Printf("Failed to restore: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("🔓 Normal operations restored successfully.")
	},
}

func init() {
	rootCmd.AddCommand(networkCmd)
	networkCmd.AddCommand(networkApplyCmd)
	networkCmd.AddCommand(networkValidateCmd)
	networkCmd.AddCommand(networkStatusCmd)
	networkCmd.AddCommand(networkRebootCmd)
	networkCmd.AddCommand(networkMapCmd)

	networkPanicCmd.Flags().BoolVar(&panicConfirm, "confirm", false, "Skip interactive confirmation prompt")
	networkCmd.AddCommand(networkPanicCmd)
	networkPanicCmd.AddCommand(networkPanicRestoreCmd)
}

func getNetworkManager() (*network.Manager, error) {
	return factory.NewManager(config.GetConfig().Network, config.GetConfig().NetworkLayout)
}
