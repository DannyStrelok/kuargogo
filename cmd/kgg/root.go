package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/infra"
	"github.com/DannyStrelok/kuargogo/internal/ui"
	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
	"github.com/DannyStrelok/kuargogo/internal/ui/menu/models"
	"github.com/DannyStrelok/kuargogo/internal/ui/wizard"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	DryRun  bool

	activeProgram  *tea.Program
	infraSyncMutex sync.Mutex
)

var rootCmd = &cobra.Command{
	Use:   "kgg",
	Short: "Kuargogo - Homelab Management CLI",
	Long: `kgg (Kuargogo) is a CLI tool to manage your homelab infrastructure.
It controls provisioning, K3s cluster lifecycle, hardware sensors (MQTT), and AI services.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.GetConfig()
		if len(cfg.Nodes) == 0 && cfg.Network.SwitchIP == "" {

			_, err := os.Stat(config.GetConfigPath())
			if os.IsNotExist(err) {
				if err := wizard.Run(os.Stdout); err != nil {
					fmt.Printf("Wizard failed: %v\n", err)
					os.Exit(1)
				}
				// Reload config after wizard
				err := config.LoadConfig("")
				if err != nil {
					fmt.Printf("Failed to load config after wizard: %v\n", err)
					os.Exit(1)
				}
			}
		}

		if len(args) == 0 {
			config.WatchEnabled = true
			// Redirect logs to file for TUI mode
			f, err := os.OpenFile(filepath.Join(config.GetConfigDir(), "kgg.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				fmt.Printf("Error opening log file: %v\n", err)
				os.Exit(1)
			}
			defer func() {
				if err := f.Close(); err != nil {
					fmt.Printf("Warning: failed to close log file: %v\n", err)
				}
			}()
			log.SetOutput(f)

			// Launch TUI (Dashboard)
			root := models.BuildMainMenu()
			menuModel := models.NewMenuModel(root, nil)
			uiEngine := engine.NewEngine(menuModel, &ui.KGGTheme{}, "kuargogo")

			p := tea.NewProgram(uiEngine)
			activeProgram = p
			if _, err := p.Run(); err != nil {
				fmt.Println("Error running dashboard:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	cobra.MousetrapHelpText = ""

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kuargogo/kuargogo.yaml)")
	rootCmd.PersistentFlags().BoolVar(&DryRun, "dry-run", false, "Simulate execution without connecting to real hardware")
	rootCmd.PersistentFlags().StringVar(&ansible.VaultPassFileOverride, "vault-password-file", "", "Path to Ansible Vault password file (overrides config)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	config.SetDryRun(DryRun)

	if err := config.LoadConfig(cfgFile); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !strings.Contains(err.Error(), "Not Found") {
			fmt.Println("Error loading config:", err)
		}
	}

	// Register auto-sync hook to keep infra-manager aligned with development machine
	config.OnConfigUpdated = func(path string) {
		if config.IsRunningOnInfraManager() {
			return // Avoid self-sync loops
		}

		// Notify active TUI if running
		if activeProgram != nil {
			activeProgram.Send(engine.ConfigReloadedMsg{})
		}

		cfg := config.GetConfig()
		infraNode := cfg.GetInfraManager()
		if infraNode == nil {
			return
		}

		keyPath, err := cfg.SSH.ExpandedKeyPath()
		if err != nil {
			log.Printf("Auto-sync: failed to expand SSH key: %v", err)
			return
		}

		// Use background goroutine to avoid blocking the UI/CLI
		go func() {
			infraSyncMutex.Lock()
			defer infraSyncMutex.Unlock()

			mgr := infra.NewManager(infraNode.User, keyPath, cfg.SSH.Port, config.IsDryRun())
			mgr.Output = io.Discard // Silent sync in background
			if err := mgr.SyncConfig(infraNode.IP, path); err != nil {
				log.Printf("Auto-sync failed for infra-manager (%s): %v", infraNode.IP, err)
			}
			if err := config.SyncPush(); err != nil {
				log.Printf("Auto-backup failed: %v", err)
			}
		}()
	}
}
