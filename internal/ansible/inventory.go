package ansible

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/deps"
)

// GenerateInventory creates a temporary inventory file and returns its path and the SSH key path used.
// It is the caller's responsibility to delete the inventory file.
func GenerateInventory(dryRun bool) (string, string, error) {
	tmpfile, err := os.CreateTemp("", "kgg-inventory-*.ini")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp inventory: %w", err)
	}

	keyPath, err := WriteInventory(tmpfile, config.GetConfig().Nodes, dryRun)
	if err != nil {
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
		return "", "", err
	}

	// Close after successful write (caller will read then remove the file)
	if err := tmpfile.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close temp inventory: %w", err)
	}

	return tmpfile.Name(), keyPath, nil
}

// WriteInventory writes an Ansible-compatible inventory to the given writer.
// Groups nodes by role: infra, server, agent, and all.
// Returns the (potentially WSL-native) SSH key path referenced in the inventory.
func WriteInventory(w io.Writer, nodes []config.Node, dryRun bool) (string, error) {
	groups := map[string][]config.Node{
		"infra":       {}, // Infrastructure Manager (RPi)
		"server":      {}, // K3s Servers (k3s-ansible standard)
		"agent":       {}, // K3s Agents (k3s-ansible standard)
		"k3s_cluster": {}, // All K3s nodes (Servers + Agents)
		"gpu_nodes":   {},
		"all":         {},
	}

	var masterIPs []string
	for _, node := range nodes {
		groups["all"] = append(groups["all"], node)
		role := strings.ToLower(node.Role)
		switch role {
		case "infra-manager":
			groups["infra"] = append(groups["infra"], node)
		case "master", "control-plane", "server":
			groups["server"] = append(groups["server"], node)
			groups["k3s_cluster"] = append(groups["k3s_cluster"], node)
			masterIPs = append(masterIPs, node.IP)
		case "worker", "agent":
			groups["agent"] = append(groups["agent"], node)
			groups["k3s_cluster"] = append(groups["k3s_cluster"], node)
		}

		if node.Labels["gpu"] == "nvidia" {
			groups["gpu_nodes"] = append(groups["gpu_nodes"], node)
		}
	}

	sshCfg := config.GetConfig()
	keyPath, err := sshCfg.SSH.ExpandedKeyPath()
	if err != nil {
		return "", fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	// Validate that the key actually exists before doing anything
	if !dryRun {
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			return "", fmt.Errorf("SSH key not found at %s.\nPlease run 'Cluster Lifecycle -> Quick Bootstrap' or 'SSH Management -> Generate Cluster Key' first", keyPath)
		}
	}

	// On Windows, sync the SSH key into WSL's native filesystem (~/.ssh/) with
	// proper 0600 permissions, then use that native path in the inventory.
	// This avoids NTFS 0777 permission issues and backslash escaping in INI files.
	if runtime.GOOS == "windows" {
		if wslKeyPath, syncErr := deps.SyncSSHKeyToWSL(keyPath); syncErr == nil {
			keyPath = wslKeyPath
		} else if wslPath, convErr := deps.ConvertToWSLPath(keyPath); convErr == nil {
			// Fallback: use /mnt/c/... path if sync fails
			keyPath = wslPath
		}
		// Sync known_hosts file independently
		_ = deps.SyncKnownHostsToWSL()
	}
	sshPort := sshCfg.SSH.Port
	if sshPort == 0 {
		sshPort = 22
	}

	// Write groups
	for group, members := range groups {
		// Only skip if the group is empty AND not one of our standard K3s/Hardware groups.
		// Standard groups should always exist to prevent Ansible pattern warnings.
		isStandardGroup := group == "server" || group == "agent" || group == "k3s_cluster" || group == "gpu_nodes"
		if len(members) == 0 && !isStandardGroup {
			continue
		}
		if _, err := fmt.Fprintf(w, "[%s]\n", group); err != nil {
			return "", fmt.Errorf("failed to write group header: %w", err)
		}
		for _, n := range members {
			user := n.User
			if user == "" {
				user = "kgg-admin"
			}

			labelStr := ""
			if len(n.Labels) > 0 {
				var labelPairs []string
				for k, v := range n.Labels {
					labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
				}
				labelStr = fmt.Sprintf(" node_labels=\"%s\"", strings.Join(labelPairs, ","))
			}

			hostName := SanitizeAnsibleHostname(n.Name)

			if _, err := fmt.Fprintf(w, "%s ansible_host=%s ansible_user=%s ansible_ssh_private_key_file=%s ansible_port=%d%s\n",
				hostName, n.IP, user, keyPath, sshPort, labelStr); err != nil {
				return "", fmt.Errorf("failed to write node entry: %w", err)
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return "", fmt.Errorf("failed to write newline: %w", err)
		}
	}

	// Write cluster-wide variables [all:vars]
	k3sCfg := sshCfg.K3s
	var k3sVars strings.Builder
	k3sVars.WriteString("[all:vars]\n")

	// Base settings
	fmt.Fprintf(&k3sVars, "k3s_ha=%v\n", k3sCfg.HA)
	fmt.Fprintf(&k3sVars, "k3s_version=%s\n", k3sCfg.Version)

	if k3sCfg.Token != "" {
		fmt.Fprintf(&k3sVars, "k3s_token=%s\n", k3sCfg.Token)
	}

	if k3sCfg.VIPInterface != "" {
		fmt.Fprintf(&k3sVars, "k3s_vip_interface=%s\n", k3sCfg.VIPInterface)
	}

	if len(k3sCfg.ServerArgs) > 0 {
		serverArgsJSON, err := json.Marshal(k3sCfg.ServerArgs)
		if err == nil {
			fmt.Fprintf(&k3sVars, "k3s_server_extra_args='%s'\n", string(serverArgsJSON))
		}
	}

	if len(k3sCfg.AgentArgs) > 0 {
		agentArgsJSON, err := json.Marshal(k3sCfg.AgentArgs)
		if err == nil {
			fmt.Fprintf(&k3sVars, "k3s_agent_extra_args='%s'\n", string(agentArgsJSON))
		}
	}

	// VIP and Server URL
	serverIP := ""
	if k3sCfg.VIP != "" {
		serverIP = k3sCfg.VIP
		fmt.Fprintf(&k3sVars, "k3s_vip_ip=%s\n", serverIP)
	} else if len(groups["server"]) > 0 {
		serverIP = groups["server"][0].IP
	}

	if serverIP != "" {
		fmt.Fprintf(&k3sVars, "k3s_server_url=https://%s:6443\n", serverIP)
	}

	// Collect TLS SANs
	var sans []string
	if k3sCfg.VIP != "" {
		sans = append(sans, k3sCfg.VIP)
	}
	sans = append(sans, masterIPs...)
	if len(sans) > 0 {
		fmt.Fprintf(&k3sVars, "k3s_tls_sans=\"%s\"\n", strings.Join(sans, ","))
	}

	if _, err := fmt.Fprint(w, k3sVars.String()); err != nil {
		return "", fmt.Errorf("failed to write all:vars section: %w", err)
	}

	return keyPath, nil
}

// SanitizeAnsibleHostname removes characters that are invalid in Ansible INI inventory
// host entries (brackets, backslashes, spaces, colons) to prevent parsing errors.
func SanitizeAnsibleHostname(name string) string {
	replacer := strings.NewReplacer(
		"[", "", "]", "",
		"\\", "",
		" ", "-",
		":", "-",
	)
	sanitized := replacer.Replace(name)
	// Collapse multiple consecutive hyphens into one
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}
	// Remove trailing/leading hyphens
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}
