package config

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// SyncPush encrypts and uploads the entire configuration to the cloud.
func SyncPush() error {
	// 1. Get current config state
	appCfg := GetAppConfig()
	if appCfg.Sync.S3.S3Url == "" {
		return fmt.Errorf("no s3 sync provider configured")
	}

	// 2. Get master passphrase
	passphrase, err := GetMasterKey()
	if err != nil {
		return fmt.Errorf("master passphrase not found in keychain. please login or set passphrase first: %w", err)
	}

	// 3. Ensure we have a salt for key derivation
	if appCfg.Sync.Salt == "" {
		salt, err := GenerateSalt()
		if err != nil {
			return err
		}
		appCfg.Sync.Salt = base64.StdEncoding.EncodeToString(salt)

		configMutex.Lock()
		appConfig.Sync.Salt = appCfg.Sync.Salt
		configMutex.Unlock()

		if err := SaveConfig(); err != nil {
			return err
		}
	}

	salt, _ := base64.StdEncoding.DecodeString(appCfg.Sync.Salt)

	// 4. Serialize the WHOLE config to YAML
	data, err := yaml.Marshal(appCfg)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	// 5. Encrypt
	encrypted, err := Encrypt(data, passphrase, salt)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	// 6. Get Provider and Push
	provider, err := GetSyncProvider()
	if err != nil {
		return err
	}

	if err := provider.Push(encrypted); err != nil {
		return err
	}

	configMutex.Lock()
	appConfig.Sync.LastSync = time.Now().Format(time.RFC3339)
	configMutex.Unlock()

	return SaveConfigQuiet()
}

// SyncPull downloads, decrypts, and restores the configuration from the cloud.
func SyncPull(passphrase string) error {
	// 1. Determine provider
	provider, err := GetSyncProvider()
	if err != nil {
		return err
	}

	// 2. Pull encrypted blob
	encrypted, err := provider.Pull()
	if err != nil {
		return err
	}

	// 3. Get Salt.
	// NOTE: provider.Pull() should have updated appConfig.Sync.Salt in memory
	// if it found it in the cloud metadata.
	appCfg := GetAppConfig()
	if appCfg.Sync.Salt == "" {
		return fmt.Errorf("local and cloud salt missing. disaster recovery is not possible without the encryption salt")
	}
	salt, _ := base64.StdEncoding.DecodeString(appCfg.Sync.Salt)

	// 4. Decrypt
	decrypted, err := Decrypt(encrypted, passphrase, salt)
	if err != nil {
		return fmt.Errorf("decryption failed (wrong passphrase?): %w", err)
	}

	// 5. Overwrite local config file
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = GetConfigPath()
	}

	if err := WriteConfigRaw(decrypted); err != nil {
		return fmt.Errorf("failed to write restored config: %w", err)
	}

	// 6. Reload into memory
	return LoadConfig(configPath)
}
