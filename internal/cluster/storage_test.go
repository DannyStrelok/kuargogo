package cluster

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

func TestCheckAndHealStorage_DryRun(t *testing.T) {
	// Create temporary config file
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
      - name: failing-node
        ip: 10.0.0.3
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

	// 1. Run without healing
	results, err := mgr.CheckAndHealStorage(masterNode, false)
	if err != nil {
		t.Fatalf("CheckAndHealStorage failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 disk results, got %d", len(results))
	}

	var healthyFound, failingFound bool
	for _, res := range results {
		switch res.NodeName {
		case "test-worker":
			healthyFound = true
			if res.Status != "healthy" {
				t.Errorf("expected test-worker status 'healthy', got '%s'", res.Status)
			}
			if res.Healed {
				t.Errorf("expected healed to be false, got true")
			}
		case "failing-node":
			failingFound = true
			if res.Status != "failing" {
				t.Errorf("expected failing-node status 'failing', got '%s'", res.Status)
			}
			if res.Healed {
				t.Errorf("expected healed to be false (heal=false), got true")
			}
		}
	}

	if !healthyFound {
		t.Error("test-worker result not found")
	}
	if !failingFound {
		t.Error("failing-node result not found")
	}

	// 2. Run with healing enabled
	results, err = mgr.CheckAndHealStorage(masterNode, true)
	if err != nil {
		t.Fatalf("CheckAndHealStorage failed: %v", err)
	}

	for _, res := range results {
		if res.NodeName == "failing-node" {
			if !res.Healed {
				t.Errorf("expected failing-node to be healed (heal=true), got false")
			}
		}
	}
}
