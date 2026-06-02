package provision

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// GenerateClusterKey generates an Ed25519 key pair at the specified path
func GenerateClusterKey(basePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(basePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Generate Ed25519 key
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	// Marshaling private key to OpenSSH format is tricky with standard lib
	// Using generic PEM encoding for now which standard ssh-keygen supports processing,
	// BUT Go's x/crypto/ssh/marshal.go can marshal private keys.
	// Actually, modern OpenSSH keys are custom PEM.
	// x/crypto/ssh has MarshalPrivateKey since v0.1.0 (recent).

	// Let's use the stdlib way if possible or x/crypto setup.
	// MarshalPrivateKey returns a PEM block.
	pemBlock, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Write Private Key
	privatePem := pem.EncodeToMemory(pemBlock)
	if err := os.WriteFile(basePath, privatePem, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Generate Public Key
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to create public key: %w", err)
	}
	publicBytes := ssh.MarshalAuthorizedKey(sshPubKey)

	// Write Public Key
	pubPath := basePath + ".pub"
	if err := os.WriteFile(pubPath, publicBytes, 0644); err != nil {
		return fmt.Errorf("failed to write public key to %s: %w", pubPath, err)
	}

	return nil
}

// EnsureClusterKey generates a key pair at basePath if it doesn't exist.
// Returns (true, nil) if a new key was generated, (false, nil) if already present.
func EnsureClusterKey(basePath string) (bool, error) {
	if _, err := os.Stat(basePath); err == nil {
		// Key already exists
		return false, nil
	}
	if err := GenerateClusterKey(basePath); err != nil {
		return false, fmt.Errorf("failed to auto-generate cluster key: %w", err)
	}
	return true, nil
}

// InstallKey copies the public key to the remote node using password authentication
func InstallKey(ip string, port int, user, password, pubKeyPath string) error {
	// Read Public Key
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key: %v", err)
	}
	pubKeyStr := strings.TrimSpace(string(pubKeyBytes))
	// SECURITY: Escape single quotes for shell command injection protection
	escapedKey := strings.ReplaceAll(pubKeyStr, "'", "'\\''")

	// Create Password Executor
	exec, err := NewPasswordExecutor(user, password)
	if err != nil {
		return fmt.Errorf("failed to create executor: %v", err)
	}
	exec.Stdout = io.Discard
	exec.Stderr = io.Discard

	// Consolidated command to setup .ssh and append key safely.
	// 1. Ensure directory exists.
	// 2. Ensure authorized_keys exists and has correct permissions.
	// 3. Check if key is already present.
	// 4. If not present:
	//    a. Check if file is non-empty and missing a trailing newline.
	//    b. If so, append a newline first.
	//    c. Append the key.
	setupCmd := fmt.Sprintf(
		"mkdir -p ~/.ssh && chmod 700 ~/.ssh && "+
			"touch ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys && "+
			"grep -qF '%s' ~/.ssh/authorized_keys || "+
			"( [ -s ~/.ssh/authorized_keys ] && [ -n \"$(tail -c1 ~/.ssh/authorized_keys 2>/dev/null)\" ] && echo >> ~/.ssh/authorized_keys; "+
			"echo '%s' >> ~/.ssh/authorized_keys )",
		escapedKey, escapedKey,
	)

	_, err = exec.ExecuteCommand(ip, port, setupCmd)
	if err != nil {
		return fmt.Errorf("failed to setup authorized_keys: %v", err)
	}

	return nil
}
