# 🗺️ Project Roadmap

This document outlines the development path for the **Kuargogo (`kuargogo`)** project, focusing on moving from the current state (v0.5.5) to a resilient `v1.0.0` release.

---

## 🎯 Current State (v0.5.5)
*The foundation is built.*
- [x] Cloud-Native Architecture (Infra Manager vs Data Plane).
- [x] Core CLI commands (`node`, `cluster`, `prep`, `ops`).
- [x] Fully functional TUI (BubbleTea) with captured output.
- [x] Ansible Playbook orchestration (Provision, K3s, Storage).
- [x] Infrastructure Manager Agent (RPi) for cluster health.


---

## 🚀 Upcoming Releases

### v0.6.0 - The "Observability & Alerting" Update
*Focus on knowing exactly what is happening in your homelab.*
- [x] **Cluster-Wide Observability**: Deployed multi-tenant LGTM stack (Prometheus, Loki, Grafana, Tempo) via Ansible.
- [x] **Multi-tenant Alerting**: Integrated AlertManager with `PrometheusRule` and `AlertmanagerConfig` CRDs for tenant isolation.
- [x] **Advanced Health Checks**: Integrated Prometheus PromQL API queries into `kgg node health` and TUI for real-time, low-overhead stats. 
- [x] **Daemon Improvements**: Add "Maintenance Mode" logic to `kgg-agent.py` to suppress WoL/Alert spam for explicitly disabled nodes.
- [x] **Network Map Enhancements**: Implemented TP-Link SG108E port-status web scraper; `kgg network map` now shows live Up/Down/Speed + device mapping.
- [x] **TUI Visuals**: Added `📊 Cluster Dashboard` with live sparkline bars (CPU, Memory, Disk) via Prometheus API and color-coded Unicode block chars.

### v0.7.0 - The "Autonomous Recovery" Update
*Focus on self-healing infrastructure, core diagnostics, and hardware pluggability.*
- [x] **K3s Node Remediation**: `kuargogo` daemon automatically drains and re-joins misbehaving worker nodes.
- [x] **Storage Healing**: Automated Longhorn replica rebuilding via API when a disk smart check fails.
- [ ] **AI Log Analysis & Ollama Fallbacks**:
  - Use local Ollama to parse failing Kubernetes pod logs and suggest SRE fixes via Telegram bot.
  - Implement fallback templates for lightweight models (Llama3-8B, Phi3) and local host proxying when a GPU is absent.
- [ ] **Windows & WSL Diagnostics**:
  - Enhance `kgg doctor` to validate active WSL distros, networks, and include a `--fix-wsl` command to repair blocked WSL network interfaces.
  - Add path checks and warning banners for missing `wslpath` or `wsl` commands.
- [ ] **Ecological Auto-Scaling**: Drains and powers off redundant compute nodes during low load periods; wakes them via Wake-on-LAN dynamically when workload demands rise.
- [ ] **Hardware Driver Abstraction (Pluggability)**:
  - Define generic `SwitchDriver` interface in `internal/network/` to support TP-Link, MikroTik, Ubiquiti, or `noop`/`simulated` drivers.
  - Allow flexible configuration of telemetry/alert prefixes instead of hardcoded `kgg/homelab` MQTT topics.
- [ ] **Enhanced Alerting**: Support for Webhooks and Discord in addition to Telegram.


### v0.8.0 - The "App Store & Progressive Delivery" Update
*Focus on application management, self-service backups, and network swappability.*
- [ ] **One-Click App Catalog**: Deploy templates (databases, media servers, proxy tools) to GitOps repositories via TUI or Telegram.
- [ ] **Autonomous Build & Deploy**: Add `kgg app deploy` to automatically build codebases, push to registries, and generate Kargo promotion Freights.
- [ ] **Database Backups Manager**: Visual snapshot list, scheduler, and one-click restores integrated in TUI/Bot via Velero.
- [ ] **Cloudflare / DNS Swappability**:
  - Allow disabling Cloudflare tunnels entirely, or swap them out for Tailscale Funnel, ngrok, or DuckDNS.
- [ ] **Proactive Resource Alerting**: Telegram daemon polls Prometheus metrics and pushes alerts with contextual remediation actions (e.g. prune caches, reboot).
- [ ] **GitOps for Bare-Metal (Drift Detection)**: Implement background checking of host states (Ansible check mode) to identify configuration drifts and allow one-click Telegram remediation.


### v0.9.0 - The "Hardening" Update
*Focus on testing, security, and multi-rack orchestration before v1.0.*
- [ ] **Full Integration Tests**: Setup Github Actions with a Mock SSH server to test `kgg bootstrap` and `kgg prep`.
- [ ] **RBAC for Telegram**: Granular permissions (e.g., allow User A to run `/status` but not `/reboot`).
- [ ] **Vault Integration**: Native integration with Hashicorp Vault or Ansible Vault for secret management (no plain-text tokens in `kuargogo.yaml`).
- [ ] **Sudo Hardening**: Restrict `kgg-agent` sudoers to only required commands (`journalctl`, `k3s`, `helm`, `reboot`) for improved host security.
- [ ] **Cluster SSH Alerts**: Implement Relay Mode (UDP) to monitor security events across all nodes from the Manager.
- [ ] **Multi-Rack Orchestration**: Support managing multiple distinct cluster contexts (`homelab`, `office`) from a single Telegram bot or TUI daemon securely.


### 🏆 v1.0.0 - General Availability
*The stable, production-ready release for the open-source community.*
- [ ] **Premium Visual Documentation**:
  - Create interactive GIFs demonstrating the Bubble Tea TUI/Huh forms.
  - Produce simple, clean architecture diagrams showing Bastion (RPi) and node compute separation.
- [ ] **GitHub Community Ready**: Add templates for issue reporting (`ISSUE_TEMPLATE`) and contributions (`PULL_REQUEST_TEMPLATE`).
- [ ] **Power Efficiency & Cost Dashboard**: Out-of-the-box Grafana dashboard correlating MQTT smart plug/switch power metrics (watts) with Kubernetes node resources (CPU/RAM/Network).
- [ ] Comprehensive User Guide and API references.
- [ ] Homebrew / APT packaging for easy installation.
- [ ] Zero known critical bugs.
- [ ] Code freeze on core architecture.

---
> **Legend**:
> [x] Completed
> [/] In Progress
> [ ] Planned
