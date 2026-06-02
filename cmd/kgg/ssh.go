package main

import (
	"fmt"
	"os"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/provision"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var sshCmd = &cobra.Command{
	Use:   "ssh-keygen",
	Short: "Generate a cluster-wide SSH key pair",
	Long:  `Generates a secure Ed25519 SSH key pair. Default path is ~/.ssh/kgg_<context>_id based on your configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		path, _ := cmd.Flags().GetString("output")
		resolvedPath, err := config.ResolveKeyPath(path)
		if err != nil {
			fmt.Printf("Error resolving key path: %v\n", err)
			return
		}
		path = resolvedPath

		if DryRun {
			fmt.Printf("[DRY RUN] Would generate SSH key pair at %s\n", path)
			return
		}

		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Key already exists at %s. Overwrite? [y/N]: ", path)
			var resp string
			if _, err := fmt.Scanln(&resp); err != nil && err.Error() != "unexpected newline" {
				fmt.Printf("Warning: input error: %v\n", err)
			}
			if len(resp) == 0 || (resp[0] != 'y' && resp[0] != 'Y') {
				fmt.Println("Aborted.")
				return
			}
		}

		fmt.Printf("Generating SSH key at %s...\n", path)
		if err := provision.GenerateClusterKey(path); err != nil {
			fmt.Printf("Error generating key: %v\n", err)
			return
		}

		fmt.Println("✅ Key generated successfully.")
		fmt.Printf("Private Key: %s\n", path)
		fmt.Printf("Public Key:  %s.pub\n", path)
		fmt.Println("\nNext step: Distribute this key to your nodes using 'kgg ssh-copy'")
	},
}

var sshCopyCmd = &cobra.Command{
	Use:   "ssh-copy",
	Short: "Install the cluster SSH key to a remote node",
	Long:  `Copies the cluster public key to a remote node's authorized_keys file using password authentication.`,
	Example: `  kgg ssh-copy --node 192.168.1.101 --user debian
  kgg ssh-copy --node 192.168.1.102 --user root --key ~/.ssh/id_rsa`,
	Run: func(cmd *cobra.Command, args []string) {
		targetIP, _ := cmd.Flags().GetString("node")
		user, _ := cmd.Flags().GetString("user")
		pass, _ := cmd.Flags().GetString("password")
		keyPath, _ := cmd.Flags().GetString("key")

		if targetIP == "" {
			fmt.Println("Error: --node is required")
			return
		}
		if user == "" {
			fmt.Println("Error: --user is required")
			return
		}

		// Resolve key path using consistent helper
		resolvedPath, err := config.ResolveKeyPath(keyPath)
		if err != nil {
			fmt.Printf("Error resolving key path: %v\n", err)
			return
		}
		keyPath = resolvedPath

		// Auto-generate cluster key if missing
		generated, genErr := provision.EnsureClusterKey(keyPath)
		if genErr != nil {
			fmt.Printf("❌ Error ensuring key: %v\n", genErr)
			return
		}
		if generated {
			fmt.Printf("🔑 Auto-generated cluster key at %s\n", keyPath)
		}

		pubKeyPath := keyPath + ".pub"

		// Resolve SSH port from config
		port := config.GetConfig().SSH.Port
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
			fmt.Println() // Newline after silent input
		}

		if DryRun {
			fmt.Printf("[DRY RUN] Would install key %s to %s@%s:%d\n", pubKeyPath, user, targetIP, port)
			return
		}

		fmt.Printf("Installing key %s to %s@%s...\n", pubKeyPath, user, targetIP)

		if err := provision.InstallKey(targetIP, port, user, pass, pubKeyPath); err != nil {
			fmt.Printf("❌ Error installing key: %v\n", err)
			return
		}

		fmt.Println("✅ Success! Key installed.")
		fmt.Printf("Test with: ssh -i %s %s@%s\n", keyPath, user, targetIP)
	},
}

func init() {
	sshCmd.Flags().String("output", "", "Output path for keys")
	rootCmd.AddCommand(sshCmd)

	sshCopyCmd.Flags().String("node", "", "Target IP address")
	sshCopyCmd.Flags().String("user", "", "SSH User (e.g. debian)")
	sshCopyCmd.Flags().String("password", "", "SSH Password (optional, will prompt)")
	sshCopyCmd.Flags().String("key", "", "Private key path (default: configured in kuargogo.yaml)")
	rootCmd.AddCommand(sshCopyCmd)
}
