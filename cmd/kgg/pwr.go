package main

import (
	"fmt"
	"os"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/hardware"
	"github.com/DannyStrelok/kuargogo/internal/provision"
	"github.com/spf13/cobra"
)

var pwrCmd = &cobra.Command{
	Use:   "pwr [on|off|reboot] [node_name]",
	Short: "Manage power state of nodes",
	Long:  `Turn nodes on via Wake-on-LAN, or turn them off/reboot via SSH.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		action := args[0]
		nodeName := args[1]

		if action != "on" && action != "off" && action != "reboot" {
			fmt.Println("Error: Action must be 'on', 'off', or 'reboot'")
			os.Exit(1)
		}

		// Find Node
		var targetNode *config.Node
		cfg := config.GetConfig()
		for _, n := range cfg.Nodes {
			if n.Name == nodeName {
				targetNode = &n
				break
			}
		}

		if targetNode == nil {
			fmt.Printf("Error: Node '%s' not found.\n", nodeName)
			os.Exit(1)
		}

		fmt.Printf("Powering %s node '%s'...\n", action, nodeName)

		if action == "on" {
			if targetNode.MAC == "" {
				fmt.Println("Error: Node has no MAC address configured for WoL.")
				return
			}

			if DryRun {
				fmt.Printf("[DRY-RUN] Sending Magic Packet to %s (%s)\n", targetNode.Name, targetNode.MAC)
				return
			}

			err := hardware.WakeOnLAN(targetNode.MAC, targetNode.IP)
			if err != nil {
				fmt.Printf("Error sending WoL packet: %v\n", err)
			} else {
				fmt.Println("Magic packet sent successfully.")
			}
			os.Exit(0)
		}

		// OFF or REBOOT -> SSH
		var pwrAction provision.PowerAction
		if action == "reboot" {
			pwrAction = provision.PowerReboot
		} else {
			pwrAction = provision.PowerOff
		}

		keyPath, err := config.ResolveKeyPath("")
		if err != nil {
			fmt.Printf("Error expanding SSH key path: %v\n", err)
			os.Exit(1)
		}

		executor, err := provision.NewExecutor(targetNode.User, keyPath, DryRun)
		if err != nil {
			fmt.Printf("Error initializing SSH executor: %v\n", err)
			return
		}

		out, err := executor.RemotePowerControl(targetNode.IP, config.GetConfig().SSH.Port, pwrAction)
		if err != nil {
			fmt.Printf("Error executing command: %v\nOutput: %s\n", err, out)
			os.Exit(1)
		} else {
			fmt.Println(out)
			fmt.Println("Command executed successfully.")
		}
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(pwrCmd)
}
