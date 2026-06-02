package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/provision"
	"github.com/spf13/cobra"
)

var sshExecCmd = &cobra.Command{
	Use:   "ssh <node_name> <command...>",
	Short: "Execute a command on a remote node via SSH",
	Long:  `Runs an arbitrary shell command on a configured node using the cluster SSH key.`,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		nodeIdentifier := args[0]
		remoteCmd := strings.Join(args[1:], " ")

		node := config.FindNode(nodeIdentifier)
		if node == nil {
			fmt.Printf("Error: Node '%s' not found.\n", nodeIdentifier)
			os.Exit(1)
		}

		keyPath, err := config.ResolveKeyPath("")
		if err != nil {
			fmt.Printf("Error resolving SSH key: %v\n", err)
			os.Exit(1)
		}

		executor, err := provision.NewExecutor(node.User, keyPath, config.IsDryRun())
		if err != nil {
			fmt.Printf("Error initializing SSH executor: %v\n", err)
			os.Exit(1)
		}

		out, err := executor.ExecuteCommand(node.IP, config.GetConfig().SSH.Port, remoteCmd)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			if out != "" {
				fmt.Printf("Output: %s\n", out)
			}
			os.Exit(1)
		}

		// Force exit to ensure background goroutines (like SSH keepalives) don't hang the process
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(sshExecCmd)
}
