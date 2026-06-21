package ansible

import (
	"bytes"
	"strings"
	"testing"

	"os"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

func TestWriteInventory_Labels(t *testing.T) {
	nodes := []config.Node{
		{
			Name: "master1",
			IP:   "10.0.0.10",
			User: "pi",
			Role: "master",
			Labels: map[string]string{
				"storage": "ssd",
				"net":     "gigabit",
			},
		},
		{
			Name: "worker1",
			IP:   "10.0.0.20",
			User: "pi",
			Role: "worker",
			Labels: map[string]string{
				"gpu": "nvidia",
			},
		},
		{
			Name: "worker2",
			IP:   "10.0.0.30",
			User: "pi",
			Role: "worker",
		},
	}

	// Setup: Create a temporary SSH key for the test
	tmpKey, err := os.CreateTemp("", "kgg-test-key")
	if err != nil {
		t.Fatalf("failed to create temp key: %v", err)
	}
	tmpKeyPath := tmpKey.Name()
	if err := tmpKey.Close(); err != nil {
		t.Fatalf("failed to close temp key: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpKeyPath)
	}()

	// Mock configuration
	origCfg := config.GetConfig()
	mockCfg := origCfg.DeepCopy()
	mockCfg.SSH.PrivateKeyPath = tmpKeyPath
	config.SetConfig(mockCfg)
	defer config.SetConfig(origCfg)

	var buf bytes.Buffer
	_, err = WriteInventory(&buf, nodes)
	if err != nil {
		t.Fatalf("WriteInventory failed: %v", err)
	}

	out := buf.String()

	// Check master1
	if !strings.Contains(out, "master1 ansible_host=10.0.0.10") {
		t.Errorf("Missing master1 entry")
	}
	if !strings.Contains(out, "node_labels=\"storage=ssd,net=gigabit\"") && !strings.Contains(out, "node_labels=\"net=gigabit,storage=ssd\"") {
		t.Errorf("Missing or incorrect labels for master1: %s", out)
	}

	// Check worker1
	if !strings.Contains(out, "worker1 ansible_host=10.0.0.20") {
		t.Errorf("Missing worker1 entry")
	}
	if !strings.Contains(out, "node_labels=\"gpu=nvidia\"") {
		t.Errorf("Missing or incorrect labels for worker1: %s", out)
	}

	// Check worker2 (should have no labels)
	if strings.Contains(out, "worker2") && strings.Contains(out[strings.Index(out, "worker2"):], "node_labels=") {
		// Strictly, we need to make sure worker2's line doesn't have node_labels
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "worker2") && strings.Contains(line, "node_labels=") {
				t.Errorf("worker2 should not have labels, but got: %s", line)
			}
		}
	}
}

func TestWriteInventory_K3sArgs(t *testing.T) {
	nodes := []config.Node{
		{
			Name: "master1",
			IP:   "10.0.0.10",
			User: "pi",
			Role: "master",
		},
	}

	tmpKey, err := os.CreateTemp("", "kgg-test-key")
	if err != nil {
		t.Fatalf("failed to create temp key: %v", err)
	}
	tmpKeyPath := tmpKey.Name()
	_ = tmpKey.Close()
	defer func() {
		_ = os.Remove(tmpKeyPath)
	}()

	origCfg := config.GetConfig()
	mockCfg := origCfg.DeepCopy()
	mockCfg.SSH.PrivateKeyPath = tmpKeyPath
	mockCfg.K3s.ServerArgs = config.ExtraArgs{
		"disable":               []any{"servicelb", "traefik"},
		"write-kubeconfig-mode": "644",
	}
	mockCfg.K3s.AgentArgs = config.ExtraArgs{
		"node-ip": "10.0.0.10",
	}
	config.SetConfig(mockCfg)
	defer config.SetConfig(origCfg)

	var buf bytes.Buffer
	_, err = WriteInventory(&buf, nodes)
	if err != nil {
		t.Fatalf("WriteInventory failed: %v", err)
	}

	out := buf.String()

	// Check if the JSON strings are written inside single quotes correctly
	if !strings.Contains(out, "k3s_server_extra_args='") {
		t.Errorf("Missing single quoted k3s_server_extra_args in inventory: %s", out)
	}
	if !strings.Contains(out, "k3s_agent_extra_args='") {
		t.Errorf("Missing single quoted k3s_agent_extra_args in inventory: %s", out)
	}

	// Verify specific JSON values
	if !strings.Contains(out, `"write-kubeconfig-mode":"644"`) {
		t.Errorf("Missing write-kubeconfig-mode JSON in inventory: %s", out)
	}
	if !strings.Contains(out, `"node-ip":"10.0.0.10"`) {
		t.Errorf("Missing node-ip JSON in inventory: %s", out)
	}
}

