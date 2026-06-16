package models

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/huh/v2"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/cluster"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/help"
	"github.com/DannyStrelok/kuargogo/internal/i18n"
	"github.com/DannyStrelok/kuargogo/internal/provision"
	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
	"github.com/DannyStrelok/kuargogo/internal/ui/menu/actions"
)

// ============================================================================
// Menu Tree Definition
// ============================================================================

func BuildMainMenu() MenuNode {
	return MenuNode{
		Title: i18n.T("menu_main"),
		DynamicChildren: func() []MenuNode {
			return []MenuNode{
				// ── Instant access to the three most-used "live cluster" views ──
				{
					Title:       "🚀 " + i18n.T("menu_k9s"),
					Description: i18n.T("menu_k9s_desc"),
					Action: func() tea.Cmd {
						if cmd := actions.LaunchK9s(); cmd != nil {
							return cmd
						}
						return engine.Push(NewK9sHandoverModel())
					},
				},
				{
					Title:       "📊 " + i18n.T("menu_metrics"),
					Description: i18n.T("menu_metrics_desc"),
					Action: func() tea.Cmd {
						return engine.Push(NewDashboardModel(actions.ClusterDashboard()))
					},
				},
				{
					Title:       "🩺 " + i18n.T("menu_health_audit"),
					Description: i18n.T("menu_health_audit_desc"),
					Action: func() tea.Cmd {
						return engine.Push(NewOutputModel(actions.Doctor()))
					},
				},
				// ── Core sections ──
				buildHardwareAndNodesNode(),
				buildClusterAndDeploymentNode(),
				buildGitOpsAndPlatformServicesNode(),
				buildDisasterRecoveryNode(),
				buildNetworkAndIntegrationsNode(),
				buildAppAndAIEcosystemNode(),
				buildDiagnosticsAndMaintenanceNode(),
				buildSecurityVaultNode(),
				buildSettingsAndSupportNode(),
			}
		},
	}
}

// ============================================================================
// Menu Section Builders
// ============================================================================

func buildHardwareAndNodesNode() MenuNode {
	return MenuNode{
		Title:       "🖥️ " + i18n.T("menu_nodes"),
		Description: i18n.T("menu_nodes_desc"),
		Children: []MenuNode{
			buildInventoryNode(),
			{
				Title:       "🔍 Discover & Auto-Add",
				Description: "Auto-register nodes via mDNS",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.DiscoverAndAdd()))
				},
			},
			{
				Title:       "🩺 Health Check",
				Description: "Run diagnostics on a specific node",
				DynamicChildren: func() []MenuNode {
					return createNodeSelector("Run Health Check", func(n config.Node) func() tea.Cmd {
						return func() tea.Cmd {
							return engine.Push(NewOutputModel(actions.HealthCheck(n)))
						}
					})
				},
			},
			{
				Title:       "Add Node",
				Description: "Manually register a device",
				Action: func() tea.Cmd {
					var node config.Node
					node.User = "root" // Default
					var labelsStr string

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().Title("Node Name").Description("e.g. worker-01").Value(&node.Name).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("name is required")
									}
									return nil
								}),
							huh.NewInput().Title("IP Address").Description("e.g. 192.168.1.50").Value(&node.IP).
								Validate(func(s string) error {
									if net.ParseIP(strings.TrimSpace(s)) == nil {
										return errors.New("invalid IP address")
									}
									return nil
								}),
							huh.NewInput().Title("SSH User").Value(&node.User),
							huh.NewSelect[string]().
								Title("Role").
								Options(
									huh.NewOption("Worker", "worker"),
									huh.NewOption("Master", "master"),
									huh.NewOption("Control Plane", "control-plane"),
									huh.NewOption("Storage", "storage"),
									huh.NewOption("Infra Manager", "infra-manager"),
								).
								Value(&node.Role),
							huh.NewSelect[string]().
								Title("Architecture").
								Options(
									huh.NewOption("amd64", "amd64"),
									huh.NewOption("arm64", "arm64"),
								).
								Value(&node.Arch),
							huh.NewInput().Title("Position").Value(&node.Position),
							huh.NewInput().Title("MAC").Value(&node.MAC),
							huh.NewInput().Title("Labels").Value(&labelsStr),
							huh.NewConfirm().Title("Maintenance").Value(&node.Maintenance),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if labelsStr != "" {
							node.Labels = make(map[string]string)
							for _, l := range strings.Split(labelsStr, ",") {
								parts := strings.SplitN(strings.TrimSpace(l), "=", 2)
								if len(parts) == 2 {
									node.Labels[parts[0]] = parts[1]
								}
							}
						}
						return actions.NodeAdd(node)
					}))
				},
			},
			{
				Title:       "✏️  Edit Node",
				Description: "Change attributes (MAC, role, labels...)",
				DynamicChildren: func() []MenuNode {
					return createNodeSelector("Edit Node", func(n config.Node) func() tea.Cmd {
						return func() tea.Cmd {
							node := n
							originalName := node.Name
							labelsStr := ""
							for k, v := range node.Labels {
								if labelsStr != "" {
									labelsStr += ","
								}
								labelsStr += k + "=" + v
							}
							node.Role = strings.TrimSpace(node.Role)
							node.Arch = strings.TrimSpace(node.Arch)

							f := huh.NewForm(
								huh.NewGroup(
									huh.NewInput().Title("Name").Value(&node.Name).
										Validate(func(s string) error {
											if strings.TrimSpace(s) == "" {
												return errors.New("name is required")
											}
											return nil
										}),
									huh.NewInput().Title("IP").Value(&node.IP).
										Validate(func(s string) error {
											if net.ParseIP(strings.TrimSpace(s)) == nil {
												return errors.New("invalid IP address")
											}
											return nil
										}),
									huh.NewInput().Title("User").Value(&node.User),
									huh.NewSelect[string]().Title("Role").
										Options(
											huh.NewOption("Worker", "worker"),
											huh.NewOption("Master", "master"),
											huh.NewOption("Control Plane", "control-plane"),
											huh.NewOption("Storage", "storage"),
											huh.NewOption("Infra Manager", "infra-manager"),
										).Value(&node.Role),
									huh.NewSelect[string]().Title("Architecture").
										Options(
											huh.NewOption("(not set)", ""),
											huh.NewOption("amd64", "amd64"),
											huh.NewOption("arm64", "arm64"),
										).Value(&node.Arch),
									huh.NewInput().Title("Position").Description("left, center, right").Value(&node.Position),
									huh.NewInput().Title("MAC Address").Description("For Wake-on-LAN").Value(&node.MAC),
									huh.NewInput().Title("Labels").Description("gpu=nvidia,storage=ssd").Value(&labelsStr),
									huh.NewConfirm().Title("Maintenance Mode").Value(&node.Maintenance),
								),
							)
							return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
								node.Labels = make(map[string]string)
								if labelsStr != "" {
									for _, l := range strings.Split(labelsStr, ",") {
										parts := strings.SplitN(strings.TrimSpace(l), "=", 2)
										if len(parts) == 2 {
											node.Labels[parts[0]] = parts[1]
										}
									}
								}
								return actions.NodeEdit(originalName, node)
							}))
						}
					})
				},
			},
			{
				Title:       "🗑️  Remove Node",
				Description: "Delete a node from configuration",
				DynamicChildren: func() []MenuNode {
					return createNodeSelector("Remove Node", func(n config.Node) func() tea.Cmd {
						return func() tea.Cmd {
							node := n
							var confirm bool
							f := huh.NewForm(
								huh.NewGroup(
									huh.NewConfirm().
										Title(fmt.Sprintf("Remove '%s' (%s)?", node.Name, node.IP)).
										Description("This will remove the node from your configuration.").
										Value(&confirm),
								),
							)
							return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
								if !confirm {
									return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled."} }
								}
								return actions.NodeRemove(node.Name)
							}))
						}
					})
				},
			},
			{
				Title:       "⚡ Power Control",
				Description: "Bulk Reboot, Shutdown or WoL",
				Action: func() tea.Cmd {
					cfg := config.GetConfig()
					if len(cfg.Nodes) == 0 {
						return func() tea.Msg { return actions.ResultMsg{Output: "❌ Error: No nodes configured in inventory."} }
					}

					var selectedNodeNames []string
					var actionStr string
					var confirm bool

					var nodeOptions []huh.Option[string]
					for _, n := range cfg.Nodes {
						nodeOptions = append(nodeOptions, huh.NewOption(n.Name, n.Name))
					}

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewMultiSelect[string]().
								Title("Select Nodes").
								Filterable(true).
								Height(6).
								Description("Use space to select, enter to continue").
								Options(nodeOptions...).
								Value(&selectedNodeNames).
								Validate(func(s []string) error {
									if len(s) == 0 {
										return errors.New("please select at least one node")
									}
									return nil
								}),
							huh.NewSelect[string]().
								Title("Action").
								Options(
									huh.NewOption("Power On (Wake-on-LAN)", "on"),
									huh.NewOption("Reboot", string(provision.PowerReboot)),
									huh.NewOption("Shutdown (Power Off)", string(provision.PowerOff)),
								).
								Value(&actionStr),
							huh.NewConfirm().
								Title("Confirm execution?").
								Value(&confirm),
						),
					)

					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if !confirm {
							return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled."} }
						}
						var targetNodes []config.Node
						for _, name := range selectedNodeNames {
							for _, n := range cfg.Nodes {
								if n.Name == name {
									targetNodes = append(targetNodes, n)
									break
								}
							}
						}
						if actionStr == "on" {
							return actions.BulkPowerOnAction(targetNodes)
						}
						return actions.BulkPowerControl(targetNodes, provision.PowerAction(actionStr))
					}))
				},
			},
			buildSSHManagementNode(),
		},
	}
}

func buildClusterAndDeploymentNode() MenuNode {
	return MenuNode{
		Title:       "🏗️ " + i18n.T("menu_cluster"),
		Description: i18n.T("menu_cluster_desc"),
		Children: []MenuNode{
			{
				Title:       "Quick Bootstrap",
				Description: "Initial setup: Network -> SSH Keys -> Provision",
				Action: func() tea.Cmd {
					var nodeName, dhcpIP, user, password string
					var createUser = true
					var skipProvision = false
					user = "debian"
					keyPath, _ := config.ResolveKeyPath("")
					cfg := config.GetConfig()
					port := cfg.SSH.Port
					if port == 0 {
						port = 22
					}

					// Prepare node options from config
					var nodeOptions []huh.Option[string]
					for _, n := range cfg.Nodes {
						nodeOptions = append(nodeOptions, huh.NewOption(fmt.Sprintf("%s (%s)", n.Name, n.IP), n.Name))
					}

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewSelect[string]().
								Title("Select Node to Bootstrap").
								Description("Choose a node from your current configuration").
								Options(nodeOptions...).
								Value(&nodeName).
								Validate(func(s string) error {
									if s == "" {
										return errors.New("node selection is required")
									}
									return nil
								}),
							huh.NewInput().Title("Current DHCP IP (optional)").
								Description("Leave empty if node is already at its final IP").
								Value(&dhcpIP).
								Validate(func(s string) error {
									if s != "" && net.ParseIP(strings.TrimSpace(s)) == nil {
										return errors.New("invalid IP address (or leave empty)")
									}
									return nil
								}),
							huh.NewInput().Title("Initial User").Value(&user),
							huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&password).
								Validate(func(s string) error {
									if s == "" {
										return errors.New("password is required")
									}
									return nil
								}),
							huh.NewConfirm().Title("Create admin user?").Description("Creates 'kgg-admin' with sudo privileges").Value(&createUser),
							huh.NewConfirm().Title("Skip provisioning?").Description("Only perform SSH key setup").Value(&skipProvision),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						// Resolve static IP from config
						staticIP := ""
						cfg := config.GetConfig()
						for _, n := range cfg.Nodes {
							if n.Name == nodeName {
								staticIP = n.IP
								break
							}
						}
						return actions.BootstrapNode(nodeName, dhcpIP, staticIP, user, password, keyPath, port, createUser, skipProvision)
					}))
				},
			},
			{
				Title:       "Provision Node",
				Description: "Install base dependencies via Ansible",
				DynamicChildren: func() []MenuNode {
					return createNodeSelector("Provision Node", func(n config.Node) func() tea.Cmd {
						return func() tea.Cmd {
							var createUser = true
							var tagsInput string
							f := huh.NewForm(
								huh.NewGroup(
									huh.NewConfirm().Title("Create 'kgg-admin' user?").Value(&createUser),
									huh.NewInput().Title("Tags (optional)").Description("user,packages,swap...").Value(&tagsInput),
								),
							)
							return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
								return actions.Provision(n, createUser, parseTags(tagsInput))
							}))
						}
					})
				},
			},
			{
				Title:       "Setup GPU",
				Description: "Install NVIDIA drivers & toolkit",
				DynamicChildren: func() []MenuNode {
					return createNodeSelector("Setup GPU", func(n config.Node) func() tea.Cmd {
						return func() tea.Cmd {
							var tagsInput string
							f := huh.NewForm(huh.NewGroup(huh.NewInput().Title("Tags (optional)").Value(&tagsInput)))
							return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
								return actions.GPUSetup(n, parseTags(tagsInput))
							}))
						}
					})
				},
			},
			{
				Title:       "Mount Storage",
				Description: "Format & Mount secondary disk",
				DynamicChildren: func() []MenuNode {
					return createNodeSelector("Mount Disk", func(n config.Node) func() tea.Cmd {
						return func() tea.Cmd {
							var disk, mountPoint string
							mountPoint = "/mnt/data"
							f := huh.NewForm(
								huh.NewGroup(
									huh.NewInput().Title("Target Disk").Value(&disk),
									huh.NewInput().Title("Mount Point").Value(&mountPoint),
									huh.NewConfirm().Title("Confirm Format?").Value(new(bool)),
								),
							)
							return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
								return actions.MountStorage(n, disk, mountPoint, nil)
							}))
						}
					})
				},
			},
			{
				Title:       "Setup NFS Mounts",
				Description: "Configure NAS shares on nodes",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.OpsNfs()))
				},
			},
			buildClusterOperationsNode(),
		},
	}
}
func buildAppAndAIEcosystemNode() MenuNode {
	return MenuNode{
		Title:       i18n.T("menu_app_ai"),
		Description: i18n.T("menu_app_ai_desc"),
		Children: []MenuNode{
			buildAIConsoleNode(),
			buildAIInsightsNode(),
			buildAppsNode(),
		},
	}
}

func buildGitOpsAndPlatformServicesNode() MenuNode {
	return MenuNode{
		Title:       "⚓ " + i18n.T("menu_gitops_platform"),
		Description: i18n.T("menu_gitops_platform_desc"),
		Children: []MenuNode{
			{
				Title:       "Deploy ArgoCD GitOps",
				Description: "Declarative continuous delivery engine",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.OpsArgoCD()))
				},
			},
			{
				Title:       "Deploy Kargo Promotion",
				Description: "Application promotion & lifecycle manager",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.OpsKargo()))
				},
			},
			{
				Title:       "Deploy Observability Stack",
				Description: "Prometheus, Grafana, and Loki (LGTM)",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.OpsObservability()))
				},
			},
			{
				Title:       "🔓 Access Grafana (Local Mode)",
				Description: "Secure tunnel to local dashboard (offline ready)",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.OpsGrafanaLocalAccess()))
				},
			},
			buildGitOpsManagementNode(),
			buildKargoPromotionNode(),
		},
	}
}

func buildDisasterRecoveryNode() MenuNode {
	return MenuNode{
		Title:       "🛡️ " + i18n.T("menu_disaster_recovery"),
		Description: i18n.T("menu_disaster_recovery_desc"),
		Children: []MenuNode{
			{
				Title:       "🛡️ Deploy Velero DR",
				Description: "Backup cluster and volumes to S3",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.OpsBackupSystem()))
				},
			},
			{
				Title:       "🛡️ Velero Manual Backup",
				Description: "Create a manual Velero backup on-demand",
				Action: func() tea.Cmd {
					nowStr := time.Now().Format("20060102-150405")
					backupName := "manual-" + nowStr
					var nsInput string
					var ttlInput = "240h0m0s"
					var confirm bool

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Backup Name").
								Description("Enter a unique name for this backup").
								Value(&backupName).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("backup name is required")
									}
									return nil
								}),
							huh.NewInput().
								Title("Namespaces to backup").
								Description("Comma-separated (e.g. 'clandestino-dev,gatus'). Leave empty for ALL.").
								Value(&nsInput),
							huh.NewInput().
								Title("TTL (Time To Live)").
								Description("e.g. '240h0m0s' (10 days) or '72h0m0s' (3 days)").
								Value(&ttlInput).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("TTL is required")
									}
									return nil
								}),
							huh.NewConfirm().
								Title("Confirm Backup?").
								Description("This will apply a manual Backup resource on the cluster.").
								Value(&confirm),
						),
					)

					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if !confirm {
							return func() tea.Msg { return actions.ResultMsg{Output: "Backup cancelled."} }
						}
						var nsList []string
						if strings.TrimSpace(nsInput) != "" {
							for _, ns := range strings.Split(nsInput, ",") {
								cleanNs := strings.TrimSpace(ns)
								if cleanNs != "" {
									nsList = append(nsList, cleanNs)
								}
							}
						}
						return actions.OpsCreateVeleroBackup(backupName, nsList, ttlInput)
					}))
				},
			},
			{
				Title:       "🛡️ Velero Restore",
				Description: "Restore cluster state from S3",
				DynamicChildren: func() []MenuNode {
					cfg := config.GetConfig()
					var master *config.Node
					for i := range cfg.Nodes {
						if cfg.Nodes[i].Role == "master" || cfg.Nodes[i].Role == "control-plane" {
							master = &cfg.Nodes[i]
							break
						}
					}
					if master == nil {
						return []MenuNode{{
							Title:       "No Master Found",
							Description: "Configure a master node first",
						}}
					}

					kp, _ := cfg.SSH.ExpandedKeyPath()
					mgr := cluster.NewManager(master.User, kp, cfg.SSH.Port, config.IsDryRun())

					backups, err := mgr.ListVeleroBackups(master.IP)
					if err != nil {
						return []MenuNode{{
							Title:       "⚠️ Error fetching backups",
							Description: err.Error(),
						}}
					}

					if len(backups) == 0 {
						return []MenuNode{{
							Title:       "No Backups Found",
							Description: "Verify S3 bucket/prefix connection in config",
						}}
					}

					var nodes []MenuNode
					for _, b := range backups {
						backup := b
						nodes = append(nodes, MenuNode{
							Title:       fmt.Sprintf("⏪ %s", backup.Name),
							Description: fmt.Sprintf("Status: %s | Created: %s", backup.Phase, backup.StartTimestamp),
							Action: func() tea.Cmd {
								var nsInput string
								var confirm bool

								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().
											Title("Namespaces to restore").
											Description("Comma-separated (e.g. 'clandestino-dev,gatus'). Leave empty for ALL.").
											Value(&nsInput),
										huh.NewConfirm().
											Title(fmt.Sprintf("Confirm Restore from %s?", backup.Name)).
											Description("This will deploy Velero restore resource on the clúster.").
											Value(&confirm),
									),
								)

								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									if !confirm {
										return func() tea.Msg { return actions.ResultMsg{Output: "Restoration cancelled."} }
									}
									var nsList []string
									if strings.TrimSpace(nsInput) != "" {
										for _, ns := range strings.Split(nsInput, ",") {
											cleanNs := strings.TrimSpace(ns)
											if cleanNs != "" {
												nsList = append(nsList, cleanNs)
											}
										}
									}
									return actions.OpsStartVeleroRestore(backup.Name, nsList)
								}))
							},
						})
					}

					return nodes
				},
			},
			{
				Title:       "💾 CNPG Database Backup (Manual)",
				Description: "Trigger an on-demand physical backup of a CNPG database to R2",
				Action: func() tea.Cmd {
					namespace := "clandestino-dev"
					clusterName := "clandestino-db"
					nowStr := time.Now().Format("20060102-150405")
					backupName := "manual-" + nowStr
					var confirm bool

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Namespace").
								Description("Namespace where the database cluster is deployed").
								Value(&namespace).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("namespace is required")
									}
									return nil
								}),
							huh.NewInput().
								Title("Cluster Name").
								Description("Name of the CloudNativePG cluster").
								Value(&clusterName).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("cluster name is required")
									}
									return nil
								}),
							huh.NewInput().
								Title("Backup Name").
								Description("Enter a unique name for this backup").
								Value(&backupName).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("backup name is required")
									}
									return nil
								}),
							huh.NewConfirm().
								Title("Confirm Backup?").
								Description("This will apply a CNPG Backup resource on the cluster.").
								Value(&confirm),
						),
					)

					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if !confirm {
							return func() tea.Msg { return actions.ResultMsg{Output: "Backup cancelled."} }
						}
						return actions.OpsCreateCNPGBackup(clusterName, backupName, namespace)
					}))
				},
			},
			{
				Title:       "📋 CNPG Database Backup (List)",
				Description: "List available physical backups of a CNPG database",
				Action: func() tea.Cmd {
					namespace := "clandestino-dev"
					clusterName := "clandestino-db"
					var confirm bool

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Namespace").
								Description("Namespace where the database cluster is deployed").
								Value(&namespace).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("namespace is required")
									}
									return nil
								}),
							huh.NewInput().
								Title("Cluster Name").
								Description("Name of the CloudNativePG cluster").
								Value(&clusterName).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("cluster name is required")
									}
									return nil
								}),
							huh.NewConfirm().
								Title("Query Backups?").
								Description("This will execute a query via SSH on the master node.").
								Value(&confirm),
						),
					)

					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if !confirm {
							return func() tea.Msg { return actions.ResultMsg{Output: "Listing cancelled."} }
						}
						return actions.OpsListCNPGBackups(clusterName, namespace)
					}))
				},
			},
			{
				Title:       "🕒 CNPG Database Time Machine (Restore / PITR)",
				Description: "Restore database state to a specific point in time (PITR)",
				DynamicChildren: func() []MenuNode {
					cfg := config.GetConfig()
					var master *config.Node
					for i := range cfg.Nodes {
						if cfg.Nodes[i].Role == "master" || cfg.Nodes[i].Role == "control-plane" {
							master = &cfg.Nodes[i]
							break
						}
					}
					if master == nil {
						return []MenuNode{{
							Title:       "No Master Found",
							Description: "Configure a master node first",
						}}
					}

					// Option to enter time manually
					nodes := []MenuNode{
						{
							Title:       "🕒 Restore to custom point-in-time",
							Description: "Enter date and time manually (RFC3339 or YYYY-MM-DD HH:MM:SS)",
							Action: func() tea.Cmd {
								return triggerCNPGRestoreForm("clandestino-dev", "clandestino-db", "")
							},
						},
					}

					kp, _ := cfg.SSH.ExpandedKeyPath()
					mgr := cluster.NewManager(master.User, kp, cfg.SSH.Port, config.IsDryRun())

					// Get list of CNPG backups (using default namespace/cluster)
					backups, err := mgr.ListCNPGBackups(master.IP, "clandestino-dev", "clandestino-db")
					if err != nil {
						// Fallback if error, just return the manual option with warning
						nodes = append(nodes, MenuNode{
							Title:       "⚠️ Error listing backups",
							Description: err.Error(),
						})
						return nodes
					}

					// Sort backups descending (newest first)
					sort.Slice(backups, func(i, j int) bool {
						t1, _ := time.Parse(time.RFC3339, backups[i].CreatedAt)
						t2, _ := time.Parse(time.RFC3339, backups[j].CreatedAt)
						return t1.After(t2)
					})

					for _, b := range backups {
						if b.Phase != "completed" {
							continue
						}
						backup := b
						nodes = append(nodes, MenuNode{
							Title:       fmt.Sprintf("⏪ %s", backup.Name),
							Description: fmt.Sprintf("Created: %s", backup.CreatedAt),
							Action: func() tea.Cmd {
								return triggerCNPGRestoreForm("clandestino-dev", "clandestino-db", backup.CreatedAt)
							},
						})
					}

					return nodes
				},
			},
			{
				Title:       "🔌 CNPG Database Local Access (Tunnel)",
				Description: "Create a secure SSH port-forwarding tunnel to query CNPG from DBeaver",
				Action: func() tea.Cmd {
					namespace := "clandestino-dev"
					clusterName := "clandestino-db"
					localPortStr := "5433"
					var confirm bool

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Namespace").
								Description("Namespace of the database cluster").
								Value(&namespace).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("namespace is required")
									}
									return nil
								}),
							huh.NewInput().
								Title("Cluster Name").
								Description("Name of the CloudNativePG cluster").
								Value(&clusterName).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("cluster name is required")
									}
									return nil
								}),
							huh.NewInput().
								Title("Local Port").
								Description("Port on your local machine to bind to").
								Value(&localPortStr).
								Validate(func(s string) error {
									p, err := strconv.Atoi(s)
									if err != nil || p < 1 || p > 65535 {
										return errors.New("must be a valid port number (1-65535)")
									}
									return nil
								}),
							huh.NewConfirm().
								Title("Establish Tunnel?").
								Description("This will bind localhost:<port> to the remote primary database instance.").
								Value(&confirm),
						),
					)

					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if !confirm {
							return func() tea.Msg { return actions.ResultMsg{Output: "Tunnel cancelled."} }
						}
						localPort, _ := strconv.Atoi(localPortStr)
						return actions.OpsCNPGTunnel(clusterName, namespace, localPort)
					}))
				},
			},
			buildCloudSyncNode(),
		},
	}
}

func buildNetworkAndIntegrationsNode() MenuNode {
	return MenuNode{
		Title:       "🔌 " + i18n.T("menu_network_integration"),
		Description: i18n.T("menu_network_integration_desc"),
		Children: []MenuNode{
			{
				Title:       "🔍 Scan Network",
				Description: "Uncover new devices (Read-only)",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.ScanNetwork()))
				},
			},
			{
				Title:       "🛡️ Deploy Cloudflare Zero Trust",
				Description: "Cert-Manager & Cloudflared Tunnels",
				Action: func() tea.Cmd {
					cfg := config.GetConfig()

					// Check if Cloudflare credentials are already configured
					hasCreds := cfg.Cloudflare.APIToken != "" &&
						cfg.Cloudflare.TunnelToken != "" &&
						cfg.Cloudflare.Email != ""

					if hasCreds {
						return engine.Push(NewOutputModel(actions.OpsCloudflare(false)))
					}

					// Prompt for account credentials (domains managed separately)
					email := cfg.Cloudflare.Email
					var apiTokenStr, tunnelTokenStr string

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Cloudflare Email").
								Description("Email associated with your Cloudflare account").
								Value(&email).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("email is required")
									}
									return nil
								}),
							huh.NewInput().
								Title("Cloudflare API Token").
								Description("API Token with Tunnel + DNS + Access permissions").
								Value(&apiTokenStr).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("API token is required")
									}
									return nil
								}),
							huh.NewInput().
								Title("Zero Trust Tunnel Token").
								Description("Token from Cloudflare Zero Trust dashboard").
								Value(&tunnelTokenStr).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("tunnel token is required")
									}
									return nil
								}),
						),
					)

					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						err := config.ModifyConfig(func(c *config.ClusterConfig) {
							c.Cloudflare.Email = strings.TrimSpace(email)
							c.Cloudflare.APIToken = config.Secret(strings.TrimSpace(apiTokenStr))
							c.Cloudflare.TunnelToken = config.Secret(strings.TrimSpace(tunnelTokenStr))
						})
						if err != nil {
							return func() tea.Msg {
								return actions.ResultMsg{Output: "❌ Failed to save config: " + err.Error()}
							}
						}
						_ = config.SaveConfig()
						return engine.Push(NewOutputModel(actions.OpsCloudflare(false)))
					}))
				},
			},
			{
				Title:       "🌐 Cloudflare Automated Provisioning",
				Description: "Auto-create tunnel, tokens & deploy",
				Action: func() tea.Cmd {
					var confirm bool
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().
								Title("Start Automated Provisioning?").
								Description("This will create a tunnel in Cloudflare and update your config.").
								Value(&confirm),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if !confirm {
							return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled."} }
						}
						return engine.Push(NewOutputModel(actions.OpsCloudflare(true)))
					}))
				},
			},
			buildDomainsAndServicesNode(),
			buildNetworkNode(),
			{
				Title:       "🤖 " + i18n.T("menu_infra"),
				Description: "RPi agent provisioning & updates",
				Children: []MenuNode{
					{
						Title:       "Provision Infra-manager (RPi)",
						Description: "Install daemon, MQTT, systemd",
						Action: func() tea.Cmd {
							return engine.Push(NewOutputModel(actions.InfraInit()))
						},
					},
					{
						Title:       "⚡ Quick Update: Bot & KGGCLI",
						Description: "Fast-track Python and binary updates",
						Action: func() tea.Cmd {
							return engine.Push(NewOutputModel(actions.InfraBotUpdate()))
						},
					},
					{
						Title:       "💓 Run Heartbeat (Health Check)",
						Description: "Cluster-wide diagnostic scan",
						Action: func() tea.Cmd {
							return engine.Push(NewOutputModel(actions.InfraHeartbeat(false)))
						},
					},
					{
						Title:       "🤖 Run AI Diagnostic Heartbeat",
						Description: "Scan with Senior SRE AI analysis",
						Action: func() tea.Cmd {
							return engine.Push(NewOutputModel(actions.InfraHeartbeat(true)))
						},
					},
				},
			},
		},
	}
}

func buildDiagnosticsAndMaintenanceNode() MenuNode {
	return MenuNode{
		Title:       "🔧 " + i18n.T("menu_sre"),
		Description: i18n.T("menu_sre_desc"),
		Children: []MenuNode{
			{
				Title:       "🩺 Run Doctor Audit",
				Description: "Collect metrics and verify status of all nodes",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.Doctor()))
				},
			},
			{
				Title:       "🔄 System Package Update",
				Description: "Run maintenance playbook on all nodes",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.OpsUpdate()))
				},
			},
			{
				Title:       "🔔 System Update + Telegram Notify",
				Description: "Update system packages and send telegram alert",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.OpsUpdateWithNotify()))
				},
			},
		},
	}
}

func buildSettingsAndSupportNode() MenuNode {
	return MenuNode{
		Title:       i18n.T("menu_settings"),
		Description: i18n.T("menu_settings_desc"),
		Children: []MenuNode{
			buildConfigNode(),
			buildHelpNode(),
		},
	}
}

func buildInventoryNode() MenuNode {
	return MenuNode{
		Title:       "Inventory",
		Description: "View all configured nodes",
		Action: func() tea.Cmd {
			return engine.Push(NewInventoryModel())
		},
	}
}



func buildNetworkNode() MenuNode {
	return MenuNode{
		Title:       "Network Switch",
		Description: "VLANs & Port Control",
		Children: []MenuNode{
			{
				Title:       "Show Status",
				Description: "Live traffic & port state",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.NetworkStatus()))
				},
			},
			{
				Title:       "Validate Connections",
				Description: "Check cabling vs YAML",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.NetworkValidate()))
				},
			},
			{
				Title:       "Apply Config",
				Description: "Enforce network layout",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.NetworkApply()))
				},
			},
			{
				Title:       "Port Inventory Map",
				Description: "Physical device layout",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.NetworkMap()))
				},
			},
			{
				Title:       "Reboot Switch",
				Description: "Restart hardware",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.NetworkReboot()))
				},
			},
		},
	}
}

func buildSSHManagementNode() MenuNode {
	return MenuNode{
		Title:       "SSH Management",
		Description: "Keys & Access",
		Children: []MenuNode{
			{
				Title:       "Generate Cluster Key",
				Description: "Create cluster SSH key",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.SSHKeyGen()))
				},
			},
			{
				Title:       "Install Key (SSH Copy)",
				Description: "Copy key to node via password",
				Action: func() tea.Cmd {
					// Setup Form
					var nodeIP, user, password string
					user = "debian"

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().Title("Node IP").Value(&nodeIP).
								Validate(func(s string) error {
									if net.ParseIP(strings.TrimSpace(s)) == nil {
										return errors.New("invalid IP address")
									}
									return nil
								}),
							huh.NewInput().Title("User").Value(&user),
							huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&password).
								Validate(func(s string) error {
									if s == "" {
										return errors.New("password is required")
									}
									return nil
								}),
						),
					)

					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						return actions.SSHCopy(nodeIP, user, password)
					}))
				},
			},
			{
				Title:       "🧹 Clean Host Key",
				Description: "Remove node from system known_hosts",
				Children: []MenuNode{
					{
						Title:       "Select from List",
						Description: "Choose a configured node to clean",
						DynamicChildren: func() []MenuNode {
							return createNodeSelector("Clean Host", func(n config.Node) func() tea.Cmd {
								return func() tea.Cmd {
									node := n
									var confirm bool
									f := huh.NewForm(
										huh.NewGroup(
											huh.NewConfirm().
												Title(fmt.Sprintf("Clean host key for %s (%s)?", node.Name, node.IP)).
												Description("This will remove the entry from your system's known_hosts file.").
												Value(&confirm),
										),
									)
									return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
										if !confirm {
											return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled."} }
										}
										return actions.CleanHost(node)
									}))
								}
							})
						},
					},
					{
						Title:       "Manual Entry",
						Description: "Enter IP or Hostname to clean",
						Action: func() tea.Cmd {
							var identifier string
							f := huh.NewForm(
								huh.NewGroup(
									huh.NewInput().
										Title("IP or Hostname").
										Description("Entry to remove from system known_hosts").
										Value(&identifier).
										Validate(func(s string) error {
											if strings.TrimSpace(s) == "" {
												return errors.New("identifier is required")
											}
											return nil
										}),
								),
							)
							return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
								return actions.CleanSystemHost(strings.TrimSpace(identifier))
							}))
						},
					},
				},
			},
			{
				Title:       "🧹 Bulk Clean All Inventory Keys",
				Description: "Remove all configured nodes from system known_hosts",
				Action: func() tea.Cmd {
					var confirm bool
					cfg := config.GetConfig()
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().
								Title("Bulk clean ALL nodes in inventory?").
								Description(fmt.Sprintf("This will remove %d nodes from known_hosts.", len(cfg.Nodes))).
								Value(&confirm),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if !confirm {
							return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled."} }
						}
						return actions.BulkCleanHosts(cfg.Nodes)
					}))
				},
			},
		},
	}
}

func buildAppsNode() MenuNode {
	return MenuNode{
		Title:       "Applications",
		Description: "Services & Backups",
		Children: []MenuNode{
			{
				Title:       "Deploy Immich",
				Description: "Self-hosted photo and video backup",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.AppDeploy("immich")))
				},
			},
			{
				Title:       "Deploy Ollama",
				Description: "Local LLM inferencing",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.AppDeploy("ollama")))
				},
			},
			{
				Title:       "Deploy HomeAssistant",
				Description: "Smart home automation setup",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.AppDeploy("homeassistant")))
				},
			},
			{
				Title:       "Trigger Backup",
				Description: "Backup apps data to external storage",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.AppBackup()))
				},
			},
		},
	}
}

func buildClusterOperationsNode() MenuNode {
	return MenuNode{
		Title:       "Cluster Operations",
		Description: "K3s Lifecycle",
		Children: []MenuNode{
			{
				Title:       "🚀 Launch K9s Dashboard",
				Description: "Real-time Kubernetes pod management (Auto-sync from live cluster)",
				Action: func() tea.Cmd {
					// 1. Dependency Check
					if cmd := actions.LaunchK9s(); cmd != nil {
						return cmd
					}
					// 2. Open Handover Model (Sync -> Exec)
					return engine.Push(NewK9sHandoverModel())
				},
			},
			{
				Title:       "Full Site Deploy",
				Description: "Provision → GPU → K3s → NFS (all nodes)",
				Action: func() tea.Cmd {
					var confirm = false
					cfg := config.GetConfig()

					// Check if HA is detected
					masterCount := 0
					for _, n := range cfg.Nodes {
						if n.Role == "master" || n.Role == "control-plane" {
							masterCount++
						}
					}

					var vipInput string
					needsVIP := masterCount > 1 && cfg.K3s.VIP == ""

					groups := []*huh.Group{
						huh.NewGroup(
							huh.NewConfirm().
								Title("Run full cluster deployment?").
								Description("This will provision, setup GPU, init K3s, join nodes, and mount NFS.").
								Value(&confirm),
						),
					}

					if needsVIP {
						groups = append(groups, huh.NewGroup(
							huh.NewInput().
								Title("HA Virtual IP Required").
								Description("Multiple servers detected. Enterprise HA requires a VIP.").
								Value(&vipInput).
								Validate(func(s string) error {
									if net.ParseIP(strings.TrimSpace(s)) == nil {
										return errors.New("invalid IP address")
									}
									return nil
								}),
						))
					}

					f := huh.NewForm(groups...)

					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if !confirm {
							return func() tea.Msg {
								return actions.ResultMsg{Output: "Cancelled."}
							}
						}

						// Save VIP if it was prompted
						if needsVIP {
							err := config.ModifyConfig(func(c *config.ClusterConfig) {
								c.K3s.VIP = strings.TrimSpace(vipInput)
							})
							if err != nil {
								return func() tea.Msg {
									return actions.ResultMsg{Output: "❌ Failed to save VIP: " + err.Error()}
								}
							}
							err = config.SaveConfig()
							if err != nil {
								return func() tea.Msg {
									return actions.ResultMsg{Output: "❌ Failed to save VIP: " + err.Error()}
								}
							}
						}

						return actions.SiteDeploy(nil)
					}))
				},
			},
			{
				Title:       "Initialize Cluster",
				Description: "Install K3s on first master",
				Action: func() tea.Cmd {
					// Find first master node
					var masterNode *config.Node
					cfg := config.GetConfig()
					for i := range cfg.Nodes {
						if cfg.Nodes[i].Role == "master" || cfg.Nodes[i].Role == "control-plane" {
							masterNode = &cfg.Nodes[i]
							break
						}
					}
					if masterNode == nil {
						return engine.Push(NewOutputModel(func() tea.Msg {
							return actions.ResultMsg{Output: "❌ No node with role 'master' found in config."}
						}))
					}

					// HA mode confirm
					var isHA = false
					var vipInput string

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().
								Title("Enable HA Mode?").
								Description("Requires 3+ master nodes for quorum").
								Value(&isHA),
						),
						huh.NewGroup(
							huh.NewInput().
								Title("HA Virtual IP").
								Description("Required for HA (e.g. 192.168.1.100)").
								Value(&vipInput).
								Validate(func(s string) error {
									if !isHA {
										return nil
									}
									if net.ParseIP(strings.TrimSpace(s)) == nil {
										return errors.New("invalid IP address (required for HA)")
									}
									return nil
								}),
						).WithHideFunc(func() bool { return !isHA || cfg.K3s.VIP != "" }),
					)

					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if isHA && cfg.K3s.VIP == "" && vipInput != "" {
							err := config.ModifyConfig(func(c *config.ClusterConfig) {
								c.K3s.VIP = strings.TrimSpace(vipInput)
							})
							if err != nil {
								return func() tea.Msg {
									return actions.ResultMsg{Output: "❌ Failed to save VIP: " + err.Error()}
								}
							}
							err = config.SaveConfig()
							if err != nil {
								return func() tea.Msg {
									return actions.ResultMsg{Output: "❌ Failed to save VIP: " + err.Error()}
								}
							}
						}
						return actions.ClusterInit(*masterNode, isHA, nil)
					}))
				},
			},
			{
				Title:       "🔧 Configure HA VIP",
				Description: "Set Virtual IP for high availability",
				Action: func() tea.Cmd {
					cfg := config.GetConfig()
					vip := cfg.K3s.VIP

					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("HA Virtual IP").
								Description("Enter a free IP (e.g. 192.168.1.100)").
								Value(&vip).
								Validate(func(s string) error {
									if s == "" {
										return nil // Optional, can be cleared
									}
									if net.ParseIP(strings.TrimSpace(s)) == nil {
										return errors.New("invalid IP address")
									}
									// Check for conflict
									for _, n := range cfg.Nodes {
										if n.IP == strings.TrimSpace(s) {
											return fmt.Errorf("IP conflicts with node '%s'", n.Name)
										}
									}
									return nil
								}),
						),
					)

					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						return actions.ConfigSetK3sVIP(strings.TrimSpace(vip))
					}))
				},
			},
			{
				Title:       "Deploy Storage (Longhorn)",
				Description: "Install distributed block storage",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.LonghornInit()))
				},
			},
			{
				Title:       "Check Storage Status",
				Description: "Verify Longhorn system health",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.LonghornStatus()))
				},
			},
			{
				Title:       "Join Node",
				Description: "Add a node to the cluster",
				DynamicChildren: func() []MenuNode {
					// Find master node for token
					var masterNode *config.Node
					cfg := config.GetConfig()
					for i := range cfg.Nodes {
						if cfg.Nodes[i].Role == "master" {
							masterNode = &cfg.Nodes[i]
							break
						}
					}
					if masterNode == nil {
						return []MenuNode{{
							Title:       "No Master Found",
							Description: "Configure a master node first",
						}}
					}

					// Live check for existing nodes
					kp, _ := cfg.SSH.ExpandedKeyPath()
					port := cfg.SSH.Port
					if port == 0 {
						port = 22
					}
					mgr := cluster.NewManager(masterNode.User, kp, port, config.IsDryRun())
					liveNodes, _ := mgr.GetLiveNodes(masterNode.IP)

					isJoined := make(map[string]bool)
					for _, name := range liveNodes {
						isJoined[name] = true
					}

					// List nodes NOT in the cluster
					nodes := []MenuNode{}
					for _, n := range cfg.Nodes {
						node := n
						// Skip if already in cluster OR is the master we are using to check
						if isJoined[node.Name] {
							continue
						}
						nodes = append(nodes, MenuNode{
							Title:       fmt.Sprintf("%s (%s)", node.Name, node.IP),
							Description: fmt.Sprintf("Join as %s", node.Role),
							Action: func() tea.Cmd {
								return engine.Push(NewOutputModel(actions.ClusterJoin(node, *masterNode, nil)))
							},
						})
					}
					if len(nodes) == 0 {
						return []MenuNode{{
							Title:       "No Other Nodes",
							Description: "Add more nodes to config first",
						}}
					}
					return nodes
				},
			},
			{
				Title:       "Drain Node",
				Description: "Evict pods before maintenance",
				DynamicChildren: func() []MenuNode {
					// Find master for kubectl
					var masterNode *config.Node
					cfg := config.GetConfig()
					for i := range cfg.Nodes {
						if cfg.Nodes[i].Role == "master" {
							masterNode = &cfg.Nodes[i]
							break
						}
					}
					if masterNode == nil {
						return []MenuNode{{Title: "No Master Found"}}
					}

					nodes := []MenuNode{}
					for _, n := range cfg.Nodes {
						node := n
						nodes = append(nodes, MenuNode{
							Title:       node.Name,
							Description: node.IP,
							Action: func() tea.Cmd {
								return engine.Push(NewOutputModel(actions.ClusterDrain(*masterNode, node.Name, nil)))
							},
						})
					}
					return nodes
				},
			},
			{
				Title:       "Reset Node",
				Description: "Uninstall K3s from node",
				DynamicChildren: func() []MenuNode {
					return createNodeSelector("Reset K3s", func(n config.Node) func() tea.Cmd {
						return func() tea.Cmd {
							return engine.Push(NewOutputModel(actions.ClusterReset(n, nil)))
						}
					})
				},
			},
			{
				Title:       "Remediate Node 🛠️",
				Description: "Drain, reset, and rejoin a worker node",
				DynamicChildren: func() []MenuNode {
					cfg := config.GetConfig()
					nodes := []MenuNode{}
					for _, n := range cfg.Nodes {
						node := n
						if node.Role == "infra-manager" {
							continue
						}

						// For this target node, find a coordinator master node that is not the target node itself
						var coordMaster *config.Node
						for i := range cfg.Nodes {
							if (cfg.Nodes[i].Role == "master" || cfg.Nodes[i].Role == "control-plane") && cfg.Nodes[i].Name != node.Name {
								coordMaster = &cfg.Nodes[i]
								break
							}
						}

						// Fallback: If no other master node is found, use the first available master
						if coordMaster == nil {
							for i := range cfg.Nodes {
								if cfg.Nodes[i].Role == "master" || cfg.Nodes[i].Role == "control-plane" {
									coordMaster = &cfg.Nodes[i]
									break
								}
							}
						}

						if coordMaster == nil {
							continue
						}

						coordinator := *coordMaster

						nodes = append(nodes, MenuNode{
							Title:       node.Name,
							Description: fmt.Sprintf("Drain, reset, and rejoin %s (via coordinator %s)", node.IP, coordinator.Name),
							Action: func() tea.Cmd {
								return engine.Push(NewOutputModel(actions.ClusterRemediate(coordinator, node.Name, nil)))
							},
						})
					}
					return nodes
				},
			},
		},
	}
}

func buildHelpNode() MenuNode {
	return MenuNode{
		Title:       "Help & Documentation",
		Description: "Guides and Reference",
		DynamicChildren: func() []MenuNode {
			svc := help.NewService()
			nodes := []MenuNode{}
			for _, t := range svc.GetTopics() {
				topic := t
				nodes = append(nodes, MenuNode{
					Title:       topic.Title,
					Description: topic.Description,
					Action: func() tea.Cmd {
						// Render markdown content
						content, err := svc.GetContent(topic.ID)
						output := ""
						if err != nil {
							output = fmt.Sprintf("Error: %v", err)
						} else {
							renderer, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(80))
							if err != nil {
								output = content // Fallback to raw content
							} else {
								output, _ = renderer.Render(content)
							}
						}

						return engine.Push(NewOutputModel(func() tea.Msg {
							return actions.ResultMsg{Output: output}
						}).WithAutoScroll(false))
					},
				})
			}
			return nodes
		},
	}
}

func buildAIConsoleNode() MenuNode {
	return MenuNode{
		Title:       "AI Console",
		Description: "Chat & Status",
		Children: []MenuNode{
			{
				Title:       "Status",
				Description: "List running models",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.AIStatus()))
				},
			},
			{
				Title:       "Pull Model",
				Description: "Download a model to AI node",
				Action: func() tea.Cmd {
					var modelName string
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Model Name").
								Description("e.g. llama3, mistral").
								Value(&modelName),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						return actions.AIPull(modelName)
					}))
				},
			},
			{
				Title:       "Interactive Chat",
				Description: "Chat with a model (Interactive)",
				Action: func() tea.Cmd {
					var modelName string
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Model Name").
								Description("e.g. llama3").
								Value(&modelName),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						return actions.AIChat(modelName)
					}))
				},
			},
		},
	}
}



// ============================================================================
// Helpers
// ============================================================================

func buildConfigNode() MenuNode {
	return MenuNode{
		Title:       "Configuration",
		Description: "Manage CLI settings",
		DynamicChildren: func() []MenuNode {
			return []MenuNode{
				{
					Title:       "Switch Context",
					Description: fmt.Sprintf("Current: %s", config.GetCurrentContext()),
					DynamicChildren: func() []MenuNode {
						return createContextSelector()
					},
				},
				{
					Title:       "⏪ Restore Backup",
					Description: "Revert to a previous configuration version",
					DynamicChildren: func() []MenuNode {
						backups, _ := config.ListBackups()
						var nodes []MenuNode
						for _, b := range backups {
							name := b
							nodes = append(nodes, MenuNode{
								Title: name,
								Action: func() tea.Cmd {
									var confirm bool
									f := huh.NewForm(huh.NewGroup(
										huh.NewConfirm().Title("Restore this backup?").
											Description("The current config will be backed up before restoration.").
											Value(&confirm),
									))
									return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
										if !confirm {
											return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled"} }
										}
										return actions.ConfigRestore(name)
									}))
								},
							})
						}
						return nodes
					},
				},
				{
					Title:       "🌍 Language / Idioma",
					Description: "Change the UI language (English/Español)",
					Action: func() tea.Cmd {
						appCfg := config.GetAppConfig()
						f := huh.NewForm(
							huh.NewGroup(
								huh.NewSelect[string]().
									Title("Select Language / Seleccionar Idioma").
									Options(
										huh.NewOption("English", "en"),
										huh.NewOption("Español", "es"),
									).
									Value(&appCfg.Lang),
							),
						)
						return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
							return actions.ConfigUpdateLang(appCfg.Lang)
						}))
					},
				},
				{
					Title:       "⚙️  Edit Active Context",
					Description: "Modify global settings for the current environment. (Auto-syncs to Infra Manager)",
					Children: []MenuNode{
						{
							Title:       "✨ Self Update",
							Description: "Check for kuargogo updates",
							Action: func() tea.Cmd {
								return engine.Push(NewOutputModel(actions.SelfUpdate()))
							},
						},
						{
							Title:       "🔑 SSH Settings",
							Description: "Private key and port",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								ssh := cfg.SSH
								sshPortStr := strconv.Itoa(ssh.Port)
								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("Private Key Path").Value(&ssh.PrivateKeyPath),
										huh.NewInput().Title("SSH Port").Value(&sshPortStr).
											Validate(func(s string) error {
												_, err := strconv.Atoi(s)
												return err
											}),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									ssh.Port, _ = strconv.Atoi(sshPortStr)
									return actions.ConfigUpdateSSH(ssh)
								}))
							},
						},
						{
							Title:       "🌐 Network Switch",
							Description: "IP, Credentials and Driver",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								netCfg := cfg.Network

								// Use temp string for Secret pointer compatibility
								passStr := string(netCfg.Password)

								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("Switch IP").Value(&netCfg.SwitchIP).
											Validate(func(s string) error {
												if s != "" && net.ParseIP(s) == nil {
													return errors.New("invalid IP")
												}
												return nil
											}),
										huh.NewInput().Title("User").Value(&netCfg.User),
										huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&passStr),
										huh.NewSelect[string]().Title("Driver").
											Options(
												huh.NewOption("TP-Link", "tplink"),
												huh.NewOption("MikroTik", "mikrotik"),
												huh.NewOption("Simulated", "simulated"),
											).Value(&netCfg.Driver),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									netCfg.Password = config.Secret(passStr)
									return actions.ConfigUpdateNetwork(netCfg)
								}))
							},
						},
						{
							Title:       "📡 MQTT Broker",
							Description: "Broker URL and Topic Prefix",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								mqtt := cfg.MQTT

								// Use temp string for Secret pointer compatibility
								passStr := string(mqtt.Password)

								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("Broker URL").Description("tcp://192.168.1.100:1883").Value(&mqtt.Broker),
										huh.NewInput().Title("Client ID").Value(&mqtt.ClientID),
										huh.NewInput().Title("Topic Prefix").Value(&mqtt.TopicPrefix),
										huh.NewInput().Title("Username (optional)").Value(&mqtt.Username),
										huh.NewInput().Title("Password (optional)").EchoMode(huh.EchoModePassword).Value(&passStr),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									mqtt.Password = config.Secret(passStr)
									return actions.ConfigUpdateMQTT(mqtt)
								}))
							},
						},
						{
							Title:       "🤖 AI Settings",
							Description: "Provider, Model and Endpoint",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								aiCfg := cfg.AI

								// Use temp string for Secret pointer compatibility
								keyStr := string(aiCfg.APIKey)

								f := huh.NewForm(
									huh.NewGroup(
										huh.NewSelect[string]().Title("Provider").
											Options(
												huh.NewOption("Ollama (Local)", "ollama"),
												huh.NewOption("OpenAI Compatible", "openai-compatible"),
												huh.NewOption("OpenAI (Cloud)", "openai"),
												huh.NewOption("Anthropic (Cloud)", "anthropic"),
												huh.NewOption("Gemini (Cloud)", "gemini"),
											).Value(&aiCfg.Provider),
										huh.NewInput().Title("Model").Description("e.g. llama3, llama3:8b").Value(&aiCfg.Model),
										huh.NewInput().Title("Endpoint URL").Description("For local/proxy services").Value(&aiCfg.Endpoint),
										huh.NewInput().Title("API Key").EchoMode(huh.EchoModePassword).Value(&keyStr),
										huh.NewConfirm().Title("Anonymize Logs").Description("Mask IPs and users before sending to AI").Value(&aiCfg.AnonymizeLogs),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									aiCfg.APIKey = config.Secret(keyStr)
									return actions.ConfigUpdateAI(aiCfg)
								}))
							},
						},
						{
							Title:       "☸️  K3s Settings",
							Description: "Token, VIP and Kubeconfig",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								k3s := cfg.K3s

								// Use temp string for Secret pointer compatibility
								tokenStr := string(k3s.Token)

								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("Cluster Token").Value(&tokenStr),
										huh.NewConfirm().Title("Enable HA Mode").
											Description("Requires 3+ master nodes for quorum").
											Value(&k3s.HA),
										huh.NewInput().Title("K3s Version").
											Description("e.g. v1.30.1+k3s1").
											Value(&k3s.Version),
										huh.NewInput().Title("HA Virtual IP").Value(&k3s.VIP).
											Validate(func(s string) error {
												if s != "" && net.ParseIP(s) == nil {
													return errors.New("invalid IP")
												}
												return nil
											}),
										huh.NewInput().Title("HA VIP Interface").
											Description("Physical interface for the VIP (leave empty for auto)").
											Value(&k3s.VIPInterface),
										huh.NewInput().Title("Kubeconfig Path").Value(&k3s.KubeconfigPath),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									k3s.Token = config.Secret(tokenStr)
									return actions.ConfigUpdateK3s(k3s)
								}))
							},
						},
						{
							Title:       "🤖 Telegram Bot",
							Description: "Bot Token and Admin ID",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								tg := cfg.Telegram
								adminIDStr := strconv.Itoa(tg.AdminID)

								// Use temp string for Secret pointer compatibility
								tokenStr := string(tg.BotToken)

								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("Bot Token").Value(&tokenStr),
										huh.NewInput().Title("Admin ID").Value(&adminIDStr).
											Validate(func(s string) error {
												_, err := strconv.Atoi(s)
												return err
											}),
										huh.NewInput().Title("Timezone").
											Description("e.g. Europe/Madrid, UTC").
											Value(&tg.Timezone),
										huh.NewInput().Title("Daily Summary Time").
											Description("24h format (e.g. 08:30)").
											Value(&tg.DailySummaryTime).
											Validate(func(s string) error {
												parts := strings.Split(s, ":")
												if len(parts) != 2 {
													return errors.New("must be in H:M format")
												}
												h, err1 := strconv.Atoi(parts[0])
												m, err2 := strconv.Atoi(parts[1])
												if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
													return errors.New("invalid time")
												}
												return nil
											}),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									tg.AdminID, _ = strconv.Atoi(adminIDStr)
									tg.BotToken = config.Secret(tokenStr)
									return actions.ConfigUpdateTelegram(tg)
								}))
							},
						},
						{
							Title:       "📂 Network Storage (NFS)",
							Description: "Enable and configure NAS shares",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								nfs := cfg.NFS
								f := huh.NewForm(
									huh.NewGroup(
										huh.NewConfirm().Title("Enable NFS?").Value(&nfs.Enabled),
										huh.NewInput().Title("NFS Server IP").Value(&nfs.Server).
											Validate(func(s string) error {
												if s != "" && net.ParseIP(s) == nil {
													return errors.New("invalid IP")
												}
												return nil
											}),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									return actions.ConfigUpdateNFS(nfs)
								}))
							},
						},
						{
							Title:       "🔍 Network Discovery (mDNS)",
							Description: "Auto-scan and WoL interface",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								d := cfg.Discovery
								f := huh.NewForm(
									huh.NewGroup(
										huh.NewConfirm().Title("Enable mDNS Scan?").Value(&d.Enabled),
										huh.NewInput().Title("Network Interface").
											Description("e.g. eth0, wlan0").
											Value(&d.Interface),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									return actions.ConfigUpdateDiscovery(d)
								}))
							},
						},
						{
							Title:       "🛑 Global Maintenance Mode",
							Description: "Suppress all alerts and bot notifications",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								maint := cfg.MaintenanceMode
								f := huh.NewForm(
									huh.NewGroup(
										huh.NewConfirm().
											Title("Enable Maintenance Mode?").
											Description("The bot will ignore incidents while active.").
											Value(&maint),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									return actions.ConfigUpdateMaintenance(maint)
								}))
							},
						},
						{
							Title:       "☁️  Cloudflare Tunnel",
							Description: "Account credentials & tunnel tokens",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								cf := cfg.Cloudflare

								apiStr := string(cf.APIToken)
								tunnelStr := string(cf.TunnelToken)

								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("Email").Value(&cf.Email),
										huh.NewInput().Title("API Token").Value(&apiStr),
										huh.NewInput().Title("Tunnel Token").Value(&tunnelStr),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									cf.APIToken = config.Secret(apiStr)
									cf.TunnelToken = config.Secret(tunnelStr)
									return actions.ConfigUpdateCloudflare(cf)
								}))
							},
						},
						{
							Title:       "💾 S3 Backup (Velero)",
							Description: "S3 Credentials and Bucket",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								bk := cfg.Backup

								// Use temp strings for Secret pointer compatibility
								accessStr := string(bk.S3AccessKey)
								secretStr := string(bk.S3SecretKey)

								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("S3 URL").Value(&bk.S3Url),
										huh.NewInput().Title("Bucket Name").Value(&bk.S3Bucket),
										huh.NewInput().Title("S3 Folder Prefix").Value(&bk.S3Prefix),
										huh.NewInput().Title("Region").Value(&bk.S3Region),
										huh.NewInput().Title("Access Key").Value(&accessStr),
										huh.NewInput().Title("Secret Key").EchoMode(huh.EchoModePassword).Value(&secretStr),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									bk.S3AccessKey = config.Secret(accessStr)
									bk.S3SecretKey = config.Secret(secretStr)
									return actions.ConfigUpdateBackup(bk)
								}))
							},
						},
						{
							Title:       "📦 Ansible & Provisioning",
							Description: "WSL Distro and Vault Password",
							Action: func() tea.Cmd {
								cfg := config.GetConfig()
								ansibleCfg := cfg.Ansible
								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("WSL Distribution").
											Description("e.g. Ubuntu, Debian, Alpine").
											Value(&ansibleCfg.WSLDistro),
										huh.NewInput().Title("Vault Password File").
											Description("Path to ansible-vault password file").
											Value(&ansibleCfg.VaultPasswordFile),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									return actions.ConfigUpdateAnsible(ansibleCfg)
								}))
							},
						},
					},
				},
				{
					Title:       "Add New Context (Wizard)",
					Description: "Create a new cluster configuration",
					Action: func() tea.Cmd {
						// Get current executable path
						exePath, err := os.Executable()
						if err != nil {
							return func() tea.Msg {
								return actions.ResultMsg{Output: fmt.Sprintf("❌ Error getting executable path: %v", err)}
							}
						}

						// Create command to run wizard as subprocess
						cmd := exec.Command(exePath, "init", "--force")
						cmd.Stdin = os.Stdin
						cmd.Stdout = os.Stdout
						cmd.Stderr = os.Stderr

						// Use tea.ExecProcess to hand over terminal control
						return tea.ExecProcess(cmd, func(err error) tea.Msg {
							if err != nil {
								return actions.ResultMsg{Output: fmt.Sprintf("❌ Wizard failed: %v", err)}
							}
							// Reload config after subprocess completes
							if loadErr := config.LoadConfig(""); loadErr != nil {
								return actions.ResultMsg{Output: fmt.Sprintf("⚠️ Wizard completed but config reload failed: %v", loadErr)}
							}
							// Return ContextSwitchedMsg to trigger PopToRoot and refresh menu
							return actions.ContextSwitchedMsg{ContextName: config.GetCurrentContext()}
						})
					},
				},
				{
					Title:       "Lint Configuration",
					Description: "Validate current settings for errors",
					Action: func() tea.Cmd {
						return engine.Push(NewOutputModel(actions.DoctorConfig()))
					},
				},
				{
					Title:       "Setup Admin PC",
					Description: "Install required CLI tools (Ansible, K9s)",
					Action: func() tea.Cmd {
						return engine.Push(NewOutputModel(actions.SetupAdminPC()))
					},
				},
				{
					Title:       "📤 Export Embedded Playbooks",
					Description: "Eject playbooks/roles to ~/.kuargogo/playbooks for customization",
					Action: func() tea.Cmd {
						// 1. Scan available playbooks
						playbooks, err := ansible.ListAvailablePlaybooks()
						if err != nil {
							return func() tea.Msg {
								return actions.ResultMsg{Output: fmt.Sprintf("❌ Error listing playbooks: %v", err)}
							}
						}

						// 2. Prepare selection options
						var options []huh.Option[string]
						for _, p := range playbooks {
							// Just show the top-level items
							options = append(options, huh.NewOption(p, p))
						}

						var selected []string
						var overwrite bool

						f := huh.NewForm(
							huh.NewGroup(
								huh.NewMultiSelect[string]().
									Title("Select Items to Export").
									Description("Items will be copied to ~/.kuargogo/playbooks/").
									Options(options...).
									Value(&selected),
								huh.NewConfirm().
									Title("Overwrite existing files?").
									Description("If unselected, existing local files will be skipped.").
									Value(&overwrite),
							),
						)

						return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
							if len(selected) == 0 {
								return func() tea.Msg { return actions.ResultMsg{Output: "No items selected."} }
							}
							return actions.ExportPlaybooks(selected, overwrite)
						}))
					},
				},
				{
					Title:       "💾 Delete Context",
					Description: "Remove a saved cluster configuration",
					DynamicChildren: func() []MenuNode {
						nodes := []MenuNode{}
						appCfg := config.GetAppConfig()
						currentCtx := config.GetCurrentContext()

						for name := range appCfg.Contexts {
							ctxName := name
							desc := "Select to delete"
							if ctxName == currentCtx {
								desc = "WARNING: Currently Active"
							}

							nodes = append(nodes, MenuNode{
								Title:       fmt.Sprintf("🗑️  %s", ctxName),
								Description: desc,
								Action: func() tea.Cmd {
									var confirm bool
									f := huh.NewForm(
										huh.NewGroup(
											huh.NewConfirm().
												Title(fmt.Sprintf("Delete Context '%s'?", ctxName)).
												Description("This action cannot be undone.").
												Value(&confirm),
										),
									)

									return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
										if !confirm {
											return func() tea.Msg {
												return actions.ResultMsg{Output: "Cancelled."}
											}
										}
										return actions.ConfigDeleteContext(ctxName)
									}))
								},
							})
						}
						return nodes
					},
				},
			}
		},
	}
}

func createContextSelector() []MenuNode {
	nodes := []MenuNode{}
	// Get thread-safe copy of full config
	appCfg := config.GetAppConfig()
	currentCtx := config.GetCurrentContext()

	// Iterate over contexts map
	for name := range appCfg.Contexts {
		ctxName := name // Capture loop var
		title := ctxName
		desc := "Select to activate"
		if ctxName == currentCtx {
			desc = "Currently Active"
			title = fmt.Sprintf("* %s", title)
		}

		nodes = append(nodes, MenuNode{
			Title:       title,
			Description: desc,
			Action: func() tea.Cmd {
				return engine.Push(NewOutputModel(actions.ConfigSwitchContext(ctxName)))
			},
		})
	}
	return nodes
}

func createNodeSelector(_ string, actionGenerator func(config.Node) func() tea.Cmd) []MenuNode {
	nodes := []MenuNode{}
	cfg := config.GetConfig()
	for _, n := range cfg.Nodes {
		node := n
		nodes = append(nodes, MenuNode{
			Title:       fmt.Sprintf("%s (%s)", node.Name, node.IP),
			Description: node.Role,
			Action:      actionGenerator(node),
		})
	}
	return nodes
}

// parseTags splits a comma-separated tags string into a slice.
// Returns nil if the input is empty (meaning "run all tasks").
func parseTags(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	var tags []string
	for _, t := range strings.Split(input, ",") {
		if trimmed := strings.TrimSpace(t); trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}

// BuildNodeOpsMenu returns a submenu with actions specific to a single node.
// This is used when a node is selected from the Inventory table.
func BuildNodeOpsMenu(n config.Node) MenuNode {
	return MenuNode{
		Title: fmt.Sprintf("Node: %s (%s)", n.Name, n.IP),
		Children: []MenuNode{
			{
				Title:       "⚡ Power On (WoL)",
				Description: "Send Wake-on-LAN magic packet",
				Action:      func() tea.Cmd { return engine.Push(NewOutputModel(actions.PowerOnAction(n))) },
			},
			{
				Title:       "🔌 Power Off (SSH)",
				Description: "Shutdown node safely via SSH",
				Action: func() tea.Cmd {
					var confirm bool
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().
								Title(fmt.Sprintf("Shutdown %s (%s)?", n.Name, n.IP)).
								Value(&confirm),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if !confirm {
							return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled."} }
						}
						return actions.PowerControl(n, provision.PowerOff)
					}))
				},
			},
			{
				Title:       "🔄 Reboot (SSH)",
				Description: "Restart node via SSH",
				Action: func() tea.Cmd {
					var confirm bool
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().
								Title(fmt.Sprintf("Reboot %s (%s)?", n.Name, n.IP)).
								Value(&confirm),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						if !confirm {
							return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled."} }
						}
						return actions.PowerControl(n, provision.PowerReboot)
					}))
				},
			},
			{
				Title:       "🩺 Health Check",
				Description: "Check CPU, Mem, Temp, and K3s status",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.HealthCheck(n)))
				},
			},
			{
				Title:       "🛠️  Provision Node",
				Description: "Install base dependencies (Ansible)",
				Action: func() tea.Cmd {
					var createUser = true
					var tagsInput string
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().Title("Create 'kgg-admin' user?").Value(&createUser),
							huh.NewInput().Title("Tags (optional)").Description("user,packages,swap...").Value(&tagsInput),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						return actions.Provision(n, createUser, parseTags(tagsInput))
					}))
				},
			},
			{
				Title:       "🔑 SSH Key Setup",
				Description: "Copy cluster public key to node",
				Action: func() tea.Cmd {
					var user, password string
					user = "debian"
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().Title("SSH User").Value(&user),
							huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&password),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						return actions.SSHCopy(n.IP, user, password)
					}))
				},
			},
			{
				Title:       "💻 SSH Console",
				Description: "Open interactive shell",
				Action:      func() tea.Cmd { return engine.Push(NewOutputModel(actions.SSHConsole(n))) },
			},
		},
	}
}

// ============================================================================
// GitOps Management Builder
// ============================================================================

func buildGitOpsManagementNode() MenuNode {
	return MenuNode{
		Title:       "⛵ " + i18n.T("menu_gitops"),
		Description: "Manage declarative ArgoCD projects and apps",
		DynamicChildren: func() []MenuNode {
			cfg := config.GetConfig()
			var children []MenuNode

			// Add Credentials Node
			children = append(children, MenuNode{
				Title:       "🔑 Private Repository Credentials",
				Description: "Manage tokens for private git repos",
				DynamicChildren: func() []MenuNode {
					cCfg := config.GetConfig()
					var credNodes []MenuNode
					for i, c := range cCfg.GitOps.Credentials {
						index := i
						credNodes = append(credNodes, MenuNode{
							Title:       fmt.Sprintf("🌐 Repo: %s", c.URL),
							Description: fmt.Sprintf("User: %s", c.Username),
							Action: func() tea.Cmd {
								var confirm bool
								f := huh.NewForm(huh.NewGroup(
									huh.NewConfirm().Title(fmt.Sprintf("Remove credential for %s?", c.URL)).Value(&confirm),
								))
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									if !confirm {
										return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled"} }
									}
									return actions.RemoveGitOpsCredential(index)
								}))
							},
						})
					}
					// Add Credential static button
					credNodes = append(credNodes, MenuNode{
						Title:       "➕ Add New Credential",
						Description: "Add username & token (PAT)",
						Action: func() tea.Cmd {
							var url, user, token, email, registry string
							user = "git"
							f := huh.NewForm(huh.NewGroup(
								huh.NewInput().Title("Repository URL").Description("e.g. https://github.com/my/repo.git").Value(&url).Validate(func(s string) error {
									if s == "" {
										return errors.New("URL is required")
									}
									return nil
								}),
								huh.NewInput().Title("Username").Description("Leave as 'git' for most tokens").Value(&user),
								huh.NewInput().Title("Password/Token").EchoMode(huh.EchoModePassword).Value(&token).Validate(func(s string) error {
									if s == "" {
										return errors.New("Password/Token is required")
									}
									return nil
								}),
								huh.NewInput().Title("Email").Value(&email),
								huh.NewInput().Title("Registry").Description("e.g. https://index.docker.io/v1/").Value(&registry),
							))
							return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
								return actions.AddGitOpsCredential(config.GitOpsCredential{URL: url, Username: user, Password: config.Secret(token), Email: email, Registry: registry})
							}))
						},
					})
					return credNodes
				},
			})

			// Iterate over projects
			for i, p := range cfg.GitOps.Projects {
				projectIndex := i // capture for closure
				projectNode := MenuNode{
					Title:       fmt.Sprintf("📂 Project: %s", p.Name),
					Description: p.Description,
					DynamicChildren: func() []MenuNode {
						// Re-fetch config inside dynamic generator
						currentCfg := config.GetConfig()
						if projectIndex >= len(currentCfg.GitOps.Projects) {
							return nil
						}
						currentProject := currentCfg.GitOps.Projects[projectIndex]
						var appNodes []MenuNode

						// Add Edit Project action
						appNodes = append(appNodes, MenuNode{
							Title:       "📝 Edit Project Details",
							Description: "Change name or description",
							Action: func() tea.Cmd {
								var name = currentProject.Name
								var desc = currentProject.Description
								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("Project Name").Value(&name).Validate(func(s string) error {
											if s == "" {
												return errors.New("name is required")
											}
											return nil
										}),
										huh.NewInput().Title("Description").Value(&desc),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									return actions.UpdateGitOpsProject(projectIndex, config.GitOpsProject{Name: name, Description: desc})
								}))
							},
						})

						// Iterate over apps
						for j, a := range currentProject.Apps {
							appIndex := j
							appNodes = append(appNodes, MenuNode{
								Title:       fmt.Sprintf("📦 App: %s", a.Name),
								Description: fmt.Sprintf("%s -> %s", a.Repo, a.Path),
								DynamicChildren: func() []MenuNode {
									return []MenuNode{
										{
											Title: "✏️ Edit App",
											Action: func() tea.Cmd {
												// re-fetch to ensure safety
												cCfg := config.GetConfig()
												currentApp := cCfg.GitOps.Projects[projectIndex].Apps[appIndex]

												var appName = currentApp.Name
												var appRepo = currentApp.Repo
												var appPath = currentApp.Path
												var appNamespace = currentApp.Namespace
												var appBranch = currentApp.Branch

												f := huh.NewForm(
													huh.NewGroup(
														huh.NewInput().Title("App Name").Value(&appName),
														huh.NewInput().Title("Git Repository (HTTPS)").Value(&appRepo),
														huh.NewInput().Title("Manifests Path").Value(&appPath),
														huh.NewInput().Title("Target Namespace").Value(&appNamespace),
														huh.NewInput().Title("Branch").Value(&appBranch),
													),
												)
												return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
													newApp := config.GitOpsApp{Name: appName, Repo: appRepo, Path: appPath, Namespace: appNamespace, Branch: appBranch}
													return actions.UpdateGitOpsApp(projectIndex, appIndex, newApp)
												}))
											},
										},
										{
											Title: "❌ Remove App",
											Action: func() tea.Cmd {
												var confirm bool
												f := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Remove App?").Value(&confirm)))
												return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
													if !confirm {
														return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled"} }
													}
													return actions.RemoveGitOpsApp(projectIndex, appIndex)
												}))
											},
										},
									}
								},
							})
						}

						// Add App Button
						appNodes = append(appNodes, MenuNode{
							Title: "➕ Add New App",
							Action: func() tea.Cmd {
								var appName, appRepo, appPath, appNamespace, appBranch string
								appBranch = "main"
								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("App Name").Value(&appName).Validate(func(s string) error {
											if s == "" {
												return errors.New("name is required")
											}
											return nil
										}),
										huh.NewInput().Title("Git Repository (HTTPS)").Value(&appRepo).Validate(func(s string) error {
											if s == "" {
												return errors.New("repo is required")
											}
											return nil
										}),
										huh.NewInput().Title("Manifests Path").Value(&appPath),
										huh.NewInput().Title("Target Namespace").Value(&appNamespace),
										huh.NewInput().Title("Branch").Value(&appBranch),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									newApp := config.GitOpsApp{Name: appName, Repo: appRepo, Path: appPath, Namespace: appNamespace, Branch: appBranch}
									return actions.AddGitOpsApp(projectIndex, newApp)
								}))
							},
						})

						// Remove Project Button
						appNodes = append(appNodes, MenuNode{
							Title: "❌ Remove Project",
							Action: func() tea.Cmd {
								var confirm bool
								f := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Remove Project and ALL Apps?").Value(&confirm)))
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									if !confirm {
										return func() tea.Msg { return actions.ResultMsg{Output: "Cancelled"} }
									}
									return actions.RemoveGitOpsProject(projectIndex)
								}))
							},
						})

						return appNodes
					},
				}
				children = append(children, projectNode)
			}

			// Add Project Button
			children = append(children, MenuNode{
				Title: "➕ Add New Project",
				Action: func() tea.Cmd {
					var name, desc string
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().Title("Project Name").Value(&name).Validate(func(s string) error {
								if s == "" {
									return errors.New("name is required")
								}
								return nil
							}),
							huh.NewInput().Title("Description").Value(&desc),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						return actions.AddGitOpsProject(config.GitOpsProject{Name: name, Description: desc})
					}))
				},
			})

			// Sync GitOps Projects Action
			children = append(children, MenuNode{
				Title:       "🔄 Sync GitOps Projects (ArgoCD)",
				Description: "Reconcile projects and apps with ArgoCD",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.SyncGitOpsState()))
				},
			})

			// Sync Pull Secrets Action
			children = append(children, MenuNode{
				Title:       "🔑 Sync Image Pull Secrets",
				Description: "Create/update registry secrets in all namespaces",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.SyncPullSecrets()))
				},
			})

			return children
		},
	}
}

// ============================================================================
// Kargo Promotion Builder
// ============================================================================

func buildKargoPromotionNode() MenuNode {
	return MenuNode{
		Title:       "🚢 " + i18n.T("menu_kargo"),
		Description: "Promote freight across stages",
		DynamicChildren: func() []MenuNode {
			cfg := config.GetConfig()
			var children []MenuNode

			editKargoPipelineAction := func(k *config.KargoPipeline, index int) tea.Cmd {
				var stagesStr string
				for i, s := range k.Stages {
					if i > 0 {
						stagesStr += ","
					}
					val := s.Name
					if s.Path != "" {
						val += ":" + s.Path
					}
					stagesStr += val
				}

				var additionalImagesStr string
				if len(k.Warehouse.AdditionalImages) > 0 {
					additionalImagesStr = strings.Join(k.Warehouse.AdditionalImages, ",")
				}

				f := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().Title("Pipeline Name").Description("Internal name for this pipeline (e.g. auth-service)").Value(&k.Name),
						huh.NewInput().Title("Namespace").Value(&k.Namespace),
						huh.NewInput().Title("Project Name").Value(&k.Project),
						huh.NewInput().Title("Warehouse Name").Value(&k.Warehouse.Name),
						huh.NewInput().Title("Main Image Repository (OCI/Git)").Value(&k.Warehouse.Repo),
						huh.NewInput().Title("Additional Images (comma separated)").Value(&additionalImagesStr),
						huh.NewInput().Title("Semver Constraint").Description("e.g. ^1.0.0 (optional)").Value(&k.Warehouse.Semver),
						huh.NewSelect[string]().Title("Image Selection Strategy").
							Options(
								huh.NewOption("SemVer (Default)", ""),
								huh.NewOption("Lexical (e.g. for hashes)", "Lexical"),
								huh.NewOption("Digest", "Digest"),
							).Value(&k.Warehouse.ImageSelectionStrategy),
						huh.NewInput().Title("Allow Tags (Regex)").Description("e.g. ^[a-f0-9]{7,8}$ (optional, use with Lexical)").Value(&k.Warehouse.AllowTags),
						huh.NewInput().Title("Git Ops Repository URL").Description("Used by Stages to write updates (Warehouse Path)").Value(&k.Warehouse.Path),
						huh.NewInput().Title("Stages (comma separated)").Description("Format: name:path (e.g. dev:environments/dev,prod:environments/prod)").Value(&stagesStr),
					),
				)

				return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
					// Parse additional images
					k.Warehouse.AdditionalImages = nil
					if additionalImagesStr != "" {
						for _, img := range strings.Split(additionalImagesStr, ",") {
							cleanImg := strings.TrimSpace(img)
							if cleanImg != "" {
								k.Warehouse.AdditionalImages = append(k.Warehouse.AdditionalImages, cleanImg)
							}
						}
					}

					// Parse stages (support name:path format and auto-link pipeline)
					if stagesStr != "" {
						k.Stages = nil
						var prevStage string
						for _, s := range strings.Split(stagesStr, ",") {
							parts := strings.Split(strings.TrimSpace(s), ":")
							name := parts[0]
							path := ""
							if len(parts) > 1 {
								path = parts[1]
							}
							if name != "" {
								stage := config.KargoStage{
									Name: name,
									Path: path,
								}
								// Auto-link to previous stage in the list for a linear pipeline
								if prevStage != "" {
									stage.Requires = []string{prevStage}
								}
								k.Stages = append(k.Stages, stage)
								prevStage = name
							}
						}
					}
					return actions.ConfigUpdateKargoPipeline(*k, index)
				}))
			}

			// 1. List Existing Pipelines
			for i := range cfg.GitOps.Pipelines {
				idx := i
				pipe := &cfg.GitOps.Pipelines[idx]
				children = append(children, MenuNode{
					Title:       fmt.Sprintf("⚙️  Configure Pipeline: %s", pipe.Name),
					Description: fmt.Sprintf("Namespace: %s, Project: %s", pipe.Namespace, pipe.Project),
					Action: func() tea.Cmd {
						return editKargoPipelineAction(pipe, idx)
					},
				})
			}

			// 2. Add New Pipeline
			children = append(children, MenuNode{
				Title:       "➕ Add New Pipeline",
				Description: "Create a new Kargo promotion pipeline for another service",
				Action: func() tea.Cmd {
					newPipe := config.KargoPipeline{
						Namespace: "kargo",
						Project:   "homelab",
						Warehouse: config.KargoWarehouse{
							Name: "default",
						},
					}
					return editKargoPipelineAction(&newPipe, -1)
				},
			})

			// 2. Deployment
			children = append(children, MenuNode{
				Title:       "🚀 Deploy Kargo Engine",
				Description: "Install Kargo in the cluster via Ansible",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.OpsKargo()))
				},
			})

			// 3. Operational views
			children = append(children,
				MenuNode{
					Title:       "🔄 Sync Kargo State",
					Description: "Reconcile Warehouse and Stages for all pipelines",
					Action: func() tea.Cmd {
						return engine.Push(NewOutputModel(actions.SyncKargoState()))
					},
				},
			)

			for _, pipe := range cfg.GitOps.Pipelines {
				p := pipe
				// Add a pipeline-scoped Monitor Dashboard option
				children = append(children, MenuNode{
					Title:       fmt.Sprintf("🔍 [%s] Monitor Pipeline Dashboard", p.Name),
					Description: fmt.Sprintf("Live Freight tracking, stages health and promotion for %s", p.Name),
					Action: func() tea.Cmd {
						return engine.Push(NewKargoDashboardModel(p.Name))
					},
				})

				// Add a pipeline-scoped List Available Freight node
				children = append(children, MenuNode{
					Title:       fmt.Sprintf("📦 [%s] List Available Freight", p.Name),
					Description: fmt.Sprintf("Show trackable artifact bundles for pipeline %s", p.Name),
					Action: func() tea.Cmd {
						return engine.Push(NewOutputModel(actions.GetKargoFreight(p.Name)))
					},
				})

				for _, stage := range p.Stages {
					s := stage
					children = append(children, MenuNode{
						Title:       fmt.Sprintf("🚀 [%s] Promote to %s", p.Name, strings.ToUpper(s.Name)),
						Description: fmt.Sprintf("Promote freight in pipeline %s to %s stage", p.Name, s.Name),
						Action: func() tea.Cmd {
							var freightID string
							f := huh.NewForm(huh.NewGroup(
								huh.NewInput().Title("Freight ID").
									Description(fmt.Sprintf("Enter the ID of the freight to promote to %s", s.Name)).
									Value(&freightID).
									Validate(func(val string) error {
										if val == "" {
											return fmt.Errorf("freight ID is required")
										}
										return nil
									}),
							))

							return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
								return engine.Push(NewOutputModel(actions.PromoteStage(p.Name, s.Name, freightID)))
							}))
						},
					})
				}
			}

			return children
		},
	}
}

func buildAIInsightsNode() MenuNode {
	return MenuNode{
		Title:       "AI Insights & Skill",
		Description: "Diagnostics & Agent Context",
		Children: []MenuNode{
			{
				Title:       "💓 Cluster Health (AI)",
				Description: "Deep scan and preventive post-mortem",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.AIHealth(true)))
				},
			},
			{
				Title:       "📝 Generate Skill Context",
				Description: "Create skill.md for external agents",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.GenerateSkill()))
				},
			},
		},
	}
}

func buildDomainsAndServicesNode() MenuNode {
	return MenuNode{
		Title:       "🌐 " + i18n.T("menu_services"),
		Description: i18n.T("menu_services_desc"),
		DynamicChildren: func() []MenuNode {
			cfg := config.GetConfig()
			var children []MenuNode

			// 1. Sync ALL domains
			children = append(children, MenuNode{
				Title:       "✨ Sincronizar Todos los Dominios",
				Description: "Aplica cambios en túnel y políticas para todos los dominios",
				Action: func() tea.Cmd {
					return engine.Push(NewOutputModel(actions.SyncCloudflare()))
				},
			})

			// 2. Add New Domain
			children = append(children, MenuNode{
				Title:       "➕ Añadir Dominio",
				Description: "Registrar un nuevo dominio en el túnel",
				Action: func() tea.Cmd {
					var domain string
					accessEnabled := true
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().Title("Dominio").Description("e.g. midominio.com").Value(&domain).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return errors.New("el dominio es requerido")
									}
									return nil
								}),
							huh.NewConfirm().Title("¿Activar Zero Trust Access?").
								Description("Protege los servicios marcados con email OTP").
								Value(&accessEnabled),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						return actions.AddCloudflareDomain(domain, accessEnabled)
					}))
				},
			})

			// 3. One node per domain
			for i, d := range cfg.Cloudflare.Domains {
				domainIndex := i
				dom := d
				accessIcon := "🔓"
				if dom.AccessEnabled {
					accessIcon = "🛡️ "
				}

				children = append(children, MenuNode{
					Title:       fmt.Sprintf("%s %s", accessIcon, dom.Domain),
					Description: fmt.Sprintf("%d servicios configurados", len(dom.Services)),
					DynamicChildren: func() []MenuNode {
						var domChildren []MenuNode

						// Sync this domain only
						domChildren = append(domChildren, MenuNode{
							Title:       "✨ Sincronizar este Dominio",
							Description: "Aplica cambios solo para " + dom.Domain,
							Action: func() tea.Cmd {
								return engine.Push(NewOutputModel(actions.SyncCloudflareDomain(domainIndex)))
							},
						})

						// Add new service
						domChildren = append(domChildren, MenuNode{
							Title:       "➕ Añadir Servicio",
							Description: "Exponer una nueva URL interna bajo " + dom.Domain,
							Action: func() tea.Cmd {
								var name, sub, target string
								protected := true
								f := huh.NewForm(
									huh.NewGroup(
										huh.NewInput().Title("Nombre del Servicio").Description("e.g. Grafana Monitoring").Value(&name).
											Validate(func(s string) error {
												if s == "" {
													return errors.New("el nombre es requerido")
												}
												return nil
											}),
										huh.NewInput().Title("Subdominio").Description("e.g. grafana  (→ grafana."+dom.Domain+")").Value(&sub).
											Validate(func(s string) error {
												if s == "" {
													return errors.New("el subdominio es requerido")
												}
												return nil
											}),
										huh.NewInput().Title("Target (Interno)").Description("e.g. http://service.namespace.svc.cluster.local:80").Value(&target).
											Validate(func(s string) error {
												if s == "" {
													return errors.New("el target es requerido")
												}
												return nil
											}),
										huh.NewConfirm().Title("¿Proteger con Zero Trust?").Description("Requiere email OTP para acceder").Value(&protected),
									),
								)
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									return actions.AddCloudflareService(domainIndex, name, sub, target, protected)
								}))
							},
						})

						// Remove domain
						domChildren = append(domChildren, MenuNode{
							Title:       "🗑️  Eliminar Dominio",
							Description: "Quitar " + dom.Domain + " y todos sus servicios",
							Action: func() tea.Cmd {
								var confirm bool
								f := huh.NewForm(huh.NewGroup(
									huh.NewConfirm().Title(fmt.Sprintf("¿Eliminar %s?", dom.Domain)).
										Description("Se eliminarán también todos sus servicios.").
										Value(&confirm),
								))
								return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
									if !confirm {
										return func() tea.Msg { return actions.ResultMsg{Output: "Cancelado"} }
									}
									return actions.RemoveCloudflareDomain(domainIndex)
								}))
							},
						})

						// Services list
						for j, svc := range dom.Services {
							svcIndex := j
							s := svc
							statusIcon := "🔓"
							if s.Protected {
								statusIcon = "🛡️ "
							}
							hostname := s.Subdomain + "." + dom.Domain
							if s.Subdomain == "" || s.Subdomain == "@" {
								hostname = dom.Domain
							}

							domChildren = append(domChildren, MenuNode{
								Title:       fmt.Sprintf("%s %s", statusIcon, s.Name),
								Description: fmt.Sprintf("https://%s → %s", hostname, s.Target),
								DynamicChildren: func() []MenuNode {
									return []MenuNode{
										{
											Title:       "✏️  Editar Servicio",
											Description: "Modificar nombre, ruta o protección",
											Action: func() tea.Cmd {
												name := s.Name
												sub := s.Subdomain
												target := s.Target
												protected := s.Protected
												f := huh.NewForm(
													huh.NewGroup(
														huh.NewInput().Title("Nombre del Servicio").Value(&name),
														huh.NewInput().Title("Subdominio").Value(&sub),
														huh.NewInput().Title("Target (Interno)").Value(&target),
														huh.NewConfirm().Title("¿Proteger con Zero Trust?").Value(&protected),
													),
												)
												return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
													return actions.UpdateCloudflareService(domainIndex, svcIndex, name, sub, target, protected)
												}))
											},
										},
										{
											Title:       "🗑️  Eliminar Servicio",
											Description: "Quitar este servicio de la configuración",
											Action: func() tea.Cmd {
												var confirm bool
												f := huh.NewForm(huh.NewGroup(
													huh.NewConfirm().Title(fmt.Sprintf("¿Eliminar %s?", s.Name)).Value(&confirm),
												))
												return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
													if !confirm {
														return func() tea.Msg { return actions.ResultMsg{Output: "Cancelado"} }
													}
													return actions.RemoveCloudflareService(domainIndex, svcIndex)
												}))
											},
										},
									}
								},
							})
						}

						return domChildren
					},
				})
			}

			return children
		},
	}
}

func buildCloudSyncNode() MenuNode {
	return MenuNode{
		Title:       "☁️ Kuargogo Cloud Sync & Backup",
		Description: "Secure off-site E2E encrypted configuration storage",
		DynamicChildren: func() []MenuNode {
			sync := config.RootConfigGetSync()
			var children []MenuNode

			masterKey, _ := config.GetMasterKey()
			isConfigured := sync.S3.S3AccessKey != "" && sync.S3.S3Url != "" && sync.S3.S3Bucket != ""
			hasPassphrase := masterKey != ""

			if !isConfigured {
				// Not configured yet — show setup
				children = append(children, MenuNode{
					Title:       "⚙️  Setup S3 Credentials",
					Description: "Configure Cloudflare R2 or AWS S3 bucket",
					Action: func() tea.Cmd {
						sync := config.RootConfigGetSync()
						s3 := sync.S3
						s3AccessKey := string(s3.S3AccessKey)
						s3SecretKey := string(s3.S3SecretKey)
						var importFromContext bool

						f := huh.NewForm(
							huh.NewGroup(
								huh.NewConfirm().
									Title("Import from active cluster context?").
									Description("Reuse the S3/Velero backup credentials already configured").
									Value(&importFromContext),
							),
							huh.NewGroup(
								huh.NewInput().Title("Endpoint URL").
									Description("e.g. https://<id>.r2.cloudflarestorage.com or https://s3.amazonaws.com").
									Value(&s3.S3Url),
								huh.NewInput().Title("Bucket Name").Value(&s3.S3Bucket),
								huh.NewInput().Title("Folder Prefix").Description("Optional: e.g. 'backups/' or 'kuargogo/'").Value(&s3.S3Prefix),
								huh.NewInput().Title("Access Key ID").Value(&s3AccessKey),
								huh.NewInput().Title("Secret Access Key").EchoMode(huh.EchoModePassword).Value(&s3SecretKey),
								huh.NewInput().Title("Region").Description("Use 'auto' for Cloudflare R2, or a region like 'eu-west-1' for AWS").Value(&s3.S3Region),
							).WithHideFunc(func() bool { return importFromContext }),
						)
						return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
							return func() tea.Msg {
								if importFromContext {
									s3 = config.GetConfig().Backup
								} else {
									s3.S3AccessKey = config.Secret(s3AccessKey)
									s3.S3SecretKey = config.Secret(s3SecretKey)
								}
								config.RootConfigSetS3(s3)
								if err := config.SaveConfig(); err != nil {
									return actions.ResultMsg{Output: "❌ " + err.Error()}
								}
								return actions.ResultMsg{Output: "✅ S3 credentials saved."}
							}
						}))
					},
				})
			} else if !hasPassphrase {
				// Configured but no passphrase yet
				children = append(children, MenuNode{
					Title:       fmt.Sprintf("📦 %s / %s", sync.S3.S3Bucket, sync.S3.S3Region),
					Description: "S3 credentials configured",
				})
				children = append(children, MenuNode{
					Title:       "🔐 Set Master Passphrase",
					Description: "Required to encrypt/decrypt your config backups",
					Action: func() tea.Cmd {
						var passphrase string
						f := huh.NewForm(
							huh.NewGroup(
								huh.NewInput().
									Title("Master Passphrase").
									Description("⚠️ CRITICAL: If lost, your backups cannot be recovered. Store it safely.").
									EchoMode(huh.EchoModePassword).
									Value(&passphrase).
									Validate(func(s string) error {
										if len(s) < 8 {
											return errors.New("passphrase must be at least 8 characters")
										}
										return nil
									}),
							),
						)
						return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
							return func() tea.Msg {
								if err := config.StoreMasterKey(passphrase); err != nil {
									return actions.ResultMsg{Output: "❌ Failed to store passphrase: " + err.Error()}
								}
								return actions.ResultMsg{Output: "✅ Master passphrase stored in OS keychain. You can now backup and restore."}
							}
						}))
					},
				})
			} else {
				// Fully configured — show operational options
				lastSync := sync.LastSync
				if lastSync == "" {
					lastSync = "never"
				}
				children = append(children, MenuNode{
					Title:       fmt.Sprintf("📦 %s  ·  %s", sync.S3.S3Bucket, sync.S3.S3Url),
					Description: fmt.Sprintf("Last sync: %s", lastSync),
				})

				children = append(children, MenuNode{
					Title:       "⬆️  Backup current config",
					Description: "Encrypt and upload to S3",
					Action: func() tea.Cmd {
						return engine.Push(NewOutputModel(actions.SyncPush()))
					},
				})

				children = append(children, MenuNode{
					Title:       "⬇️  Restore from cloud",
					Description: "Download, decrypt and overwrite local config",
					Action: func() tea.Cmd {
						var master string
						f := huh.NewForm(
							huh.NewGroup(
								huh.NewInput().Title("Master Passphrase").
									Description("Enter the key used to encrypt your backup").
									EchoMode(huh.EchoModePassword).Value(&master),
							),
						)
						return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
							return actions.SyncPull(master)
						}))
					},
				})

				children = append(children, MenuNode{
					Title:       "🔐 Change Master Passphrase",
					Description: "Update the encryption key stored in OS keychain",
					Action: func() tea.Cmd {
						var passphrase string
						f := huh.NewForm(
							huh.NewGroup(
								huh.NewInput().
									Title("New Master Passphrase").
									Description("⚠️ Store it safely — losing it means losing access to your backups.").
									EchoMode(huh.EchoModePassword).
									Value(&passphrase).
									Validate(func(s string) error {
										if len(s) < 8 {
											return errors.New("passphrase must be at least 8 characters")
										}
										return nil
									}),
							),
						)
						return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
							return func() tea.Msg {
								if err := config.StoreMasterKey(passphrase); err != nil {
									return actions.ResultMsg{Output: "❌ " + err.Error()}
								}
								return actions.ResultMsg{Output: "✅ Passphrase updated."}
							}
						}))
					},
				})

				children = append(children, MenuNode{
					Title:       "⚙️  Edit S3 Credentials",
					Description: "Change bucket, endpoint or keys",
					Action: func() tea.Cmd {
						sync := config.RootConfigGetSync()
						s3 := sync.S3
						s3AccessKey := string(s3.S3AccessKey)
						s3SecretKey := string(s3.S3SecretKey)
						f := huh.NewForm(
							huh.NewGroup(
								huh.NewInput().Title("Endpoint URL").Value(&s3.S3Url),
								huh.NewInput().Title("Bucket Name").Value(&s3.S3Bucket),
								huh.NewInput().Title("Folder Prefix").Value(&s3.S3Prefix),
								huh.NewInput().Title("Access Key ID").Value(&s3AccessKey),
								huh.NewInput().Title("Secret Access Key").EchoMode(huh.EchoModePassword).Value(&s3SecretKey),
								huh.NewInput().Title("Region").Description("Use 'auto' for R2").Value(&s3.S3Region),
							),
						)
						return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
							return func() tea.Msg {
								s3.S3AccessKey = config.Secret(s3AccessKey)
								s3.S3SecretKey = config.Secret(s3SecretKey)
								config.RootConfigSetS3(s3)
								if err := config.SaveConfig(); err != nil {
									return actions.ResultMsg{Output: "❌ " + err.Error()}
								}
								return actions.ResultMsg{Output: "✅ Credentials updated."}
							}
						}))
					},
				})

				children = append(children, MenuNode{
					Title:       "🗑️  Clear credentials",
					Description: "Remove S3 config and passphrase from this machine",
					Action: func() tea.Cmd {
						return engine.Push(NewOutputModel(actions.SyncLogout()))
					},
				})
			}

			return children
		},
	}
}
func buildSecurityVaultNode() MenuNode {
	return MenuNode{
		Title:       "🔐 " + i18n.T("menu_security_vault"),
		Description: i18n.T("menu_security_vault_desc"),
		Children: []MenuNode{
			{
				Title:       "🔑 Set/Update Master Passphrase",
				Description: "Encrypt configuration and store key in OS Keychain",
				Action: func() tea.Cmd {
					var passphrase string
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Set Master Passphrase").
								Description("This key encrypts your kuargogo.yaml. If you are in WSL, set KGG_MASTER_PASSPHRASE env var if needed.").
								EchoMode(huh.EchoModePassword).
								Value(&passphrase).
								Validate(func(s string) error {
									if len(s) < 8 {
										return errors.New("passphrase must be at least 8 characters")
									}
									return nil
								}),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						return actions.OpsSetMasterPassphrase(passphrase)
					}))
				},
			},
			{
				Title:       "📄 View Decrypted Config",
				Description: "Reveal plaintext configuration (Sudo-style challenge)",
				Action: func() tea.Cmd {
					var passphrase string
					f := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Security Challenge").
								Description("⚠️  WARNING: High Sensitivity Area. You are about to reveal secrets in plain text. Enter Master Passphrase:").
								EchoMode(huh.EchoModePassword).
								Value(&passphrase),
						),
					)
					return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
						return actions.OpsViewDecryptedConfig(passphrase)
					}))
				},
			},
		},
	}
}

func triggerCNPGRestoreForm(namespace, sourceCluster, timeStr string) tea.Cmd {
	targetCluster := sourceCluster + "-pitr"
	force := false
	var confirm bool

	f := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Namespace").
				Description("Namespace of the cluster").
				Value(&namespace).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("namespace is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Source Cluster Name").
				Description("Name of the cluster to recover").
				Value(&sourceCluster).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("source cluster name is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Target Recovery Time").
				Description("UTC timestamp format: YYYY-MM-DD HH:MM:SS or RFC3339").
				Value(&timeStr).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("target time is required")
					}
					_, err := cluster.ParseTargetTime(s)
					if err != nil {
						return fmt.Errorf("invalid time: %w", err)
					}
					return nil
				}),
			huh.NewInput().
				Title("Target Cluster Name").
				Description("Name for the recovered cluster").
				Value(&targetCluster).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("target cluster name is required")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Overwrite Active Cluster? (In-Place Restore)").
				Description("If yes, deletes active cluster & PVCs first. Target Name is ignored.").
				Value(&force),
			huh.NewConfirm().
				Title("Proceed with Restore?").
				Value(&confirm),
		),
	)

	return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
		if !confirm {
			return func() tea.Msg { return actions.ResultMsg{Output: "Restoration cancelled."} }
		}

		if force {
			var confirmInput string
			fConfirm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(fmt.Sprintf("⚠️ DANGER: In-place restore will DELETE cluster %q and all its data. Type the cluster name to confirm:", sourceCluster)).
						Value(&confirmInput).
						Validate(func(s string) error {
							if s != sourceCluster {
								return fmt.Errorf("must type %q exactly", sourceCluster)
							}
							return nil
						}),
				),
			)
			return engine.Push(NewFormModel(fConfirm, func(form *huh.Form) tea.Cmd {
				return actions.OpsRestoreCNPGCluster(sourceCluster, sourceCluster, namespace, timeStr, true)
			}))
		}

		return actions.OpsRestoreCNPGCluster(sourceCluster, targetCluster, namespace, timeStr, false)
	}))
}

