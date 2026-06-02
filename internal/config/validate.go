package config

import (
	"fmt"
	"net"
	"strings"
)

// Validate checks the consistency and correctness of the ClusterConfig
func (c *ClusterConfig) Validate() error {
	var errors []string

	// 1. Validate Nodes
	seenNames := make(map[string]bool)
	seenIPs := make(map[string]bool)
	hasMaster := false

	for i, node := range c.Nodes {
		context := fmt.Sprintf("Node[%d] (%s)", i, node.Name)

		// Name uniqueness
		if node.Name == "" {
			errors = append(errors, fmt.Sprintf("%s: Name is required", context))
		} else {
			if seenNames[node.Name] {
				errors = append(errors, fmt.Sprintf("%s: Duplicate name '%s'", context, node.Name))
			}
			seenNames[node.Name] = true
		}

		// IP Validation
		if node.IP == "" {
			errors = append(errors, fmt.Sprintf("%s: IP is required", context))
		} else {
			if net.ParseIP(node.IP) == nil {
				errors = append(errors, fmt.Sprintf("%s: Invalid IP address '%s'", context, node.IP))
			}
			if seenIPs[node.IP] {
				errors = append(errors, fmt.Sprintf("%s: Duplicate IP '%s'", context, node.IP))
			}
			seenIPs[node.IP] = true
		}

		// Role Validation
		validRoles := map[string]bool{"master": true, "worker": true, "infra-manager": true, "control-plane": true}
		if !validRoles[node.Role] {
			errors = append(errors, fmt.Sprintf("%s: Invalid role '%s'. Supported: master, worker, infra-manager", context, node.Role))
		}
		if node.Role == "master" || node.Role == "control-plane" {
			hasMaster = true
		}
	}

	if c.K3s.Version == "" {
		c.K3s.Version = "v1.30.1+k3s1"
	}

	// HA Check: Multiple servers require a VIP
	masterCount := 0
	for _, n := range c.Nodes {
		if n.Role == "master" || n.Role == "control-plane" {
			masterCount++
		}
	}

	if masterCount > 1 && c.K3s.VIP == "" {
		errors = append(errors, "HA Cluster: Multiple master nodes detected, but K3s VIP is missing. High Availability requires a Virtual IP.")
	}

	if len(c.Nodes) > 0 && !hasMaster {
		errors = append(errors, "Cluster: No node with role 'master' found") // Warning? Or Error? Let's say error for a cluster CLI.
	}

	// 2. SSH Validation
	if c.SSH.PrivateKeyPath == "" {
		errors = append(errors, "SSH: PrivateKeyPath is missing")
	}

	// 3. MQTT Validation
	if c.MQTT.Broker != "" {
		if !strings.HasPrefix(c.MQTT.Broker, "tcp://") && !strings.HasPrefix(c.MQTT.Broker, "ssl://") {
			errors = append(errors, fmt.Sprintf("MQTT: Broker URL '%s' must start with tcp:// or ssl://", c.MQTT.Broker))
		}
	}

	// 4. Network Validation
	if c.Network.SwitchIP != "" {
		if net.ParseIP(c.Network.SwitchIP) == nil {
			errors = append(errors, fmt.Sprintf("Network: Invalid Switch IP '%s'", c.Network.SwitchIP))
		}
	}

	// 5. K3s VIP Validation
	if c.K3s.VIP != "" {
		if net.ParseIP(c.K3s.VIP) == nil {
			errors = append(errors, fmt.Sprintf("K3s: Invalid HA Virtual IP '%s'", c.K3s.VIP))
		} else {
			// Check for conflict with existing nodes
			for _, node := range c.Nodes {
				if node.IP == c.K3s.VIP {
					errors = append(errors, fmt.Sprintf("K3s: HA Virtual IP '%s' conflicts with node '%s'", c.K3s.VIP, node.Name))
				}
			}
		}
	}

	// 6. Cloudflare Validation
	if c.Cloudflare.Email != "" && !strings.Contains(c.Cloudflare.Email, "@") {
		errors = append(errors, fmt.Sprintf("Cloudflare: Invalid email '%s'", c.Cloudflare.Email))
	}

	// 7. Backup Validation
	if c.Backup.S3Url != "" {
		if !strings.HasPrefix(c.Backup.S3Url, "http://") && !strings.HasPrefix(c.Backup.S3Url, "https://") {
			errors = append(errors, fmt.Sprintf("Backup: S3 URL '%s' must start with http:// or https://", c.Backup.S3Url))
		}
	}

	// 8. AI Validation
	if c.AI.Provider != "" {
		validProviders := map[string]bool{
			"ollama": true, "openai-compatible": true,
			"anthropic": true, "openai": true, "gemini": true,
		}
		if !validProviders[c.AI.Provider] {
			errors = append(errors, fmt.Sprintf("AI: Invalid provider '%s'. Supported: ollama, openai-compatible, anthropic, openai, gemini", c.AI.Provider))
		}

		if (c.AI.Provider == "anthropic" || c.AI.Provider == "openai" || c.AI.Provider == "gemini") && c.AI.APIKey == "" {
			// Instead of a hard error, we could warn, but for validation let's require it if specified
			errors = append(errors, fmt.Sprintf("AI: API Key is required for provider '%s'", c.AI.Provider))
		}

		if c.AI.Endpoint != "" {
			if !strings.HasPrefix(c.AI.Endpoint, "http://") && !strings.HasPrefix(c.AI.Endpoint, "https://") {
				errors = append(errors, fmt.Sprintf("AI: Endpoint '%s' must start with http:// or https://", c.AI.Endpoint))
			}
		}
	}

	// 9. Telegram Validation
	if c.Telegram.BotToken != "" {
		if c.Telegram.Timezone == "" {
			errors = append(errors, "Telegram: Timezone is required (e.g. Europe/Madrid)")
		}
		if c.Telegram.DailySummaryTime != "" {
			parts := strings.Split(c.Telegram.DailySummaryTime, ":")
			if len(parts) != 2 {
				errors = append(errors, fmt.Sprintf("Telegram: Invalid DailySummaryTime '%s'. Must be H:M format", c.Telegram.DailySummaryTime))
			}
		}
	}

	// 10. NFS Validation
	if c.NFS.Enabled {
		if c.NFS.Server == "" {
			errors = append(errors, "NFS: Server IP is required when enabled")
		} else if net.ParseIP(c.NFS.Server) == nil {
			errors = append(errors, fmt.Sprintf("NFS: Invalid Server IP '%s'", c.NFS.Server))
		}

		for i, share := range c.NFS.Shares {
			if share.Src == "" || share.Dest == "" {
				errors = append(errors, fmt.Sprintf("NFS: Share[%d] requires both Src and Dest paths", i))
			}
		}
	}

	// 11. Discovery Validation
	if c.Discovery.Enabled {
		if c.Discovery.Interface == "" {
			errors = append(errors, "Discovery: Network Interface is required when enabled")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration errors found:\n- %s", strings.Join(errors, "\n- "))
	}

	return nil
}
