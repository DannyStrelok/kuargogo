package apps

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/hardware"
)

// Manager handles deployment and backups
type Manager struct {
	DryRun     bool
	MQTTClient *hardware.Client
	Output     io.Writer
}

// NewManager creates a new apps manager
func NewManager(dryRun bool, mqttClient *hardware.Client) *Manager {
	return &Manager{
		DryRun:     dryRun,
		MQTTClient: mqttClient,
		Output:     os.Stdout,
	}
}

// DeployApp applies a raw YAML manifest on the master node
func (m *Manager) DeployApp(appName, manifest, masterName string) error {
	if _, err := fmt.Fprintf(m.Output, "Deploying %s...\n", appName); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}

	// Dump manifest to a temporary file locally so Ansible can read it
	tmpFile, err := os.CreateTemp("", "kgg-manifest-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp manifest file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }() // Clean up

	if _, err := tmpFile.WriteString(manifest); err != nil {
		return fmt.Errorf("failed to write manifest to temp file: %v", err)
	}
	_ = tmpFile.Close() // Close so Ansible can read it

	// Run Ansible Playbook
	result, err := ansible.RunAppDeploy(m.DryRun, nil, masterName, tmpFile.Name(), m.Output)
	if err != nil {
		return fmt.Errorf("failed to deploy %s: %w", appName, err)
	}

	if !result.Success {
		return fmt.Errorf("deployment of %s failed with exit code %d", appName, result.ExitCode)
	}

	if m.DryRun {
		if _, err := fmt.Fprintf(m.Output, "[DRY-RUN] Manifest for %s applies successfully.\n", appName); err != nil {
			log.Printf("Warning: failed to write status: %v", err)
		}
	} else {
		if _, err := fmt.Fprintln(m.Output, "Deployment successful."); err != nil {
			log.Printf("Warning: failed to write status: %v", err)
		}
	}
	return nil
}

// Backup triggers a backup with visual feedback
func (m *Manager) Backup(masterName, topicPrefix string) error {
	if _, err := fmt.Fprintln(m.Output, "Initiating System Backup..."); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}

	// 1. Visual Signal: Blue Blinking (Do Not Unplug)
	if m.MQTTClient != nil {
		if err := m.MQTTClient.SetColor("blue", "blinking", topicPrefix, "global"); err != nil {
			log.Printf("Warning: failed to set LED color: %v\n", err)
		}
	} else if m.DryRun {
		if _, err := fmt.Fprintln(m.Output, "[DRY-RUN] MQTT: Set LEDs to BLUE BLINKING"); err != nil {
			log.Printf("Warning: failed to write status: %v", err)
		}
	}

	// 2. Execute Backup Playbook
	result, err := ansible.RunAppBackup(m.DryRun, nil, masterName, m.Output)
	if err != nil {
		if m.MQTTClient != nil {
			if err := m.MQTTClient.SetColor("red", "static", topicPrefix, "global"); err != nil {
				log.Printf("Warning: failed to set LED color: %v\n", err)
			}
		}
		return fmt.Errorf("backup failed: %w", err)
	}

	if !result.Success {
		if m.MQTTClient != nil {
			if err := m.MQTTClient.SetColor("red", "static", topicPrefix, "global"); err != nil {
				log.Printf("Warning: failed to set LED color: %v\n", err)
			}
		}
		return fmt.Errorf("backup failed with exit code %d", result.ExitCode)
	}

	// 3. Visual Signal: Green Static (Safe)
	if m.MQTTClient != nil {
		if err := m.MQTTClient.SetColor("green", "static", topicPrefix, "global"); err != nil {
			log.Printf("Warning: failed to set LED color: %v\n", err)
		}
	} else if m.DryRun {
		if _, err := fmt.Fprintln(m.Output, "[DRY-RUN] MQTT: Set LEDs to GREEN STATIC"); err != nil {
			log.Printf("Warning: failed to write status: %v", err)
		}
	}

	return nil
}
