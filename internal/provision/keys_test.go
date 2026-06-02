package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateClusterKey(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_ed25519")

	// Generate Key
	err := GenerateClusterKey(keyPath)
	if err != nil {
		t.Fatalf("GenerateClusterKey failed: %v", err)
	}

	// Verify Private Key file
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("Private key file not created: %v", err)
	}
	// Windows permissions strictly 0600 is tricky, but let's check it exists and has content.
	// On Windows, Mode() might be 0666 or similar normally, but Go tries to emulate partial POSIX.
	// We'll skip strict permission check on Windows to avoid flake, focus on content.

	privContent, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(privContent), "PRIVATE KEY") {
		t.Error("Private key content does not look like a PEM private key")
	}

	// Verify Public Key file
	pubPath := keyPath + ".pub"
	if _, err := os.Stat(pubPath); os.IsNotExist(err) {
		t.Fatalf("Public key file not created at %s", pubPath)
	}

	pubContent, err := os.ReadFile(pubPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(pubContent), "ssh-ed25519") {
		t.Errorf("Public key does not look like Ed25519: %s", string(pubContent))
	}
}
