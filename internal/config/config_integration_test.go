package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Integration(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "kuargogo.yaml")

	content := []byte(`
current_context: test-ctx
contexts:
  test-ctx:
    nodes:
      - name: int-test-master
        ip: 10.0.0.1
        role: master
    ssh:
      private_key_path: ~/.ssh/id_rsa
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	// 1. Test Loading Valid Config
	if err := LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if GetCurrentContext() != "test-ctx" {
		t.Errorf("expected context 'test-ctx', got '%s'", GetCurrentContext())
	}

	nodeName := GetConfig().Nodes[0].Name
	if nodeName != "int-test-master" {
		t.Errorf("expected node name 'int-test-master', got '%s'", nodeName)
	}

	// 2. Test implicit context usage
	// appConfig should have been populated
	if len(GetAppConfig().Contexts) != 1 {
		t.Errorf("expected 1 context, got %d", len(GetAppConfig().Contexts))
	}
}

func TestLoadConfig_LegacyMigration(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "legacy.yaml")

	// Legacy config has no "contexts" key, just root "nodes"
	content := []byte(`
nodes:
  - name: legacy-master
    ip: 192.168.1.50
    role: master
ssh:
  private_key_path: ~/.ssh/id_rsa
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	if err := LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Should have migrated to 'default' context
	if GetCurrentContext() != "default" {
		t.Errorf("expected migrated context 'default', got '%s'", GetCurrentContext())
	}

	if len(GetAppConfig().Contexts) != 1 {
		t.Errorf("expected 1 context after migration, got %d", len(GetAppConfig().Contexts))
	}

	ctx, ok := GetAppConfig().Contexts["default"]
	if !ok {
		t.Fatal("default context not found in map")
	}

	if len(ctx.Nodes) != 1 || ctx.Nodes[0].Name != "legacy-master" {
		t.Errorf("migration failed to move nodes correctly")
	}
}

func TestNodeManagement_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "kuargogo.yaml")

	content := []byte(`
current_context: default
contexts:
  default:
    nodes:
      - name: test-node-1
        ip: 10.0.0.1
        role: master
      - name: test-node-2
        ip: 10.0.0.2
        role: worker
    ssh:
      private_key_path: ~/.ssh/id_rsa
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	if err := LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// 1. Test FindNode
	node1 := FindNode("test-node-1")
	if node1 == nil || node1.IP != "10.0.0.1" {
		t.Errorf("FindNode by name failed")
	}

	node2 := FindNode("10.0.0.2")
	if node2 == nil || node2.Name != "test-node-2" {
		t.Errorf("FindNode by IP failed")
	}

	missing := FindNode("non-existent")
	if missing != nil {
		t.Errorf("FindNode should return nil for missing node")
	}

	// 2. Test UpdateNode
	node1.Role = "control-plane"
	node1.MAC = "AA:BB:CC:DD:EE:FF"
	if err := UpdateNode("test-node-1", *node1); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}

	updated := FindNode("test-node-1")
	if updated == nil || updated.Role != "control-plane" || updated.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("UpdateNode didn't persist changes")
	}

	// 3. Test RemoveNode
	if err := RemoveNode("test-node-2"); err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}

	if FindNode("test-node-2") != nil {
		t.Errorf("RemoveNode didn't remove the node")
	}

	cfg := GetConfig()
	if len(cfg.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(cfg.Nodes))
	}
}
