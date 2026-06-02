# 🗺️ Project Roadmap

This document outlines the development path for the **Kuargogo (`kuargogo`)** project, focusing on moving from the current state (v0.5.5) to a professional, resilient `v1.0.0` release.

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
*Focus on knowing exactly what is happening in the rack.*
- [x] **Cluster-Wide Observability**: Deployed multi-tenant LGTM stack (Prometheus, Loki, Grafana, Tempo) via Ansible.
- [x] **Multi-tenant Alerting**: Integrated AlertManager with `PrometheusRule` and `AlertmanagerConfig` CRDs for tenant isolation.
- [x] **Advanced Health Checks**: Integrated Prometheus PromQL API queries into `kgg node health` and TUI for real-time, low-overhead stats. 
- [x] **Daemon Improvements**: Add "Maintenance Mode" logic to `kgg-agent.py` to suppress WoL/Alert spam for explicitly disabled nodes.
- [x] **Network Map Enhancements**: Implemented TP-Link SG108E port-status web scraper; `kgg network map` now shows live Up/Down/Speed + device mapping.
- [x] **TUI Visuals**: Added `📊 Cluster Dashboard` with live sparkline bars (CPU, Memory, Disk) via Prometheus API and color-coded Unicode block chars.

### v0.7.0 - The "Autonomous Recovery" Update
*Focus on self-healing infrastructure.*
- [ ] **K3s Node Remediation**: `kuargogo` daemon automatically drains and re-joins misbehaving worker nodes.
- [ ] **Storage Healing**: Automated Longhorn replica rebuilding via API when a disk smart check fails.
- [ ] **AI Log Analysis**: Use local Ollama to parse failing Kubernetes pod logs and suggest fixes via Telegram bot.

- [ ] **Open Source Readiness**: Move to multi-rack support and public contribution guidelines.
- [ ] **Enhanced Alerting**: Support for Webhooks and Discord in addition to Telegram.


### v0.9.0 - The "Hardening" Update
*Focus on testing, security, and polish before v1.0.*
- [ ] **Full Integration Tests**: Setup Github Actions with a Mock SSH server to test `kgg bootstrap` and `kgg prep`.
- [ ] **RBAC for Telegram**: Granular permissions (e.g., allow User A to run `/status` but not `/reboot`).
- [ ] **Vault Integration**: Native integration with Hashicorp Vault or Ansible Vault for secret management (no plain-text tokens in `kuargogo.yaml`).
- [ ] **Sudo Hardening**: Restrict `kgg-agent` sudoers to only required commands (`journalctl`, `k3s`, `helm`, `reboot`) for improved host security.
- [ ] **Cluster SSH Alerts**: Implement Relay Mode (UDP) to monitor security events across all nodes from the Manager.

### 🏆 v1.0.0 - General Availability
*The stable, production-ready release.*
- [ ] Comprehensive User Guide and API references.
- [ ] Homebrew / APT packaging for easy installation.
- [ ] Zero known critical bugs.
- [ ] Code freeze on core architecture.

---
> **Legend**:
> [x] Completed
> [/] In Progress
> [ ] Planned
