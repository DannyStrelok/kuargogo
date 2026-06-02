package actions

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/apps"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/hardware"
)

// AppDeploy deploys an application using the Ansible apps role.
func AppDeploy(appName string) tea.Cmd {
	return func() tea.Msg {
		template := apps.GetTemplate(appName)
		if template == "" {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: Unknown application '%s'. Supported: immich, ollama, homeassistant", appName)}
		}

		var masterNode *config.Node
		cfg := config.GetConfig()
		for i := range cfg.Nodes {
			if cfg.Nodes[i].Role == "master" {
				masterNode = &cfg.Nodes[i]
				break
			}
		}

		if masterNode == nil {
			return ResultMsg{Output: "❌ Error: No node with role 'master' found in config."}
		}

		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			// Initialize manager
			mgr := apps.NewManager(config.IsDryRun(), nil)
			mgr.Output = writer

			err := mgr.DeployApp(appName, template, masterNode.Name)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error deploying %s: %v", appName, err)
				return
			}
			ch <- fmt.Sprintf("\n✅ %s deployed successfully!", appName)
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// AppBackup triggers a system backup with visual hardware feedback.
func AppBackup() tea.Cmd {
	return func() tea.Msg {
		var masterNode *config.Node
		cfg := config.GetConfig()
		for i := range cfg.Nodes {
			if cfg.Nodes[i].Role == "master" {
				masterNode = &cfg.Nodes[i]
				break
			}
		}

		if masterNode == nil {
			return ResultMsg{Output: "❌ Error: No node with role 'master' found in config."}
		}

		// Init MQTT for LED control
		mqttCfg := cfg.MQTT
		client, err := hardware.NewClient(
			mqttCfg.Broker,
			mqttCfg.ClientID+"-apps",
			mqttCfg.Username,
			string(mqttCfg.Password),
			false, // DryRun=false
		)
		if err != nil {
			// Non-fatal, we just won't have LED feedback
		} else if client != nil {
			if token := client.Connect(); token.Wait() && token.Error() != nil {
				client = nil
			} else {
				defer client.Disconnect(250)
			}
		}

		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			mgr := apps.NewManager(config.IsDryRun(), client)
			mgr.Output = writer

			err = mgr.Backup(masterNode.Name, mqttCfg.TopicPrefix)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error during backup: %v", err)
				return
			}

			ch <- "\n✅ Backup completed successfully!"
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}
