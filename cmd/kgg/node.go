package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/huh/v2"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/discovery"
	"github.com/DannyStrelok/kuargogo/internal/observability"
	"github.com/DannyStrelok/kuargogo/internal/provision"
	"github.com/spf13/cobra"
)

// Node management commands
var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage rack nodes",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Use subcommands: add, edit, remove, scan, list, health")
	},
}

var nodeAddCmd = &cobra.Command{
	Use:   "add <name> <ip> <role>",
	Short: "Add a new node to the configuration",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		name, ip, role := args[0], args[1], args[2]

		user, _ := cmd.Flags().GetString("user")
		arch, _ := cmd.Flags().GetString("arch")
		pos, _ := cmd.Flags().GetString("position")
		mac, _ := cmd.Flags().GetString("mac")
		labels, _ := cmd.Flags().GetStringSlice("label")

		labelMap := make(map[string]string)
		for _, l := range labels {
			parts := strings.SplitN(l, "=", 2)
			if len(parts) == 2 {
				labelMap[parts[0]] = parts[1]
			}
		}

		// Check duplicates
		cfg := config.GetConfig()
		for _, n := range cfg.Nodes {
			if n.Name == name || n.IP == ip {
				fmt.Printf("Error: Node with name '%s' or IP '%s' already exists.\n", name, ip)
				return
			}
		}

		newNode := config.Node{
			Name:     name,
			IP:       ip,
			Role:     role,
			User:     user,
			Arch:     arch,
			Position: pos,
			MAC:      mac,
			Labels:   labelMap,
		}
		cfg.Nodes = append(cfg.Nodes, newNode)

		config.UpdateNodes(cfg.Nodes)
		if err := config.SaveConfig(); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}

		fmt.Printf("✅ Node added: %s (%s) role=%s arch=%s user=%s\n", name, ip, role, arch, user)
	},
}

var nodeScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan local network for devices (mDNS)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Scanning for _ssh._tcp, _workstation._tcp, and _rk._tcp services...")

		results, err := discovery.ScanCommonServices(5 * time.Second)
		if err != nil {
			fmt.Printf("Scan failed: %v\n", err)
			return
		}

		if len(results) == 0 {
			fmt.Println("No devices found.")
			fmt.Println("💡 Hint: If on Windows, check Firewall allows kgg.exe to listen on UDP 5353.")
			return
		}

		fmt.Println("Scan check complete. Found devices:")
		for _, res := range results {
			// MDNSService struct: HostName, IPs (slice), Port, TXT
			ip := "unknown"
			if len(res.IPs) > 0 {
				ip = res.IPs[0]
			}
			fmt.Printf("[FOUND] %s (%s) - %s [Port: %d]\n", res.Instance, ip, res.HostName, res.Port)
		}
	},
}

var nodeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured nodes",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Configured nodes:")
		cfg := config.GetConfig()
		for _, n := range cfg.Nodes {
			extra := ""
			if n.MAC != "" {
				extra += fmt.Sprintf(" mac=%s", n.MAC)
			}
			if n.Arch != "" {
				extra += fmt.Sprintf(" arch=%s", n.Arch)
			}
			for k, v := range n.Labels {
				extra += fmt.Sprintf(" %s=%s", k, v)
			}
			fmt.Printf("- %s: %s (%s)%s\n", n.Name, n.IP, n.Role, extra)
		}
	},
}

var healthVerbose bool
var healthAll bool

var nodeHealthCmd = &cobra.Command{
	Use:   "health [name|ip]",
	Short: "Check node health (NVMe, CPU, memory, disks)",
	Long:  `Runs comprehensive health checks on a node including NVMe SMART data, CPU temperature, memory usage, and disk status.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var targetNode *config.Node

		if healthAll {
			// Run health check on all nodes
			for _, node := range config.GetConfig().Nodes {
				if node.Role == "infra-manager" {
					continue // Skip Raspberry Pi
				}
				fmt.Printf("\n%s\n", strings.Repeat("=", 60))
				runHealthCheck(node.IP, node.Name, node.User)
			}
			return
		}

		if len(args) > 0 {
			targetIdentifier := args[0]
			// Find node by IP or Name
			cfg := config.GetConfig()
			for _, n := range cfg.Nodes {
				if n.IP == targetIdentifier || n.Name == targetIdentifier {
					targetNode = &n
					break
				}
			}
			if targetNode == nil {
				fmt.Printf("Error: Node '%s' not found in configuration (try Name or IP)\n", targetIdentifier)
				return
			}
		} else {
			fmt.Println("Error: specify a node Name/IP or use --all flag")
			fmt.Println("Usage: kgg node health <name|ip> or kgg node health --all")
			return
		}

		runHealthCheck(targetNode.IP, targetNode.Name, targetNode.User)
	},
}

func runHealthCheck(ip, name, user string) {
	fmt.Printf("🔍 Health Check: %s (%s)\n", name, ip)
	fmt.Println(strings.Repeat("-", 40))

	keyPath, err := config.GetConfig().SSH.ExpandedKeyPath()
	if err != nil {
		fmt.Printf("❌ Error expanding SSH key path: %v\n", err)
		return
	}

	executor, err := provision.NewExecutor(user, keyPath, DryRun)
	if err != nil {
		fmt.Printf("❌ SSH connection failed: %v\n", err)
		return
	}
	executor.Stdout = os.Stdout
	executor.Stderr = os.Stderr

	// 1. Fetch metrics from Prometheus API (Fast)
	// We always query through a master node to ensure internal cluster DNS/IP access
	var promExecutor *provision.Executor
	var masterIP string
	cfg := config.GetConfig()

	// Find the best node to act as metrics gateway (any master)
	for _, n := range cfg.Nodes {
		if n.Role == "master" || n.Role == "control-plane" {
			masterIP = n.IP
			// If analyzing a master, we already have an executor
			if n.IP == ip {
				promExecutor = executor
			} else {
				// Otherwise, create a temporary executor for the master
				ex, err := provision.NewExecutor(n.User, keyPath, DryRun)
				if err == nil {
					promExecutor = ex
				}
			}
			break
		}
	}

	allResults := []provision.HealthCheckResult{}

	if promExecutor != nil {
		promClient := observability.NewClient(promExecutor, masterIP, cfg.SSH.Port)
		promResults, err := promClient.GetNodeMetrics(name, ip)
		if err != nil {
			fmt.Printf("⚠️  Prometheus metrics unavailable: %v\n", err)
		} else {
			allResults = append(allResults, promResults...)
		}
	} else {
		fmt.Println("⚠️  Skipping Prometheus metrics: No master node available to proxy the request.")
	}

	// 2. Fetch legacy checks (NVMe, AppArmor, iSCSI)
	legacyResults, err := executor.RunHealthCheck(ip, cfg.SSH.Port)
	if err != nil {
		fmt.Printf("❌ Error running legacy health checks: %v\n", err)
		return
	}
	allResults = append(allResults, legacyResults...)
	for _, res := range allResults {
		if res.Error != nil {
			fmt.Printf("%s %s: ❌ Error (%v)\n", res.Icon, res.Name, res.Error)
		} else {
			fmt.Printf("%s %s: %s\n", res.Icon, res.Name, res.Result)
		}
	}

	// NVMe detailed status (optional verbose)
	if healthVerbose {
		fmt.Println("\n📊 Detailed NVMe SMART Log:")
		out, err := executor.RunNVMeVerbose(ip, config.GetConfig().SSH.Port)
		if err == nil {
			fmt.Println(out)
		} else {
			fmt.Printf("Error: %v\n", err)
		}
	}
}

// --- Node Edit ---
var nodeEditCmd = &cobra.Command{
	Use:   "edit <name|ip>",
	Short: "Edit a node's attributes",
	Long: `Edit node attributes interactively or via flags.

Examples:
  kgg node edit hp-prodesk                           # interactive form
  kgg node edit hp-prodesk --mac 98:90:96:00:00:02   # direct update
  kgg node edit worker-01 --role master --label gpu=nvidia`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]
		node := config.FindNode(identifier)
		if node == nil {
			fmt.Printf("Error: Node '%s' not found in configuration\n", identifier)
			return
		}

		originalName := node.Name

		// Check if any flags were explicitly set for direct update mode
		flagsSet := cmd.Flags().Changed("name") || cmd.Flags().Changed("ip") ||
			cmd.Flags().Changed("user") || cmd.Flags().Changed("role") ||
			cmd.Flags().Changed("arch") || cmd.Flags().Changed("position") ||
			cmd.Flags().Changed("mac") || cmd.Flags().Changed("label")

		if flagsSet {
			// Direct flag mode
			if v, _ := cmd.Flags().GetString("name"); v != "" {
				node.Name = v
			}
			if v, _ := cmd.Flags().GetString("ip"); v != "" {
				node.IP = v
			}
			if v, _ := cmd.Flags().GetString("user"); v != "" {
				node.User = v
			}
			if v, _ := cmd.Flags().GetString("role"); v != "" {
				node.Role = v
			}
			if v, _ := cmd.Flags().GetString("arch"); v != "" {
				node.Arch = v
			}
			if v, _ := cmd.Flags().GetString("position"); v != "" {
				node.Position = v
			}
			if v, _ := cmd.Flags().GetString("mac"); v != "" {
				node.MAC = v
			}
			if labels, _ := cmd.Flags().GetStringSlice("label"); len(labels) > 0 {
				if node.Labels == nil {
					node.Labels = make(map[string]string)
				}
				for _, l := range labels {
					parts := strings.SplitN(l, "=", 2)
					if len(parts) == 2 {
						node.Labels[parts[0]] = parts[1]
					}
				}
			}
		} else {
			// Interactive form mode
			labelsStr := ""
			for k, v := range node.Labels {
				if labelsStr != "" {
					labelsStr += ","
				}
				labelsStr += k + "=" + v
			}

			node.Role = strings.TrimSpace(node.Role)
			node.Arch = strings.TrimSpace(node.Arch)

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().Title("Name").Value(&node.Name),
					huh.NewInput().Title("IP").Value(&node.IP),
					huh.NewInput().Title("User").Value(&node.User),
					huh.NewSelect[string]().Title("Role").
						Options(
							huh.NewOption("master", "master"),
							huh.NewOption("worker", "worker"),
							huh.NewOption("infra-manager", "infra-manager"),
						).
						Value(&node.Role),
					huh.NewSelect[string]().Title("Architecture").
						Options(
							huh.NewOption("(not set)", ""),
							huh.NewOption("amd64", "amd64"),
							huh.NewOption("arm64", "arm64"),
						).
						Value(&node.Arch),
					huh.NewInput().Title("Position").Description("Physical position: left, center, right").Value(&node.Position),
					huh.NewInput().Title("MAC Address").Description("For Wake-on-LAN").Value(&node.MAC),
					huh.NewInput().Title("Labels").Description("Comma-separated: gpu=nvidia,storage=ssd").Value(&labelsStr),
				),
			)

			if err := form.Run(); err != nil {
				fmt.Printf("Cancelled: %v\n", err)
				return
			}

			// Parse labels from form
			node.Labels = make(map[string]string)
			if labelsStr != "" {
				for _, l := range strings.Split(labelsStr, ",") {
					parts := strings.SplitN(strings.TrimSpace(l), "=", 2)
					if len(parts) == 2 {
						node.Labels[parts[0]] = parts[1]
					}
				}
			}
		}

		if err := config.UpdateNode(originalName, *node); err != nil {
			fmt.Printf("Error updating node: %v\n", err)
			return
		}
		if err := config.SaveConfig(); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}

		fmt.Printf("✅ Node '%s' updated successfully.\n", node.Name)
	},
}

// --- Node Remove ---
var removeForce bool

var nodeRemoveCmd = &cobra.Command{
	Use:   "remove <name|ip>",
	Short: "Remove a node from the configuration",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]
		node := config.FindNode(identifier)
		if node == nil {
			fmt.Printf("Error: Node '%s' not found in configuration\n", identifier)
			return
		}

		if !removeForce {
			fmt.Printf("Remove node '%s' (%s, role=%s)? [y/N]: ", node.Name, node.IP, node.Role)
			reader := bufio.NewReader(os.Stdin)
			resp, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Warning: input error: %v\n", err)
			}
			resp = strings.TrimSpace(resp)

			if len(resp) == 0 || (resp[0] != 'y' && resp[0] != 'Y') {
				fmt.Println("Aborted.")
				return
			}
		}

		if err := config.RemoveNode(identifier); err != nil {
			fmt.Printf("Error removing node: %v\n", err)
			return
		}
		if err := config.SaveConfig(); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}

		fmt.Printf("✅ Node '%s' removed.\n", node.Name)
	},
}

// --- Node Clean Host ---
var nodeCleanHostCmd = &cobra.Command{
	Use:   "clean-host <name|ip>",
	Short: "Remove node's host key from system known_hosts",
	Long:  `Removes any entry for the specified node (by name or IP) from your system's ~/.ssh/known_hosts file.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]
		targetIP := identifier

		// Try to find node in configuration to get its IP
		if node := config.FindNode(identifier); node != nil {
			targetIP = node.IP
		}

		fmt.Printf("🧹 Cleaning host key for %s (%s) from system known_hosts...\n", identifier, targetIP)

		if err := provision.RemoveSystemHostKey(targetIP); err != nil {
			fmt.Printf("❌ Error removing from system known_hosts: %v\n", err)
		}
		if err := provision.RemoveHostKey(targetIP); err != nil {
			fmt.Printf("❌ Error removing from kgg_known_hosts: %v\n", err)
		}

		// Also try to clean by identifier if it's different from IP (e.g. hostname)
		if identifier != targetIP {
			_ = provision.RemoveSystemHostKey(identifier)
			_ = provision.RemoveHostKey(identifier)
		}

		fmt.Println("✅ Done. You can now SSH to the host again.")
	},
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeAddCmd)

	nodeAddCmd.Flags().String("user", "kgg-admin", "SSH user")
	nodeAddCmd.Flags().String("arch", "", "Architecture (amd64, arm64)")
	nodeAddCmd.Flags().String("position", "", "Physical position")
	nodeAddCmd.Flags().String("mac", "", "MAC address for Wake-on-LAN")
	nodeAddCmd.Flags().StringSlice("label", nil, "Labels (key=value, repeatable)")

	nodeCmd.AddCommand(nodeListCmd)
	nodeCmd.AddCommand(nodeScanCmd)
	nodeCmd.AddCommand(nodeCleanHostCmd)

	nodeHealthCmd.Flags().BoolVarP(&healthAll, "all", "a", false, "Run health check on all nodes")
	nodeHealthCmd.Flags().BoolVarP(&healthVerbose, "verbose", "v", false, "Show detailed NVMe SMART log")
	nodeCmd.AddCommand(nodeHealthCmd)

	nodeEditCmd.Flags().String("name", "", "New node name")
	nodeEditCmd.Flags().String("ip", "", "New IP address")
	nodeEditCmd.Flags().String("user", "", "SSH user")
	nodeEditCmd.Flags().String("role", "", "Node role (master, worker, infra-manager)")
	nodeEditCmd.Flags().String("arch", "", "Architecture (amd64, arm64)")
	nodeEditCmd.Flags().String("position", "", "Physical position")
	nodeEditCmd.Flags().String("mac", "", "MAC address for Wake-on-LAN")
	nodeEditCmd.Flags().StringSlice("label", nil, "Labels (key=value, repeatable)")
	nodeCmd.AddCommand(nodeEditCmd)

	nodeRemoveCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Skip confirmation")
	nodeCmd.AddCommand(nodeRemoveCmd)

	nodeStatusCmd.Flags().Bool("json", false, "Output in JSON format")
	nodeCmd.AddCommand(nodeStatusCmd)
}

var nodeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show real-time resource status for all nodes",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.GetConfig()
		jsonFlag, _ := cmd.Flags().GetBool("json")

		// Find a master node to act as proxy for Prometheus
		var masterNode *config.Node
		for _, n := range cfg.Nodes {
			if n.Role == "master" || n.Role == "control-plane" {
				masterNode = &n
				break
			}
		}

		if masterNode == nil {
			fmt.Println("Error: No master node found to query metrics.")
			return
		}

		type NodeStatus struct {
			Name   string `json:"name"`
			Role   string `json:"role"`
			Status string `json:"status"`
			CPU    string `json:"cpu"`
			RAM    string `json:"ram"`
			Disk   string `json:"disk"`
			Uptime string `json:"uptime,omitempty"`
		}

		resultsChan := make(chan NodeStatus, len(cfg.Nodes))

		// Attempt to initialize Prometheus client once
		var prom *observability.Client
		if masterNode != nil {
			keyPath, _ := cfg.SSH.ExpandedKeyPath()
			executor, err := provision.NewExecutor(masterNode.User, keyPath, config.IsDryRun())
			if err == nil {
				if jsonFlag {
					executor.Stdout = io.Discard // Silences stdout for JSON output
					executor.Stderr = io.Discard
				}
				prom = observability.NewClient(executor, masterNode.IP, cfg.SSH.Port)
			}
		}

		// Check all nodes in parallel
		for _, n := range cfg.Nodes {
			go func(node config.Node) {
				st := NodeStatus{Name: node.Name, Role: node.Role, Status: "offline"}

				// 1. Try Prometheus First
				if prom != nil {
					metrics, err := prom.GetNodeMetrics(node.Name, node.IP)
					// Only use Prometheus if it successfully returned at least one metric.
					// Otherwise, fall back to SSH to guarantee we get CPU/RAM/Disk stats.
					if err == nil && len(metrics) > 0 {
						st.Status = "online"
						for _, m := range metrics {
							switch m.Name {
							case "CPU Usage":
								st.CPU = m.Result
							case "Memory Usage":
								st.RAM = m.Result
							case "Disk Usage (/)":
								st.Disk = m.Result
							case "Uptime":
								st.Uptime = m.Result
							}
						}
						resultsChan <- st
						return
					}
				}

				// 2. Fallback to SSH Check
				keyPath, _ := cfg.SSH.ExpandedKeyPath()
				executor, err := provision.NewExecutor(node.User, keyPath, config.IsDryRun())
				if err == nil {
					if jsonFlag {
						executor.Stdout = io.Discard // Silences stdout for JSON output
						executor.Stderr = io.Discard
					}
					// Lightweight check: get uptime and resources in one go
					// CPU: calculation from /proc/stat
					// RAM: free | awk '/Mem:/ {print $3/$2 * 100.0}'
					// Disk: df / --output=pcent | tail -1
					cmd := `uptime -p && 
						   awk '/cpu / {print ($2+$4)*100/($2+$4+$5)}' /proc/stat && 
						   free | awk '/Mem:/ {print $3/$2 * 100.0}' && 
						   df / --output=pcent | tail -1`

					out, err := executor.ExecuteCommand(node.IP, cfg.SSH.Port, cmd)
					if err == nil {
						st.Status = "online"
						lines := strings.Split(strings.TrimSpace(out), "\n")
						if len(lines) >= 4 {
							st.Uptime = strings.TrimSpace(lines[0])
							st.CPU = fmt.Sprintf("%.1f%%", parsePercent(lines[1]))
							st.RAM = fmt.Sprintf("%.1f%%", parsePercent(lines[2]))
							st.Disk = strings.TrimSpace(lines[3])
						}
					}
				}

				resultsChan <- st
			}(n)
		}

		var statuses []NodeStatus
		for i := 0; i < len(cfg.Nodes); i++ {
			statuses = append(statuses, <-resultsChan)
		}

		if jsonFlag {
			data, _ := json.MarshalIndent(statuses, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, st := range statuses {
				icon := "🟢"
				if st.Status == "offline" {
					icon = "🔴"
				}
				fmt.Printf("%s [%s] %-15s CPU: %-7s RAM: %-7s Disk: %s\n", icon, st.Status, st.Name, st.CPU, st.RAM, st.Disk)
			}
		}
		os.Exit(0)
	},
}

func parsePercent(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(strings.TrimSpace(s), "%f", &f)
	return f
}
