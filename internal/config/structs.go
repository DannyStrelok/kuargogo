package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Secret is a string type that automatically encrypts/decrypts when marshaled to/from YAML.
type Secret string

// Global state to track if we failed to decrypt any secrets during the last load.
var FailedDecryptionCount int

func (s Secret) MarshalYAML() (any, error) {
	plain := string(s)
	if plain == "" {
		return "", nil
	}

	// If it's already encrypted (shouldn't happen in memory but just in case), return as is
	if strings.HasPrefix(plain, "!vaultENC:") {
		return plain, nil
	}

	// Try to encrypt using Master Key
	passphrase, err := GetMasterKey()
	if err != nil || passphrase == "" {
		// If no passphrase is found, we keep it as plain text (or user might want to block this)
		// Based on user feedback, we encrypt "automatically if passphrase exists".
		return plain, nil
	}

	// Get Salt
	saltStr := RootConfigGetSync().Salt
	if saltStr == "" {
		// No salt yet, can't encrypt safely
		return plain, nil
	}
	salt, _ := base64.StdEncoding.DecodeString(saltStr)

	encrypted, err := Encrypt([]byte(plain), passphrase, salt)
	if err != nil {
		return nil, err
	}

	return "!vaultENC:" + base64.StdEncoding.EncodeToString(encrypted), nil
}

func (s *Secret) UnmarshalYAML(unmarshal func(any) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return err
	}

	decrypted, success := decryptSecretValue(value)
	if !success && strings.HasPrefix(value, "!vaultENC:") {
		FailedDecryptionCount++
	}
	*s = Secret(decrypted)
	return nil
}

// decryptSecretValue handles the logic of checking for !vaultENC: prefix and decrypting.
// Returns (plaintext, success). If it was NOT encrypted, returns (value, true).
// If it WAS encrypted but failed to decrypt, returns (value, false).
func decryptSecretValue(value string) (string, bool) {
	if !strings.HasPrefix(value, "!vaultENC:") {
		return value, true
	}

	// It's encrypted, try to decrypt
	encoded := strings.TrimPrefix(value, "!vaultENC:")
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return value, false
	}

	passphrase, err := GetMasterKey()
	if err != nil || passphrase == "" {
		return value, false
	}

	// Extract Salt directly from Viper to avoid locking the configMutex during LoadConfig.
	// This prevents a deadlock when loadConfigInternal holds the lock and mapstructure decoding runs.
	saltStr := viper.GetString("sync.salt")
	if saltStr == "" {
		// Fallback: If Viper hasn't registered 'sync.salt', the config is useless for decryption anyway
		return value, false
	}
	salt, _ := base64.StdEncoding.DecodeString(saltStr)

	plaintext, err := Decrypt(ciphertext, passphrase, salt)
	if err != nil {
		return value, false
	}

	return string(plaintext), true
}

// Config holds the global configuration for kuargogo

type DiscoveryConfig struct {
	Enabled   bool   `mapstructure:"enabled" yaml:"enabled"`
	Interface string `mapstructure:"interface" yaml:"interface"`
}

// RootConfig holds the set of all contexts and the current selection
type RootConfig struct {
	Version        string                   `mapstructure:"version" yaml:"version"`
	CurrentContext string                   `mapstructure:"current_context" yaml:"current_context"`
	Contexts       map[string]ClusterConfig `mapstructure:"contexts" yaml:"contexts"`
	Sync           SyncSettings             `mapstructure:"sync" yaml:"sync,omitempty"`
	Lang           string                   `mapstructure:"lang" yaml:"lang,omitempty"`
}

type SyncSettings struct {
	Provider string `mapstructure:"provider" yaml:"provider"` // r2, s3, etc.
	LastSync string `mapstructure:"last_sync" yaml:"last_sync,omitempty"`
	Salt     string `mapstructure:"salt" yaml:"salt,omitempty"` // Salt for Master Key derivation
	S3       Backup `mapstructure:"s3" yaml:"s3,omitempty"`     // S3-compatible cloud settings
}

// ClusterConfig holds the configuration for a single environment/cluster
// This was formerly 'Config'
type ClusterConfig struct {
	Nodes           []Node          `mapstructure:"nodes" yaml:"nodes"`
	MaintenanceMode bool            `mapstructure:"maintenance_mode" yaml:"maintenance_mode"` // Global suppression
	Network         Network         `mapstructure:"network" yaml:"network"`
	NetworkLayout   NetworkLayout   `mapstructure:"netwokgg_layout" yaml:"netwokgg_layout"`
	SSH             SSH             `mapstructure:"ssh" yaml:"ssh"`
	MQTT            MQTT            `mapstructure:"mqtt" yaml:"mqtt"`
	K3s             K3s             `mapstructure:"k3s" yaml:"k3s"`
	Ansible         Ansible         `mapstructure:"ansible" yaml:"ansible,omitempty"`
	Telegram        Telegram        `mapstructure:"telegram" yaml:"telegram"`
	Discovery       DiscoveryConfig `mapstructure:"discovery" yaml:"discovery"`
	HardwareEnabled bool            `mapstructure:"hardware_enabled" yaml:"hardware_enabled"`
	Backup          Backup          `mapstructure:"backup" yaml:"backup,omitempty"`
	Cloudflare      Cloudflare      `mapstructure:"cloudflare" yaml:"cloudflare,omitempty"`
	GitOps          GitOps          `mapstructure:"gitops" yaml:"gitops,omitempty"`
	AI              AIConfig        `mapstructure:"ai" yaml:"ai,omitempty"`
	NFS             NFS             `mapstructure:"nfs" yaml:"nfs,omitempty"`
	Monitoring      Monitoring      `mapstructure:"monitoring" yaml:"monitoring,omitempty"`
	LastCloudSync   string          `mapstructure:"last_cloud_sync" yaml:"last_cloud_sync,omitempty"`
}

// AIConfig holds settings for local or cloud AI providers
type AIConfig struct {
	Provider      string `mapstructure:"provider" yaml:"provider"`             // ollama, openai-compatible, anthropic, openai, gemini
	Model         string `mapstructure:"model" yaml:"model"`                   // e.g., llama3, claude-3-5-sonnet-20240620
	APIKey        Secret `mapstructure:"api_key" yaml:"api_key,omitempty"`     // Optional for local
	Endpoint      string `mapstructure:"endpoint" yaml:"endpoint,omitempty"`   // For local (Ollama/LocalAI) or custom proxy
	AnonymizeLogs bool   `mapstructure:"anonymize_logs" yaml:"anonymize_logs"` // Default: true
}

// GitOps holds ArgoCD and Kargo declarative configuration
type GitOps struct {
	Projects    []GitOpsProject    `mapstructure:"projects" yaml:"projects,omitempty" json:"projects,omitempty"`
	Credentials []GitOpsCredential `mapstructure:"credentials" yaml:"credentials,omitempty" json:"credentials,omitempty"`
	KargoEngine *KargoEngine       `mapstructure:"kargo_engine,omitempty" yaml:"kargo_engine,omitempty" json:"kargo_engine,omitempty"`
	Pipelines   []KargoPipeline    `mapstructure:"pipelines" yaml:"pipelines,omitempty" json:"pipelines,omitempty"`
}

// KargoEngine holds the global configuration for the Kargo installation
type KargoEngine struct {
	AdminPassword     Secret `mapstructure:"admin_password,omitempty" yaml:"admin_password,omitempty" json:"admin_password,omitempty"`
	AdminPasswordHash string `mapstructure:"admin_password_hash,omitempty" yaml:"admin_password_hash,omitempty" json:"admin_password_hash,omitempty"`
	TokenSigningKey   Secret `mapstructure:"token_signing_key,omitempty" yaml:"token_signing_key,omitempty" json:"token_signing_key,omitempty"`
}

// KargoPipeline holds the configuration for artifact promotion
type KargoPipeline struct {
	Name      string         `mapstructure:"name,omitempty" yaml:"name,omitempty" json:"name,omitempty"`
	Namespace string         `mapstructure:"namespace" yaml:"namespace" json:"namespace"`
	Project   string         `mapstructure:"project" yaml:"project" json:"project"`
	Warehouse KargoWarehouse `mapstructure:"warehouse" yaml:"warehouse" json:"warehouse"`
	Stages    []KargoStage   `mapstructure:"stages" yaml:"stages" json:"stages"`
}

type KargoWarehouse struct {
	Name                   string   `mapstructure:"name" yaml:"name" json:"name"`
	Repo                   string   `mapstructure:"repo" yaml:"repo" json:"repo"`
	AdditionalImages       []string `mapstructure:"additional_images,omitempty" yaml:"additional_images,omitempty" json:"additional_images,omitempty"`
	Path                   string   `mapstructure:"path" yaml:"path" json:"path"` // OJO: Usado como URL Git
	Semver                 string   `mapstructure:"semver,omitempty" yaml:"semver,omitempty" json:"semver,omitempty"`
	ImageSelectionStrategy string   `mapstructure:"image_selection_strategy,omitempty" yaml:"image_selection_strategy,omitempty" json:"image_selection_strategy,omitempty"`
	AllowTags              string   `mapstructure:"allow_tags,omitempty" yaml:"allow_tags,omitempty" json:"allow_tags,omitempty"`
}

type KargoStage struct {
	Name     string   `mapstructure:"name" yaml:"name" json:"name"`
	Path     string   `mapstructure:"path,omitempty" yaml:"path,omitempty" json:"path,omitempty"`
	Requires []string `mapstructure:"requires,omitempty" yaml:"requires,omitempty" json:"requires,omitempty"`
}

type GitOpsCredential struct {
	URL      string `mapstructure:"url"      yaml:"url"      json:"url"`
	Username string `mapstructure:"username" yaml:"username" json:"username"`
	Password Secret `mapstructure:"password" yaml:"password" json:"password"`
	// Registry is the container registry server (e.g. ghcr.io, docker.io).
	// When set, kuargogo will create an imagePullSecret named "<registry-slug>-pull-secret"
	// in every namespace defined across all GitOps apps.
	Registry string `mapstructure:"registry" yaml:"registry,omitempty" json:"registry,omitempty"`
	// Email is used in the dockerconfigjson auth blob.
	// Defaults to "<username>@<registry>" if omitted.
	Email string `mapstructure:"email" yaml:"email,omitempty" json:"email,omitempty"`
}

type GitOpsProject struct {
	Name        string      `mapstructure:"name" yaml:"name" json:"name"`
	Description string      `mapstructure:"description" yaml:"description,omitempty" json:"description,omitempty"`
	ManagedEnv  bool        `mapstructure:"managed_env" yaml:"managed_env,omitempty" json:"managed_env,omitempty"`
	Repo        string      `mapstructure:"repo" yaml:"repo,omitempty" json:"repo,omitempty"`
	Apps        []GitOpsApp `mapstructure:"apps" yaml:"apps,omitempty" json:"apps,omitempty"`
}

type GitOpsApp struct {
	Name      string `mapstructure:"name" yaml:"name" json:"name"`
	Repo      string `mapstructure:"repo" yaml:"repo" json:"repo"`
	Path      string `mapstructure:"path" yaml:"path,omitempty" json:"path,omitempty"`
	Namespace string `mapstructure:"namespace" yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Branch    string `mapstructure:"branch" yaml:"branch,omitempty" json:"branch,omitempty"`
	// Helm-specific fields (optional). When Chart is set, the app is deployed as a Helm chart.
	Chart        string `mapstructure:"chart" yaml:"chart,omitempty" json:"chart,omitempty"`
	ChartVersion string `mapstructure:"chart_version" yaml:"chart_version,omitempty" json:"chart_version,omitempty"`
	ValuesFile   string `mapstructure:"values_file" yaml:"values_file,omitempty" json:"values_file,omitempty"`
	ValuesRepo   string `mapstructure:"values_repo" yaml:"values_repo,omitempty" json:"values_repo,omitempty"`
	ValuesBranch string `mapstructure:"values_branch" yaml:"values_branch,omitempty" json:"values_branch,omitempty"`
}

// IsHelm returns true if this app should be deployed as a Helm chart.
func (a GitOpsApp) IsHelm() bool {
	return a.Chart != ""
}

// Cloudflare holds account-level credentials and a list of managed domains.
// All domains share the same Cloudflare account and tunnel.
type Cloudflare struct {
	// Account-level credentials (shared across all domains in this context)
	Email    string `mapstructure:"email"     yaml:"email"`
	APIToken Secret `mapstructure:"api_token" yaml:"api_token"`
	// Cloudflare account identifier (resolved automatically on provisioning)
	AccountID string `mapstructure:"account_id" yaml:"account_id,omitempty"`
	// Tunnel credentials (one tunnel per cluster context)
	TunnelToken Secret `mapstructure:"tunnel_token" yaml:"tunnel_token"`
	TunnelID    string `mapstructure:"tunnel_id"   yaml:"tunnel_id,omitempty"`
	// Default list of emails authorized to access protected services via Zero Trust
	AccessEmails []string `mapstructure:"access_emails" yaml:"access_emails,omitempty"`
	// Domains managed under this tunnel
	Domains []CloudflareDomain `mapstructure:"domains" yaml:"domains,omitempty"`
}

// CloudflareDomain groups services that share the same DNS zone.
type CloudflareDomain struct {
	Domain        string              `mapstructure:"domain"         yaml:"domain"`
	ZoneID        string              `mapstructure:"zone_id"        yaml:"zone_id,omitempty"`
	AccessEnabled bool                `mapstructure:"access_enabled" yaml:"access_enabled,omitempty"`
	Services      []CloudflareService `mapstructure:"services"      yaml:"services,omitempty"`
}

// CloudflareService represents a single service exposed via a Cloudflare Tunnel.
type CloudflareService struct {
	Name      string `mapstructure:"name"      yaml:"name"`
	Subdomain string `mapstructure:"subdomain" yaml:"subdomain"`
	Target    string `mapstructure:"target"    yaml:"target"`
	Protected bool   `mapstructure:"protected" yaml:"protected"`
}

// Monitoring holds credentials and settings for observability
type Monitoring struct {
	GrafanaAdminPassword Secret `mapstructure:"grafana_admin_password" yaml:"grafana_admin_password,omitempty"`
}

// Backup holds Disaster Recovery S3 credentials (Velero)
type Backup struct {
	S3Url       string `mapstructure:"s3_url" yaml:"s3_url"`
	S3Bucket    string `mapstructure:"s3_bucket" yaml:"s3_bucket"`
	S3Prefix    string `mapstructure:"s3_prefix" yaml:"s3_prefix,omitempty"`
	S3Region    string `mapstructure:"s3_region" yaml:"s3_region"`
	S3AccessKey Secret `mapstructure:"s3_access_key" yaml:"s3_access_key"`
	S3SecretKey Secret `mapstructure:"s3_secret_key" yaml:"s3_secret_key"`
}

// Network holds switch credentials and connection details
type Network struct {
	SwitchIP string `mapstructure:"switch_ip" yaml:"switch_ip"`
	User     string `mapstructure:"user" yaml:"user"`
	Password Secret `mapstructure:"pass" yaml:"pass"`
	APIPort  int    `mapstructure:"api_port" yaml:"api_port"` // For RouterOS or future API usage
	Driver   string `mapstructure:"driver" yaml:"driver"`     // tplink, mikrotik, simulated
}

// NetworkLayout defines the desired state of the switch (Declared Infrastructure)
type NetworkLayout struct {
	// VLANs map a VLAN Name (e.g. "vlan_10_safe") to a list of Ports (e.g. ["port_1", "port_3"])
	VLANs        map[string][]string `mapstructure:"vlans" yaml:"vlans"`
	IGMPSnooping bool                `mapstructure:"igmp_snooping" yaml:"igmp_snooping"`
}

// Node represents a machine in the homelab
type Node struct {
	Name        string            `mapstructure:"name" yaml:"name"`
	IP          string            `mapstructure:"ip" yaml:"ip"`
	User        string            `mapstructure:"user" yaml:"user"`
	Role        string            `mapstructure:"role" yaml:"role"`         // control-plane, master, worker
	Arch        string            `mapstructure:"arch" yaml:"arch"`         // arm64, amd64
	Position    string            `mapstructure:"position" yaml:"position"` // left, center, right
	MAC         string            `mapstructure:"mac" yaml:"mac"`           // for WoL
	Maintenance bool              `mapstructure:"maintenance" yaml:"maintenance"`
	Labels      map[string]string `mapstructure:"labels" yaml:"labels,omitempty"`
}

// FindGPUNodes returns all nodes that have GPU capability.
// A node is considered GPU-capable if it has a label "gpu" set to "nvidia".
func FindGPUNodes() []Node {
	cfg := GetConfig()
	var gpuNodes []Node
	for _, n := range cfg.Nodes {
		if n.Labels["gpu"] == "nvidia" {
			gpuNodes = append(gpuNodes, n)
		}
	}
	return gpuNodes
}

// SSH holds SSH connection details
type SSH struct {
	PrivateKeyPath string `mapstructure:"private_key_path" yaml:"private_key_path"`
	Port           int    `mapstructure:"port" yaml:"port"`
}

// ExpandedKeyPath returns the SSH private key path with ~ expanded to home directory
func (s SSH) ExpandedKeyPath() (string, error) {
	if envPath := os.Getenv("KGG_CLUSTER_KEY_PATH"); envPath != "" {
		return envPath, nil
	}

	keyPath := s.PrivateKeyPath
	if strings.HasPrefix(keyPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		keyPath = filepath.Join(home, keyPath[2:])
	}
	return keyPath, nil
}

// MQTT holds access details for the broker
type MQTT struct {
	Broker      string `mapstructure:"broker" yaml:"broker"`
	ClientID    string `mapstructure:"client_id" yaml:"client_id"`
	Username    string `mapstructure:"username" yaml:"username,omitempty"`
	Password    Secret `mapstructure:"password" yaml:"password,omitempty"`
	TopicPrefix string `mapstructure:"topic_prefix" yaml:"topic_prefix"`
}

// ExtraArgs represents custom CLI flags/arguments passed to K3s.
// It supports both legacy string slice format and modern dictionary format.
type ExtraArgs map[string]any

// UnmarshalYAML implements custom unmarshaling to support both formats.
func (ea *ExtraArgs) UnmarshalYAML(value *yaml.Node) error {
	// Try to decode as map
	var m map[string]any
	if err := value.Decode(&m); err == nil {
		*ea = ExtraArgs(m)
		return nil
	}

	// Try to decode as slice
	var slice []string
	if err := value.Decode(&slice); err != nil {
		return err
	}

	res := make(map[string]any)
	for _, arg := range slice {
		cleanArg := strings.TrimPrefix(arg, "--")
		cleanArg = strings.TrimPrefix(cleanArg, "-")

		parts := strings.SplitN(cleanArg, "=", 2)
		key := strings.TrimSpace(parts[0])
		var val any = true
		if len(parts) > 1 {
			val = strings.TrimSpace(parts[1])
		}

		if existing, ok := res[key]; ok {
			if existSlice, ok := existing.([]any); ok {
				res[key] = append(existSlice, val)
			} else {
				res[key] = []any{existing, val}
			}
		} else {
			res[key] = val
		}
	}

	*ea = ExtraArgs(res)
	return nil
}

// K3s holds cluster configuration
type K3s struct {
	Token          Secret    `mapstructure:"token" yaml:"token"`
	KubeconfigPath string    `mapstructure:"kubeconfig_path" yaml:"kubeconfig_path"`
	VIP            string    `mapstructure:"vip" yaml:"vip"`
	VIPInterface   string    `mapstructure:"vip_interface" yaml:"vip_interface"`
	HA             bool      `mapstructure:"ha" yaml:"ha"`
	Version        string    `mapstructure:"version" yaml:"version"`
	ServerArgs     ExtraArgs `mapstructure:"server_args" yaml:"server_args,omitempty"`
	AgentArgs      ExtraArgs `mapstructure:"agent_args" yaml:"agent_args,omitempty"`
}

// ExpandedKubeconfigPath returns the kubeconfig path with ~ expanded to home directory
func (k K3s) ExpandedKubeconfigPath() (string, error) {
	path := k.KubeconfigPath
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return path, nil
}

// Ansible holds Ansible-specific configuration
type Ansible struct {
	VaultPasswordFile string `mapstructure:"vault_password_file" yaml:"vault_password_file,omitempty"`
	WSLDistro         string `mapstructure:"wsl_distro" yaml:"wsl_distro,omitempty"`
}

type Telegram struct {
	BotToken         Secret `mapstructure:"bot_token" yaml:"bot_token"`
	AdminID          int    `mapstructure:"admin_id" yaml:"admin_id"`
	Timezone         string `mapstructure:"timezone" yaml:"timezone"`
	DailySummaryTime string `mapstructure:"daily_summary_time" yaml:"daily_summary_time"`
}

// NFS holds network storage configuration
type NFS struct {
	Enabled bool       `mapstructure:"enabled" yaml:"enabled"`
	Server  string     `mapstructure:"server" yaml:"server"`
	Shares  []NFSShare `mapstructure:"shares" yaml:"shares,omitempty"`
}

type NFSShare struct {
	Src  string `mapstructure:"src" yaml:"src"`
	Dest string `mapstructure:"dest" yaml:"dest"`
	Opts string `mapstructure:"opts" yaml:"opts"`
}

// DeepCopy creates a safe copy of RootConfig including the Contexts map
func (r RootConfig) DeepCopy() RootConfig {
	out := r
	if r.Contexts != nil {
		out.Contexts = make(map[string]ClusterConfig, len(r.Contexts))
		for k, v := range r.Contexts {
			out.Contexts[k] = v.DeepCopy()
		}
	}
	out.Sync = r.Sync.DeepCopy()
	return out
}

func (s SyncSettings) DeepCopy() SyncSettings {
	return s
}

// DeepCopy creates a safe copy of ClusterConfig to prevent race conditions
func (c ClusterConfig) DeepCopy() ClusterConfig {
	// Start with shallow copy of value types
	out := c

	// Deep copy Nodes slice
	if c.Nodes != nil {
		out.Nodes = make([]Node, len(c.Nodes))
		for i, n := range c.Nodes {
			out.Nodes[i] = n.DeepCopy()
		}
	}

	// Deep copy NetworkLayout (Maps)
	out.NetworkLayout = c.NetworkLayout.DeepCopy()
	// Deep copy GitOps
	out.GitOps = c.GitOps.DeepCopy()
	// Deep copy NFS
	out.NFS = c.NFS.DeepCopy()

	return out
}

// DeepCopy for GitOps
func (g GitOps) DeepCopy() GitOps {
	out := g
	if g.Projects != nil {
		out.Projects = make([]GitOpsProject, len(g.Projects))
		for i, p := range g.Projects {
			out.Projects[i] = p.DeepCopy()
		}
	}
	if g.Credentials != nil {
		out.Credentials = make([]GitOpsCredential, len(g.Credentials))
		copy(out.Credentials, g.Credentials)
	}
	if g.KargoEngine != nil {
		out.KargoEngine = g.KargoEngine.DeepCopy()
	}
	if g.Pipelines != nil {
		out.Pipelines = make([]KargoPipeline, len(g.Pipelines))
		for i, p := range g.Pipelines {
			out.Pipelines[i] = *p.DeepCopy()
		}
	}
	return out
}

func (k *KargoEngine) DeepCopy() *KargoEngine {
	if k == nil {
		return nil
	}
	out := *k
	return &out
}

func (k *KargoPipeline) DeepCopy() *KargoPipeline {
	if k == nil {
		return nil
	}
	out := *k
	if k.Stages != nil {
		out.Stages = make([]KargoStage, len(k.Stages))
		copy(out.Stages, k.Stages)
	}
	return &out
}

func (n NFS) DeepCopy() NFS {
	out := n
	if n.Shares != nil {
		out.Shares = make([]NFSShare, len(n.Shares))
		copy(out.Shares, n.Shares)
	}
	return out
}

func (p GitOpsProject) DeepCopy() GitOpsProject {
	out := p
	if p.Apps != nil {
		out.Apps = make([]GitOpsApp, len(p.Apps))
		copy(out.Apps, p.Apps)
	}
	return out
}

// GetInfraManager returns the node acting as the infrastructure manager (role: infra).
// Returns nil if no such node is found.
func (c ClusterConfig) GetInfraManager() *Node {
	for _, n := range c.Nodes {
		if n.Role == "infra-manager" {
			return &n
		}
	}
	return nil
}

// DeepCopy for Node (handling Maps)
func (n Node) DeepCopy() Node {
	out := n
	if n.Labels != nil {
		out.Labels = make(map[string]string, len(n.Labels))
		for k, v := range n.Labels {
			out.Labels[k] = v
		}
	}
	return out
}

// DeepCopy for NetworkLayout
func (nl NetworkLayout) DeepCopy() NetworkLayout {
	out := nl
	if nl.VLANs != nil {
		out.VLANs = make(map[string][]string, len(nl.VLANs))
		for k, v := range nl.VLANs {
			// Copy slice value
			newSlice := make([]string, len(v))
			copy(newSlice, v)
			out.VLANs[k] = newSlice
		}
	}
	return out
}
