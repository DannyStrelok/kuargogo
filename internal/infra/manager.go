package infra

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/provision"
)

type Manager struct {
	User    string
	KeyPath string
	Port    int
	DryRun  bool
	Output  io.Writer // Destination for progress messages (defaults to os.Stdout)
}

func NewManager(user, keyPath string, port int, dryRun bool) *Manager {
	return &Manager{
		User:    user,
		KeyPath: keyPath,
		Port:    port,
		DryRun:  dryRun,
		Output:  os.Stdout, // Default to stdout for backward compatibility
	}
}

// SyncConfig uploads a local configuration file to the infrastructure manager's home directory.
func (m *Manager) SyncConfig(ip, localPath string) error {
	executor, err := provision.NewExecutor(m.User, m.KeyPath, m.DryRun)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintln(m.Output, "Syncing configuration to infra-manager..."); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}

	// 1. Read local config
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local config: %w", err)
	}

	// 2. Upload using cat/tee (minimal dependencies)
	// We sync to both the current user's home and the kgg-admin user's home
	// to ensure the Telegram bot and other services find the config.
	// Step 1: Ensure target directories exist
	mkdirCmd := "mkdir -p ~/.kuargogo && sudo mkdir -p /home/kgg-admin/.kuargogo"
	if _, err := executor.ExecuteCommand(ip, m.Port, mkdirCmd); err != nil {
		return fmt.Errorf("failed to create config directories: %w", err)
	}

	// Step 2: Write the config file to both locations using a heredoc
	syncCmd := fmt.Sprintf("cat << 'KGGEOF' | tee ~/.kuargogo/kuargogo.yaml | sudo tee /home/kgg-admin/.kuargogo/kuargogo.yaml > /dev/null\n%s\nKGGEOF", string(content))
	out, err := executor.ExecuteCommand(ip, m.Port, syncCmd)
	if err != nil {
		return fmt.Errorf("failed to sync config: %w\nOutput: %s", err, out)
	}

	// 3. Inject Master Passphrase into environment for headless operations (fallback)
	// We add it to .bashrc for future 'kgg' calls on that host
	if mKey, err := config.GetMasterKey(); err == nil && mKey != "" {
		injectCmd := fmt.Sprintf("grep -q 'KGG_MASTER_PASSPHRASE' ~/.bashrc || echo 'export KGG_MASTER_PASSPHRASE=%s' >> ~/.bashrc", mKey)
		_, _ = executor.ExecuteCommand(ip, m.Port, injectCmd)
	}

	if _, err := fmt.Fprintln(m.Output, "✅ Configuration synced successfully!"); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}

	return nil
}
