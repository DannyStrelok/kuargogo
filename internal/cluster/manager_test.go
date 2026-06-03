package cluster

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

func TestRemediateNode_DryRun(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "kuargogo.yaml")

	content := []byte(`
current_context: test-ctx
contexts:
  test-ctx:
    nodes:
      - name: test-master
        ip: 10.0.0.1
        role: master
      - name: test-worker
        ip: 10.0.0.2
        role: worker
    ssh:
      private_key_path: ~/.ssh/id_rsa
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	if err := config.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	var buf bytes.Buffer
	mgr := NewManager("kgg-admin", "~/.ssh/id_rsa", 22, true)
	mgr.Output = &buf

	masterNode := &config.Node{
		Name: "test-master",
		IP:   "10.0.0.1",
		Role: "master",
	}

	err := mgr.RemediateNode(masterNode, "test-worker", []string{"test-tag"})
	if err != nil {
		t.Fatalf("RemediateNode dry-run failed: %v", err)
	}

	outputStr := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Draining node test-worker")) {
		t.Errorf("expected output to contain drain message, got: %s", outputStr)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Resetting K3s on node test-worker")) {
		t.Errorf("expected output to contain reset message, got: %s", outputStr)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Fetching join token from master test-master")) {
		t.Errorf("expected output to contain fetch token message, got: %s", outputStr)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Rejoining node test-worker as agent")) {
		t.Errorf("expected output to contain rejoin message, got: %s", outputStr)
	}
}
