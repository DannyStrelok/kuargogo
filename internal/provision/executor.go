package provision

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/DannyStrelok/kuargogo/internal/deps"
)

// RemoveHostKey removes all entries for the given hostname/IP from the custom kgg_known_hosts file.
// This is useful during bootstrap when a node's IP is reassigned or its OS is reinstalled.
func RemoveHostKey(hostname string) error {
	path, err := knownHostsFilePath()
	if err != nil {
		return err
	}
	return removeHostKeyFromFile(hostname, path)
}

// RemoveSystemHostKey removes all entries for the given hostname/IP from the system's
// standard known_hosts file (~/.ssh/known_hosts).
// On Windows, it also attempts to clean the WSL (Ubuntu) known_hosts file.
func RemoveSystemHostKey(hostname string) error {
	// 1. Clean native OS known_hosts
	home, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(home, ".ssh", "known_hosts")
		// Try ssh-keygen -R first as it handles hashed entries
		if errK := removeHostKeyWithKeygen(hostname, path); errK != nil {
			// Fallback to manual if keygen fails or is not in PATH
			_ = removeHostKeyFromFile(hostname, path)
		}
	}

	// 2. If on Windows, also clean WSL (Ubuntu) known_hosts
	if runtime.GOOS == "windows" {
		_ = removeWSLHostKey(hostname)
	}

	return nil
}

// removeHostKeyWithKeygen uses the system's ssh-keygen -R command to remove a host.
// This is preferred over manual parsing as it handles hashed entries correctly.
func removeHostKeyWithKeygen(hostname, path string) error {
	cmd := exec.Command("ssh-keygen", "-R", hostname, "-f", path)
	return cmd.Run()
}

// removeWSLHostKey executes ssh-keygen -R inside the WSL Ubuntu distribution.
// This is critical on Windows since Ansible runs inside WSL and uses its own known_hosts.
func removeWSLHostKey(hostname string) error {
	// We use the configured or detected distro from deps
	cmd := exec.Command("wsl", "-d", deps.GetWSLDistro(), "-e", "ssh-keygen", "-R", hostname)
	return cmd.Run()
}

// removeHostKeyFromFile is a private helper that performs the actual deletion
// from a specific known_hosts file path.
func removeHostKeyFromFile(hostname, path string) error {
	// Read everything
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to remove
		}
		return fmt.Errorf("failed to read known_hosts: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	changed := false

	// Patterns to look for (exact hostname or normalized version)
	searchPatterns := []string{hostname, knownhosts.Normalize(hostname)}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			newLines = append(newLines, line)
			continue
		}

		shouldRemove := false
		for _, pattern := range searchPatterns {
			// knownhosts lines start with host pattern (comma separated if multiple)
			parts := strings.SplitN(trimmed, " ", 2)
			if len(parts) > 0 {
				hosts := strings.Split(parts[0], ",")
				for _, h := range hosts {
					// Handle cases: "1.2.3.4", "[1.2.3.4]", "[1.2.3.4]:22", or "host:22"
					if h == pattern || h == "["+pattern+"]" ||
						strings.HasPrefix(h, pattern+":") ||
						strings.HasPrefix(h, "["+pattern+"]:") {
						shouldRemove = true
						break
					}
				}
			}
			if shouldRemove {
				break
			}
		}

		if shouldRemove {
			changed = true
			continue
		}
		newLines = append(newLines, line)
	}

	if !changed {
		return nil
	}

	// Write back
	output := strings.Join(newLines, "\n")
	return os.WriteFile(path, []byte(output), 0600)
}

// Executor handles SSH connections and command execution
type Executor struct {
	Config *ssh.ClientConfig
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
}

// NewExecutor creates a new SSH executor with Private Key Auth
func NewExecutor(user, keyPath string, dryRun bool) (*Executor, error) {
	if dryRun {
		return &Executor{DryRun: true, Stdout: os.Stdout, Stderr: os.Stderr}, nil
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %w", err)
	}

	// Host Key Checking
	// We use TOFU (Trust On First Use) by default to simplify homelab management,
	// especially when nodes are re-imaged. Strict checking is still active
	// if the host is already in kgg_known_hosts (prevents MITM).
	hostKeyCallback, err := getTOFUHostKeyCallback()
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         5 * time.Second,
	}

	return &Executor{
		Config: config,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}, nil
}

// NewPasswordExecutor creates a new SSH executor with Password Auth.
// Uses TOFU (Trust On First Use) host key handling: accepts and persists
// unknown host keys on first connection, rejects changed keys.
func NewPasswordExecutor(user, password string) (*Executor, error) {
	hostKeyCallback, err := getTOFUHostKeyCallback()
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         5 * time.Second,
	}

	return &Executor{
		Config: config,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}, nil
}

// knownHostsFilePath returns the path to kgg_known_hosts and ensures it exists.
func knownHostsFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	knownHostsPath := filepath.Join(home, ".ssh", "kgg_known_hosts")

	// Ensure file exists
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0700); err != nil {
			return "", fmt.Errorf("failed to create .ssh directory: %v", err)
		}
		if err := os.WriteFile(knownHostsPath, []byte{}, 0600); err != nil {
			return "", fmt.Errorf("failed to create known_hosts file: %v", err)
		}
	}
	return knownHostsPath, nil
}

// getHostKeyCallback returns a strict host key callback using kgg_known_hosts.
// Used by NewExecutor (key-based auth) where TOFU is not appropriate.
// func getHostKeyCallback() (ssh.HostKeyCallback, error) {
// 	path, err := knownHostsFilePath()
// 	if err != nil {
// 		return nil, err
// 	}
// 	return knownhosts.New(path)
// }

// getTOFUHostKeyCallback returns a Trust-On-First-Use host key callback.
// - Unknown hosts: accepted and persisted to kgg_known_hosts
// - Known hosts: validated strictly (rejects changed keys for MITM protection)
func getTOFUHostKeyCallback() (ssh.HostKeyCallback, error) {
	path, err := knownHostsFilePath()
	if err != nil {
		return nil, err
	}

	// Load existing known hosts for strict checking of KNOWN hosts
	existingCb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load known hosts: %w", err)
	}

	callback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		// Check against existing known hosts
		err := existingCb(hostname, remote, key)
		if err == nil {
			// Host is known and key matches
			return nil
		}

		// If the error is a key mismatch (host known but key changed), reject it
		var keyErr *knownhosts.KeyError
		if ok := isKeyError(err, &keyErr); ok && len(keyErr.Want) > 0 {
			// Host key CHANGED → reject (MITM protection)
			return fmt.Errorf("HOST KEY CHANGED for %s. This could indicate a MITM attack. "+
				"Remove the old entry from %s and retry", hostname, path)
		}

		// Host is unknown → TOFU: accept and persist
		line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("failed to open known_hosts for writing: %w", err)
		}
		defer func() { _ = f.Close() }()

		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("failed to write host key: %w", err)
		}

		return nil
	}

	return callback, nil
}

// isKeyError checks if err is a *knownhosts.KeyError via errors.As-style check.
func isKeyError(err error, target **knownhosts.KeyError) bool {
	if ke, ok := err.(*knownhosts.KeyError); ok {
		*target = ke
		return true
	}
	return false
}

// VerifySSHAccess performs a lightweight pre-flight check to verify SSH key
// authentication works for the given node.
func VerifySSHAccess(ip string, port int, user, keyPath string, dryRun bool) error {
	executor, err := NewExecutor(user, keyPath, dryRun)
	if err != nil {
		return fmt.Errorf("SSH pre-flight failed: %w", err)
	}
	executor.Stdout = io.Discard
	executor.Stderr = io.Discard

	_, err = executor.ExecuteCommand(ip, port, "echo ok")
	if err != nil {
		return fmt.Errorf("SSH pre-flight failed for %s@%s:%d: %w", user, ip, port, err)
	}
	return nil
}

// WaitSSHPort waits for the SSH port (TCP) to become open.
func WaitSSHPort(ip string, port int, timeout time.Duration) error {
	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	start := time.Now()
	for time.Since(start) < timeout {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout waiting for port %d on %s", port, ip)
}

// WaitSSH waits for the SSH service to become available and accept the key.
// Useful after reboots or IP changes. Timeout is 120 seconds.
func WaitSSH(ip string, port int, user, keyPath string, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		err := VerifySSHAccess(ip, port, user, keyPath, false)
		if err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out waiting for SSH on %s after %v", ip, timeout)
}

// Execute runs a command on the remote host
func (e *Executor) ExecuteCommand(host string, port int, command string) (string, error) {
	if e.DryRun {
		return fmt.Sprintf("[DRY-RUN] Executing on %s:%d:\n%s", host, port, command), nil
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := ssh.Dial("tcp", addr, e.Config)
	if err != nil {
		return "", fmt.Errorf("failed to dial: %w", err)
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	// Default to os.Stdout/Stderr if not set, though NewExecutor sets them.
	// We want to capture output AND stream it if configured.
	stdoutW := e.Stdout
	if stdoutW == nil {
		stdoutW = os.Stdout
	}
	stderrW := e.Stderr
	if stderrW == nil {
		stderrW = os.Stderr
	}

	session.Stdout = io.MultiWriter(stdoutW, &stdoutBuf)
	session.Stderr = io.MultiWriter(stderrW, &stderrBuf)

	if err := session.Run(command); err != nil {
		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return stdoutBuf.String(), fmt.Errorf("failed to run command: %v (stderr: %s)", err, stderrStr)
		}
		return stdoutBuf.String(), fmt.Errorf("failed to run command: %v", err)
	}

	return stdoutBuf.String(), nil
}
