package main

import (
	"fmt"
	"os"

	"github.com/DannyStrelok/kuargogo/internal/apps"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/hardware"
	"github.com/spf13/cobra"
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage homelab applications and backups",
}

var deployCmd = &cobra.Command{
	Use:   "deploy [immich|ollama|homeassistant]",
	Short: "Deploy a critical application to the cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		appName := args[0]
		template := apps.GetTemplate(appName)
		if template == "" {
			fmt.Printf("Error: Unknown application '%s'. Supported: immich, ollama, homeassistant\n", appName)
			return
		}

		// Find Master Node
		var masterNode *config.Node
		cfg := config.GetConfig()
		for _, n := range cfg.Nodes {
			if n.Role == "master" || n.Role == "control-plane" {
				masterNode = &n
				break
			}
		}

		if masterNode == nil {
			fmt.Println("Error: No node with role 'master' found.")
			return
		}

		// Initialize manager
		mgr := apps.NewManager(
			DryRun,
			nil, // No MQTT needed for deploy by default
		)
		mgr.Output = os.Stdout

		err := mgr.DeployApp(appName, template, masterNode.Name)
		if err != nil {
			fmt.Printf("Error deploying %s: %v\n", appName, err)
		}
	},
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Trigger a system backup with visual hardware feedback",
	Run: func(cmd *cobra.Command, args []string) {
		// Find Master Node
		var masterNode *config.Node
		cfg := config.GetConfig()
		for _, n := range cfg.Nodes {
			if n.Role == "master" || n.Role == "control-plane" {
				masterNode = &n
				break
			}
		}

		if masterNode == nil {
			fmt.Println("Error: No node with role 'master' found.")
			return
		}

		// Init MQTT for LED control
		mqttCfg := config.GetConfig().MQTT
		client, err := hardware.NewClient(
			mqttCfg.Broker,
			mqttCfg.ClientID+"-apps",
			mqttCfg.Username,
			string(mqttCfg.Password),
			DryRun,
		)
		if err != nil {
			fmt.Printf("Warning: Could not create MQTT client: %v\n", err)
		} else if !DryRun && client != nil {
			if token := client.Connect(); token.Wait() && token.Error() != nil {
				fmt.Printf("Warning: Could not connect to MQTT: %v\n", token.Error())
				client = nil
			}
		}
		mgr := apps.NewManager(
			DryRun,
			client,
		)
		mgr.Output = os.Stdout

		err = mgr.Backup(masterNode.Name, mqttCfg.TopicPrefix)
		if err != nil {
			fmt.Printf("Error during backup: %v\n", err)
		}

		if client != nil && !DryRun {
			client.Disconnect(250)
		}
	},
}

func init() {
	appCmd.AddCommand(deployCmd)
	appCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(appCmd)
}
