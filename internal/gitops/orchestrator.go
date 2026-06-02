package gitops

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// Orchestrator manages the synchronization of the local GitOps configuration
// with the live ArgoCD instance in the cluster using kubectl apply.
type Orchestrator struct {
	Output io.Writer
	DryRun bool
}

// NewOrchestrator creates a new orchestrator with stdout as default output.
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{Output: os.Stdout}
}

// Sync applies all projects, credentials, and applications defined in the config to the cluster.
// It continues through all items even on partial failures, collecting errors at the end.
func (o *Orchestrator) Sync(cfg config.ClusterConfig) error {
	// Fail fast if kubectl is not available.
	if err := checkKubectl(); err != nil {
		return err
	}

	kubeconfigPath, err := cfg.K3s.ExpandedKubeconfigPath()
	if err != nil {
		return err
	}

	// Standard ArgoCD namespace — consistent with the Ansible installation.
	argocdNamespace := "argocd"

	var errs []string

	// 1. Sync Kargo Resources
	if len(cfg.GitOps.Pipelines) > 0 {
		_, _ = fmt.Fprintln(o.Output, "\n🚢 Syncing Kargo Pipelines...")
		for _, pipe := range cfg.GitOps.Pipelines {
			_, _ = fmt.Fprintf(o.Output, "   📦 Pipeline: %s\n", pipe.Name)
			if err := o.syncKargoResources(kubeconfigPath, pipe); err != nil {
				msg := fmt.Sprintf("kargo pipeline %q: %v", pipe.Name, err)
				_, _ = fmt.Fprintf(o.Output, "      ❌ %s\n", msg)
				errs = append(errs, msg)
			} else {
				_, _ = fmt.Fprintln(o.Output, "      ✅ Synced resources.")
			}
		}
	}

	// 2. Sync Credentials (as Kubernetes Secrets with ArgoCD labels)
	_, _ = fmt.Fprintln(o.Output, "\n🔑 Syncing Git Repository Credentials...")
	if len(cfg.GitOps.Credentials) == 0 {
		_, _ = fmt.Fprintln(o.Output, "   ℹ️  No credentials configured.")
	}
	for _, cred := range cfg.GitOps.Credentials {
		if cred.URL == "" && cred.Registry == "" {
			_, _ = fmt.Fprintln(o.Output, "   ⚠️  Skipping credential with no URL or Registry.")
			continue
		}

		// Helper to sync a single secret
		syncSecret := func(url, username, password, secretType string) {
			if url == "" {
				return
			}
			// Sync to ArgoCD namespace
			if err := o.applyRepositorySecret(kubeconfigPath, argocdNamespace, url, username, password, secretType); err != nil {
				msg := fmt.Sprintf("repo %q: %v", url, err)
				_, _ = fmt.Fprintf(o.Output, "   ❌ %s\n", msg)
				errs = append(errs, msg)
			} else {
				_, _ = fmt.Fprintf(o.Output, "   ✅ Synced %s credentials: %s (ArgoCD)\n", secretType, url)
			}
			// Sync to Kargo Project namespaces
			for _, pipe := range cfg.GitOps.Pipelines {
				projectNS := pipe.Project
				if projectNS == "" {
					projectNS = pipe.Namespace
				}
				if projectNS == "" {
					projectNS = "kargo"
				}
				if err := o.applyRepositorySecret(kubeconfigPath, projectNS, url, username, password, secretType); err != nil {
					_, _ = fmt.Fprintf(o.Output, "   ⚠️  Could not sync %s credentials to Kargo project %q: %v\n", secretType, projectNS, err)
				} else {
					_, _ = fmt.Fprintf(o.Output, "   ✅ Synced %s credentials: %s (Kargo: %s)\n", secretType, url, pipe.Name)
				}
			}
		}

		// 1. Sync Git credentials
		syncSecret(cred.URL, cred.Username, string(cred.Password), "git")

		// 2. Sync OCI/Helm credentials
		syncSecret(cred.Registry, cred.Username, string(cred.Password), "helm")
	}

	// 2. Sync Projects (as ArgoCD AppProjects)
	_, _ = fmt.Fprintln(o.Output, "\n📂 Syncing AppProjects...")
	if len(cfg.GitOps.Projects) == 0 {
		_, _ = fmt.Fprintln(o.Output, "   ℹ️  No projects configured.")
	}
	for _, proj := range cfg.GitOps.Projects {
		if proj.Name == "" {
			_, _ = fmt.Fprintln(o.Output, "   ⚠️  Skipping project with empty name.")
			continue
		}
		if err := o.applyAppProject(kubeconfigPath, argocdNamespace, proj); err != nil {
			msg := fmt.Sprintf("project %q: %v", proj.Name, err)
			_, _ = fmt.Fprintf(o.Output, "   ❌ %s\n", msg)
			errs = append(errs, msg)
		} else {
			_, _ = fmt.Fprintf(o.Output, "   ✅ Synced project: %s\n", proj.Name)
		}

		// 2.1 Sync ApplicationSet if project is managed
		if proj.ManagedEnv {
			_, _ = fmt.Fprintf(o.Output, "   🚀 Syncing ApplicationSet for project %s...\n", proj.Name)
			if err := o.applyApplicationSet(kubeconfigPath, argocdNamespace, proj); err != nil {
				msg := fmt.Sprintf("project %q application-set: %v", proj.Name, err)
				_, _ = fmt.Fprintf(o.Output, "   ❌ %s\n", msg)
				errs = append(errs, msg)
			} else {
				_, _ = fmt.Fprintf(o.Output, "   ✅ Synced ApplicationSet: %s-apps\n", proj.Name)
			}
		}
	}

	// 3. Sync Applications (as ArgoCD Applications)
	_, _ = fmt.Fprintln(o.Output, "\n⛵ Syncing Applications...")
	appCount := 0
	for _, proj := range cfg.GitOps.Projects {
		if proj.ManagedEnv {
			_, _ = fmt.Fprintf(o.Output, "   ℹ️  Skipping individual app sync for managed project %s (ApplicationSet handles it)\n", proj.Name)
			continue
		}
		for _, app := range proj.Apps {
			appCount++
			if err := o.validateApp(app); err != nil {
				msg := fmt.Sprintf("app %q: validation failed: %v", app.Name, err)
				_, _ = fmt.Fprintf(o.Output, "   ⚠️  Skipping %s — %v\n", app.Name, err)
				errs = append(errs, msg)
				continue
			}
			versionInfo := app.Branch
			if app.IsHelm() {
				versionInfo = app.ChartVersion
			}

			if err := o.applyApplication(kubeconfigPath, argocdNamespace, proj.Name, app); err != nil {
				msg := fmt.Sprintf("app %q: %v", app.Name, err)
				_, _ = fmt.Fprintf(o.Output, "   ❌ %s\n", msg)
				errs = append(errs, msg)
			} else {
				_, _ = fmt.Fprintf(o.Output, "   ✅ Synced app: %s (project: %s, version/branch: %s)\n",
					app.Name, proj.Name, versionInfo)
			}
		}
	}
	if appCount == 0 {
		_, _ = fmt.Fprintln(o.Output, "   ℹ️  No apps configured in any project.")
	}

	// 4. Sync Image Pull Secrets (Automatic Cluster-wide Registry Credentials)
	_, _ = fmt.Fprintln(o.Output, "\n🔑 Syncing Image Pull Secrets in all target namespaces...")
	pss := NewPullSecretsService()
	pss.Output = o.Output
	pss.DryRun = o.DryRun
	if err := pss.Sync(cfg); err != nil {
		msg := fmt.Sprintf("pull-secrets: %v", err)
		errs = append(errs, msg)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d error(s) during sync:\n  - %s", len(errs), strings.Join(errs, "\n  - "))
	}
	return nil
}

// validateApp checks that mandatory fields are present before applying.
// Helm apps require chart + chart_version + values_repo + values_branch.
// Kustomize apps require path + branch.
func (o *Orchestrator) validateApp(app config.GitOpsApp) error {
	if app.Name == "" {
		return fmt.Errorf("name is required")
	}
	if app.Repo == "" {
		return fmt.Errorf("repo is required")
	}
	if app.IsHelm() {
		if app.ChartVersion == "" {
			return fmt.Errorf("chart_version is required for Helm apps")
		}
		// Only require values repo/branch if a values file is specified
		if app.ValuesFile != "" {
			if app.ValuesRepo == "" || app.ValuesBranch == "" {
				return fmt.Errorf("values_repo and values_branch are required when values_file is specified")
			}
		}
	} else {
		if app.Branch == "" {
			return fmt.Errorf("branch is required")
		}
		if app.Path == "" {
			return fmt.Errorf("path is required")
		}
	}
	return nil
}

// applyRepositorySecret creates an ArgoCD and Kargo repository secret in the cluster.
func (o *Orchestrator) applyRepositorySecret(kubeconfig, ns, url, username, password, secretType string) error {
	name := repoSecretName(url)

	kargoCredType := "git"
	repoURL := url
	repoURLIsRegex := "false"

	switch secretType {
	case "helm":
		kargoCredType = "image" // Kargo uses 'image' for container registries, while ArgoCD uses 'helm'
		// Make it a regex to match any image under this registry domain
		repoURL = "^" + regexp.QuoteMeta(url) + "/.*$"
		repoURLIsRegex = "true"
	case "git":
		trimmedURL := strings.TrimSuffix(url, "/")
		if !strings.HasSuffix(trimmedURL, ".git") {
			// If the Git URL is an organization/user level URL instead of a specific .git repository,
			// make it a regex so Kargo can match any repository under this organization/user.
			repoURL = "^" + regexp.QuoteMeta(trimmedURL) + "(/.*)?$"
			repoURLIsRegex = "true"
		}
	}

	data := map[string]any{
		"Name":           name,
		"Namespace":      ns,
		"SecretType":     secretType,
		"KargoCredType":  kargoCredType,
		"URL":            url,
		"RepoURL":        repoURL,
		"RepoURLIsRegex": repoURLIsRegex,
		"Username":       username,
		"Password":       strings.TrimSpace(password),
	}

	yaml, err := o.executeTemplate(repositorySecretTemplate, data)
	if err != nil {
		return err
	}

	return o.kubectlApply(kubeconfig, yaml)
}

// applyAppProject creates or updates an ArgoCD AppProject.
func (o *Orchestrator) applyAppProject(kubeconfig, ns string, proj config.GitOpsProject) error {
	desc := proj.Description
	if desc == "" {
		desc = "Project managed by kuargogo"
	}

	data := map[string]any{
		"Name":        proj.Name,
		"Namespace":   ns,
		"Description": desc,
	}

	yaml, err := o.executeTemplate(appProjectTemplate, data)
	if err != nil {
		return err
	}

	return o.kubectlApply(kubeconfig, yaml)
}

// applyApplication creates or updates an ArgoCD Application.
func (o *Orchestrator) applyApplication(kubeconfig, ns, projectName string, app config.GitOpsApp) error {
	targetNs := app.Namespace
	if targetNs == "" {
		targetNs = "default"
	}

	data := map[string]any{
		"Name":            app.Name,
		"Namespace":       ns,
		"Project":         projectName,
		"TargetNamespace": targetNs,
		"IsHelm":          app.IsHelm(),
		"Repo":            app.Repo,
		"Branch":          app.Branch,
		"Path":            app.Path,
		"Chart":           app.Chart,
		"ChartVersion":    app.ChartVersion,
		"ValuesFile":      app.ValuesFile,
		"ValuesRepo":      app.ValuesRepo,
		"ValuesBranch":    app.ValuesBranch,
	}

	yaml, err := o.executeTemplate(applicationTemplate, data)
	if err != nil {
		return err
	}

	return o.kubectlApply(kubeconfig, yaml)
}

// applyApplicationSet creates or updates an ArgoCD ApplicationSet.
func (o *Orchestrator) applyApplicationSet(kubeconfig, ns string, proj config.GitOpsProject) error {
	data := map[string]any{
		"Name":      proj.Name,
		"Namespace": ns,
		"Repo":      proj.Repo,
	}

	yaml, err := o.executeTemplate(applicationSetTemplate, data)
	if err != nil {
		return err
	}

	return o.kubectlApply(kubeconfig, yaml)
}

func (o *Orchestrator) syncKargoResources(kubeconfig string, kargo config.KargoPipeline) error {
	projectNS := kargo.Project
	if projectNS == "" {
		projectNS = kargo.Namespace
	}
	if projectNS == "" {
		projectNS = "kargo"
	}

	// 0. Explicitly create the namespace with the Kargo label to avoid admission webhook conflicts
	nsYAML := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    kargo.akuity.io/project: "true"
`, projectNS)
	if err := o.kubectlApply(kubeconfig, nsYAML); err != nil {
		return err
	}

	// 1. Sync Kargo Project
	projectData := map[string]interface{}{
		"Name": projectNS,
	}
	projectYAML, err := o.executeTemplate(kargoProjectTemplate, projectData)
	if err != nil {
		return err
	}
	if err := o.kubectlApply(kubeconfig, projectYAML); err != nil {
		return err
	}

	// 2. Sync Kargo Warehouse
	var allImages []string
	if kargo.Warehouse.Repo != "" {
		allImages = append(allImages, kargo.Warehouse.Repo)
	}
	for _, img := range kargo.Warehouse.AdditionalImages {
		if img != "" {
			allImages = append(allImages, img)
		}
	}
	if len(allImages) == 0 {
		return fmt.Errorf("warehouse has no valid image repositories configured — set 'warehouse.repo' in kargo config (run: kgg kargo set --repo <image>)")
	}

	warehouseData := map[string]interface{}{
		"Name":                   kargo.Warehouse.Name,
		"Namespace":              projectNS,
		"AllImages":              allImages,
		"Semver":                 kargo.Warehouse.Semver,
		"ImageSelectionStrategy": kargo.Warehouse.ImageSelectionStrategy,
		"AllowTags":              kargo.Warehouse.AllowTags,
	}
	warehouseYAML, err := o.executeTemplate(kargoWarehouseTemplate, warehouseData)
	if err != nil {
		return err
	}
	if err := o.kubectlApply(kubeconfig, warehouseYAML); err != nil {
		return err
	}

	// 3. Sync Kargo Stages
	for _, stage := range kargo.Stages {
		repoPath := stage.Path
		// If stage path is not set, we don't default it to kargo.Warehouse.Path anymore
		// because Warehouse.Path is used as the Git URL.
		// It should ideally be set, but we will leave it as is if empty.

		stageData := map[string]interface{}{
			"Name":      stage.Name,
			"Project":   projectNS,
			"Warehouse": kargo.Warehouse.Name,
			"Upstream":  stage.Requires,
			"RepoURL":   kargo.Warehouse.Path, // User specified: Path is used as Git URL
			"RepoPath":  repoPath,
			"AllImages": allImages,
		}
		stageYAML, err := o.executeTemplate(kargoStageTemplate, stageData)
		if err != nil {
			return err
		}
		if err := o.kubectlApply(kubeconfig, stageYAML); err != nil {
			return err
		}
	}

	return nil
}

// executeTemplate parses and executes a Go template with the given data.
func (o *Orchestrator) executeTemplate(tmplStr string, data interface{}) (string, error) {
	funcMap := template.FuncMap{
		"yamlQuote": yamlQuote,
		"default": func(defVal, val interface{}) interface{} {
			if val == nil || val == "" {
				return defVal
			}
			return val
		},
	}

	tmpl, err := template.New("manifest").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// kubectlApply runs `kubectl apply -f -` with the given YAML manifest piped via stdin.
// It forwards kubectl's informational output (created/configured/unchanged) to o.Output.
func (o *Orchestrator) kubectlApply(kubeconfig, yaml string) error {
	if o.DryRun {
		_, _ = fmt.Fprintln(o.Output, "--- [DRY RUN] ---")
		_, _ = fmt.Fprintln(o.Output, yaml)
		_, _ = fmt.Fprintln(o.Output, "---")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Env = kubeconfigEnv(kubeconfig)
	cmd.Stdin = strings.NewReader(yaml)

	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err != nil {
		return fmt.Errorf("kubectl apply: %w\n%s", err, outStr)
	}
	// Forward kubectl's informational lines (e.g. "app/foo created") to the writer.
	if outStr != "" {
		_, _ = fmt.Fprintf(o.Output, "      → %s\n", outStr)
	}
	return nil
}

// repoSecretName derives a stable, DNS-safe Kubernetes secret name from a repo URL.
// Matches Ansible's logic: "repo-" + MD5(url) truncated to 15 chars.
func repoSecretName(url string) string {
	hash := fmt.Sprintf("%x", md5.Sum([]byte(url)))
	if len(hash) > 15 {
		hash = hash[:15]
	}
	return "repo-" + hash
}

// yamlQuote wraps a string in double quotes and escapes internal double quotes.
// Used to safely embed free-text fields into YAML manifests.
func yamlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// checkKubectl verifies that kubectl is available in the system PATH.
func checkKubectl() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found in PATH — install it from https://kubernetes.io/docs/tasks/tools/")
	}
	return nil
}
