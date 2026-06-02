package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/spf13/cobra"
)

var storageCmd = &cobra.Command{
	Use:   "storage [init|status|mount]",
	Short: "Manage distributed block storage (Longhorn) and mounts",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		action := args[0]

		switch action {
		case "mount":
			runMountStorage(cmd)
		case "init", "status":
			runLonghorn(action)
		default:
			fmt.Println("Unknown action. Use: init, status, mount")
			os.Exit(1)
		}
	},
}

func runMountStorage(cmd *cobra.Command) {
	nodeName, _ := cmd.Flags().GetString("node")
	disk, _ := cmd.Flags().GetString("disk")
	mountPoint, _ := cmd.Flags().GetString("mount")
	tagsFlag, _ := cmd.Flags().GetString("tags")
	var tags []string
	if tagsFlag != "" {
		tags = strings.Split(tagsFlag, ",")
	}

	if nodeName == "" || disk == "" || mountPoint == "" {
		fmt.Println("Error: --node, --disk, and --mount flags are required for mount action")
		return
	}

	// Verify node exists
	cfg := config.GetConfig()
	var targetNode *config.Node
	for _, n := range cfg.Nodes {
		if n.Name == nodeName {
			targetNode = &n
			break
		}
	}

	if targetNode == nil {
		fmt.Printf("Error: Node '%s' not found in configuration\n", nodeName)
		return
	}

	fmt.Printf("🔧 Mounting %s to %s on node %s (%s)...\n", disk, mountPoint, targetNode.Name, targetNode.IP)
	fmt.Println("⚠️  Warning: This will partition and format the disk if not already done.")

	result, err := ansible.RunMountStorage(targetNode.Name, disk, mountPoint, DryRun, tags, nil)
	if err != nil {
		fmt.Printf("❌ Error running playbook: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("❌ Mount failed with exit code %d\n", result.ExitCode)
		return
	}

	fmt.Printf("✅ Storage mounted successfully on %s!\n", targetNode.Name)
}

func runLonghorn(action string) {
	// Refactored to use Ansible
	switch action {
	case "init":
		fmt.Println("Deploying Longhorn v1.10.1 to the cluster via Ansible...")
		result, err := ansible.RunLonghornInit(DryRun, nil, os.Stdout)
		if err != nil {
			fmt.Printf("❌ Error triggering deployment: %v\n", err)
		} else if !result.Success {
			fmt.Printf("❌ Deployment failed with exit code %d\n", result.ExitCode)
		} else {
			fmt.Println("✅ Longhorn deployed successfully.")
			fmt.Println("Note: It may take a few minutes for all pods to start.")
		}
	case "status":
		fmt.Println("Checking Longhorn System Status via Ansible...")
		result, err := ansible.RunLonghornStatus("", DryRun, os.Stdout)
		if err != nil {
			fmt.Printf("❌ Error checking status: %v\n", err)
		} else if !result.Success {
			fmt.Printf("❌ Status check failed with exit code %d\n", result.ExitCode)
		}
	}
}

func init() {
	storageCmd.Flags().String("node", "", "Target node name (required for mount)")
	storageCmd.Flags().String("disk", "", "Target disk device e.g. /dev/sdb (required for mount)")
	storageCmd.Flags().String("mount", "", "Mount point path e.g. /mnt/data (required for mount)")
	storageCmd.Flags().String("tags", "", "Comma-separated Ansible tags to run (e.g. partition,mount)")
	rootCmd.AddCommand(storageCmd)
}
