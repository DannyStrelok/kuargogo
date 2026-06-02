package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/provision"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Full node bootstrap: keygen + ssh-copy + provision",
	Long: `Performs the complete bootstrap flow for a new node:
  1. Network Bootstrap (optional DHCP -> Static IP)
  2. Generates SSH key pair (if missing)
  3. Installs public key via password authentication (TOFU)
  4. Verifies SSH key authentication
  5. Runs final provisioning via Ansible

This is a unified command that ensures nodes are correctly integrated into the cluster.`,
	Example: `  kgg bootstrap --node 192.168.1.100 --dhcp 192.168.1.45 --user debian
  kgg bootstrap --node raspberry-pi-3 --user debian --create-user`,
	Run: func(cmd *cobra.Command, args []string) {
		nodeRef, _ := cmd.Flags().GetString("node")
		dhcpIP, _ := cmd.Flags().GetString("dhcp")
		user, _ := cmd.Flags().GetString("user")
		pass, _ := cmd.Flags().GetString("password")
		keyPath, _ := cmd.Flags().GetString("key")
		skipProvision, _ := cmd.Flags().GetBool("skip-provision")
		createUser, _ := cmd.Flags().GetBool("create-user")
		tagsFlag, _ := cmd.Flags().GetString("tags")

		if nodeRef == "" {
			fmt.Println("Error: --node is required (node name or IP in config)")
			return
		}
		if user == "" {
			fmt.Println("Error: --user is required")
			return
		}

		// Resolve key path
		cfg := config.GetConfig()
		resolvedPath, err := config.ResolveKeyPath(keyPath)
		if err != nil {
			fmt.Printf("Error resolving key path: %v\n", err)
			return
		}
		keyPath = resolvedPath

		// Find node in config to get its final Static IP and Role
		var nodeName, staticIP, nodeRole string
		for _, n := range cfg.Nodes {
			if n.Name == nodeRef || n.IP == nodeRef {
				nodeName = n.Name
				staticIP = n.IP
				nodeRole = n.Role
				break
			}
		}

		if nodeName == "" {
			fmt.Printf("Error: Node '%s' not found in kuargogo.yaml. Add it first.\n", nodeRef)
			return
		}

		// If DHCP IP is provided, use it for the initial connection
		targetIP := staticIP
		if dhcpIP != "" {
			targetIP = dhcpIP
		}

		// Resolve port
		port := cfg.SSH.Port
		if port == 0 {
			port = 22
		}

		// Prompt for password if missing
		if pass == "" {
			fmt.Printf("Enter password for %s@%s: ", user, targetIP)
			bytePass, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				fmt.Printf("\nError reading password: %v\n", err)
				return
			}
			pass = string(bytePass)
			fmt.Println()
		}

		var tags []string
		if tagsFlag != "" {
			tags = strings.Split(tagsFlag, ",")
		}

		// Execute unified FullBootstrap
		err = provision.FullBootstrap(provision.FullBootstrapOptions{
			NodeName:      nodeName,
			DHCP_IP:       dhcpIP,
			StaticIP:      staticIP,
			User:          user,
			Password:      pass,
			KeyPath:       keyPath,
			SSHPort:       port,
			CreateUser:    createUser,
			SkipProvision: skipProvision,
			Tags:          tags,
			Role:          nodeRole,
			DryRun:        DryRun,
			Output:        os.Stdout,
		})

		if err != nil {
			fmt.Printf("\n❌ Bootstrap failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	bootstrapCmd.Flags().String("node", "", "Target node NAME or IP from config (required)")
	bootstrapCmd.Flags().String("dhcp", "", "Temporary DHCP IP if node is not yet at its static IP")
	bootstrapCmd.Flags().String("user", "", "SSH user for initial password auth (required)")
	bootstrapCmd.Flags().String("password", "", "SSH password (will prompt if omitted)")
	bootstrapCmd.Flags().String("key", "", "SSH private key path (default: from config)")
	bootstrapCmd.Flags().Bool("skip-provision", false, "Skip Ansible provisioning after key setup")
	bootstrapCmd.Flags().Bool("create-user", false, "Create 'kgg-admin' user during provisioning")
	bootstrapCmd.Flags().String("tags", "", "Comma-separated Ansible tags for provisioning")
	rootCmd.AddCommand(bootstrapCmd)
}
