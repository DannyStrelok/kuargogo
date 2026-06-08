package config

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/i18n"
	"github.com/fsnotify/fsnotify"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// appConfig holds the full configuration including all contexts
var appConfig RootConfig

// globalConfig stores the ACTIVE configuration context (proxy)
var globalConfig ClusterConfig

// configMutex protects access to appConfig and globalConfig
var configMutex sync.RWMutex

// dryRunFlag stores runtime --dry-run state, accessible from internal packages
var dryRunFlag bool

// WatchEnabled controls whether Viper starts a background file watcher.
// Should only be enabled for long-running processes (TUI).
var WatchEnabled bool

var (
	lastInternalWrite      time.Time
	lastInternalWriteMutex sync.Mutex
	writeMutex             sync.Mutex
)

// OnConfigUpdated is a hook that is called whenever the configuration is modified and saved to disk.
// This allows orchestration layers (like cmd/kgg) to trigger side effects (like sync to infra-manager)
// without creating circular dependencies in internal packages.
var OnConfigUpdated func(path string)

// SetDryRun stores the --dry-run flag for access by internal packages (TUI actions).
func SetDryRun(v bool) {
	dryRunFlag = v
}

// IsDryRun returns the current --dry-run flag state.
func IsDryRun() bool {
	return dryRunFlag
}

// ResolveKeyPath returns the effective SSH private key path.
// It prioritizes the provided override, then the config, then a default (~/.ssh/kgg_cluster_id).
func ResolveKeyPath(override string) (string, error) {
	if override != "" {
		return expandPath(override)
	}

	if envPath := os.Getenv("KGG_CLUSTER_KEY_PATH"); envPath != "" {
		return envPath, nil
	}

	cfg := GetConfig()
	if cfg.SSH.PrivateKeyPath != "" {
		return cfg.SSH.ExpandedKeyPath()
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".ssh", "kgg_cluster_id"), nil
}

func GetConfigDir() string {
	home := getHomeDir()
	if home == "" {
		return ".kuargogo" // Fallback to local hidden dir
	}
	return filepath.Join(home, ".kuargogo")
}

func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return home
	}

	// Fallback for Linux/macOS
	if h := os.Getenv("HOME"); h != "" {
		return h
	}

	// Fallback for Windows
	if h := os.Getenv("USERPROFILE"); h != "" {
		return h
	}

	return ""
}

// expandPath is a helper for ResolveKeyPath to handle ~/ and ~\ prefix
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// GetConfig returns a thread-safe copy of the active configuration
// The returned struct is a deep copy, safe to use without locks.
func GetConfig() ClusterConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalConfig.DeepCopy()
}

// GetActiveContext returns the active configuration (alias for GetConfig)
func GetActiveContext() ClusterConfig {
	return GetConfig()
}

// GetContext returns a specific configuration context
func GetContext(name string) (ClusterConfig, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	cfg, ok := appConfig.Contexts[name]
	if !ok {
		return ClusterConfig{}, fmt.Errorf("context '%s' not found", name)
	}
	return cfg.DeepCopy(), nil
}

// ListContexts returns a list of available context names
func ListContexts() []string {
	configMutex.RLock()
	defer configMutex.RUnlock()

	var names []string
	for k := range appConfig.Contexts {
		names = append(names, k)
	}
	return names
}

// GetCurrentContext returns the name of the currently active context
func GetCurrentContext() string {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return appConfig.CurrentContext
}

// GetAppConfig returns a thread-safe deep copy of the root configuration
func GetAppConfig() RootConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return appConfig.DeepCopy()
}

// ModifyAppConfig allows safe modification of the root configuration.
func ModifyAppConfig(fn func(*RootConfig)) error {
	configMutex.Lock()
	defer configMutex.Unlock()
	fn(&appConfig)
	return nil
}

// SwitchContext updates the active context thread-safely
func SwitchContext(contextName string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	targetCfg, ok := appConfig.Contexts[contextName]
	if !ok {
		return fmt.Errorf("context '%s' not found", contextName)
	}

	appConfig.CurrentContext = contextName
	globalConfig = targetCfg
	return nil
}

// AddContext adds or updates a context and sets it as active thread-safely
func AddContext(name string, cfg ClusterConfig) {
	configMutex.Lock()
	defer configMutex.Unlock()

	// Ensure map exists
	if appConfig.Contexts == nil {
		appConfig.Contexts = make(map[string]ClusterConfig)
	}

	// Update Context
	appConfig.Contexts[name] = cfg
	appConfig.CurrentContext = name

	// Update active global config
	globalConfig = cfg

	// Ensure version is set if fresh
	if appConfig.Version == "" {
		appConfig.Version = "v1"
	}
}

// DeleteContext removes a context safely
func DeleteContext(name string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if _, ok := appConfig.Contexts[name]; !ok {
		return fmt.Errorf("context '%s' not found", name)
	}

	if name == appConfig.CurrentContext {
		return fmt.Errorf("cannot delete active context '%s'", name)
	}

	delete(appConfig.Contexts, name)
	return nil
}

// LoadConfig initializes Viper and reads the configuration file
func LoadConfig(cfgFile string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	// Reset global state to ensure clean load (crucial for tests and reloads)
	appConfig = RootConfig{}
	viper.Reset()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		if dir := GetConfigDir(); dir != "" {
			viper.AddConfigPath(dir)
		}
		viper.AddConfigPath("/etc")
		viper.SetConfigType("yaml")
		viper.SetConfigName("kuargogo")
	}

	viper.SetEnvPrefix("KGG")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		// If config file not found, we might be in init mode, so just return
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Initial load
	if err := loadConfigInternal(); err != nil {
		return err
	}

	if WatchEnabled {
		viper.OnConfigChange(func(e fsnotify.Event) {
			// Only handle Write events to avoid loops or noise
			if e.Op&fsnotify.Write != fsnotify.Write {
				return
			}

			// Time-based debouncer: ignore events within 1 second of an internal write
			lastInternalWriteMutex.Lock()
			isInternal := time.Since(lastInternalWrite) < 1*time.Second
			lastInternalWriteMutex.Unlock()
			if isInternal {
				return
			}

			log.Printf("🔄 Config file change detected: %s", e.Name)

			// Give the OS a tiny bit of time to release the file lock (common in Windows editors)
			time.Sleep(100 * time.Millisecond)

			configMutex.Lock()
			defer configMutex.Unlock()

			// 1. MUST re-read the file into Viper's internal map
			if err := viper.ReadInConfig(); err != nil {
				log.Printf("❌ Error re-reading config: %v", err)
				return
			}

			// 2. Reparse Viper into our structs
			if err := loadConfigInternal(); err != nil {
				log.Printf("❌ Error parsing reloaded config: %v", err)
			} else {
				log.Println("✅ Configuration reloaded successfully")
				if OnConfigUpdated != nil {
					// Execute hook in a goroutine to avoid blocking the watcher
					go OnConfigUpdated(e.Name)
				}
			}
		})
		viper.WatchConfig()
	}

	return nil
}

// loadConfigInternal parses viper into appConfig and sets globalConfig.
// It assumes the caller holds the lock.
func loadConfigInternal() error {
	// Reset the failed decryption counter before each load
	FailedDecryptionCount = 0

	// 1. Attempt to parse as RootConfig (New Structure)
	// We use CaseInsensitive struct tag matching and our custom Secret decode hook
	if err := viper.Unmarshal(&appConfig, func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "mapstructure"
		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeHookFunc(time.RFC3339),
			mapstructure.StringToSliceHookFunc(","),
			secretDecodeHook(),
			extraArgsDecodeHook(),
		)
	}); err != nil {
		return fmt.Errorf("failed to unmarshal root config: %w", err)
	}

	// 2. Migration Logic: If no contexts found but we have nodes, implies Legacy Config
	if len(appConfig.Contexts) == 0 {
		var legacyConf ClusterConfig
		// Try to unmarshal into ClusterConfig to see if it matches legacy structure
		if err := viper.Unmarshal(&legacyConf, func(dc *mapstructure.DecoderConfig) {
			dc.TagName = "mapstructure"
			dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeHookFunc(time.RFC3339),
				mapstructure.StringToSliceHookFunc(","),
				secretDecodeHook(),
				extraArgsDecodeHook(),
			)
		}); err == nil && len(legacyConf.Nodes) > 0 {
			// Found legacy data. Migrate it.
			appConfig.Contexts = make(map[string]ClusterConfig)
			appConfig.Contexts["default"] = legacyConf
			appConfig.CurrentContext = "default"
			// Set globalConfig so SaveConfig doesn't overwrite with empty
			globalConfig = legacyConf
			// Save immediately to persist migration
			// NOTE: calling SaveConfig here might be recursive or disallowed if we hold lock?
			// SaveConfig also acquires lock. We should split SaveConfig or be careful.
			// Ideally we don't save during a hot-reload or read.
			// But this is migration. Migration happens only on first read usually.
			// Let's defer saving or just do it. But we hold the lock!
			// We need a version of SaveConfig that doesn't lock or release lock briefly.
			// Since we claim GlobalConfig is set, we can just let it be in memory for now?
			// Or better: don't call SaveConfig inside loadConfigInternal.
		}
	}

	// 3. Set Active Context
	if appConfig.CurrentContext == "" {
		appConfig.CurrentContext = "default"
	}

	// Create map if nil
	if appConfig.Contexts == nil {
		appConfig.Contexts = make(map[string]ClusterConfig)
	}

	// Default Version (Migration)
	if appConfig.Version == "" {
		appConfig.Version = "v1" // Assume v1 if missing during load
	}

	// Load active context into globalConfig
	var ok bool
	globalConfig, ok = appConfig.Contexts[appConfig.CurrentContext]
	if !ok {
		// if context not found, init empty (or warn)
		globalConfig = ClusterConfig{}
	}

	// Apply defaults before validation
	if globalConfig.Network.SwitchIP != "" && globalConfig.Network.Driver == "" {
		globalConfig.Network.Driver = "tplink"
	}

	// 4. Validate Configuration
	if len(globalConfig.Nodes) > 0 { // Only validate if we actually have data
		if err := globalConfig.Validate(); err != nil {
			return fmt.Errorf("invalid configuration in context '%s': %w", appConfig.CurrentContext, err)
		}
	}

	// 5. Initialize i18n
	i18n.SetLang(appConfig.Lang)

	return nil
}

// SaveConfig writes the appConfig (Root) to disk
func SaveConfig() error {
	return saveConfigInternal(true)
}

// SaveConfigQuiet writes the configuration to disk WITHOUT triggering update hooks.
// This is used for internal updates like synchronization timestamps.
func SaveConfigQuiet() error {
	return saveConfigInternal(false)
}

func saveConfigInternal(triggerHook bool) error {
	configMutex.Lock()

	// 1. Sync globalConfig back to appConfig before saving
	if appConfig.Contexts == nil {
		appConfig.Contexts = make(map[string]ClusterConfig)
	}
	appConfig.Contexts[appConfig.CurrentContext] = globalConfig

	// 2. Create a snapshot for marshaling
	snap := appConfig.DeepCopy()
	configMutex.Unlock()

	// 3. Marshal to YAML
	data, err := yaml.Marshal(snap)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 4. Safety check: don't write empty/corrupted config
	if len(data) < 10 {
		return fmt.Errorf("refusing to save suspiciously small config (%d bytes)", len(data))
	}

	return writeConfigAtomic(data, triggerHook)
}

// WriteConfigRaw performs an atomic write of raw YAML data to the configuration file.
// This is used for restoration and external synchronization.
func WriteConfigRaw(data []byte) error {
	return writeConfigAtomic(data, true)
}

func writeConfigAtomic(data []byte, triggerHook bool) error {
	// 5. Atomic Write with Mutex and Debouncer
	writeMutex.Lock()
	defer writeMutex.Unlock()

	path := viper.ConfigFileUsed()
	if path == "" {
		path = GetConfigPath()
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 5b. Rotate history before overwriting (if file exists)
	if _, err := os.Stat(path); err == nil {
		if err := rotateConfigHistory(path); err != nil {
			log.Printf("Warning: failed to rotate config history: %v", err)
		}
	}

	// Set the debouncer timestamp BEFORE the write
	lastInternalWriteMutex.Lock()
	lastInternalWrite = time.Now()
	lastInternalWriteMutex.Unlock()

	// Write to temporary file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary config: %w", err)
	}

	// Atomic rename (best effort on Windows)
	success := false
	if err := os.Rename(tmpPath, path); err == nil {
		success = true
	} else {
		// On Windows, rename fails if target exists.
		_ = os.Remove(path)
		if err := os.Rename(tmpPath, path); err == nil {
			success = true
		} else {
			// Fallback to direct write if rename is impossible
			if err := os.WriteFile(path, data, 0600); err == nil {
				success = true
				_ = os.Remove(tmpPath)
			}
		}
	}

	if !success {
		return fmt.Errorf("failed to save config: atomic write failed")
	}

	// Update Viper's internal state to match what we just wrote.
	// This ensures that subsequent calls to viper.Get() return up-to-date data.
	_ = viper.ReadInConfig()

	if triggerHook && OnConfigUpdated != nil {
		OnConfigUpdated(path)
	}

	return nil
}

// rotateConfigHistory creates a backup of the current config file in .kuargogo/config_history
// and keeps only the most recent 40 versions.
func rotateConfigHistory(currentPath string) error {
	historyDir := filepath.Join(GetConfigDir(), "config_history")
	if err := os.MkdirAll(historyDir, 0700); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// 1. Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	baseName := filepath.Base(currentPath)
	backupName := fmt.Sprintf("%s.%s.bak", baseName, timestamp)
	backupPath := filepath.Join(historyDir, backupName)

	// 2. Copy current file to history (read/write to ensure cross-partition safety)
	data, err := os.ReadFile(currentPath)
	if err != nil {
		return fmt.Errorf("failed to read current config for backup: %w", err)
	}
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	// 3. Prune old history files (keep max 40)
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return nil // Non-critical if we can't prune
	}

	var backups []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".bak") {
			backups = append(backups, entry)
		}
	}

	if len(backups) <= 40 {
		return nil
	}

	// Sort by modification time (oldest first)
	sort.Slice(backups, func(i, j int) bool {
		infoI, _ := backups[i].Info()
		infoJ, _ := backups[j].Info()
		if infoI == nil || infoJ == nil {
			return false
		}
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	// Delete oldest files
	toDelete := len(backups) - 40
	for i := 0; i < toDelete; i++ {
		_ = os.Remove(filepath.Join(historyDir, backups[i].Name()))
	}

	return nil
}

// ListBackups returns a list of available configuration backups, sorted by date (newest first).
func ListBackups() ([]string, error) {
	historyDir := filepath.Join(GetConfigDir(), "config_history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	var backups []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".bak") {
			backups = append(backups, entry)
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		infoI, _ := backups[i].Info()
		infoJ, _ := backups[j].Info()
		if infoI == nil || infoJ == nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})

	var names []string
	for _, b := range backups {
		names = append(names, b.Name())
	}

	return names, nil
}

// RestoreBackup replaces the current configuration with the specified backup file.
// It creates a fresh backup of the current state before overwriting.
func RestoreBackup(backupName string) error {
	historyDir := filepath.Join(GetConfigDir(), "config_history")
	backupPath := filepath.Join(historyDir, backupName)

	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup file not found: %s", backupName)
	}

	path := viper.ConfigFileUsed()
	if path == "" {
		path = GetConfigPath()
	}

	// 1. Create a "safety" backup of the current state before restoring
	if _, err := os.Stat(path); err == nil {
		_ = rotateConfigHistory(path)
	}

	// 2. Read the backup content
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// 3. Write it back to the main config path
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to restore config file: %w", err)
	}

	// 4. Reload the configuration in memory
	return LoadConfig(path)
}

// ModifyConfig safely updates the active configuration
func ModifyConfig(operation func(*ClusterConfig)) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	// Pass a pointer to the globalConfig.
	// Since we hold the lock, modification in place is safe relative to other writers.
	operation(&globalConfig)

	// Sync to Contexts map immediately
	if appConfig.Contexts == nil {
		appConfig.Contexts = make(map[string]ClusterConfig)
	}
	appConfig.Contexts[appConfig.CurrentContext] = globalConfig

	return nil
}

// UpdateNodes updates the nodes for the ACTIVE context
func UpdateNodes(nodes []Node) {
	if err := ModifyConfig(func(c *ClusterConfig) {
		c.Nodes = nodes
	}); err != nil {
		log.Printf("failed to update nodes: %v", err)
	}
}

// UpdateNode replaces a single node by its original name in the active context.
func UpdateNode(originalName string, updated Node) error {
	return ModifyConfig(func(c *ClusterConfig) {
		for i, n := range c.Nodes {
			if n.Name == originalName {
				c.Nodes[i] = updated
				return
			}
		}
	})
}

// RemoveNode removes a node by name or IP from the active context.
func RemoveNode(identifier string) error {
	return ModifyConfig(func(c *ClusterConfig) {
		for i, n := range c.Nodes {
			if n.Name == identifier || n.IP == identifier {
				c.Nodes = append(c.Nodes[:i], c.Nodes[i+1:]...)
				return
			}
		}
	})
}

// FindNode finds a node by name or IP in the active configuration.
// Returns a pointer to a copy of the node, or nil if not found.
// The returned *Node is disconnected from the global config;
// mutating it will NOT update the stored configuration.
// To persist changes, use UpdateNode() or ModifyConfig().
func FindNode(identifier string) *Node {
	cfg := GetConfig()
	for _, n := range cfg.Nodes {
		if n.Name == identifier || n.IP == identifier {
			return &n
		}
	}
	return nil
}

// SetConfig updates the active configuration context (used by Wizard/Init)
func SetConfig(cfg ClusterConfig) {
	configMutex.Lock()
	defer configMutex.Unlock()
	globalConfig = cfg
}

// GetConfigPath returns the standard path for the config file.
// It prioritizes viper.ConfigFileUsed() if available, then falls back to ~/.kuargogo/kuargogo.yaml.
func GetConfigPath() string {
	if path := viper.ConfigFileUsed(); path != "" {
		return path
	}
	dir := GetConfigDir()
	if dir == "" || dir == ".kuargogo" {
		return "kuargogo.yaml"
	}
	return filepath.Join(dir, "kuargogo.yaml")
}

// IsRunningOnInfraManager checks if the current hostname matches the configured infra-manager.
func IsRunningOnInfraManager() bool {
	cfg := GetConfig()
	infraNode := cfg.GetInfraManager()
	if infraNode == nil {
		return false
	}

	hostname, err := os.Hostname()
	if err != nil {
		return false
	}

	// Check against Name and IP
	return hostname == infraNode.Name || hostname == infraNode.IP
}

// RootConfigGetSync returns the global synchronization settings.
func RootConfigGetSync() SyncSettings {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return appConfig.Sync.DeepCopy()
}

// RootConfigSetS3 updates the global S3-compatible synchronization settings thread-safely.
func RootConfigSetS3(s3 Backup) {
	configMutex.Lock()
	defer configMutex.Unlock()
	appConfig.Sync.Provider = "s3"
	appConfig.Sync.S3 = s3
}

// UnlockConfig attempts to unlock the current configuration with the provided passphrase.
// If successful, it stores the key in the keychain and reloads the config.
func UnlockConfig(passphrase string) error {
	// 1. Verify passphrase by attempting a dummy derivation or just trying a full reload
	// For kuargogo, we use the Pull/Restore or specific field decryption.
	// We'll store it then reload.
	if err := StoreMasterKey(passphrase); err != nil {
		return fmt.Errorf("failed to save passphrase: %w", err)
	}

	// 2. Reload config to decrypt fields
	path := viper.ConfigFileUsed()
	if path == "" {
		path = GetConfigPath()
	}
	return LoadConfig(path)
}

// GetFailedDecryptionCount returns the number of secrets that could not be decrypted during the last load.
func GetFailedDecryptionCount() int {
	return FailedDecryptionCount
}

// EnsureSalt generates a new salt if it's missing in the configuration context.
// Returns the current (or new) salt in Base64 format.
func EnsureSalt() (string, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	if appConfig.Sync.Salt != "" {
		return appConfig.Sync.Salt, nil
	}

	salt, err := GenerateSalt()
	if err != nil {
		return "", err
	}

	appConfig.Sync.Salt = base64.StdEncoding.EncodeToString(salt)
	return appConfig.Sync.Salt, nil
}

// secretDecodeHook is a mapstructure DecodeHookFunc that handles the Secret type.
// It detects fields of type Secret and automatically attempts to decrypt them if needed.
func secretDecodeHook() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {

		// If the target type is Secret, we process it
		if t != reflect.TypeOf(Secret("")) {
			return data, nil
		}

		// The source data must be a string (the potentially encrypted !vaultENC:...)
		str, ok := data.(string)
		if !ok {
			return data, nil
		}

		// Use our refactored helper from structs.go
		decrypted, _ := decryptSecretValue(str)
		// Note: we ignore the success/failure return here as FailedDecryptionCount
		// is already handled inside decryptSecretValue or in the UnmarshalYAML logic.
		// Wait, I should handle FailedDecryptionCount here too if I want it to be accurate.

		if strings.HasPrefix(str, "!vaultENC:") && decrypted == str {
			// Decryption failed (since it's still prefixed and doesn't match original if decrypted)
			// Actually decryptSecretValue returns original if fails.
			FailedDecryptionCount++
		}

		return Secret(decrypted), nil
	}
}

// extraArgsDecodeHook is a mapstructure DecodeHookFunc that handles the ExtraArgs type.
func extraArgsDecodeHook() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {

		if t != reflect.TypeOf(ExtraArgs(nil)) {
			return data, nil
		}

		res := make(ExtraArgs)

		// Case 1: Source is a map
		if m, ok := data.(map[string]interface{}); ok {
			for k, v := range m {
				res[k] = v
			}
			return res, nil
		}
		if m, ok := data.(map[interface{}]interface{}); ok {
			for k, v := range m {
				if ks, ok := k.(string); ok {
					res[ks] = v
				}
			}
			return res, nil
		}

		// Case 2: Source is a slice (legacy string slice format)
		var slice []string
		if s, ok := data.([]interface{}); ok {
			for _, item := range s {
				if str, ok := item.(string); ok {
					slice = append(slice, str)
				}
			}
		} else if s, ok := data.([]string); ok {
			slice = s
		}

		if len(slice) > 0 {
			for _, arg := range slice {
				cleanArg := strings.TrimPrefix(arg, "--")
				cleanArg = strings.TrimPrefix(cleanArg, "-")

				parts := strings.SplitN(cleanArg, "=", 2)
				key := strings.TrimSpace(parts[0])
				var val interface{} = true
				if len(parts) > 1 {
					val = strings.TrimSpace(parts[1])
				}

				if existing, ok := res[key]; ok {
					if existSlice, ok := existing.([]interface{}); ok {
						res[key] = append(existSlice, val)
					} else {
						res[key] = []interface{}{existing, val}
					}
				} else {
					res[key] = val
				}
			}
			return res, nil
		}

		return data, nil
	}
}
