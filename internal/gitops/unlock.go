package gitops

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// AppUnlockService manages live ArgoCD application unlocking operations.
type AppUnlockService struct {
	Output io.Writer
	DryRun bool
}

// NewAppUnlockService creates a new AppUnlockService.
func NewAppUnlockService() *AppUnlockService {
	return &AppUnlockService{
		Output: os.Stdout,
	}
}

// ResolveKubeconfigPath resolves the active kubeconfig path.
// It prioritizes:
// 1. KUBECONFIG environment variable.
// 2. Configured K3s Kubeconfig path.
// 3. Standard default ~/.kube/config.
func ResolveKubeconfigPath() (string, error) {
	if envPath := os.Getenv("KUBECONFIG"); envPath != "" {
		return envPath, nil
	}

	cfg := config.GetConfig()
	if cfg.K3s.KubeconfigPath != "" {
		return cfg.K3s.ExpandedKubeconfigPath()
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory for default kubeconfig: %w", err)
	}
	return filepath.Join(home, ".kube", "config"), nil
}

// Unlock patches the application CRD to remove finalizers and force deletion.
func (s *AppUnlockService) Unlock(appName string, kubeconfig string) error {
	if err := checkKubectl(); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(s.Output, "🔓 Unlocking ArgoCD Application '%s' in namespace 'argocd'...\n", appName)

	if s.DryRun {
		_, _ = fmt.Fprintf(s.Output, "[DRY RUN] Would execute: kubectl patch application %s -n argocd --type merge -p '{\"metadata\":{\"finalizers\":null}}'\n", appName)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "patch", "application", appName,
		"-n", "argocd", "--type", "merge", "-p", `{"metadata":{"finalizers":null}}`)
	cmd.Env = kubeconfigEnv(kubeconfig)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl patch failed: %w\n%s", err, string(out))
	}

	_, _ = fmt.Fprintf(s.Output, "✅ Application '%s' successfully unlocked (finalizers cleared).\n", appName)
	return nil
}
