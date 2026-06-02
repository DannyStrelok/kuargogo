package main

import (
	"fmt"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/spf13/cobra"
)

var gpuCmd = &cobra.Command{
	Use:   "setup-gpu",
	Short: "Install NVIDIA drivers and Container Toolkit",
	Long: `Targeted at nodes with NVIDIA GPUs. Installs drivers and configures the container toolkit.

By default, runs on ALL GPU-capable nodes (label 'gpu: nvidia' in kuargogo.yaml).
Use --node to target a specific node by name.

Examples:
  kgg setup-gpu                    # Setup all GPU nodes
  kgg setup-gpu --node gpu-worker  # Setup a specific node
  kgg setup-gpu --dry-run          # Preview without applying`,
	Run: func(cmd *cobra.Command, args []string) {
		targetName, _ := cmd.Flags().GetString("node")
		tagsFlag, _ := cmd.Flags().GetString("tags")
		var tags []string
		if tagsFlag != "" {
			tags = strings.Split(tagsFlag, ",")
		}

		var targets []config.Node

		if targetName != "" {
			// Specific node requested
			cfg := config.GetConfig()
			for _, n := range cfg.Nodes {
				if n.Name == targetName {
					targets = append(targets, n)
					break
				}
			}
			if len(targets) == 0 {
				fmt.Printf("Error: Node '%s' not found in configuration\n", targetName)
				return
			}
		} else {
			// Auto-detect GPU nodes
			targets = config.FindGPUNodes()
			if len(targets) == 0 {
				fmt.Println("Error: No GPU-capable nodes found.")
				fmt.Println("Hint: Set label 'gpu: nvidia' in kuargogo.yaml for GPU-capable nodes")
				return
			}
		}

		fmt.Printf("Found %d GPU node(s) to configure:\n", len(targets))
		for _, n := range targets {
			fmt.Printf("  • %s (%s)\n", n.Name, n.IP)
		}
		fmt.Println()

		for _, n := range targets {
			fmt.Printf("🔧 Setting up GPU on %s (%s)...\n", n.Name, n.IP)

			result, err := ansible.RunGPUSetup(n.Name, DryRun, tags, nil)
			if err != nil {
				fmt.Printf("❌ Error on %s: %v\n\n", n.Name, err)
				continue
			}

			if !result.Success {
				fmt.Printf("❌ GPU setup failed on %s (exit code %d)\n\n", n.Name, result.ExitCode)
				continue
			}

			fmt.Printf("✅ GPU setup complete on %s! A reboot is recommended.\n\n", n.Name)
		}
	},
}

func init() {
	gpuCmd.Flags().String("node", "", "Target a specific node by name (default: all GPU nodes)")
	gpuCmd.Flags().String("tags", "", "Comma-separated Ansible tags to run (e.g. drivers,toolkit)")
	rootCmd.AddCommand(gpuCmd)
}
