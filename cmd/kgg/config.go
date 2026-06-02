package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(getContextsCmd)
	configCmd.AddCommand(useContextCmd)
	configCmd.AddCommand(setContextCmd)
	configCmd.AddCommand(deleteContextCmd)
	configCmd.AddCommand(lintCmd)
	configCmd.AddCommand(currentContextCmd)
	configCmd.AddCommand(setCloudflareCmd)
	configCmd.AddCommand(setAnsibleCmd)
	configCmd.AddCommand(restoreCmd)
}

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Validate the current configuration context",
	Run: func(cmd *cobra.Command, args []string) {
		current := config.GetCurrentContext()
		fmt.Printf("Linting configuration for context '%s'...\n", current)

		// For validation, we might want to validate the *specific* context if asked,
		// but typically we validate the *active* one or the one being linted.
		// If linting current context:
		cfg := config.GetConfig()
		err := cfg.Validate()
		if err != nil {
			fmt.Println("❌ Configuration has errors:")
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println("✅ Configuration is valid.")
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration contexts (multi-cluster support)",
}

var currentContextCmd = &cobra.Command{
	Use:   "current-context",
	Short: "Display the current-context",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.GetCurrentContext())
	},
}

var getContextsCmd = &cobra.Command{
	Use:   "get-contexts",
	Short: "List all available contexts",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%-10s %-20s\n", "CURRENT", "NAME")

		names := config.ListContexts()
		sort.Strings(names)

		current := config.GetCurrentContext()
		for _, name := range names {
			prefix := " "
			if name == current {
				prefix = "*"
			}
			fmt.Printf("%-10s %-20s\n", prefix, name)
		}
	},
}

var useContextCmd = &cobra.Command{
	Use:   "use-context [name]",
	Short: "Switch to a different context",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := args[0]

		if err := config.SwitchContext(target); err != nil {
			fmt.Printf("Error: %v. Use 'kgg config set-context %s' to create it.\n", err, target)
			return
		}

		if err := config.SaveConfig(); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}
		fmt.Printf("Switched to context %q.\n", target)
	},
}

var setContextCmd = &cobra.Command{
	Use:   "set-context [name]",
	Short: "Create a new context (cloning current) or update existing",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		_, err := config.GetContext(name)
		if err == nil {
			fmt.Printf("Context %q already exists. Switching to it.\n", name)
			// Switch to existing
			if err := config.SwitchContext(name); err != nil {
				fmt.Printf("Error switching: %v\n", err)
				return
			}
		} else {
			// Clone current config as base for new context
			// We use GetConfig which is the thread safe current loaded struct
			currentCfg := config.GetConfig()
			config.AddContext(name, currentCfg)
			fmt.Printf("Context %q configured (cloned from current).\n", name)
		}

		if err := config.SaveConfig(); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}
	},
}

var deleteContextCmd = &cobra.Command{
	Use:   "delete-context [name]",
	Short: "Delete a specific context",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		if name == config.GetCurrentContext() {
			fmt.Println("Cannot delete the currently active context. Switch to another one first.")
			return
		}

		// Delete logic needs helper or we create one now?
		// For now we can access map thread-safely via helper if we had DeleteContext.
		// Let's modify directly via mutex or add DeleteContext helper in config.go?
		// User asked for safety. Let's add DeleteContext to config.go first.
		if err := config.DeleteContext(name); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if err := config.SaveConfig(); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}
		fmt.Printf("Context %q deleted.\n", name)
	},
}

var setCloudflareCmd = &cobra.Command{
	Use:   "set-cloudflare",
	Short: "Configure Cloudflare Zero Trust credentials",
	Long: `Set Cloudflare credentials for Cert-Manager and Zero Trust Tunnels.

These credentials are stored in your kuargogo.yaml and used when deploying
the Cloudflare Tunnel and Let's Encrypt SSL certificates.

Examples:
  kgg config set-cloudflare --domain chefclandestino.es --email user@example.com --api-token xxx --tunnel-token yyy
  kgg config set-cloudflare --domain chefclandestino.es   # Set only the domain`,
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		apiToken, _ := cmd.Flags().GetString("api-token")
		tunnelToken, _ := cmd.Flags().GetString("tunnel-token")
		accountID, _ := cmd.Flags().GetString("account-id")
		tunnelID, _ := cmd.Flags().GetString("tunnel-id")

		if email == "" && apiToken == "" && tunnelToken == "" && accountID == "" && tunnelID == "" {
			return fmt.Errorf("at least one flag is required")
		}

		err := config.ModifyConfig(func(c *config.ClusterConfig) {
			if email != "" {
				c.Cloudflare.Email = email
			}
			if apiToken != "" {
				c.Cloudflare.APIToken = config.Secret(apiToken)
			}
			if tunnelToken != "" {
				c.Cloudflare.TunnelToken = config.Secret(tunnelToken)
			}
			if accountID != "" {
				c.Cloudflare.AccountID = accountID
			}
			if tunnelID != "" {
				c.Cloudflare.TunnelID = tunnelID
			}
		})
		if err != nil {
			return fmt.Errorf("failed to update config: %w", err)
		}

		if err := config.SaveConfig(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("✅ Cloudflare configuration updated.")

		cfg := config.GetConfig()
		fmt.Printf("  Email:        %s\n", maskEmpty(cfg.Cloudflare.Email))
		fmt.Printf("  API Token:    %s\n", maskSecret(string(cfg.Cloudflare.APIToken)))
		fmt.Printf("  Account ID:   %s\n", maskEmpty(cfg.Cloudflare.AccountID))
		fmt.Printf("  Tunnel Token: %s\n", maskSecret(string(cfg.Cloudflare.TunnelToken)))
		fmt.Printf("  Tunnel ID:    %s\n", maskEmpty(cfg.Cloudflare.TunnelID))
		fmt.Printf("  Domains:      %d configured\n", len(cfg.Cloudflare.Domains))

		return nil
	},
}

var setAnsibleCmd = &cobra.Command{
	Use:   "set-ansible",
	Short: "Configure Ansible settings (WSL and Vault)",
	Long: `Set Ansible configuration for provisioning.
	
On Windows, the --distro flag defines which WSL Linux distribution is used.
The --vault-file flag points to the file containing the vault password.

Examples:
  kgg config set-ansible --distro Debian
  kgg config set-ansible --vault-file ~/.ssh/my_vault_pass`,
	RunE: func(cmd *cobra.Command, args []string) error {
		distro, _ := cmd.Flags().GetString("distro")
		vaultFile, _ := cmd.Flags().GetString("vault-file")

		if distro == "" && vaultFile == "" {
			return fmt.Errorf("at least one flag is required (--distro, --vault-file)")
		}

		err := config.ModifyConfig(func(c *config.ClusterConfig) {
			if distro != "" {
				c.Ansible.WSLDistro = distro
			}
			if vaultFile != "" {
				c.Ansible.VaultPasswordFile = vaultFile
			}
		})
		if err != nil {
			return fmt.Errorf("failed to update config: %w", err)
		}

		if err := config.SaveConfig(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("✅ Ansible configuration updated.")

		// Show current state
		cfg := config.GetConfig()
		fmt.Printf("  WSL Distro:         %s\n", maskEmpty(cfg.Ansible.WSLDistro))
		fmt.Printf("  Vault Password File: %s\n", maskEmpty(cfg.Ansible.VaultPasswordFile))

		return nil
	},
}

func init() {
	setCloudflareCmd.Flags().String("email", "", "Cloudflare account email")
	setCloudflareCmd.Flags().String("api-token", "", "Cloudflare API Token (Tunnel + DNS + Access permissions)")
	setCloudflareCmd.Flags().String("tunnel-token", "", "Zero Trust Tunnel Token")
	setCloudflareCmd.Flags().String("account-id", "", "Cloudflare Account ID")
	setCloudflareCmd.Flags().String("tunnel-id", "", "Cloudflare Tunnel ID")

	setAnsibleCmd.Flags().String("distro", "", "WSL Distribution (e.g. Ubuntu, Debian)")
	setAnsibleCmd.Flags().String("vault-file", "", "Ansible Vault password file path")
}

func maskSecret(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

func maskEmpty(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

var restoreCmd = &cobra.Command{
	Use:   "restore [backup-name]",
	Short: "Restore a previous configuration version",
	Long: `List or restore configuration backups from the history directory (~/.kuargogo/config_history).
	
If no backup-name is provided, it lists all available backups.
If a backup-name is provided, it restores that version and creates a backup of the current state.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			backups, err := config.ListBackups()
			if err != nil {
				fmt.Printf("❌ Error listing backups: %v\n", err)
				os.Exit(1)
			}

			if len(backups) == 0 {
				fmt.Println("No backups found in history.")
				return
			}

			fmt.Println("Available configuration backups (newest first):")
			for _, b := range backups {
				fmt.Printf("  - %s\n", b)
			}
			fmt.Println("\nTo restore, use: kgg config restore [backup-name]")
			return
		}

		backupName := args[0]
		fmt.Printf("⏳ Restoring configuration from %s...\n", backupName)

		if err := config.RestoreBackup(backupName); err != nil {
			fmt.Printf("❌ Restoration failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Configuration restored successfully.")
	},
}
