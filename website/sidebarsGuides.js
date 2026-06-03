module.exports = {
  guidesSidebar: [
    {
      type: 'doc',
      id: 'workflow-roadmap',
      label: '🗺️ Workflow Roadmap',
    },
    {
      type: 'category',
      label: 'Phase 1: Hardware & OS',
      collapsed: false,
      items: [
        { type: 'doc', id: 'hardware-preparation', label: '🖥️ Hardware Preparation' },
        { type: 'doc', id: 'debian_install', label: '🐧 Debian Installation' },
      ],
    },
    {
      type: 'category',
      label: 'Phase 2: Bootstrap & Keying',
      collapsed: false,
      items: [
        { type: 'doc', id: 'provisioning', label: '🛠️ Node Provisioning' },
      ],
    },
    {
      type: 'category',
      label: 'Phase 3: Cluster Lifecycle',
      collapsed: false,
      items: [
        { type: 'doc', id: 'cluster-and-services', label: '☸️ Cluster & Storage' },
      ],
    },
    {
      type: 'category',
      label: 'Phase 4: Telemetry & Alerting',
      collapsed: false,
      items: [
        { type: 'doc', id: 'observability', label: '📊 Observability Stack' },
        { type: 'doc', id: 'telegram-setup', label: '🤖 Telegram Orchestrator' },
      ],
    },
    {
      type: 'category',
      label: 'Phase 5: GitOps & Ingress',
      collapsed: false,
      items: [
        { type: 'doc', id: 'gitops-and-secrets', label: '⛵ GitOps & Vault' },
        { type: 'doc', id: 'cloudflare-zero-trust', label: '🌐 Cloudflare Ingress' },
      ],
    },
    {
      type: 'category',
      label: 'Automation & Reference',
      collapsed: true,
      items: [
        { type: 'doc', id: 'automation-cicd', label: '🤖 CI/CD Automation' },
        { type: 'doc', id: 'cloud-sync-and-backup', label: '☁️ Cloud Sync & Backups' },
        { type: 'doc', id: 'new-project-guide', label: '🏗️ New Project Guide' },
      ],
    },
  ],
};
