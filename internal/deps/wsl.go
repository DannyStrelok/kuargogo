package deps

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"context"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// GetWSLDistro returns the configured WSL distribution name or "Ubuntu" by default.
func GetWSLDistro() string {
	distro := config.GetConfig().Ansible.WSLDistro
	if distro == "" {
		return "Ubuntu" // Standard default for kuargogo
	}
	return distro
}

func ConvertToWSLPath(windowsPath string) (string, error) {
	if runtime.GOOS != "windows" {
		return windowsPath, nil
	}

	// Expand leading tilde (~/) if present
	if strings.HasPrefix(windowsPath, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			windowsPath = filepath.Join(home, windowsPath[2:])
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "wsl", "-d", GetWSLDistro(), "-e", "wslpath", "-a", "-u", windowsPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("wslpath timed out: your WSL distribution might be unresponsive")
		}
		return "", fmt.Errorf("wslpath failed for %s: %w (%s)", windowsPath, err, out.String())
	}
	return strings.TrimSpace(out.String()), nil
}

// CheckWSLUbuntu verifies if WSL with Ubuntu is working seamlessly.
func CheckWSLUbuntu() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "wsl", "-d", GetWSLDistro(), "-e", "true")
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("WSL is unresponsive (timeout). Please restart WSL with 'wsl --shutdown' in a terminal")
		}
		return fmt.Errorf("ubuntu WSL distribution is not available. Please install it with 'wsl --install -d Ubuntu'")
	}
	return nil
}

// CheckWSLCommand verifies if a command exists inside the WSL Ubuntu distribution
func CheckWSLCommand(command string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "wsl", "-d", GetWSLDistro(), "-e", "which", command)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("WSL is unresponsive while checking for %s", command)
		}
		return fmt.Errorf("command %s not found in WSL Ubuntu", command)
	}
	return nil
}

// SyncSSHKeyToWSL copies the SSH private key (and its .pub) from the Windows filesystem
// into WSL's native ~/.ssh/ directory with proper 0600 permissions.
// It returns the WSL-native path to the private key (e.g. /home/user/.ssh/kgg_cluster_id).
// If the key already exists in WSL and is up-to-date, it skips the copy.
func SyncSSHKeyToWSL(windowsKeyPath string) (string, error) {
	if runtime.GOOS != "windows" {
		return windowsKeyPath, nil
	}

	// Expand leading tilde (~/) if present
	if strings.HasPrefix(windowsKeyPath, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			windowsKeyPath = filepath.Join(home, windowsKeyPath[2:])
		}
	}

	// Validate Source Key exists on Windows before trying to sync
	if _, err := os.Stat(windowsKeyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("SSH key not found at %s", windowsKeyPath)
	}

	// 1. Get the WSL user's home directory
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var homeBuf bytes.Buffer
	homeCmd := exec.CommandContext(ctx, "wsl", "-d", GetWSLDistro(), "-e", "sh", "-c", "echo $HOME")
	homeCmd.Stdout = &homeBuf
	if err := homeCmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("failed to get WSL home directory: service timed out")
		}
		return "", fmt.Errorf("failed to get WSL home directory: %w", err)
	}
	wslHome := strings.TrimSpace(homeBuf.String())
	if wslHome == "" {
		return "", fmt.Errorf("WSL home directory is empty")
	}

	// 2. Derive the destination key name from the Windows path
	// e.g. C:\Users\Daniel\.ssh\kgg_cluster_id -> kgg_cluster_id
	keyName := windowsKeyPath
	if idx := strings.LastIndexAny(keyName, "\\/"); idx >= 0 {
		keyName = keyName[idx+1:]
	}
	wslKeyPath := wslHome + "/.ssh/" + keyName

	// 3. Convert the Windows source path to /mnt/... so WSL can read it
	wslSourcePath, err := ConvertToWSLPath(windowsKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to convert key path: %w", err)
	}

	// 4. Ensure ~/.ssh exists, copy key with proper permissions (idempotent)
	// Uses cp only if the source is newer or destination doesn't exist
	script := fmt.Sprintf(`
		mkdir -p "%s/.ssh" && chmod 700 "%s/.ssh"
		if [ ! -f "%s" ] || [ "%s" -nt "%s" ]; then
			cp "%s" "%s" && chmod 600 "%s"
			# Also copy .pub if it exists
			if [ -f "%s.pub" ]; then
				cp "%s.pub" "%s.pub" && chmod 644 "%s.pub"
			fi
		fi
	`, wslHome, wslHome,
		wslKeyPath, wslSourcePath, wslKeyPath,
		wslSourcePath, wslKeyPath, wslKeyPath,
		wslSourcePath, wslSourcePath, wslKeyPath, wslKeyPath,
	)

	syncCmd := exec.CommandContext(ctx, "wsl", "-d", GetWSLDistro(), "-e", "sh", "-c", script)
	var stderr bytes.Buffer
	syncCmd.Stderr = &stderr
	if err := syncCmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("failed to sync SSH key to WSL: service timed out")
		}
		return "", fmt.Errorf("failed to sync SSH key to WSL: %w (%s)", err, stderr.String())
	}

	return wslKeyPath, nil
}

// IsWindowsPath returns true if the string looks like a Windows absolute path or UNC path.
func IsWindowsPath(s string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	// Detect absolute path with drive letter (C:\...) or UNC path (\\...)
	if (len(s) >= 3 && s[1] == ':' && s[2] == '\\') || strings.HasPrefix(s, "\\\\") {
		return true
	}
	// Also detect paths starting with backslash (absolute from current drive)
	if strings.HasPrefix(s, "\\") && !strings.HasPrefix(s, "\\\\") {
		return true
	}
	return false
}

// SyncKnownHostsToWSL copies the kgg_known_hosts file from Windows ~/.ssh
// into WSL's native ~/.ssh/ directory to ensure Ansible uses the same verified keys.
func SyncKnownHostsToWSL() error {
	if runtime.GOOS != "windows" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	windowsKnownHosts := filepath.Join(home, ".ssh", "kgg_known_hosts")
	if _, err := os.Stat(windowsKnownHosts); os.IsNotExist(err) {
		return nil // Nothing to sync yet
	}

	wslSourcePath, err := ConvertToWSLPath(windowsKnownHosts)
	if err != nil {
		return fmt.Errorf("failed to convert wsl path for known_hosts: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var homeBuf bytes.Buffer
	homeCmd := exec.CommandContext(ctx, "wsl", "-d", GetWSLDistro(), "-e", "sh", "-c", "echo $HOME")
	homeCmd.Stdout = &homeBuf
	if err := homeCmd.Run(); err != nil {
		return fmt.Errorf("failed to get wsl home for known_hosts sync: %w", err)
	}
	wslHome := strings.TrimSpace(homeBuf.String())
	if wslHome == "" {
		return fmt.Errorf("WSL home is empty")
	}

	script := fmt.Sprintf(`
		mkdir -p "%s/.ssh" && chmod 700 "%s/.ssh"
		if [ ! -f "%s/.ssh/kgg_known_hosts" ] || [ "%s" -nt "%s/.ssh/kgg_known_hosts" ]; then
			cp "%s" "%s/.ssh/kgg_known_hosts"
			chmod 644 "%s/.ssh/kgg_known_hosts"
		fi
	`, wslHome, wslHome, wslHome, wslSourcePath, wslHome, wslSourcePath, wslHome, wslHome)

	syncCmd := exec.CommandContext(ctx, "wsl", "-d", GetWSLDistro(), "-e", "sh", "-c", script)
	if err := syncCmd.Run(); err != nil {
		return fmt.Errorf("failed to sync kgg_known_hosts to WSL: %w", err)
	}

	return nil
}
