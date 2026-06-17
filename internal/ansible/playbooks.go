package ansible

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/deps"
)

// NodeMetrics holds the parsed metrics from Ansible doctor output.
type NodeMetrics struct {
	Host    string `json:"host"`
	IP      string `json:"ip"`
	CPUTemp string `json:"cpu_temp"`
	Disk    string `json:"disk"`
	Mem     string `json:"mem"`
	Load    string `json:"load"`
	Uptime  string `json:"uptime"`
}

// ParseDoctorMetrics extracts METRICS_JSON lines from Ansible output.
func ParseDoctorMetrics(output string) []NodeMetrics {
	var metrics []NodeMetrics

	re := regexp.MustCompile(`METRICS_JSON:(\{[^}]+\})`)
	matches := re.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			var m NodeMetrics
			if err := json.Unmarshal([]byte(match[1]), &m); err == nil {
				metrics = append(metrics, m)
			}
		}
	}

	return metrics
}

// ExtractClusterToken parses the Ansible output for the CLUSTER_TOKEN prefix and returns the token.
func ExtractClusterToken(output string) string {
	re := regexp.MustCompile(`CLUSTER_TOKEN=([^\s"]+)`)
	match := re.FindStringSubmatch(output)
	if len(match) >= 2 {
		return match[1]
	}
	return ""
}

// FindPlaybookDir locates the infrastructure/playbooks directory.
// It prioritizes local overrides during development, otherwise it extracts from the embedded Go binary cache.
func FindPlaybookDir() (string, error) {
	cwd, err := os.Getwd()
	if err == nil {
		candidates := []string{
			filepath.Join(cwd, "infrastructure", "playbooks"),
			filepath.Join(cwd, "..", "..", "infrastructure", "playbooks"),
		}

		for _, path := range candidates {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				absPath, _ := filepath.Abs(path)
				return absPath, nil
			}
		}
	}

	// Always default to embedded playbooks extraction
	return EnsurePlaybooksExtracted()
}

// runPlaybook is the shared entrypoint for all playbook execution.
// It locates the playbook directory, configures the runner, and executes.
func runPlaybook(playbook, limit string, dryRun bool, tags []string, extraVars map[string]string, output io.Writer, skipHostKey bool) (*Result, error) {
	playbookDir, err := FindPlaybookDir()
	if err != nil {
		return nil, err
	}

	runner := NewRunner(playbookDir)
	runner.DryRun = dryRun
	runner.Tags = tags
	runner.SkipHostKeyChecking = skipHostKey
	if output != nil {
		runner.Output = output
	}

	return runner.Run(playbook, limit, extraVars)
}

// RunProvision executes the provision.yml playbook against a specific node.
func RunProvision(nodeName string, createUser bool, password string, dryRun bool, tags []string, output io.Writer) (*Result, error) {
	extraVars := map[string]string{
		"create_user": fmt.Sprintf("%v", createUser),
		"kgg_user":    "kgg-admin",
	}

	// For initial provisioning of some Debian distros, we need the sudo password
	// if we haven't established passwordless sudo yet.
	if password != "" {
		extraVars["ansible_become_password"] = password
	}

	// Pass the cluster public key path so the common role installs the correct key
	if createUser {
		keyPath, err := config.ResolveKeyPath("")
		if err == nil && keyPath != "" {
			if _, err := os.Stat(keyPath + ".pub"); err == nil {
				extraVars["kgg_cluster_pubkey"] = keyPath + ".pub"
			}
		}
	}

	return runPlaybook("provision.yml", nodeName, dryRun, tags, extraVars, output, true)
}

// RunGPUSetup executes the gpu-setup.yml playbook against a specific node.
func RunGPUSetup(nodeName string, dryRun bool, tags []string, output io.Writer) (*Result, error) {
	return runPlaybook("gpu-setup.yml", nodeName, dryRun, tags, nil, output, false)
}

// RunMountStorage executes the mount-storage.yml playbook.
func RunMountStorage(nodeName, disk, mountPoint string, dryRun bool, tags []string, output io.Writer) (*Result, error) {
	return runPlaybook("mount-storage.yml", nodeName, dryRun, tags, map[string]string{
		"target_disk": disk,
		"mount_point": mountPoint,
	}, output, false)
}

// RunK3sInit executes the cluster-deploy.yml playbook to install K3s on the master node.
func RunK3sInit(nodeName string, ha bool, vipIP string, dryRun bool, tags []string, output io.Writer) (*Result, error) {
	extraVars := map[string]string{
		"k3s_ha": fmt.Sprintf("%v", ha),
	}
	if vipIP != "" {
		extraVars["k3s_vip_ip"] = vipIP
	}
	// cluster-deploy.yml is the unified playbook for cluster lifecycle
	return runPlaybook("cluster-deploy.yml", nodeName, dryRun, tags, extraVars, output, true)
}

// RunK3sJoin executes the cluster-deploy.yml playbook to add a node to the cluster.
func RunK3sJoin(nodeName, masterIP, token, role, vipIP string, dryRun bool, tags []string, output io.Writer) (*Result, error) {
	extraVars := map[string]string{
		"master_ip": masterIP,
		"k3s_token": token,
		"join_role": role,
	}
	if vipIP != "" {
		extraVars["k3s_vip_ip"] = vipIP
	}
	// cluster-deploy.yml handles join nodes via roles and inventory groups
	return runPlaybook("cluster-deploy.yml", nodeName, dryRun, tags, extraVars, output, true)
}

// RunK3sDrain executes the k3s-drain.yml playbook on the master to drain a target node.
func RunK3sDrain(masterNodeName, targetNodeName string, dryRun bool, tags []string, output io.Writer) (*Result, error) {
	return runPlaybook("k3s-drain.yml", masterNodeName, dryRun, tags, map[string]string{
		"target_node_name": targetNodeName,
	}, output, true)
}

// RunK3sReset executes the k3s-reset.yml playbook to uninstall K3s from a node.
func RunK3sReset(nodeName string, dryRun bool, tags []string, output io.Writer) (*Result, error) {
	return runPlaybook("k3s-reset.yml", nodeName, dryRun, tags, nil, output, true)
}

// RunSite executes the site.yml master playbook for full cluster deployment.
func RunSite(dryRun bool, tags []string, output io.Writer) (*Result, error) {
	cfg := config.GetConfig()
	extraVars := map[string]string{
		"k3s_ha": fmt.Sprintf("%v", cfg.K3s.HA),
	}

	if cfg.K3s.VIP != "" {
		extraVars["k3s_vip_ip"] = cfg.K3s.VIP
	}

	if extraVars["k3s_vip_ip"] != "" && output != nil {
		_, _ = fmt.Fprintf(output, "🌐 Using Cluster VIP: %s\n", extraVars["k3s_vip_ip"])
	}

	// Inject all platform configuration
	for k, v := range getNFSExtraVars(cfg) {
		extraVars[k] = v
	}
	for k, v := range getGitOpsExtraVars(cfg) {
		extraVars[k] = v
	}
	for k, v := range getCloudflareExtraVars(cfg) {
		extraVars[k] = v
	}

	return runPlaybook("site.yml", "", dryRun, tags, extraVars, output, true)
}

// RunDoctor executes the doctor.yml playbook for health diagnostics.
func RunDoctor(dryRun bool, tags []string, output io.Writer) (*Result, error) {
	return runPlaybook("doctor.yml", "", dryRun, tags, nil, output, false)
}

// crossCompileKGGCLI compiles the kuargogo binary for the target Linux architectures
// using the Go installation on the admin host (Windows/macOS/Linux).
// Binaries are placed in infrastructure/bin/ relative to the project root.
func crossCompileKGGCLI(projectRoot string, output io.Writer) error {
	binDir := filepath.Join(projectRoot, "infrastructure", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Architectures to compile: arm64 (RPi 3 64-bit) and amd64 (x86 servers)
	targets := []struct {
		GOARCH string
		GOARM  string
	}{
		{"arm64", ""},
		{"amd64", ""},
		{"arm", "7"}, // 32-bit ARM (RPi with 32-bit OS)
	}

	for _, t := range targets {
		outPath := filepath.Join(binDir, fmt.Sprintf("kgg-%s", t.GOARCH))
		_, _ = fmt.Fprintf(output, "🔨 Compiling kuargogo for linux/%s → %s\n", t.GOARCH, outPath)

		cmd := exec.Command("go", "build",
			"-o", outPath,
			"-ldflags", "-s -w",
			"./cmd/kgg",
		)
		cmd.Dir = projectRoot
		cmd.Env = append(os.Environ(),
			"GOOS=linux",
			"GOARCH="+t.GOARCH,
			"CGO_ENABLED=0",
		)
		if t.GOARM != "" {
			cmd.Env = append(cmd.Env, "GOARM="+t.GOARM)
		}

		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to compile for linux/%s: %w\n%s", t.GOARCH, err, out)
		}
	}

	return nil
}

// findProjectRoot walks up from the playbook directory to find the Go module root.
func findProjectRoot() (string, error) {
	// When running in dev, the executable is in the project root.
	// Use the OS to find the executable path.
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Walk up until we find go.mod
	dir := filepath.Dir(exe)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback: use cwd
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not find project root (go.mod not found)")
	}
	_ = runtime.GOOS // suppress unused import
	return cwd, nil
}

// RunInfraInit executes the infra-init.yml playbook to provision the infrastructure manager.
// It first cross-compiles the kuargogo binary for Linux targets using the local Go installation.
func RunInfraInit(dryRun bool, tags []string, limit string, extraVars map[string]string, output io.Writer) (*Result, error) {
	// Step 1: Cross-compile binaries before invoking Ansible
	projectRoot, err := findProjectRoot()
	if err != nil {
		_, _ = fmt.Fprintf(output, "⚠️  Could not determine project root: %v\n", err)
	} else {
		_, _ = fmt.Fprintln(output, "🔧 Pre-compiling kuargogo binaries for Linux targets...")
		if err := crossCompileKGGCLI(projectRoot, output); err != nil {
			return nil, fmt.Errorf("cross-compilation failed: %w", err)
		}
		_, _ = fmt.Fprintln(output, "✅ Binaries ready.")
	}

	// Step 2: Run the playbook (Skip host key checking for init)
	return runPlaybook("infra-init.yml", limit, dryRun, tags, extraVars, output, true)
}

// RunInfraBotUpdate executes a specialized version of infra-init.yml that only
// updates the Telegram bot Python code and the kuargogo binary.
// It uses tags [bot, rkcli] to skip heavy system tasks (apt, pip, etc).
func RunInfraBotUpdate(dryRun bool, extraVars map[string]string, output io.Writer) (*Result, error) {
	cfg := config.GetConfig()
	node := cfg.GetInfraManager()
	if node == nil {
		return nil, fmt.Errorf("no infrastructure manager found in configuration")
	}

	// Step 1: Cross-compile binaries before invoking Ansible
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find project root: %w", err)
	}
	_, _ = fmt.Fprintln(output, "🔧 Pre-compiling kuargogo binaries for Linux targets...")
	if err := crossCompileKGGCLI(projectRoot, output); err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintln(output, "✅ Binaries ready.")

	// Ensure kgg_config_file is always present
	if extraVars == nil {
		extraVars = make(map[string]string)
	}
	if _, ok := extraVars["kgg_config_file"]; !ok {
		configPath := config.GetConfigPath()
		extraVars["kgg_config_file"] = configPath
		if content, err := os.ReadFile(configPath); err == nil {
			extraVars["kgg_config_content"] = string(content)
		}
	}

	// Filter by tags to make it fast
	tags := []string{"bot", "rkcli"}

	return runPlaybook("infra-init.yml", node.Name, dryRun, tags, extraVars, output, true)
}

// RunLonghornInit executes the longhorn.yml playbook to deploy the storage class.
func RunLonghornInit(dryRun bool, tags []string, output io.Writer) (*Result, error) {
	return runPlaybook("longhorn.yml", "", dryRun, tags, nil, output, false)
}

// RunLonghornStatus executes the longhorn-status.yml playbook to check pod status.
func RunLonghornStatus(masterIP string, dryRun bool, output io.Writer) (*Result, error) {
	return runPlaybook("longhorn-status.yml", "", dryRun, nil, nil, output, false)
}

// RunOpsBackup executes the ops-velero.yml playbook to install Velero Disaster Recovery.
func RunOpsBackup(dryRun bool, tags []string, extraVars map[string]string, output io.Writer) (*Result, error) {
	if extraVars == nil {
		cfg := config.GetConfig()
		extraVars = getBackupExtraVars(cfg)
	}
	return runPlaybook("ops-velero.yml", "", dryRun, tags, extraVars, output, false)
}

// PreprocessInfraVars builds the common set of extra-vars required by infra-init.yml.
// This ensures consistency between CLI and TUI deployments.
func PreprocessInfraVars(node *config.Node) (map[string]string, error) {
	cfg := config.GetConfig()

	// Resolve the actual cluster private key on the host to copy it to the RPi
	clusterKeySrc, err := config.ResolveKeyPath("")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cluster private key: %w", err)
	}

	// Serialize nodes for health check logic in kgg-agent.py
	type nodeData struct {
		Name string `json:"name"`
		IP   string `json:"ip"`
		Role string `json:"role"`
		MAC  string `json:"mac"`
		User string `json:"user"`
	}
	var nodes []nodeData
	for _, n := range cfg.Nodes {
		if n.Role == "infra-manager" {
			continue
		}
		role := "agent"
		if n.Role == "master" || n.Role == "control-plane" {
			role = "server"
		}
		nodes = append(nodes, nodeData{
			Name: n.Name,
			IP:   n.IP,
			Role: role,
			MAC:  n.MAC,
			User: n.User,
		})
	}
	nodesJSON, _ := json.Marshal(nodes)

	// Determine the destination path for the cluster private key on the RPi.
	// We want to use the same filename configured in the user's config to avoid mismatches.
	keyName := "kgg_cluster_id" // Default
	if cfg.SSH.PrivateKeyPath != "" {
		keyName = filepath.Base(cfg.SSH.PrivateKeyPath)
	}

	configPath := config.GetConfigPath()

	vars := map[string]string{
		"telegram_token":               string(cfg.Telegram.BotToken),
		"telegram_admin_id":            fmt.Sprintf("%d", cfg.Telegram.AdminID),
		"telegram_timezone":            cfg.Telegram.Timezone,
		"telegram_summary_time":        cfg.Telegram.DailySummaryTime,
		"kgg_maintenance_mode":         fmt.Sprintf("%t", cfg.MaintenanceMode),
		"kgg_cluster_private_key_src":  clusterKeySrc,
		"kgg_cluster_private_key_path": fmt.Sprintf("/home/kgg-admin/.ssh/%s", keyName),
		"nodes":                        string(nodesJSON),
		"kgg_manager_ip":               node.IP,
		"kgg_node_name":                node.Name,
		"kgg_config_file":              configPath,
		"kgg_hardware_enabled":         fmt.Sprintf("%t", cfg.HardwareEnabled),
	}

	// Read config content to avoid WSL/DrvFs 0-byte file copy bugs
	if content, err := os.ReadFile(configPath); err == nil {
		vars["kgg_config_content"] = string(content)
	}

	// Add master passphrase if available locally (for distributed decryption)
	if mKey, err := config.GetMasterKey(); err == nil && mKey != "" {
		vars["kgg_master_passphrase"] = mKey
	}

	// Inject NFS configuration
	for k, v := range getNFSExtraVars(cfg) {
		vars[k] = v
	}

	// Optional: Pass WOL interface if discovery is configured
	if cfg.Discovery.Interface != "" {
		vars["kgg_wol_interface"] = cfg.Discovery.Interface
	}

	return vars, nil
}

// RunOpsObservability executes the ops-observability.yml playbook to deploy the LGTM stack.
func RunOpsObservability(dryRun bool, tags []string, extraVars map[string]string, output io.Writer) (*Result, error) {
	if extraVars == nil {
		cfg := config.GetConfig()
		extraVars = getCloudflareExtraVars(cfg)

		// Add Monitoring vars
		for k, v := range getMonitoringExtraVars(cfg) {
			extraVars[k] = v
		}

		// Add Infrastructure vars for the MQTT bridge
		infraVars := getInfrastructureExtraVars(cfg)
		for k, v := range infraVars {
			extraVars[k] = v
		}
	}
	return runPlaybook("ops-observability.yml", "", dryRun, tags, extraVars, output, false)
}

// RunOpsCloudflare executes the ops-cloudflare.yml playbook to deploy Cert-Manager and Zero Trust Tunnels.
func RunOpsCloudflare(dryRun bool, tags []string, extraVars map[string]string, output io.Writer) (*Result, error) {
	if extraVars == nil {
		cfg := config.GetConfig()
		extraVars = getCloudflareExtraVars(cfg)
	}
	return runPlaybook("ops-cloudflare.yml", "", dryRun, tags, extraVars, output, false)
}

// RunOpsArgoCD executes the ops-argocd.yml playbook to install the GitOps engine.
func RunOpsArgoCD(dryRun bool, tags []string, extraVars map[string]string, output io.Writer) (*Result, error) {
	if extraVars == nil {
		cfg := config.GetConfig()
		extraVars = getGitOpsExtraVars(cfg)
	}
	return runPlaybook("ops-argocd.yml", "", dryRun, tags, extraVars, output, false)
}

// RunOpsKargo executes the ops-kargo.yml playbook to install the Promotion engine.
func RunOpsKargo(dryRun bool, tags []string, extraVars map[string]string, output io.Writer) (*Result, error) {
	if extraVars == nil {
		cfg := config.GetConfig()
		extraVars = getGitOpsExtraVars(cfg)
	}
	return runPlaybook("ops-kargo.yml", "", dryRun, tags, extraVars, output, false)
}

// getNFSExtraVars builds the common set of extra-vars required for NFS support across playbooks.
func getNFSExtraVars(cfg config.ClusterConfig) map[string]string {
	vars := map[string]string{
		"nfs_enabled": fmt.Sprintf("%t", cfg.NFS.Enabled),
		"nfs_server":  cfg.NFS.Server,
	}

	if len(cfg.NFS.Shares) > 0 {
		sharesJSON, _ := json.Marshal(cfg.NFS.Shares)
		vars["nfs_shares"] = string(sharesJSON)
	}

	return vars
}

// getGitOpsExtraVars serializes GitOps configuration for Ansible.
func getGitOpsExtraVars(cfg config.ClusterConfig) map[string]string {
	vars := make(map[string]string)

	// Serialize the entire GitOps struct to JSON for the 'argocd' role
	gitopsJSON, _ := json.Marshal(cfg.GitOps)
	vars["gitops_config"] = string(gitopsJSON)

	// User preference: .homelab for internal routing
	vars["argocd_ingress_host"] = "argocd.homelab"

	if cfg.GitOps.KargoEngine != nil {
		if cfg.GitOps.KargoEngine.AdminPasswordHash != "" {
			vars["kargo_admin_password_hash"] = cfg.GitOps.KargoEngine.AdminPasswordHash
		}
		if cfg.GitOps.KargoEngine.TokenSigningKey != "" {
			vars["kargo_token_signing_key"] = string(cfg.GitOps.KargoEngine.TokenSigningKey)
		}
	}

	return vars
}

// getCloudflareExtraVars builds the extra-vars required for Cloudflare ops.
// It sends account/tunnel credentials plus the first domain for backward compatibility
// with existing Ansible roles that expect a single cloudflare_domain.
func getCloudflareExtraVars(cfg config.ClusterConfig) map[string]string {
	vars := map[string]string{
		"cloudflare_api_token":    string(cfg.Cloudflare.APIToken),
		"cloudflare_tunnel_token": string(cfg.Cloudflare.TunnelToken),
		"cloudflare_email":        cfg.Cloudflare.Email,
	}

	if cfg.Cloudflare.AccountID != "" {
		vars["cloudflare_account_id"] = cfg.Cloudflare.AccountID
	}
	if cfg.Cloudflare.TunnelID != "" {
		vars["cloudflare_tunnel_id"] = cfg.Cloudflare.TunnelID
	}

	// Primary domain (first in list) for single-domain Ansible roles
	if len(cfg.Cloudflare.Domains) > 0 {
		primary := cfg.Cloudflare.Domains[0]
		vars["cloudflare_domain"] = primary.Domain
		if primary.ZoneID != "" {
			vars["cloudflare_zone_id"] = primary.ZoneID
		}
	}

	// Full domains list as JSON for multi-domain capable roles
	if domainsJSON, err := json.Marshal(cfg.Cloudflare.Domains); err == nil {
		vars["cloudflare_domains"] = string(domainsJSON)
	}

	return vars
}

// getInfrastructureExtraVars detects infrastructure-level constants like the MQTT broker (infra-manager) IP.
func getInfrastructureExtraVars(cfg config.ClusterConfig) map[string]string {
	vars := make(map[string]string)

	for _, n := range cfg.Nodes {
		if n.Role == "infra-manager" {
			vars["mqtt_broker_host"] = n.IP
			vars["mqtt_broker_port"] = "1883" // Default Mosquitto port
			break
		}
	}

	return vars
}

func getMonitoringExtraVars(cfg config.ClusterConfig) map[string]string {
	vars := make(map[string]string)
	if cfg.Monitoring.GrafanaAdminPassword != "" {
		vars["grafana_admin_password"] = string(cfg.Monitoring.GrafanaAdminPassword)
	}
	return vars
}

// getBackupExtraVars builds the extra-vars required for Velero backups.
func getBackupExtraVars(cfg config.ClusterConfig) map[string]string {
	return map[string]string{
		"velero_s3_url":        cfg.Backup.S3Url,
		"velero_s3_bucket":     cfg.Backup.S3Bucket,
		"velero_s3_prefix":     cfg.Backup.S3Prefix,
		"velero_s3_region":     cfg.Backup.S3Region,
		"velero_s3_access_key": string(cfg.Backup.S3AccessKey),
		"velero_s3_secret_key": string(cfg.Backup.S3SecretKey),
		"velero_s3_readonly":   fmt.Sprintf("%t", cfg.Backup.ReadOnly),
	}
}

// RunBootstrap executes the bootstrap.yml playbook to perform initial OS/Network setup.
func RunBootstrap(nodeName string, dhcpIP string, staticIP string, user string, password string, dryRun bool, tags []string, output io.Writer) (*Result, error) {
	extraVars := map[string]string{
		"ansible_user":            user,
		"ansible_password":        password,
		"ansible_become_password": password, // Crucial for fresh Debian installs
		"ansible_host":            dhcpIP,
		"static_ip":               staticIP,
		"kgg_user":                "kgg-admin",
	}

	return runPlaybook("bootstrap.yml", nodeName, dryRun, tags, extraVars, output, true)
}

// RunMigrateUser executes the migrate-rk-to-kgg.yml playbook to transition nodes from rk-admin to kgg-admin.
func RunMigrateUser(limit string, oldUser, newUser, sshKey string, dryRun bool, output io.Writer) (*Result, error) {
	extraVars := make(map[string]string)
	if oldUser != "" {
		extraVars["old_user"] = oldUser
		extraVars["ansible_user"] = oldUser
	}
	if newUser != "" {
		extraVars["new_user"] = newUser
	}

	// Resolve the cluster public key path to copy to the new user
	if sshKey != "" {
		// Clean up the .pub extension if the user accidentally passed the public key file instead of the private key
		if before, ok := strings.CutSuffix(sshKey, ".pub"); ok {
			sshKey = before
		}

		localKeyPath := sshKey

		if runtime.GOOS == "windows" {
			if wslKeyPath, syncErr := deps.SyncSSHKeyToWSL(sshKey); syncErr == nil {
				sshKey = wslKeyPath
			} else if wslPath, convErr := deps.ConvertToWSLPath(sshKey); convErr == nil {
				sshKey = wslPath
			}
		}

		extraVars["ansible_ssh_private_key_file"] = sshKey
		if _, err := os.Stat(localKeyPath + ".pub"); err == nil {
			extraVars["kgg_cluster_pubkey"] = sshKey + ".pub"
		}
	} else {
		keyPath, err := config.ResolveKeyPath("")
		if err == nil && keyPath != "" {
			if _, err := os.Stat(keyPath + ".pub"); err == nil {
				extraVars["kgg_cluster_pubkey"] = keyPath + ".pub"
			}
		}
	}

	return runPlaybook("migrate-rk-to-kgg.yml", limit, dryRun, nil, extraVars, output, true)
}
