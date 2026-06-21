package actions

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/cloudflare"
	"github.com/DannyStrelok/kuargogo/internal/cluster"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/deps"
	"github.com/DannyStrelok/kuargogo/internal/notify"
	"github.com/DannyStrelok/kuargogo/internal/provision"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// OpsUpdate runs the maintenance playbook on all nodes.
// It captures and streams output for TUI display.
func OpsUpdate() tea.Cmd {
	return func() tea.Msg {
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v\n\nPlease install Ansible first.", err)}
		}

		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			msg := "🚀 Starting System Package Update on all nodes...\n"
			if config.IsDryRun() {
				msg = "🧪 [DRY RUN] Starting System Package Update simulation...\n"
			}
			_, _ = writer.Write([]byte(msg))

			playbookDir, err := ansible.FindPlaybookDir()
			if err != nil {
				ch <- fmt.Sprintf("❌ Error: %v", err)
				return
			}
			runner := ansible.NewRunner(playbookDir)
			runner.Output = writer

			result, err := runner.Run("update.yml", "", nil)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ Update failed: %v", err)
				return
			}

			if result.Success {
				ch <- fmt.Sprintf("\n✅ Update completed successfully in %s", result.Duration.Round(1))
			} else {
				ch <- fmt.Sprintf("\n❌ Update failed (exit code: %d)", result.ExitCode)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsNfs runs the NFS setup playbook on all nodes.
// It captures and streams output for TUI display.
func OpsNfs() tea.Cmd {
	return func() tea.Msg {
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v\n\nPlease install Ansible first.", err)}
		}

		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			msg := "🚀 Starting NFS Setup on all nodes...\n"
			if config.IsDryRun() {
				msg = "🧪 [DRY RUN] Starting NFS Setup simulation...\n"
			}
			_, _ = writer.Write([]byte(msg))

			playbookDir, err := ansible.FindPlaybookDir()
			if err != nil {
				ch <- fmt.Sprintf("❌ Error: %v", err)
				return
			}
			runner := ansible.NewRunner(playbookDir)
			runner.Output = writer

			result, err := runner.Run("setup-nfs.yml", "", nil)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ NFS setup failed: %v", err)
				return
			}

			if result.Success {
				ch <- fmt.Sprintf("\n✅ NFS setup completed successfully in %s", result.Duration.Round(1))
			} else {
				ch <- fmt.Sprintf("\n❌ NFS setup failed (exit code: %d)", result.ExitCode)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsUpdateWithNotify runs update and sends Telegram notification.
func OpsUpdateWithNotify() tea.Cmd {
	return func() tea.Msg {
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v", err)}
		}

		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			msg := "🚀 Starting System Package Update with Telegram alerts...\n"
			if config.IsDryRun() {
				msg = "🧪 [DRY RUN] Starting System Package Update (Notify) simulation...\n"
			}
			_, _ = writer.Write([]byte(msg))

			playbookDir, err := ansible.FindPlaybookDir()
			if err != nil {
				ch <- fmt.Sprintf("❌ Error: %v", err)
				return
			}
			runner := ansible.NewRunner(playbookDir)

			// Capture output for both writer and notifier
			var buf bytes.Buffer
			runner.Output = io.MultiWriter(writer, &buf)

			result, err := runner.Run("update.yml", "", nil)

			// Patch result stdout/stderr with collected buffer
			if result != nil {
				result.Stdout = buf.String()
			}

			// Send notification
			notifier := notify.NewTelegramNotifier()
			if notifier.IsConfigured() && result != nil {
				_ = notifier.NotifyAnsibleResult(result)
			}

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Update failed: %v", err)
				return
			}

			notifyStatus := ""
			if notifier.IsConfigured() {
				notifyStatus = "\n📱 Telegram notification sent"
			}

			if result != nil && result.Success {
				ch <- fmt.Sprintf("\n✅ Update completed successfully%s", notifyStatus)
			} else {
				exitCode := -1
				if result != nil {
					exitCode = result.ExitCode
				}
				ch <- fmt.Sprintf("\n❌ Update failed (exit code: %d)%s", exitCode, notifyStatus)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsBackupSystem deploys Velero Disaster Recovery using S3 credentials.
func OpsBackupSystem() tea.Cmd {
	return func() tea.Msg {
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v\n\nPlease install Ansible first.", err)}
		}

		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			msg := "🚀 Starting Velero Disaster Recovery deployment...\n"
			if config.IsDryRun() {
				msg = "🧪 [DRY RUN] Starting Velero Disaster Recovery simulation...\n"
			}
			_, _ = writer.Write([]byte(msg))

			result, err := ansible.RunOpsBackup(config.IsDryRun(), nil, nil, writer)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ Deployment failed: %v", err)
				return
			}

			if result.Success {
				ch <- "\n✅ Velero Master Backup Deployed successfully."
			} else {
				ch <- fmt.Sprintf("\n❌ Velero deployment failed (exit code: %d)", result.ExitCode)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsObservability deploys the LGTM Stack with real-time log streaming.
func OpsObservability() tea.Cmd {
	return func() tea.Msg {
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v\n\nPlease install Ansible first.", err)}
		}

		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			msg := "🚀 Starting LGTM Observability Stack deployment...\n"
			if config.IsDryRun() {
				msg = "🧪 [DRY RUN] Starting LGTM Observability Stack simulation...\n"
			}
			_, _ = writer.Write([]byte(msg))

			// 1. Ensure Secure Credentials
			cfg := config.GetConfig()
			if cfg.Monitoring.GrafanaAdminPassword == "" {
				_, _ = writer.Write([]byte("🔑 Generating secure Grafana admin password...\n"))
				newPass := generateSecurePassword(24)
				_ = config.ModifyConfig(func(c *config.ClusterConfig) {
					c.Monitoring.GrafanaAdminPassword = config.Secret(newPass)
				})
				_ = config.SaveConfig()
			}

			// 2. Run Ansible Playbook (will use the configured password)
			result, err := ansible.RunOpsObservability(config.IsDryRun(), nil, nil, writer)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ Deployment failed: %v", err)
				return
			}
			if !result.Success {
				ch <- fmt.Sprintf("\n❌ Deployment failed (exit code %d)", result.ExitCode)
				return
			}

			ch <- "\n✅ LGTM Observability Stack Deployed."

			// 3. Automate Cloudflare Exposure
			if err := RunCloudflareSync(writer); err != nil {
				ch <- fmt.Sprintf("\n⚠️  Cloudflare exposure automation failed: %v", err)
			}

			ch <- "\n\n✨ Observability hardening completed successfully!"
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsCloudflare deploys Cert-Manager and Cloudflared Zero Trust.
func OpsCloudflare(provision bool) tea.Cmd {
	return func() tea.Msg {
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v\n\nPlease install Ansible first.", err)}
		}

		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			msg := "🚀 Starting Cloudflare Zero Trust deployment...\n"
			if config.IsDryRun() {
				msg = "🧪 [DRY RUN] Starting Cloudflare Zero Trust simulation...\n"
			}
			_, _ = writer.Write([]byte(msg))

			if provision {
				_, _ = writer.Write([]byte("🌐 Automating Cloudflare Tunnel provisioning...\n"))

				if _, err := cloudflare.ProvisionAndSaveTunnel(context.Background(), writer); err != nil {
					ch <- fmt.Sprintf("❌ Error: %v", err)
					return
				}

				ch <- "✅ Cloudflare tunnel provisioned and saved. Proceeding with deployment...\n"
			}

			// Run Ansible Playbook
			result, err := ansible.RunOpsCloudflare(config.IsDryRun(), nil, nil, writer)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ Ansible Error: %v", err)
				return
			}

			if result.Success {
				ch <- "\n✅ Cloudflare Zero Trust Infrastructure Deployed."
				ch <- "\n✨ All systems go! Your cluster is now securely bridged via Cloudflare."
			} else {
				ch <- fmt.Sprintf("\n❌ Deployment failed (exit code %d)", result.ExitCode)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// CloudflareDomainsMsg carries the list of discovered domains
type CloudflareDomainsMsg struct {
	Domains []string
	Err     error
}

// GetCloudflareDomains fetches available zones from Cloudflare for the TUI picker.
func GetCloudflareDomains() tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		if cfg.Cloudflare.APIToken == "" {
			return CloudflareDomainsMsg{Err: fmt.Errorf("cloudflare APIToken not set")}
		}

		mgr, err := cloudflare.NewManager(cfg.Cloudflare)
		if err != nil {
			return CloudflareDomainsMsg{Err: err}
		}

		zones, err := mgr.ListZones(context.Background())
		if err != nil {
			return CloudflareDomainsMsg{Err: err}
		}

		return CloudflareDomainsMsg{Domains: zones}
	}
}

// OpsArgoCD deploys the GitOps Engine
func OpsArgoCD() tea.Cmd {
	return func() tea.Msg {
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v\n\nPlease install Ansible first.", err)}
		}

		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			msg := "🚀 Starting ArgoCD GitOps Engine deployment...\n"
			if config.IsDryRun() {
				msg = "🧪 [DRY RUN] Starting ArgoCD GitOps Engine simulation...\n"
			}
			_, _ = writer.Write([]byte(msg))

			// Buffer to capture full output for password extraction
			var fullBuf bytes.Buffer
			multiWriter := io.MultiWriter(writer, &fullBuf)

			result, err := ansible.RunOpsArgoCD(config.IsDryRun(), nil, nil, multiWriter)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ Deployment failed: %v", err)
				return
			}

			output := fullBuf.String()
			// Extract password if available
			re := regexp.MustCompile(`ARGOCD_ADMIN_PASSWORD=([^\s]+)`)
			match := re.FindStringSubmatch(output)
			passMsg := ""
			if len(match) > 1 {
				passMsg = fmt.Sprintf("\n\n🔑 Initial Admin Password: %s", match[1])
			}

			if result.Success {
				ch <- fmt.Sprintf("\n✅ ArgoCD GitOps Engine Deployed%s", passMsg)
			} else {
				ch <- fmt.Sprintf("\n❌ ArgoCD deployment failed (exit code %d)", result.ExitCode)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsKargo deploys the Promotion Engine
func OpsKargo() tea.Cmd {
	return func() tea.Msg {
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v\n\nPlease install Ansible first.", err)}
		}

		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			// 1. Ensure Kargo Admin Credentials
			cfg := config.GetConfig()
			if cfg.GitOps.KargoEngine == nil {
				_ = config.ModifyConfig(func(c *config.ClusterConfig) {
					c.GitOps.KargoEngine = &config.KargoEngine{}
				})
				cfg.GitOps.KargoEngine = &config.KargoEngine{}
			}

			if cfg.GitOps.KargoEngine.AdminPassword == "" || cfg.GitOps.KargoEngine.AdminPasswordHash == "" {
				_, _ = writer.Write([]byte("🔑 Generating secure Kargo admin credentials...\n"))

				pass := string(cfg.GitOps.KargoEngine.AdminPassword)
				if pass == "" {
					pass = generateSecurePassword(24)
				}

				hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
				if err != nil {
					ch <- fmt.Sprintf("❌ Hash generation failed: %v\n", err)
					return
				}

				_ = config.ModifyConfig(func(c *config.ClusterConfig) {
					if c.GitOps.KargoEngine == nil {
						c.GitOps.KargoEngine = &config.KargoEngine{}
					}
					c.GitOps.KargoEngine.AdminPassword = config.Secret(pass)
					c.GitOps.KargoEngine.AdminPasswordHash = string(hash)
				})
				_ = config.SaveConfig()
			}

			// 2. Run Ansible Playbook
			result, err := ansible.RunOpsKargo(config.IsDryRun(), nil, nil, writer)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ Deployment failed: %v", err)
				return
			}

			if result.Success {
				ch <- "\n✅ Kargo Promotion Engine Deployed"
				finalCfg := config.GetConfig()
				if finalCfg.GitOps.KargoEngine != nil {
					ch <- fmt.Sprintf("\n🔑 Admin Password: %s", string(finalCfg.GitOps.KargoEngine.AdminPassword))
				}
				ch <- "\n🌐 Access it via your Kargo ingress host."
			} else {
				ch <- fmt.Sprintf("\n❌ Kargo deployment failed (exit code %d)", result.ExitCode)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsSetMasterPassphrase stores the master key and triggers a full config re-save (encryption).
func OpsSetMasterPassphrase(passphrase string) tea.Cmd {
	return func() tea.Msg {
		if passphrase == "" {
			return ResultMsg{Output: "❌ Error: Passphrase cannot be empty."}
		}

		// 1. Store locally in keyring
		if err := config.StoreMasterKey(passphrase); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error storing master key: %v\n\nTip: If you are in WSL, set the KGG_MASTER_PASSPHRASE environment variable in your .bashrc instead.", err)}
		}

		// 2. Ensure Salt exists
		salt, err := config.EnsureSalt()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error generating salt: %v", err)}
		}

		// 3. Save Config to trigger encryption
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error saving/encrypting configuration: %v", err)}
		}

		return ResultMsg{Output: fmt.Sprintf("✅ Master Passphrase set successfully!\n\n🔐 All sensitive fields in kuargogo.yaml are now encrypted.\n\n🧂 CLUSTER SALT (Backup this!):\n%s", salt)}
	}
}

// OpsViewDecryptedConfig verifies the passphrase and returns a plaintext YAML view of the active context.
func OpsViewDecryptedConfig(passphrase string) tea.Cmd {
	return func() tea.Msg {
		// 1. Verify Passphrase (sudo-style challenge)
		stored, err := config.GetMasterKey()
		if err != nil || stored == "" {
			return ResultMsg{Output: "❌ Error: Master Passphrase not found in keyring. Please set it first."}
		}
		if passphrase != stored {
			return ResultMsg{Output: "❌ Access Denied: Incorrect Master Passphrase. View blocked for security."}
		}

		// 2. Marshall to Plaintext YAML
		// Hack: json.Marshal doesn't trigger MarshalYAML, so it sees Secrets as strings.
		// We then unmarshal that JSON back into a generic map and re-marshal to YAML.
		cfg := config.GetConfig()

		jsonData, err := json.Marshal(cfg)
		if err != nil {
			return ResultMsg{Output: "❌ Error serializing config: " + err.Error()}
		}

		var genericMap map[string]interface{}
		if err := json.Unmarshal(jsonData, &genericMap); err != nil {
			return ResultMsg{Output: "❌ Error processing data: " + err.Error()}
		}

		yamlData, err := yaml.Marshal(genericMap)
		if err != nil {
			return ResultMsg{Output: "❌ Error generating YAML: " + err.Error()}
		}

		ctxName := config.GetCurrentContext()
		header := "⚠️  DECRYPTED CONFIGURATION VIEW (CONFIDENTIAL)\n"
		header += "--------------------------------------------------\n"
		header += "Context: " + ctxName + "\n"
		header += "--------------------------------------------------\n\n"

		return ResultMsg{Output: header + "```yaml\n" + string(yamlData) + "\n```"}
	}
}

// OpsGrafanaLocalAccess creates an SSH tunnel to Grafana and opens the browser.
func OpsGrafanaLocalAccess() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)

		ctx, cancel := context.WithCancel(context.Background())
		RegisterTunnel(cancel)

		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			tm := cluster.NewTunnelManager(writer)

			ch <- "🚀 Initializing Local Access to Grafana...\n"

			// Start tunnel on port 3000
			err := tm.StartGrafanaTunnel(ctx, 3000)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ Failed: %v", err)
				return
			}

			// Open browser
			ch <- "🌐 Opening your browser at http://localhost:3000...\n"
			err = cluster.OpenBrowser("http://localhost:3000")
			if err != nil {
				ch <- fmt.Sprintf("⚠️  Note: Tunnel is active, but couldn't open browser automatically: %v\n", err)
			}

			ch <- "\n🔓 Local Access ACTIVE"
			ch <- "\n----------------------------------------"
			ch <- "\nPress 'esc' or 'q' to close the tunnel and return."

			// Wait for context cancellation (when user pops the view)
			<-ctx.Done()
			ch <- "\n🔌 Closing tunnel..."
			time.Sleep(500 * time.Millisecond)
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

func generateSecurePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	password := make([]byte, length)
	for i := range password {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		password[i] = charset[num.Int64()]
	}
	return string(password)
}

// ConfigUpdateKargoPipeline updates or adds a Kargo pipeline in the active context.
// If index is -1, it appends a new one.
func ConfigUpdateKargoPipeline(pipeline config.KargoPipeline, index int) tea.Cmd {
	return func() tea.Msg {
		_ = config.ModifyConfig(func(c *config.ClusterConfig) {
			if index >= 0 && index < len(c.GitOps.Pipelines) {
				c.GitOps.Pipelines[index] = pipeline
			} else {
				c.GitOps.Pipelines = append(c.GitOps.Pipelines, pipeline)
			}
		})
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error saving configuration: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Kargo pipeline %q updated successfully.", pipeline.Name)}
	}
}

func LaunchK9s() tea.Cmd {
	if err := deps.CheckDependency("k9s"); err != nil {
		return func() tea.Msg {
			return ResultMsg{Output: "❌ K9s not found. Please install it first from 'Setup Admin PC' menu."}
		}
	}
	return nil
}

// RemoteSyncKubeconfig fetches the live kubeconfig from a master node and patches it with the cluster VIP.
func RemoteSyncKubeconfig() error {
	cfg := config.GetConfig()

	// 1. Identify a Master Node
	var master *config.Node
	for _, n := range cfg.Nodes {
		if n.Role == "master" || n.Role == "control-plane" {
			master = &n
			break
		}
	}
	if master == nil {
		return fmt.Errorf("no master node found in configuration")
	}

	// 2. Setup SSH
	keyPath, err := cfg.SSH.ExpandedKeyPath()
	if err != nil {
		return err
	}

	executor, err := provision.NewExecutor(master.User, keyPath, false)
	if err != nil {
		return err
	}
	executor.Stdout = io.Discard
	executor.Stderr = io.Discard

	// 3. Fetch remote config
	output, err := executor.ExecuteCommand(master.IP, cfg.SSH.Port, "sudo cat /etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return fmt.Errorf("could not fetch /etc/rancher/k3s/k3s.yaml from %s: %w", master.IP, err)
	}

	// 4. Patch URL
	target := cfg.K3s.VIP
	if target == "" {
		target = master.IP
	}
	// Replace the internal loopback with the public/VIP address
	patched := strings.Replace(output, "127.0.0.1", target, 1)

	// 5. Save locally
	localPath, err := cfg.K3s.ExpandedKubeconfigPath()
	if err != nil || localPath == "" {
		return fmt.Errorf("local kubeconfig path is invalid")
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
		return err
	}

	return os.WriteFile(localPath, []byte(patched), 0600)
}

// ConfigRestore performs a configuration restoration from the TUI.
func ConfigRestore(backupName string) tea.Cmd {
	return func() tea.Msg {
		if err := config.RestoreBackup(backupName); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Restoration failed: %v", err)}
		}
		return ResultMsg{Output: "✅ Configuration restored successfully.\n\nThe cluster state has been reloaded in memory."}
	}
}
