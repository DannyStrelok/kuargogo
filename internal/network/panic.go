package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

type k8sAppList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	} `json:"items"`
}

type k8sResourceList struct {
	Items []struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	} `json:"items"`
}

// TriggerPanic executes the homelab isolation sequence
func (m *Manager) TriggerPanic(out io.Writer, cfg config.ClusterConfig) error {
	policy := cfg.Network.PanicPolicy

	_, _ = fmt.Fprintln(out, "🚨 TRIGGERING HOMELAB PANIC MODE ISOLATION...")

	kubeconfig, err := resolveKubeconfigPath(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(out, "⚠️  Could not resolve kubeconfig path: %v\n", err)
	}

	reachable := false
	if err == nil {
		_, _ = fmt.Fprintln(out, "📡 Checking Kubernetes cluster reachability...")
		reachable = isClusterReachable(kubeconfig)
	}

	if reachable {
		_, _ = fmt.Fprintln(out, "✅ Cluster is reachable. Starting software-level isolation...")

		// 1. Software / GitOps Isolation
		if policy.SoftwareIsolation {
			_, _ = fmt.Fprintln(out, "   🔒 Freezing GitOps & ArgoCD sync policies...")
			if err := m.freezeArgoCD(out, kubeconfig); err != nil {
				_, _ = fmt.Fprintf(out, "   ❌ ArgoCD freeze failed: %v\n", err)
			} else {
				_, _ = fmt.Fprintln(out, "   ✅ ArgoCD sync policies frozen successfully.")
			}
		}

		// 2. Cloudflare Isolation
		if policy.CloudflareKill {
			_, _ = fmt.Fprintln(out, "   🔒 Disabling Cloudflare Tunnels (scaling to 0 replicas)...")
			if err := m.scaleCloudflare(out, kubeconfig, 0); err != nil {
				_, _ = fmt.Fprintf(out, "   ❌ Cloudflare Tunnel disable failed: %v\n", err)
			} else {
				_, _ = fmt.Fprintln(out, "   ✅ Cloudflare Tunnels scaled down to 0.")
			}
		}
	} else {
		_, _ = fmt.Fprintln(out, "⚠️  Cluster is unreachable. Skipping software-level isolation.")
	}

	// 3. Network Isolation
	if policy.NetworkIsolation == "shutdown" || policy.NetworkIsolation == "vlan" {
		if cfg.Network.UplinkPort == "" {
			return fmt.Errorf("network isolation is configured but uplink_port is not set in config")
		}

		_, _ = fmt.Fprintln(out, "📡 Connecting to smart switch...")
		if err := m.driver.Connect(); err != nil {
			return fmt.Errorf("failed to connect to switch: %w", err)
		}
		defer func() { _ = m.driver.Close() }()

		switch policy.NetworkIsolation {
		case "shutdown":
			_, _ = fmt.Fprintf(out, "   🔒 Shutting down Uplink port (%s)...\n", cfg.Network.UplinkPort)
			if err := m.driver.SetPortState(cfg.Network.UplinkPort, false); err != nil {
				return fmt.Errorf("failed to shut down uplink port: %w", err)
			}
			_, _ = fmt.Fprintln(out, "   ✅ Uplink port is DOWN.")
		case "vlan":
			vlan := cfg.Network.QuarantineVLAN
			if vlan == 0 {
				vlan = 666 // Default quarantine VLAN
			}
			_, _ = fmt.Fprintf(out, "   🔒 Moving Uplink port (%s) to Quarantine VLAN %d...\n", cfg.Network.UplinkPort, vlan)
			if err := m.driver.SetPortVLAN(cfg.Network.UplinkPort, vlan); err != nil {
				return fmt.Errorf("failed to reassign uplink port to quarantine VLAN: %w", err)
			}
			_, _ = fmt.Fprintln(out, "   ✅ Uplink port moved to quarantine VLAN.")
		}
	} else {
		_, _ = fmt.Fprintln(out, "ℹ️  Network-level switch isolation skipped (policy: none/not set).")
	}

	if policy.NotifyAdmin {
		_, _ = fmt.Fprintln(out, "\n🚨 ADMIN NOTIFICATION: Panic isolation completed successfully. homelab network is isolated.")
	}

	_ = config.ModifyConfig(func(c *config.ClusterConfig) {
		c.Network.PanicActive = true
	})
	_ = config.SaveConfig()

	return nil
}

// RestorePanic restores normal configurations from panic mode
func (m *Manager) RestorePanic(out io.Writer, cfg config.ClusterConfig) error {
	policy := cfg.Network.PanicPolicy

	_, _ = fmt.Fprintln(out, "🔓 RESTORING HOMELAB FROM PANIC MODE...")

	// 1. Restore Switch Port State / VLAN
	if policy.NetworkIsolation == "shutdown" || policy.NetworkIsolation == "vlan" {
		if cfg.Network.UplinkPort == "" {
			return fmt.Errorf("network isolation is configured but uplink_port is not set in config")
		}

		_, _ = fmt.Fprintln(out, "📡 Connecting to smart switch...")
		if err := m.driver.Connect(); err != nil {
			return fmt.Errorf("failed to connect to switch: %w", err)
		}
		defer func() { _ = m.driver.Close() }()

		switch policy.NetworkIsolation {
		case "shutdown":
			_, _ = fmt.Fprintf(out, "   🔓 Enabling Uplink port (%s)...\n", cfg.Network.UplinkPort)
			if err := m.driver.SetPortState(cfg.Network.UplinkPort, true); err != nil {
				return fmt.Errorf("failed to restore uplink port state: %w", err)
			}
			_, _ = fmt.Fprintln(out, "   ✅ Uplink port is UP.")
		case "vlan":
			_, _ = fmt.Fprintf(out, "   🔓 Restoring Uplink port (%s) to default VLAN 1...\n", cfg.Network.UplinkPort)
			if err := m.driver.SetPortVLAN(cfg.Network.UplinkPort, 1); err != nil {
				return fmt.Errorf("failed to restore uplink port VLAN: %w", err)
			}
			_, _ = fmt.Fprintln(out, "   ✅ Uplink port restored to VLAN 1.")
		}
	}

	kubeconfig, err := resolveKubeconfigPath(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(out, "⚠️  Could not resolve kubeconfig path: %v\n", err)
	}

	reachable := false
	if err == nil {
		_, _ = fmt.Fprintln(out, "📡 Checking Kubernetes cluster reachability (waiting up to 10s for uplink)...")
		for range 5 {
			if isClusterReachable(kubeconfig) {
				reachable = true
				break
			}
			time.Sleep(2 * time.Second)
		}
	}

	if reachable {
		_, _ = fmt.Fprintln(out, "✅ Cluster is reachable. Restoring software configurations...")

		// 2. Restore Cloudflare Tunnels (scale deployment up to 1 replica)
		if policy.CloudflareKill {
			_, _ = fmt.Fprintln(out, "   🔓 Restoring Cloudflare Tunnels (scaling to 1 replica)...")
			if err := m.scaleCloudflare(out, kubeconfig, 1); err != nil {
				_, _ = fmt.Fprintf(out, "   ❌ Cloudflare Tunnel restore failed: %v\n", err)
			} else {
				_, _ = fmt.Fprintln(out, "   ✅ Cloudflare Tunnels scaled to 1 replica.")
			}
		}

		// 3. Restore Software / GitOps auto-sync
		if policy.SoftwareIsolation {
			_, _ = fmt.Fprintln(out, "   🔓 Restoring ArgoCD auto-sync policies...")
			if err := m.restoreArgoCD(out, kubeconfig); err != nil {
				_, _ = fmt.Fprintf(out, "   ❌ ArgoCD restore failed: %v\n", err)
			} else {
				_, _ = fmt.Fprintln(out, "   ✅ ArgoCD sync policies restored successfully.")
			}
		}
	} else {
		_, _ = fmt.Fprintln(out, "⚠️  Cluster is unreachable. Cannot restore software configurations automatically.")
	}

	_ = config.ModifyConfig(func(c *config.ClusterConfig) {
		c.Network.PanicActive = false
	})
	_ = config.SaveConfig()

	return nil
}

func (m *Manager) freezeArgoCD(out io.Writer, kubeconfig string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "get", "application", "-A", "-o", "json")
	cmd.Env = kubeconfigEnv(kubeconfig)

	data, err := cmd.Output()
	if err != nil {
		return err
	}

	var apps k8sAppList
	if err := json.Unmarshal(data, &apps); err != nil {
		return err
	}

	for _, app := range apps.Items {
		_, _ = fmt.Fprintf(out, "      → Freezing app %s in namespace %s...\n", app.Metadata.Name, app.Metadata.Namespace)
		patchCmd := exec.CommandContext(ctx, "kubectl", "patch", "application", app.Metadata.Name,
			"-n", app.Metadata.Namespace, "--type", "merge", "-p", `{"spec":{"syncPolicy":null}}`)
		patchCmd.Env = kubeconfigEnv(kubeconfig)
		if err := patchCmd.Run(); err != nil {
			_, _ = fmt.Fprintf(out, "      ⚠️  Failed to patch app %s: %v\n", app.Metadata.Name, err)
		}
	}

	return nil
}

func (m *Manager) restoreArgoCD(out io.Writer, kubeconfig string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "get", "application", "-A", "-o", "json")
	cmd.Env = kubeconfigEnv(kubeconfig)

	data, err := cmd.Output()
	if err != nil {
		return err
	}

	var apps k8sAppList
	if err := json.Unmarshal(data, &apps); err != nil {
		return err
	}

	for _, app := range apps.Items {
		_, _ = fmt.Fprintf(out, "      → Restoring auto-sync for app %s in namespace %s...\n", app.Metadata.Name, app.Metadata.Namespace)
		patchCmd := exec.CommandContext(ctx, "kubectl", "patch", "application", app.Metadata.Name,
			"-n", app.Metadata.Namespace, "--type", "merge", "-p", `{"spec":{"syncPolicy":{"automated":{"prune":true,"selfHeal":true}}}}`)
		patchCmd.Env = kubeconfigEnv(kubeconfig)
		if err := patchCmd.Run(); err != nil {
			_, _ = fmt.Fprintf(out, "      ⚠️  Failed to restore app %s: %v\n", app.Metadata.Name, err)
		}
	}

	return nil
}

func (m *Manager) scaleCloudflare(out io.Writer, kubeconfig string, replicas int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "get", "deployments", "-A", "-o", "json")
	cmd.Env = kubeconfigEnv(kubeconfig)

	data, err := cmd.Output()
	if err != nil {
		return err
	}

	var resources k8sResourceList
	if err := json.Unmarshal(data, &resources); err != nil {
		return err
	}

	found := false
	for _, res := range resources.Items {
		name := strings.ToLower(res.Metadata.Name)
		if strings.Contains(name, "cloudflare-tunnel") || strings.Contains(name, "cloudflared") {
			found = true
			_, _ = fmt.Fprintf(out, "      → Scaling deployment %s in namespace %s to %d replicas...\n", res.Metadata.Name, res.Metadata.Namespace, replicas)
			scaleCmd := exec.CommandContext(ctx, "kubectl", "scale", "deployment", res.Metadata.Name,
				"-n", res.Metadata.Namespace, fmt.Sprintf("--replicas=%d", replicas))
			scaleCmd.Env = kubeconfigEnv(kubeconfig)
			if err := scaleCmd.Run(); err != nil {
				_, _ = fmt.Fprintf(out, "      ⚠️  Failed to scale deployment %s: %v\n", res.Metadata.Name, err)
			}
		}
	}

	if !found {
		_, _ = fmt.Fprintln(out, "      ℹ️  No Cloudflare Tunnel deployments found in cluster.")
	}

	return nil
}

func isClusterReachable(kubeconfig string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "cluster-info")
	cmd.Env = kubeconfigEnv(kubeconfig)
	return cmd.Run() == nil
}

func resolveKubeconfigPath(cfg config.ClusterConfig) (string, error) {
	p := cfg.K3s.KubeconfigPath
	if p == "" {
		p = "~/.kube/config"
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		p = filepath.Join(home, p[2:])
	}
	return p, nil
}

func kubeconfigEnv(kubeconfigPath string) []string {
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "KUBECONFIG=") {
			env[i] = "KUBECONFIG=" + kubeconfigPath
			return env
		}
	}
	return append(env, "KUBECONFIG="+kubeconfigPath)
}
