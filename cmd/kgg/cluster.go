package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/cluster"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/spf13/cobra"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster [init|join|drain|reset|remediate]",
	Short: "Manage K3s cluster lifecycle",
	Args:  cobra.ExactArgs(1), // init, join, drain, reset, remediate
	Run: func(cmd *cobra.Command, args []string) {
		action := args[0]
		tagsFlag, _ := cmd.Flags().GetString("tags")
		var tags []string
		if tagsFlag != "" {
			tags = strings.Split(tagsFlag, ",")
		}

		// Find Master Node (Coordinator)
		var masterNode *config.Node
		cfg := config.GetConfig()
		targetNodeName, _ := cmd.Flags().GetString("name")
		for i := range cfg.Nodes {
			n := &cfg.Nodes[i]
			if n.Role == "master" || n.Role == "control-plane" {
				// For drain or remediate, try to find a master that is not the target node being acted upon
				if (action == "drain" || action == "remediate") && targetNodeName != "" && n.Name == targetNodeName {
					continue
				}
				masterNode = n
				break
			}
		}
		if masterNode == nil {
			// Fallback: If no other master found, just use the first available master
			for i := range cfg.Nodes {
				n := &cfg.Nodes[i]
				if n.Role == "master" || n.Role == "control-plane" {
					masterNode = n
					break
				}
			}
		}
		if masterNode == nil {
			fmt.Println("Error: No Master node defined in config.")
			os.Exit(1)
		}

		switch action {
		case "init":
			runClusterInit(cmd, masterNode, tags)
		case "join":
			runClusterJoin(cmd, masterNode, tags)
		case "drain":
			runClusterDrain(cmd, masterNode, tags)
		case "reset":
			runClusterReset(cmd, tags)
		case "remediate":
			runClusterRemediate(cmd, masterNode, tags)
		default:
			fmt.Println("Unknown action. Use: init, join, drain, reset, remediate")
		}
	},
}

func runClusterInit(cmd *cobra.Command, masterNode *config.Node, tags []string) {
	isHA, _ := cmd.Flags().GetBool("ha")

	// HA validation: require at least 3 servers for quorum
	if isHA {
		masterCount := 0
		cfg := config.GetConfig()
		for _, n := range cfg.Nodes {
			if n.Role == "master" || n.Role == "control-plane" {
				masterCount++
			}
		}
		if masterCount < 3 {
			fmt.Printf("Error: HA mode requires at least 3 nodes with role 'master' (found %d).\n", masterCount)
			fmt.Println("Tip: Update your kuargogo.yaml to set role: 'master' for all compute nodes.")
			return
		}
	}

	fmt.Printf("🚀 Initializing K3s Master on %s (%s) [HA: %v]...\n", masterNode.Name, masterNode.IP, isHA)

	vip := config.GetConfig().K3s.VIP
	result, err := ansible.RunK3sInit(masterNode.Name, isHA, vip, DryRun, tags, nil)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("❌ Init failed with exit code %d\n", result.ExitCode)
		return
	}

	fmt.Println("✅ Master initialized successfully!")

	// Try to extract token from playbook output
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.Contains(line, "CLUSTER_TOKEN=") {
			parts := strings.SplitN(line, "CLUSTER_TOKEN=", 2)
			if len(parts) == 2 {
				token := strings.TrimSpace(strings.Trim(parts[1], "\""))
				fmt.Printf("🔑 Cluster Token: %s\n", token)
			}
		}
	}
}

func runClusterJoin(cmd *cobra.Command, masterNode *config.Node, tags []string) {
	targetIP, _ := cmd.Flags().GetString("node")
	if targetIP == "" {
		fmt.Println("Error: --node [IP] required for join")
		return
	}

	// Find Target Node
	var joinNode *config.Node
	cfg := config.GetConfig()
	for _, n := range cfg.Nodes {
		if n.IP == targetIP {
			joinNode = &n
			break
		}
	}
	if joinNode == nil {
		fmt.Printf("Error: Node %s not in config.\n", targetIP)
		return
	}

	// Get token from master (kept as SSH for now)
	keyPath, err := cfg.SSH.ExpandedKeyPath()
	if err != nil {
		fmt.Printf("Error expanding SSH key path: %v\n", err)
		return
	}
	mgr := cluster.NewManager(masterNode.User, keyPath, cfg.SSH.Port, DryRun)
	token, err := mgr.GetMasterToken(masterNode.IP)
	if err != nil {
		fmt.Printf("❌ Error fetching token from Master: %v\n", err)
		return
	}

	// Determine join role
	role := "agent"
	if joinNode.Role == "master" || joinNode.Role == "control-plane" || joinNode.Role == "server" {
		role = "server"
	}

	fmt.Printf("🔗 Joining %s (%s) as %s...\n", joinNode.Name, joinNode.IP, role)

	vip := config.GetConfig().K3s.VIP
	result, err := ansible.RunK3sJoin(joinNode.Name, masterNode.IP, token, role, vip, DryRun, tags, nil)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("❌ Join failed with exit code %d\n", result.ExitCode)
		return
	}

	fmt.Println("✅ Node joined successfully!")
	if joinNode.Labels["gpu"] == "nvidia" {
		fmt.Println("🎮 GPU node detected. Ensure 'kgg setup-gpu' has been run on this node.")
	}
}

func runClusterDrain(cmd *cobra.Command, masterNode *config.Node, tags []string) {
	targetNodeName, _ := cmd.Flags().GetString("name")
	if targetNodeName == "" {
		fmt.Println("Error: --name [NodeName] required for drain")
		return
	}

	fmt.Printf("🔄 Draining node %s...\n", targetNodeName)

	result, err := ansible.RunK3sDrain(masterNode.Name, targetNodeName, DryRun, tags, nil)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("❌ Drain failed with exit code %d\n", result.ExitCode)
		return
	}

	fmt.Println("✅ Node drained successfully!")
}

func runClusterReset(cmd *cobra.Command, tags []string) {
	targetIP, _ := cmd.Flags().GetString("node")
	if targetIP == "" {
		// Fallback to name
		targetName, _ := cmd.Flags().GetString("name")
		if targetName != "" {
			cfg := config.GetConfig()
			for _, n := range cfg.Nodes {
				if n.Name == targetName {
					targetIP = n.IP
					break
				}
			}
		}
	}

	if targetIP == "" {
		fmt.Println("Error: --node [IP] or --name [Name] required for reset")
		return
	}

	// Find node config
	var targetNode *config.Node
	cfg := config.GetConfig()
	for _, n := range cfg.Nodes {
		if n.IP == targetIP {
			targetNode = &n
			break
		}
	}
	if targetNode == nil {
		fmt.Printf("Error: Node %s not found in configuration.\n", targetIP)
		return
	}

	fmt.Printf("🗑️  Resetting K3s on %s (%s)...\n", targetNode.Name, targetNode.IP)

	result, err := ansible.RunK3sReset(targetNode.Name, DryRun, tags, nil)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("❌ Reset failed with exit code %d\n", result.ExitCode)
		return
	}

	fmt.Println("✅ K3s uninstalled successfully!")
}

func runClusterRemediate(cmd *cobra.Command, masterNode *config.Node, tags []string) {
	targetNodeName, _ := cmd.Flags().GetString("name")
	if targetNodeName == "" {
		fmt.Println("Error: --name [NodeName] required for remediate")
		os.Exit(1)
	}

	cfg := config.GetConfig()
	keyPath, err := cfg.SSH.ExpandedKeyPath()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	mgr := cluster.NewManager(masterNode.User, keyPath, cfg.SSH.Port, DryRun)
	mgr.Output = os.Stdout

	fmt.Printf("🛠️  Starting K3s Node Remediation for %s...\n", targetNodeName)
	err = mgr.RemediateNode(masterNode, targetNodeName, tags)
	if err != nil {
		fmt.Printf("❌ Remediation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Remediation completed successfully for %s!\n", targetNodeName)
}

func init() {
	clusterCmd.Flags().String("node", "", "Target Node IP (join/reset)")
	clusterCmd.Flags().String("name", "", "Target Node Name (drain/reset)")
	clusterCmd.Flags().Bool("ha", false, "Initialize cluster in HA mode (init only)")
	clusterCmd.Flags().String("tags", "", "Comma-separated Ansible tags to run")
	rootCmd.AddCommand(clusterCmd)
}
