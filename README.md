# 🚀 Kuargogo (kgg)

<p align="center">
  <img width="700" height="596" alt="kuargogo" src="https://github.com/user-attachments/assets/6b83eb69-58ec-481d-869e-3ea20cde3662" /><br />
  <a href="https://github.com/DannyStrelok/kuargogo/actions"><img src="https://github.com/DannyStrelok/kuargogo/actions/workflows/ci.yml/badge.svg?branch=main" alt="Build Status"></a>
  <a href="https://github.com/DannyStrelok/kuargogo/releases"><img src="https://img.shields.io/github/v/release/DannyStrelok/kuargogo?include_prereleases" alt="Latest Release"></a>
  <a href="https://github.com/DannyStrelok/kuargogo"><img src="https://img.shields.io/github/go-mod/go-version/DannyStrelok/kuargogo" alt="Go Version"></a>
  <a href="https://www.gnu.org/licenses/agpl-3.0"><img src="https://img.shields.io/badge/License-AGPL_v3-blue.svg" alt="License: AGPL v3"></a>
</p>

**Kuargogo (`kgg`)** is a open-source command-center CLI and TUI designed for modern homelabs. Built in Go, Kuargogo bridges the gap between infrastructure management, container orchestration, hardware telemetry, and local AI capabilities. 

Unlike generic management tools, Kuargogo embraces a **Dual-Plane Architecture** that separates the lightweight, always-on **Control Plane** (run on a Raspberry Pi bastion) from the heavy **Data Plane** (your compute and K3s cluster). It provides automated provisioning, real-time alerting, local LLM integrations, and a sleek interactive Terminal User Interface (TUI) to control your entire rack.

---

## ✨ Key Features

*   **⚡ Zero-Touch Node Provisioning**: Automatically bootstrap, harden (SSH/UFW), and prepare clean Linux nodes for K3s deployment in seconds.
*   **🧠 Local AI Integration (Ollama)**: Diagnose system anomalies, query cluster status, and analyze logs using interactive, local artificial intelligence directly in your terminal.
*   **🤖 "The Voice" Telegram Orchestrator**: A secure Telegram bot bridge allowing you to check system health (`/status`), reboot nodes (`/power`), view live journal logs (`/logs`), and receive instant warning notifications.
*   **🛡️ Smart Self-Healing & Telemetry**: Auto-recovers frozen compute nodes via Wake-on-LAN (WoL) and monitors critical hardware temperatures and disk thresholds securely over MQTT.
*   **🎨 Slate-Sleek TUI & CLI**: A state-of-the-art terminal interface crafted with Bubble Tea and Huh, alongside a comprehensive, scriptable Cobra CLI.
*   **🔌 Playbook Overlay System**: Unmatched flexibility for open-source users. Easily extend, overlay, or override core Ansible automation files under `~/.kuargogo/playbooks/` without maintaining forks.

---

## 🏗️ System Architecture

Kuargogo organizes your homelab into two specialized layers to ensure maximum uptime, power efficiency, and security:

```
  ┌─────────────────────────────────────────────────────────────┐
  │                   ADMIN WORKSTATION (Laptop)                │
  │                  kgg CLI / Slate-Sleek TUI                  │
  └──────────────────────────────┬──────────────────────────────┘
                                 │ SSH / MQTT (Direct or WSL)
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ CONTROL PLANE: "The Brain" (Raspberry Pi Bastion Host)          │
│ 🔸 Always-On  🔸 low-power  🔸 MQTT Broker  🔸 Telegram Bot     │
│ 🔸 kgg-agent & kgg-telegram daemon                              │
└────────────────────────────────┬────────────────────────────────┘
                                 │
                 ┌───────────────┴───────────────┐
                 │ SSH / WoL / Telemetry         │
                 ▼                               ▼
┌─────────────────────────────────┐ ┌─────────────────────────────┐
│ DATA PLANE: K3s Master (HP)     │ │ DATA PLANE: Workers (Lenovo)│
│ 🔹 Heavy Compute & K3s Master   │ │ 🔹 K3s Workers & longhorn   │
│ 🔹 8TB Shared NFS Storage       │ │ 🔹 GPU AI Accelerators      │
└─────────────────────────────────┘ └─────────────────────────────┘
```

### 1. Control Plane (Infrastructure Manager)
A lightweight, low-power Raspberry Pi 3/4/5 acts as the orchestrator and central gateway.
*   **Active Monitoring**: Pings cluster nodes, queries K3s API, and monitors Longhorn storage status.
*   **Automated Recovery**: Detects offline nodes, fires Wake-on-LAN (WoL) packets, and manages smart reboots.
*   **Unified Bastion**: Serves as a single, hardened entry point for all remote cluster administration tasks.

### 2. Data Plane (Compute & Storage Node-Grid)
Your heavy-lifting nodes (e.g., HP ProDesk, Lenovo Mini-PCs) running K3s, storage engines, and local container workloads.
*   **Orchestration**: K3s Kubernetes cluster for elastic container delivery.
*   **Storage**: Dynamic local volumes managed via Longhorn and network-mounted NFS.
*   **AI Accelerator**: CUDA-enabled GPUs (e.g., Nvidia Tesla P4) hosting local AI services.

---

## 🚀 Installation & Getting Started

Kuargogo runs on your **Admin Workstation** (Windows, macOS, or Linux). It communicates with your remote nodes using standard, agentless SSH connections.

> [!IMPORTANT]
> **Windows Users**: While `kuargogo` is compiled as a native Windows binary (`kgg-cli.exe`), commands relying on Ansible (such as `prep`, `bootstrap`, `ops`, and `cluster`) execute through **WSL (Windows Subsystem for Linux) with Ubuntu** to ensure optimal performance. The CLI handles this transition transparently.

### 📦 Pre-Built Binaries (Recommended)

Get the latest release from the [GitHub Releases page](https://github.com/DannyStrelok/kuargogo/releases).

#### Linux (amd64 / Standard Servers)
```bash
VERSION=v0.1.0
curl -L "https://github.com/DannyStrelok/kuargogo/releases/download/${VERSION}/kuargogo_linux_amd64.tar.gz" | tar xz
sudo mv kgg /usr/local/bin/kgg
```

#### macOS (Apple Silicon M1/M2/M3)
```bash
VERSION=v0.1.0
curl -L "https://github.com/DannyStrelok/kuargogo/releases/download/${VERSION}/kuargogo_darwin_arm64.tar.gz" | tar xz
sudo mv kgg /usr/local/bin/kgg
```

#### Windows
Download the native installer `kgg-setup-<version>.exe` from the Releases page. It will automatically register the binary to your environment `PATH`.

---

## 🔑 Initial Configuration & SSH Setup

Before running cluster operations, you need to configure your environment and set up secure SSH keying:

1.  **Initialize Config**: Generate a default `kuargogo.yaml` configuration profile:
    ```bash
    kgg init
    ```
    *This will prompt an interactive terminal form to build your nodes configuration easily.*

2.  **Generate Cluster Keys**:
    ```bash
    kgg ssh-keygen
    ```
    *This generates standard cryptographic keys at `~/.ssh/kgg_cluster_id`.*

3.  **Distribute Keys**: Securely copy the keys to your target nodes:
    ```bash
    kgg ssh-copy --node 192.168.1.101 --user debian
    ```

---

## 📝 Example Configuration (`kuargogo.yaml`)

```yaml
nodes:
  - name: "rpi-infra-mgr"
    ip: "192.168.1.100"
    user: "kgg-admin"
    role: "infra-manager"
    arch: "arm64"
    position: "right"
    mac: "b8:27:eb:11:22:33"
  - name: "hp-master"
    ip: "192.168.1.101"
    user: "kgg-admin"
    role: "master"
    arch: "amd64"
    position: "left"
    mac: "98:90:96:aa:bb:cc"
  - name: "lenovo-worker-1"
    ip: "192.168.1.102"
    user: "kgg-admin"
    role: "worker"
    arch: "amd64"
    position: "center"

ssh:
  private_key_path: "~/.ssh/kgg_cluster_id"
  port: 22

mqtt:
  broker: "tcp://192.168.1.100:1883"
  client_id: "kuargogo-admin"
  topic_prefix: "kgg/homelab"

k3s:
  token: "your-super-secret-cluster-token"

telegram:
  bot_token: "123456789:AABBCC-YourBotTokenFromBotFather"
  admin_id: 123456789
  timezone: "Europe/Madrid"
```

---

## 🎨 Customizing Playbooks & Roles (Overlay System)

Kuargogo embeds production-ready Ansible automation right inside its binary. However, open-source customization is fully supported via the **Overlay System**:

At runtime, `kuargogo` automatically blends its internal playbooks with your local modifications located under:
📂 **`~/.kuargogo/playbooks/`**

### Overlay Mechanics
*   **Playbook Override**: Place a file at `~/.kuargogo/playbooks/provision.yml` to replace the default provisioning sequence with your custom tasks.
*   **Modular Enhancements**: Override individual role tasks, for example:
    `~/.kuargogo/playbooks/roles/k3s-prep/tasks/main.yml`
    This file takes precedence while other files within the role (such as default values or templates) are still seamlessly loaded from the embedded core.

---

## 💻 Core Command Set

Run `kgg help` to view all CLI flags.

*   **`kgg init`**: Launches a sleek, interactive terminal form to create or edit `kuargogo.yaml`.
*   **`kgg bootstrap`**: Automates full OS prep, installs standard tools, and configures SSH.
*   **`kgg prep`**: Provisions prerequisites, docker runtimes, GPU setup, and storage.
*   **`kgg cluster`**: Initializes or joins nodes to the K3s cluster.
*   **`kgg pwr [on|off|reboot]`**: Direct hardware power controls using WoL and IPMI.
*   **`kgg ai chat`**: Launches an interactive chat session with your local Ollama AI engine.
*   **`kgg doctor`**: Diagnoses rack nodes, parses metrics, and checks for hardware alerts.

---

## 🛠️ Development

We organize our Go source code following modern patterns:

```text
kuargogo/
├── cmd/
│   └── kgg/             # Cobra command handlers and CLI entrypoint
├── internal/
│   ├── config/          # Config structs, save/load & atomic file operations
│   ├── ansible/         # Embedded FS assets, runner, and metrics parsing
│   ├── provision/       # SSH executor, key replication & bootstrap logic
│   ├── cluster/         # K3s installation, joins, and cluster drains
│   ├── network/         # Hardware network maps and telemetry
│   ├── hardware/        # MQTT clients, topic configurations & WoL utilities
│   ├── ai/              # Local Ollama LLM integration & prompt templates
│   ├── notify/          # Telegram notification templates
│   └── ui/              # Bubble Tea interactive menus & wizard setup
├── infrastructure/
│   └── playbooks/       # Embedded Ansible roles and YAML playbooks
└── README.md
```

### Build from source
```bash
git clone https://github.com/DannyStrelok/kuargogo.git
cd kuargogo
go mod tidy
go build -o kgg.exe ./cmd/kgg
```

---

## 📚 Technical Documentation

Explore the comprehensive technical references and guides for Kuargogo:

*   🚀 **[Master Deployment Roadmap](docs/DEPLOYMENT_GUIDE.md)**: Phase-by-phase portal to bootstrap your entire homelab.
*   📖 **[Command Reference](docs/COMMANDS.md)**: Full syntax definitions and command examples for the CLI.
*   🧠 **[System Architecture](docs/ARCHITECTURE.md)**: Software layer charts, telemetry flows, and self-healing systems.
*   🗺️ **[Project Roadmap](docs/ROADMAP.md)**: Completed milestones and planned feature updates.
*   📦 **[Release Guide](docs/RELEASING.md)**: Step-by-step pipeline to tag and distribute new versions.

---

## 🛡️ License & Dual-Licensing

Distributed under the **GNU AGPLv3 License**. Created by [DannyStrelok](https://github.com/DannyStrelok).

For commercial usage, proprietary deployments, or dedicated enterprise support, please reach out to the author to acquire a **Commercial License**.

Feel free to open issues, submit pull requests, and customize Kuargogo for your own awesome homelab! 🚀
