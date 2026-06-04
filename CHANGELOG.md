# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2026-04-15
### Added
- **Local Vault**: Field-level encryption for `kuargogo.yaml` using AES-256-GCM.
- **TUI Security Menu**: New interface to manage the Master Passphrase and view the Cluster Salt.
- **OS Keyring Integration**: Support for native secure storage (Keychain, Credential Manager, Secret Service).
- **Secure Ansible Variable Passing**: extra-vars are now passed via temporary JSON files to hide secrets from the OS process list (`ps aux`).
- **Distributed Master Key Propagation**: `KGG_MASTER_PASSPHRASE` is automatically injected into remote nodes during Ansible deployments and configuration syncs.

### Added
- **Cloudflare Zero Trust Automation**: New generic multi-service management system enabling automated exposure of internal apps via Tunnels and Zero Trust Access (Email OTP).
- **Service Management TUI**: New "**🌐 Dominios & Servicios**" main menu node providing full CRUD (Add, Edit, Delete) capabilities for Cloudflare subdomains.
- **Cloudflare Sync**: New `kgg cloudflare sync` command to orchestrate the entire Zero Trust infrastructure (Tunnel Ingress, DNS, SSL, and Access Policies) in one pass.
- **Automated Observability Hardening**: Integrated 24-character random password generation and automated Zero Trust provisioning during the observability stack deployment (`kgg ops observability`).
- **Remote Commands**: New `kgg ssh <node> <command...>` to execute shell commands on nodes using cluster logic.
- **Display Control**: New `kgg screen [on|off]` placeholder command for physical display interaction.
- **GitOps Management**: Complete `kgg gitops` command suite to manage projects, applications, and private repository credentials from both CLI and TUI.
- **Private Repository Support**: Integrated support for Personal Access Tokens (PAT) for private Git repositories, enabling secure ArgoCD deployments of projects.
- **Sealed Secrets**: Automatic deployment and native integration with **Bitnami Sealed Secrets** for end-to-end encrypted secret management within GitOps workflows.
- **Bulk Power Control (TUI)**: New multi-select TUI menu for managing power states (Wake-on-LAN, Reboot, Shutdown) across multiple cluster nodes simultaneously.
- **Documentation**: New comprehensive guide `internal/help/docs/06-gitops-and-secrets.md` explaining the full GitOps and Secrets workflow.
- **Specialized Update**: New `kgg infra bot` command (and TUI button) for fast-tracking Telegram Bot and `kuargogo` binary updates on the Infrastructure Manager.
- **AI-Native Integration**: Refactored AI client to be provider-agnostic, supporting local-first (Ollama) and cloud services (OpenAI, Anthropic, Gemini).
- **Proactive AI Heartbeat**: New cluster-wide diagnostic scan (`kgg infra heartbeat --ai`) with automated SRE analysis, repair suggestions, and log anonymization for privacy.
- **Agent Skill Generation**: New `kgg ai generate-skill` command to produce `skill.md`, enabling external AI agents to understand and interact with the cluster securely.
- **AI TUI Console**: Integrated AI diagnostics and provider configuration directly into the TUI menu.
- **Natural Language Interface (NLI)**: New `kgg ai interpret` command that classifies human intent into structured JSON, acting as the "Brain" for external integrations.
- **Standalone Telegram Bridge**: Re-architected the bot as a dedicated `kgg-telegram` service ("The Voice"). It features an asynchronous, AI-driven interface with intent interpretation.
- **Secure Challenge-Response**: Implemented a mandatory confirmation flow with 60-second timeouts for all destructive Telegram actions (Reboot, Shutdown).
- **Proactive Health Monitoring**: Automated daily cluster status reports delivered via Telegram at a user-defined time, with full timezone support (`pytz`).
- **NFS Storage Management**: Comprehensive TUI and CLI support for managing NFS shares. Includes automatic provisioning and synchronization via Ansible.
- **Enhanced Configuration TUI**: New dedicated configuration sub-menus for Telegram (Timezone, Summary Time), NFS (Server, Enabled), Discovery (Network Interface), and Global Maintenance Mode.
- **Validation Engine**: Expanded configuration validator to ensure consistency across all new parameters (IPs, time formats, required interfaces).
- **Ansible Engine**: Implemented a "Playbook Overlay" system. Users can now override or extend embedded Ansible playbooks and roles by placing custom files in `~/.kuargogo/playbooks/`. These files are merged with the core playbooks at runtime, allowing for deep customization without modifying the CLI source code.

### Changed
- **Architecture (Cloudflare)**: Refactored Cloudflare logic to use a declarative model in `kuargogo.yaml`, removing hard-coded service associations and enabling generic service syncing.
- **SDK Compatibility**: Upgraded `cloudflare-go` integration to `v0.116.0` using the new `Params` struct pattern for all API calls.
- **Architecture (Infra Agent)**: Migrated `kgg-telegram` ("The Voice") and `kgg-agent` ("The Brain") from templated `.j2` files to pure Python scripts for improved IDE support and maintainability.
- **Unified Logic**: Telegram bot and Infrastructure Agent refactored to use the `kuargogo` binary for all node, power, and hardware operations (centralized logic).
- **Architecture (GitOps)**: Refactored GitOps logic into a dedicated `internal/gitops` package, strictly separating business logic (Manager) from CLI/TUI layers for better maintainability (SoC).
- **Power Logic Refactor**: Deduplicated power control implementation into a core execution engine shared by both single and bulk TUI actions.
- **Bot Logic**: Telegram Bot menu expanded with "Cluster Doctor", "AI Models Status", and "Site Deploy (Dry Run)" diagnostic actions.
- **Config Strategy**: `kuargogo` binary now supports `/etc/kuargogo.yaml` and `$HOME/kuargogo.yaml` as fallback configuration search paths.
- **Architecture**: Refactored provisioning roles to separate "Universal OS Base" (`common`) from "Kubernetes Prep" (`k3s-prep`).
- **Provisioning (Infra-Manager)**: Strengthened Raspberry Pi 3 provisioning by adding `common` and `unattended-upgrades` roles to `infra-init.yml`.
- **Provisioning (HA)**: Moved Etcd performance tuning, disk latency checks, and K3s-specific firewall rules to the new `k3s-prep` role.
- **Node Discovery**: Optimized mDNS scanner to run service queries in parallel, significantly improving reliability and speed.
- **Node Discovery**: Added documentation for configuring Avahi `ssh.service` on Debian nodes for auto-discovery.

### Fixed
- **API Types**: Resolved type mismatches in Zero Trust Access Application and Policy creation caused by SDK version discrepancies in `cloudflare-go`.
- **Systemd Serialization**: Corrected JSON environment variable nesting in systemd services by using compact separators to prevent parsing errors.
- **SSH Connectivity**: Hardened SSH host key verification in the runner for improved cross-platform reliability.
- **GPU Role**: Fixed a bug where the NVIDIA Container Toolkit repository architecture `$(ARCH)` was not expanding during provisioning.
- **K3s Role**: Fixed a missing `--tls-san` flag when joining additional server nodes in HA mode, preventing certificate validation errors.
- **Firewall**: Corrected a misleading comment for port 10250 (Kubelet API).
- **NFS**: Added a skip condition to the NFS playbook to prevent failures on clusters without a configured NFS server.
- **CLI**: Updated `node add` to default to `kgg-admin` as the SSH user instead of `pi`.
- **Telegram Bot**: Fixed SSH key distribution to the Raspberry Pi manager, ensuring it can perform remote shutdowns.
- **Telegram Bot**: Hardened remote shutdown/reboot commands using absolute paths and improved error reporting.
- **Infra Manager**: Fixed a critical bug in `kgg infra init` where the RPi's own access key was being copied as the cluster private key. Now the actual cluster key source is resolved and distributed correctly.
- **Telegram Bot**: Hardened remote power management commands (`shutdown`, `reboot`) to use standardized `systemctl` calls and improved non-interactive `sudo -n` usage.
- **Telegram Bot**: Improved error reporting for remote SSH commands to distinguish between authentication failures and permission issues.
- **Setup**: Added `sudo` to the common dependencies list in the `common` role to ensure remote management works on minimal base images.
- **Setup**: Improved Windows installation robustness. Added explicit PATH refresh check after Python installation and automatic detection of the Python `Scripts` folder to help the user configure environment variables.
- **Node Discovery**: Fixed a race condition in the parallel mDNS scanner that caused it to miss responses during short timeouts.
- **TUI Dashboard**: Added `📊 Cluster Dashboard` to TUI with live sparkline bars (CPU, Memory, Disk) for all K3s nodes, powered by Prometheus API. Uses Unicode block characters with color-coded severity (green/yellow/red).
- **Network Map**: Implemented TP-Link SG108E port-status scraper (`GetStatus`) for real-time Up/Down/Speed detection via HTTP. Enhanced `kgg network map` CLI display with proper state icons and unicode table formatting.
- **Health Diagnostics**: Upgraded `kgg node health` and TUI checks to use a unified Prometheus API client (via SSH bridging) for instant PromQL-based metrics (CPU, RAM, Disk) instead of slow individual Bash scripts.
- **Observability**: Added Grafana Tempo (Distributed Tracing) to the central `kgg ops observability` stack.
- **Observability**: Enabled centralized, multi-tenant alerting architecture via `PrometheusRule` and `AlertmanagerConfig` CRDs.
- **Documentation**: Developed comprehensive integration guide (`OBSERVABILITY.md`) for tenant projects.
- **Node Management**: New `kgg node edit` command to modify node attributes interactively or via flags.
- **Node Management**: New `kgg node remove` command to delete nodes from configuration.
- **TUI Integration**: Added `Edit Node` and `Remove Node` to the Node Management TUI menu.
- **Node Discovery**: New `kgg discover` command to automatically find nodes via mDNS.
- **LLDP Support**: Real LLDP discovery implementation for Linux using `mdlayher/packet` (Stub for other OS).
- **Documentation**: New `COMMANDS.md` for detailed CLI reference.
- **Network Management**: New `kgg network` command suite (`status`, `apply`, `validate`, `reboot`).
- **Declarative Network**: Define VLANs and port layout in `kuargogo.yaml`.
- **Switch Drivers**: Added `Simulated` driver for testing and scaffold for `TP-Link SG108E`.
- **Documentation**: New `HARDWARE.md` for detailed specs, BOM, and pinouts.
- **Documentation**: New `CONTRIBUTING.md` guide for developers.
- **FAQ**: Added Troubleshooting/FAQ section to `README.md`.
- **TUI Integration**: Added `kgg infra` commands (`init`, `deploy-esp32`) to the TUI menu under Hardware Control.
- **AI Guidelines**: Created `AGENTS.md` to ensure architectural and TUI-safe consistency in future AI-assisted development.
- **Architecture**: Added Mermaid system diagram to `ARCHITECTURE.md`.
- **K9s Integration**: New `kgg console` command to launch K9s with auto-configured kubeconfig (`[K9S]` dependency).
- **Ansible Engine**: New `kgg ops` command suite (`update`, `nfs`) for Ansible-powered operations across all nodes (`[ANSIBLE]` dependency).
- **Telegram Notifications**: New `--notify` flag on ops commands to send Telegram alerts on completion.
- **Health Diagnostics**: New `kgg doctor` command to collect and display cluster-wide metrics in a table (`[ANSIBLE]` dependency).
- **Dependency Checker**: New `internal/deps` package for verifying external tool availability.
- **Dynamic Inventory**: Ansible inventory generator from `kuargogo.yaml` node configuration.
- **Bootstrap Command**: New `kgg bootstrap` command to perform full node SSH setup (keygen + ssh-copy + verify + provision) in one step.
- **TOFU SSH**: `NewPasswordExecutor` now auto-accepts and persists unknown host keys on first connection (Trust On First Use).
- **Auto Keygen**: `kgg ssh-copy` auto-generates cluster key if missing (CLI and TUI).
- **Pre-flight SSH Check**: `kgg prep` verifies SSH key access before launching Ansible playbooks.
- **TUI Quick Bootstrap**: New "Quick Bootstrap" entry in Node Management menu.
- **TUI Input Validation**: IP address and required field validation in SSH Copy and Add Node forms.

### Changed
- **Ansible Engine (Windows)**: Re-architected Ansible execution on Windows. Instead of native Python/pip, the CLI now transparently proxies all `ansible-playbook` calls through **WSL (Ubuntu)**. This resolves long-standing compatibility issues with Ansible on Windows while maintaining a native TUI/CLI experience.
- **Path Translation**: Implemented automatic Windows-to-WSL path translation (`C:\...` -> `/mnt/c/...`) for dynamic inventories and playbooks.
- **Dependency Installer**: Updated `kgg setup` to automatically detect or install WSL Ubuntu and provision Ansible within the Linux environment.
- **Storage Backend**: Migrated Longhorn installation (`kgg storage init`) from raw SSH execution to declarative Ansible playbooks.
- **Application Backend**: Migrated `kgg app deploy` and `kgg app backup` from raw SSH `kubectl` execution to native `kubernetes.core.k8s` Ansible playbooks.
- **Infrastructure Backend**: Migrated `kgg infra init` (Raspberry Pi setup) from multi-line strings to the `infra-agent` Ansible role.
- **Documentation**: Refactored `BUILD_RACK.md` to focus solely on assembly instructions (specs moved to `HARDWARE.md`).
- **Documentation**: Merged "Getting Started" and "Installation" sections in `README.md` for better flow.
- **Config**: Explicitly documented `mqtt.topic_prefix` in `README.md` configuration example.

### Added
- **High Availability**: Added support for K3s HA mode with embedded etcd enabled by default.
- **High Availability**: Added `kube-vip` static pod manifest generation to provide an HA API Server endpoint.
- **Database**: Added CloudNativePG operator Ansible role for deploying PostgreSQL HA clusters backed by Longhorn.

### Fixed
- **LLDP**: Fixed Linux implementation to correctly parse TLVs and use raw sockets.

## [v0.0.1] - 20/01/2026
### Added
- Initial release of Kuargogo CLI.
- Basic node management (`add`, `list`, `health`).
- Provisioning scripts for Debian/K3s.
- Hardware control (Fans, LEDs, Power).
