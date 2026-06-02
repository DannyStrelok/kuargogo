# 📖 Kuargogo (`kgg`) - Command Reference

Welcome to the comprehensive command reference for **Kuargogo (`kgg`)**. This guide details every command, flag, and option available in the CLI and TUI, organized by functional area to help you orchestrate your homelab with precision.

---

## 🌍 Global Flags

These flags can be appended to any `kgg` command to modify its execution context:

| Flag | Description | Default |
| :--- | :--- | :--- |
| `--config <string>` | Absolute or relative path to your `kuargogo.yaml` profile. | `./kuargogo.yaml` or `~/.kuargogo/kuargogo.yaml` |
| `--dry-run` | **Simulation Mode**. Prints execution plans, shell commands, and playbooks without executing them. | `false` |
| `--vault-password-file <path>` | Path to your Ansible Vault password file for decrypting local files. | `""` (Overrides default config setting) |

---

## 📦 Node & Inventory Management

Manage node declarations, query telemetry, scan local networks, and run hardware diagnostics.

| Command | Description | Example Syntax |
| :--- | :--- | :--- |
| `kgg inventory` | Displays configured nodes and roles. Pass `-v` for a premium ASCII representation of your hardware rack. | `kgg inventory -v` |
| `kgg node list` | Lists all nodes declared in the active context, along with their roles and IP addresses. | `kgg node list` |
| `kgg node add` | Appends a new node definition to your active `kuargogo.yaml` file. | `kgg node add lenovo-3 192.168.1.104 worker` |
| `kgg node edit` | Interactive wizard to update a node's specs (IP, MAC address, hardware position, architecture). | `kgg node edit hp-master --mac 98:90:96:aa:bb:cc` |
| `kgg node remove` | Deletes a node definition from the configuration. Use `--force` to skip verification. | `kgg node remove lenovo-3 --force` |
| `kgg node status` | Fetches real-time compute telemetry (CPU, memory, disk usage, uptime). Uses Prometheus or falls back to SSH. | `kgg node status` |
| `kgg node status --json` | Outputs system status in structured JSON format (primarily consumed by the Telegram Bot API). | `kgg node status --json` |
| `kgg node scan` | **Read-only network scan**. Utilizes ARP/ping sweeps to discover active network devices. | `kgg node scan` |
| `kgg discover` | **Automated enrollment**. Scans your local network subnet and automatically adds newly discovered nodes to the config. | `kgg discover` |
| `kgg node health` | Checks critical host health metrics (NVMe SMART status, CPU core temperatures, system loads). | `kgg node health lenovo-worker-1` |
| `kgg node clean-host` | Removes a target node's legacy SSH key fingerprint from your workstation's `known_hosts` file. | `kgg node clean-host 192.168.1.102` |

---

## 🛠️ Provisioning & Setup

Bootstrap clean nodes, manage SSH credentials, and run base OS automation.

| Command | Description | Example Syntax |
| :--- | :--- | :--- |
| `kgg init` | Launches a sleek, interactive terminal questionnaire to build your `kuargogo.yaml` configuration. | `kgg init` |
| `kgg ssh-keygen` | Generates a standard cluster SSH keypair securely at `~/.ssh/kgg_cluster_id`. | `kgg ssh-keygen` |
| `kgg ssh-copy` | Distributes your cluster public key to a node using password authentication (utilizing TOFU for host keys). | `kgg ssh-copy --node 192.168.1.101 --user debian` |
| `kgg bootstrap` | **Automated Pipeline**. Runs keygen, ssh-copy (TOFU), verifies keys, and provisions a clean node in one command. | `kgg bootstrap --node 192.168.1.101 --user debian` |
| `kgg prep` | `[ANSIBLE]` Prepares host nodes for cluster integration (installs Docker/K3s dependencies, UFW rules). | `kgg prep --node lenovo-worker-1 --tags common,firewall` |
| `kgg setup-gpu` | `[ANSIBLE]` Provisions Nvidia drivers, CUDA, and the NVIDIA Container Toolkit on nodes containing the `gpu: nvidia` flag. | `kgg setup-gpu --node lenovo-worker-1` |
| `kgg mount-storage` | `[ANSIBLE]` Sets up secondary storage drives and automates persistent mount points in `/etc/fstab`. | `kgg mount-storage --node hp-master` |
| `kgg ssh` | Executes an arbitrary shell command on a target node using your secure cluster SSH key. | `kgg ssh lenovo-worker-1 "df -h"` |
| `kgg site` | `[ANSIBLE]` **Full Orchestration Deployment**. Builds the entire cluster stack (Provisioning -> GPU -> K3s -> Storage). | `kgg site --tags k3s,init` |

---

## ☸️ K3s Cluster Lifecycle

Deploy and orchestrate your Kubernetes K3s node grid.

| Command | Description | Example Syntax |
| :--- | :--- | :--- |
| `kgg cluster init` | `[ANSIBLE]` Configures the K3s control plane on the master node and extracts the secure cluster token. | `kgg cluster init --ha` |
| `kgg cluster join` | `[ANSIBLE]` Joins a compute node to the existing cluster (supports `--role worker` or `--role master`). | `kgg cluster join --node lenovo-worker-1` |
| `kgg cluster drain` | `[ANSIBLE]` Gracefully drains pods and schedules cordon mode on a worker node to prepare for physical maintenance. | `kgg cluster drain --node lenovo-worker-1` |
| `kgg cluster reset` | `[ANSIBLE]` Completely uninstalls K3s, wipes container runtime data, and cleans system iptables/interfaces. | `kgg cluster reset --node lenovo-worker-1` |

---

## 🎛️ Hardware Power & Bastion (Control Plane)

Control bare-metal hardware power states and RPi infrastructure services.

| Command | Description | Example Syntax |
| :--- | :--- | :--- |
| `kgg pwr` | Controls bare-metal nodes. Fires Wake-on-LAN (WoL) for `on`, and triggers secure ACPI shutdowns via SSH for `off`. | `kgg pwr off lenovo-worker-1` |
| `kgg infra init` | `[ANSIBLE]` Cross-compiles the Linux binaries and provisions the Raspberry Pi Infrastructure Manager. | `kgg infra init` |
| `kgg infra heartbeat` | Triggers a system-wide diagnostic check. Pass the `--ai` flag to feed raw diagnostics into a local LLM. | `kgg infra heartbeat --ai` |
| `kgg infra bot` | `[ANSIBLE]` **Hot Swap**. Deploys updated `kgg_telegram` daemon files and native binaries to the RPi bastion. | `kgg infra bot` |

---

## 🔐 Multi-Cluster & Security Vault

Manage multiple cluster profiles and encrypt sensitive fields at rest.

| Command | Description | Example Syntax |
| :--- | :--- | :--- |
| `kgg config get-contexts` | Lists all declared environment profiles defined in your workspace. | `kgg config get-contexts` |
| `kgg config current-context`| Prints the name of the currently active configuration profile. | `kgg config current-context` |
| `kgg config use-context` | Switches your active terminal context to another configured environment. | `kgg config use-context production-rack` |
| `kgg config set-context` | Creates or duplicates a context profile. | `kgg config set-context staging-rack` |
| `kgg config delete-context` | Safely removes a context profile completely from your local configuration. | `kgg config delete-context staging-rack` |
| `kgg config lint` | Validates your active `kuargogo.yaml` schema for potential syntax anomalies or missing properties. | `kgg config lint` |

### 🔒 Configuration Encryption (AES-256-GCM)

Kuargogo contains a native, military-grade configuration vault to secure tokens, S3 secret keys, and passwords inside `kuargogo.yaml`:

*   **Keychain Integration**: On Windows (Credential Manager), macOS (Keychain), and Linux (Secret Service), your master decryption key is securely stored in your OS's hardware-backed keyring.
*   **Headless Support**: For headless environments or CI/CD pipelines, you can set the `KGG_MASTER_PASSPHRASE` environment variable to automate decryption seamlessly.
*   **Automatic Encryption**: Open the `kgg` TUI, navigate to **🔐 Security & Vault**, and set your Master Passphrase. Sensitive config parameters will instantly be converted to `!vaultENC:<hash>` values.

---

## 🧠 AI & Application Stacks

Manage local LLM models and core infrastructure services.

| Command | Description | Example Syntax |
| :--- | :--- | :--- |
| `kgg ai status` | Queries Ollama server status and lists active models currently residing in GPU VRAM. | `kgg ai status` |
| `kgg ai pull` | Instructs the Ollama host node to fetch a new LLM model from the central registry. | `kgg ai pull llama3` |
| `kgg ai chat` | Launches an interactive, context-aware REPL terminal session with your local LLM engine. | `kgg ai chat -m mistral` |
| `kgg ai generate-skill` | Exports a structured `skill.md` playbook summarizing your cluster configurations for external AI agents. | `kgg ai generate-skill` |
| `kgg app deploy` | `[ANSIBLE]` Provisions predefined application charts and services (e.g. Immich, Home Assistant). | `kgg app deploy home-assistant` |
| `kgg app backup` | `[ANSIBLE]` Manually triggers an instant S3 cluster backup with status reports. | `kgg app backup` |
| `kgg storage` | `[ANSIBLE]` Checks dynamic volume status and manages Longhorn components. | `kgg storage status` |

---

## ⛵ GitOps & Kargo Multi-Stage Promotions

Declaratively manage GitOps repositories, application definitions, and multi-stage Kargo deployment pipelines.

| Command | Description | Example Syntax |
| :--- | :--- | :--- |
| `kgg gitops list` | Renders a tree view representing your declared repositories, projects, and synced GitOps applications. | `kgg gitops list` |
| `kgg gitops project add` | Adds a new project boundary (logical collection of related applications). | `kgg gitops project add backend --desc "Core services"` |
| `kgg gitops project remove` | Removes a project boundary and cordons its associated apps from the cluster. | `kgg gitops project remove backend` |
| `kgg gitops app add` | Declares a new ArgoCD application and appends its sync path/repository to the active context. | `kgg gitops app add backend auth-api https://git.local/apps path/k8s` |
| `kgg gitops app remove` | Removes a declared application from the configuration. | `kgg gitops app remove backend auth-api` |
| `kgg gitops repo add` | Saves private Git repository credentials (PAT/SSH) securely using vault-encryption. | `kgg gitops repo add https://git.local/apps.git ghp_MyToken` |
| `kgg gitops kargo init` | Provisions Kargo promotion pipelines on your cluster. | `kgg gitops kargo init` |
| `kgg gitops kargo sync` | Translates your local declarative pipelines into active cluster Kargo resources. | `kgg gitops kargo sync` |
| `kgg gitops kargo status` | Displays active pipeline metrics, deployed Freight IDs, and individual Stage readiness. | `kgg gitops kargo status --pipeline app-pipeline` |
| `kgg gitops kargo freight` | Queries the cluster to list all available, verified Freight packets for a pipeline. | `kgg gitops kargo freight --pipeline app-pipeline` |
| `kgg gitops kargo promote` | Triggers a manual stage promotion of a specific Freight ID to a target environment stage (e.g., test -> prod). | `kgg gitops kargo promote production auth-freight-1 --pipeline app-pipeline` |

---

## 🛡️ Dominios, Servicios & Cloudflare Automation

Configure local host exposures and enforce passwordless Cloudflare Zero Trust (Email OTP) policies.

*   **`kgg cloudflare sync`**: Deploys Zero Trust policies, tunnels, and Ingress parameters.
*   **`kgg ops cloudflare-tunnel`**: `[ANSIBLE]` Automatically registers and installs Cert-Manager and Cloudflare Zero Trust Tunnels.
*   **`kgg ops observability`**: `[ANSIBLE]` Deploys the LGTM observability stack and automatically links Grafana with secure Zero Trust protection if configured.

### 📄 Service Exposure Schema (`kuargogo.yaml`)

Configure local service exposure to subdomains through Cloudflare Zero Trust Tunnels securely:

```yaml
cloudflare:
  access_enabled: true
  access_emails:
    - admin@chefclandestino.es
  services:
    - name: "Grafana Monitoring"
      subdomain: "grafana"
      target: "http://kube-prometheus-stack-grafana.monitoring.svc.cluster.local:80"
      protected: true # Enforces Cloudflare Access OTP for admin emails
    - name: "ArgoCD UI"
      subdomain: "argocd"
      target: "http://argocd-server.argocd.svc.cluster.local:80"
      protected: false # Publicly accessible via Secure Tunnel (no OTP required)
```

---

## ⚙️ Administrative Operations

General maintenance commands, package managers, and binary updaters.

| Command | Description | Example Syntax |
| :--- | :--- | :--- |
| `kgg setup` | Inspects system packagers and automatically provisions workstation dependencies (Ansible, K9s, Helm). | `kgg setup` |
| `kgg ops update` | `[ANSIBLE]` Performs standard rolling updates (`apt update && apt upgrade`) across all compute nodes. | `kgg ops update` |
| `kgg ops nfs` | `[ANSIBLE]` Re-mounts and configures NFS shares from your NAS device. | `kgg ops nfs` |
| `kgg playbooks export` | Extracts all embedded Ansible playbooks/roles directly to `~/.kuargogo/playbooks/` for manual edits. | `kgg playbooks export --all` |
| `kgg ops backup-system` | `[ANSIBLE]` Installs and deploys Velero Disaster Recovery utilizing your S3 bucket configurations. | `kgg ops backup-system` |
| `kgg env` | Displays active binary metadata, resolved file paths, context caches, and OS diagnostics. | `kgg env` |
| `kgg update` | Queries the upstream repository and executes automated in-place upgrades of the `kgg` binary. | `kgg update` |

---

> [!TIP]
> All Ansible-backed commands (marked with `[ANSIBLE]`) support custom playbook tagging (e.g. `--tags "common,firewall"`) to accelerate operations by restricting task execution ranges.
