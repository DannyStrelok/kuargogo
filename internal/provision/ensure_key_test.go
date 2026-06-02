package provision

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureClusterKey_GeneratesWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_cluster_key")

	generated, err := EnsureClusterKey(keyPath)
	if err != nil {
		t.Fatalf("EnsureClusterKey failed: %v", err)
	}
	if !generated {
		t.Error("Expected generated=true when key doesn't exist")
	}

	// Verify files were created
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("Private key not created: %v", err)
	}
	if _, err := os.Stat(keyPath + ".pub"); err != nil {
		t.Errorf("Public key not created: %v", err)
	}
}

func TestEnsureClusterKey_SkipsWhenExists(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "existing_key")

	// Pre-create the key
	if err := os.WriteFile(keyPath, []byte("fake-private-key"), 0600); err != nil {
		t.Fatal(err)
	}

	generated, err := EnsureClusterKey(keyPath)
	if err != nil {
		t.Fatalf("EnsureClusterKey failed: %v", err)
	}
	if generated {
		t.Error("Expected generated=false when key already exists")
	}

	// Verify original content unchanged
	content, _ := os.ReadFile(keyPath)
	if string(content) != "fake-private-key" {
		t.Error("Existing key was overwritten")
	}
}
