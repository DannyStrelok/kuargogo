package main

import (
	"fmt"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/provision"
	"github.com/spf13/cobra"
)

var prepCmd = &cobra.Command{
	Use:   "prep",
	Short: "Provision nodes via SSH",
	Long:  `Configures a node with basic headless settings, installs updates, and essential packages for K3s HA with Longhorn.`,
	Run: func(cmd *cobra.Command, args []string) {
		targetIP, _ := cmd.Flags().GetString("node")
		if targetIP == "" {
			fmt.Println("Error: --node [ip] flag is required")
			return
		}

		createUser, _ := cmd.Flags().GetBool("create-user")
		tagsFlag, _ := cmd.Flags().GetString("tags")
		var tags []string
		if tagsFlag != "" {
			tags = strings.Split(tagsFlag, ",")
		}

		// Find node config for context
		cfg := config.GetConfig()
		targetNode := config.FindNode(targetIP)
		if targetNode == nil {
			fmt.Printf("Error: Node '%s' not found in configuration\n", targetIP)
			return
		}

		fmt.Printf("Provisioning node %s (%s) via Ansible...\n", targetNode.Name, targetNode.IP)

		// Pre-flight: verify SSH key access
		kp, err := cfg.SSH.ExpandedKeyPath()
		if err != nil {
			fmt.Printf("❌ Error expanding key path: %v\n", err)
			return
		}
		port := cfg.SSH.Port
		if port == 0 {
			port = 22
		}
		if err := provision.VerifySSHAccess(targetNode.IP, port, targetNode.User, kp, DryRun); err != nil {
			fmt.Printf("❌ %v\n", err)
			return
		}
		fmt.Println("✅ SSH pre-flight check passed")

		result, err := ansible.RunProvision(targetNode.Name, createUser, "", DryRun, tags, nil)
		if err != nil {
			fmt.Printf("Error running playbook: %v\n", err)
			return
		}

		if !result.Success {
			fmt.Printf("Provisioning failed with exit code %d\n", result.ExitCode)
			return
		}

		fmt.Println("Provisioning complete!")
		fmt.Println("\n✅ Node is ready for K3s HA cluster with Longhorn storage.")
		if createUser {
			fmt.Println("👤 User 'kgg-admin' created with sudo privileges.")
		}
		fmt.Println("📝 Note: A reboot may be required for kernel modules to fully load.")
	},
}

func init() {
	prepCmd.Flags().String("node", "", "IP address of the node to provision")
	prepCmd.Flags().Bool("create-user", false, "Create 'kgg-admin' user with sudo privileges")
	prepCmd.Flags().String("tags", "", "Comma-separated Ansible tags to run (e.g. firewall,kernel)")
	rootCmd.AddCommand(prepCmd)
}
