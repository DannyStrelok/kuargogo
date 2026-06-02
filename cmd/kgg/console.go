package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/deps"
)

var consoleNs string

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Launch K9s terminal UI for Kubernetes",
	Long: `Opens K9s with the cluster's kubeconfig automatically configured.
	
Requires k9s to be installed on the system.
Configure kubeconfig_path in kuargogo.yaml under the k3s section.

Examples:
  kgg console              # Open K9s with default namespace
  kgg console --ns kube-system  # Open K9s in kube-system namespace`,
	RunE: runConsole,
}

func init() {
	rootCmd.AddCommand(consoleCmd)
	consoleCmd.Flags().StringVar(&consoleNs, "ns", "", "Kubernetes namespace to start in")
}

func runConsole(cmd *cobra.Command, args []string) error {
	// 1. Check k9s is installed
	if err := deps.CheckDependency("k9s"); err != nil {
		return err
	}

	// 2. Resolve kubeconfig path
	kubeconfigPath := config.GetConfig().K3s.KubeconfigPath
	if kubeconfigPath == "" {
		return fmt.Errorf("kubeconfig_path not configured in kuargogo.yaml under k3s section")
	}

	// Expand ~ if present
	if strings.HasPrefix(kubeconfigPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to expand home directory: %w", err)
		}
		kubeconfigPath = home + kubeconfigPath[1:]
	}

	// 3. Build command arguments
	k9sArgs := []string{}
	if consoleNs != "" {
		k9sArgs = append(k9sArgs, "-n", consoleNs)
	}

	// 4. Create command with environment
	k9sCmd := exec.Command("k9s", k9sArgs...)
	k9sCmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
	k9sCmd.Stdin = os.Stdin
	k9sCmd.Stdout = os.Stdout
	k9sCmd.Stderr = os.Stderr

	// 5. Run interactively
	fmt.Printf("Launching K9s with kubeconfig: %s\n", kubeconfigPath)
	if consoleNs != "" {
		fmt.Printf("Starting in namespace: %s\n", consoleNs)
	}

	return k9sCmd.Run()
}
