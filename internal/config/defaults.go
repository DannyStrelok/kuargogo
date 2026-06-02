package config

import (
	"fmt"
)

// GetDefaultConfig returns a RootConfig with sample "default" content.
// It uses input parameters for dynamic values like Telegram credentials and context name.
func GetDefaultConfig(contextName, tgToken string, tgAdminID int) RootConfig {
	if contextName == "" {
		contextName = "kgg_cluster_id"
	}

	keyName := fmt.Sprintf("kgg_%s_id", contextName)

	return RootConfig{
		Version:        "v1",
		Lang:           "en",
		CurrentContext: contextName,
		Contexts: map[string]ClusterConfig{
			contextName: {
				Nodes: []Node{
					{
						Name:     "rpi-infra-mgr",
						IP:       "192.168.1.100",
						User:     "pi",
						Role:     "infra-manager",
						Arch:     "arm64",
						Position: "right",
						MAC:      "b8:27:eb:00:00:01",
					},
					{
						Name:     "hp-prodesk",
						IP:       "192.168.1.101",
						User:     "debian",
						Role:     "master",
						Arch:     "amd64",
						Position: "left",
						MAC:      "98:90:96:00:00:02",
					},
					{
						Name:     "lenovo-master-1",
						IP:       "192.168.1.102",
						User:     "debian",
						Role:     "master",
						Arch:     "amd64",
						Position: "center",
						MAC:      "00:23:24:00:00:03",
					},
				},
				Network: Network{
					SwitchIP: "192.168.0.1",
					User:     "admin",
					Password: Secret("admin"),
					Driver:   "tplink",
					APIPort:  80,
				},
				NetworkLayout: NetworkLayout{
					VLANs: map[string][]string{
						"vlan_default": {"port_1", "port_2", "port_3", "port_4", "port_5", "port_6", "port_7", "port_8"},
					},
					IGMPSnooping: true,
				},
				SSH: SSH{
					PrivateKeyPath: fmt.Sprintf("~/.ssh/%s", keyName),
					Port:           22,
				},
				MQTT: MQTT{
					Broker:      "tcp://192.168.1.100:1883",
					ClientID:    "kuargogo-admin",
					TopicPrefix: "kgg/homelab",
				},
				Discovery: DiscoveryConfig{
					Enabled:   true,
					Interface: "eth0",
				},
				K3s: K3s{
					Token:          Secret("replace-with-your-secret-token"),
					KubeconfigPath: "/etc/rancher/k3s/k3s.yaml",
					VIP:            "",
					HA:             true,
				},
				Telegram: Telegram{
					BotToken:         Secret(tgToken),
					AdminID:          tgAdminID,
					Timezone:         "Europe/Madrid",
					DailySummaryTime: "08:30",
				},
				Cloudflare: Cloudflare{
					Email:       "",
					APIToken:    "",
					AccountID:   "",
					TunnelToken: "",
					TunnelID:    "",
				},
				AI: AIConfig{
					Provider:      "ollama",
					Model:         "llama3",
					AnonymizeLogs: true,
				},
				NFS: NFS{
					Enabled: false,
					Server:  "192.168.1.5",
					Shares: []NFSShare{
						{Src: "/volume1/data", Dest: "/mnt/data", Opts: "rw,sync,no_subtree_check"},
					},
				},
				Ansible: Ansible{
					WSLDistro:         "Ubuntu",
					VaultPasswordFile: "~/.ssh/kgg_cluster_id",
				},
				MaintenanceMode: false,
				HardwareEnabled: false,
			},
		},
	}
}
