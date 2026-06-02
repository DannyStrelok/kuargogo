package provision

import (
	"os"
	"strings"
	"testing"
)

func TestExecutor_DryRun(t *testing.T) {
	// Initialize Executor with DryRun = true
	executor, err := NewExecutor("user", "/tmp/fake-key", true)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	host := "192.168.1.100"
	port := 22
	cmd := "echo hello"

	output, err := executor.ExecuteCommand(host, port, cmd)
	if err != nil {
		t.Fatalf("ExecuteCommand (DryRun) failed: %v", err)
	}

	// Verify output format
	expectedSubstrings := []string{
		"[DRY-RUN]",
		"192.168.1.100:22",
		"echo hello",
	}

	for _, s := range expectedSubstrings {
		if !strings.Contains(output, s) {
			t.Errorf("Expected output to contain '%s', got: %s", s, output)
		}
	}
}

func TestRemoveHostKeyFromFile(t *testing.T) {
	// Create a temporary known_hosts file
	tmpFile := t.TempDir() + "/known_hosts"
	content := []string{
		"192.168.1.41 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI...",
		"192.168.1.42 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAJ...",
		"[192.168.1.41]:22 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTI...",
		"myhostname ssh-rsa AAAAB3Nza1yc2EAAAADAQABAAABA...",
		"# A comment line",
		"",
		"192.168.1.43,otherhost ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAK...",
	}

	err := os.WriteFile(tmpFile, []byte(strings.Join(content, "\n")), 0600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Test 1: Remove by IP 192.168.1.41
	err = removeHostKeyFromFile("192.168.1.41", tmpFile)
	if err != nil {
		t.Errorf("removeHostKeyFromFile failed: %v", err)
	}

	// Verify entries for 1.41 are gone (both plain and bracketed)
	newContent, _ := os.ReadFile(tmpFile)
	newStr := string(newContent)
	if strings.Contains(newStr, "192.168.1.41 ") || strings.Contains(newStr, "[192.168.1.41]:22") {
		t.Errorf("Entry for 192.168.1.41 still exists after removal:\n%s", newStr)
	}

	// Verify other entries still exist
	if !strings.Contains(newStr, "192.168.1.42") {
		t.Error("Relevent entry for 192.168.1.42 was accidentally removed")
	}
	if !strings.Contains(newStr, "myhostname") {
		t.Error("Relevent entry for myhostname was accidentally removed")
	}

	// Test 2: Remove by hostname
	err = removeHostKeyFromFile("myhostname", tmpFile)
	if err != nil {
		t.Errorf("removeHostKeyFromFile failed for hostname: %v", err)
	}
	newContent, _ = os.ReadFile(tmpFile)
	newStr = string(newContent)
	if strings.Contains(newStr, "myhostname") {
		t.Error("Entry for myhostname still exists after removal")
	}

	// Test 3: Remove multi-host entry
	err = removeHostKeyFromFile("otherhost", tmpFile)
	if err != nil {
		t.Errorf("removeHostKeyFromFile failed for multi-host: %v", err)
	}
	newContent, _ = os.ReadFile(tmpFile)
	newStr = string(newContent)
	if strings.Contains(newStr, "otherhost") {
		t.Error("Entry for otherhost still exists after removal")
	}
}
