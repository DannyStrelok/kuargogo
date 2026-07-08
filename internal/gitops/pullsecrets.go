package gitops

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// PullSecretsService creates and maintains Kubernetes imagePullSecrets
// derived from the GitOps credentials stored in kuargogo.yaml.
type PullSecretsService struct {
	Output io.Writer
	DryRun bool
}

// NewPullSecretsService creates a service instance with stdout as default output.
func NewPullSecretsService() *PullSecretsService {
	return &PullSecretsService{Output: os.Stdout}
}

// SecretName derives a canonical, predictable Kubernetes secret name from a registry hostname.
// e.g. "ghcr.io" → "ghcr-io-pull-secret"
func SecretName(registry string) string {
	slug := strings.NewReplacer(".", "-", "/", "-").Replace(registry)
	return slug + "-pull-secret"
}

// Sync iterates all GitOps credentials that have a Registry defined and
// creates/updates an imagePullSecret in every namespace used by GitOps apps.
func (s *PullSecretsService) Sync(cfg config.ClusterConfig) error {
	// Collect namespaces from all GitOps app definitions (deduplicated)
	namespaces := collectNamespaces(cfg)
	if len(namespaces) == 0 {
		_, _ = fmt.Fprintln(s.Output, "⚠️  No namespaces found in GitOps projects. Nothing to do.")
		return nil
	}

	_, _ = fmt.Fprintf(s.Output, "📦 Target namespaces: %s\n\n", strings.Join(namespaces, ", "))

	kubeconfigPath, err := resolveKubeconfigPath(cfg)
	if err != nil {
		return err
	}

	syncedAny := false
	for _, cred := range cfg.GitOps.Credentials {
		if cred.Registry == "" {
			continue
		}
		syncedAny = true

		email := cred.Email
		if email == "" {
			email = fmt.Sprintf("%s@%s", cred.Username, cred.Registry)
		}

		secretName := SecretName(cred.Registry)
		_, _ = fmt.Fprintf(s.Output, "🔑 Syncing pull secret '%s' for registry '%s'...\n",
			secretName, cred.Registry)

		for _, ns := range namespaces {
			if err := s.ensureNamespace(kubeconfigPath, ns); err != nil {
				_, _ = fmt.Fprintf(s.Output, "   ❌ [%s] namespace: %v\n", ns, err)
				continue
			}

			if err := s.upsertSecret(kubeconfigPath, ns, secretName, cred.Registry,
				cred.Username, string(cred.Password), email); err != nil {
				_, _ = fmt.Fprintf(s.Output, "   ❌ [%s] %v\n", ns, err)
			} else {
				mode := "applied"
				if s.DryRun {
					mode = "dry-run"
				}
				_, _ = fmt.Fprintf(s.Output, "   ✅ [%s] secret '%s' %s\n", ns, secretName, mode)
			}
		}
		_, _ = fmt.Fprintln(s.Output)
	}

	if !syncedAny {
		_, _ = fmt.Fprintln(s.Output, "ℹ️  No credentials with 'registry' field found. "+
			"Add 'registry: ghcr.io' to a credential in kuargogo.yaml to enable pull secret automation.")
	}

	return nil
}

// upsertSecret creates or updates a docker-registry Secret idempotently using
// the `kubectl create secret --dry-run=client | kubectl apply` pattern.
func (s *PullSecretsService) upsertSecret(kubeconfigPath, namespace, secretName,
	server, username, password, email string) error {
	// Build the create command (generates YAML without applying)
	createArgs := []string{
		"create", "secret", "docker-registry", secretName,
		"--docker-server=" + server,
		"--docker-username=" + username,
		"--docker-password=" + password,
		"--docker-email=" + email,
		"--namespace=" + namespace,
		"--dry-run=client",
		"-o", "yaml",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	createCmd := exec.CommandContext(ctx, "kubectl", createArgs...)
	createCmd.Env = kubeconfigEnv(kubeconfigPath)

	if s.DryRun {
		// In dry-run mode just show what would be created
		_, _ = fmt.Fprintf(s.Output, "   [dry-run] kubectl %s\n", strings.Join(createArgs, " "))
		return nil
	}

	yamlBytes, err := createCmd.Output()
	if err != nil {
		return fmt.Errorf("generating secret YAML: %w", err)
	}

	// Pipe the generated YAML into `kubectl apply`
	applyCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-", "--namespace="+namespace)
	applyCmd.Env = kubeconfigEnv(kubeconfigPath)
	applyCmd.Stdin = strings.NewReader(string(yamlBytes))

	out, err := applyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("applying secret: %w\n%s", err, string(out))
	}

	return nil
}

// collectNamespaces gathers all unique namespaces referenced by GitOps apps.
func collectNamespaces(cfg config.ClusterConfig) []string {
	seen := make(map[string]bool)
	var result []string
	for _, proj := range cfg.GitOps.Projects {
		if proj.ManagedEnv {
			envs := []string{"dev", "test", "prod"}
			for _, env := range envs {
				ns := fmt.Sprintf("%s-%s", proj.Name, env)
				if !seen[ns] {
					seen[ns] = true
					result = append(result, ns)
				}
			}
		}
		for _, app := range proj.Apps {
			if app.Namespace != "" && !seen[app.Namespace] {
				seen[app.Namespace] = true
				result = append(result, app.Namespace)
			}
		}
	}
	return result
}

// resolveKubeconfigPath expands ~ in the configured kubeconfig path.
func resolveKubeconfigPath(cfg config.ClusterConfig) (string, error) {
	p := cfg.K3s.KubeconfigPath
	if p == "" {
		p = "~/.kube/config"
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		p = filepath.Join(home, p[2:])
	}
	return p, nil
}

// kubeconfigEnv returns the process environment with KUBECONFIG set.
func kubeconfigEnv(kubeconfigPath string) []string {
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "KUBECONFIG=") {
			env[i] = "KUBECONFIG=" + kubeconfigPath
			return env
		}
	}
	return append(env, "KUBECONFIG="+kubeconfigPath)
}

// ensureNamespace checks if the namespace exists, and creates it if missing.
func (s *PullSecretsService) ensureNamespace(kubeconfigPath, namespace string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// check if namespace exists
	checkCmd := exec.CommandContext(ctx, "kubectl", "get", "namespace", namespace)
	checkCmd.Env = kubeconfigEnv(kubeconfigPath)
	if err := checkCmd.Run(); err == nil {
		// Namespace already exists
		return nil
	}

	if s.DryRun {
		_, _ = fmt.Fprintf(s.Output, "   [dry-run] kubectl create namespace %s\n", namespace)
		return nil
	}

	// Create it
	createCmd := exec.CommandContext(ctx, "kubectl", "create", "namespace", namespace)
	createCmd.Env = kubeconfigEnv(kubeconfigPath)
	out, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating namespace: %w\n%s", err, string(out))
	}
	_, _ = fmt.Fprintf(s.Output, "   ✨ [%s] namespace created\n", namespace)
	return nil
}
