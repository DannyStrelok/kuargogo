package cluster

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

type DiskStatus struct {
	NodeName  string `json:"node_name"`
	DiskID    string `json:"disk_id"`
	Path      string `json:"path"`
	Device    string `json:"device"`
	Status    string `json:"status"` // "healthy", "failing", "missing_smartctl", "unsupported", "unknown"
	Message   string `json:"message"`
	Healed    bool   `json:"healed"`
}

type LonghornNodeList struct {
	Items []LonghornNode `json:"items"`
}

type LonghornNode struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Disks map[string]LonghornDiskSpec `json:"disks"`
	} `json:"spec"`
}

type LonghornDiskSpec struct {
	Path              string `json:"path"`
	AllowScheduling   bool   `json:"allowScheduling"`
	EvictionRequested bool   `json:"evictionRequested"`
}

// CheckAndHealStorage checks the SMART health of all disks registered in Longhorn
// and optionally triggers eviction/disables scheduling on failing disks.
func (m *Manager) CheckAndHealStorage(masterNode *config.Node, heal bool) ([]DiskStatus, error) {
	var out string

	if m.DryRun {
		// Mock data for dry-run
		out = `{
			"items": [
				{
					"metadata": {
						"name": "test-worker"
					},
					"spec": {
						"disks": {
							"disk-1": {
								"path": "/var/lib/longhorn",
								"allowScheduling": true,
								"evictionRequested": false
							}
						}
					}
				},
				{
					"metadata": {
						"name": "failing-node"
					},
					"spec": {
						"disks": {
							"disk-fail": {
								"path": "/mnt/data",
								"allowScheduling": true,
								"evictionRequested": false
							}
						}
					}
				}
			]
		}`
	} else {
		cmd := "sudo k3s kubectl get nodes.longhorn.io -n longhorn-system -o json"
		executor, err := m.getExecutor()
		if err != nil {
			return nil, fmt.Errorf("failed to get executor: %w", err)
		}

		out, err = executor.ExecuteCommand(masterNode.IP, m.Port, cmd)
		if err != nil {
			return nil, fmt.Errorf("failed to query Longhorn nodes: %w", err)
		}
	}

	var nodeList LonghornNodeList
	if err := json.Unmarshal([]byte(out), &nodeList); err != nil {
		return nil, fmt.Errorf("failed to parse Longhorn nodes JSON: %w", err)
	}

	var results []DiskStatus
	cfg := config.GetConfig()

	for _, lhNode := range nodeList.Items {
		// Find matching config node
		var targetNode *config.Node
		for i := range cfg.Nodes {
			if cfg.Nodes[i].Name == lhNode.Metadata.Name {
				targetNode = &cfg.Nodes[i]
				break
			}
		}

		// If the node is not in config, we skip it
		if targetNode == nil {
			continue
		}

		for diskID, diskSpec := range lhNode.Spec.Disks {
			status := DiskStatus{
				NodeName: lhNode.Metadata.Name,
				DiskID:   diskID,
				Path:     diskSpec.Path,
				Status:   "unknown",
			}

			var device string
			var checkErr error
			var isFailing bool
			var isMissingSmartctl bool
			var isUnsupported bool

			if m.DryRun {
				device = "/dev/sdb"
				if lhNode.Metadata.Name == "failing-node" {
					isFailing = true
				}
			} else {
				// 1. Get block device from path
				executor, err := m.getExecutor()
				if err != nil {
					status.Message = fmt.Sprintf("Executor error: %v", err)
					results = append(results, status)
					continue
				}

				dfCmd := fmt.Sprintf("df -P %s | tail -1 | awk '{print $1}'", diskSpec.Path)
				mountSource, err := executor.ExecuteCommand(targetNode.IP, m.Port, dfCmd)
				if err != nil {
					status.Message = fmt.Sprintf("Failed to get device for path: %v", err)
					results = append(results, status)
					continue
				}

				device = getRawBlockDevice(strings.TrimSpace(mountSource))
				if device == "" {
					status.Message = fmt.Sprintf("Could not resolve raw device from mount source: %s", mountSource)
					results = append(results, status)
					continue
				}

				// 2. Run SMART check
				smartCmd := fmt.Sprintf("sudo smartctl -H %s", device)
				smartOut, err := executor.ExecuteCommand(targetNode.IP, m.Port, smartCmd)

				// Analyze exit code or output
				if err != nil {
					// Check if smartctl is missing (usually exit status 127)
					if strings.Contains(err.Error(), "127") || strings.Contains(smartOut, "command not found") {
						isMissingSmartctl = true
					} else if strings.Contains(err.Error(), "status 8") || strings.Contains(smartOut, "FAILED") {
						// exit code 8 indicates SMART status check failed (Bit 3)
						isFailing = true
					} else {
						// For VM/virtual drives, smartctl returns exit status 2 or "Device does not support SMART"
						isUnsupported = true
						checkErr = err
					}
				} else {
					// Exit code 0, check output content just in case
					if strings.Contains(smartOut, "FAILED") || strings.Contains(smartOut, "FAILING") {
						isFailing = true
					}
				}
			}

			status.Device = device

			if isMissingSmartctl {
				status.Status = "missing_smartctl"
				status.Message = "smartctl is not installed on the node."
			} else if isFailing {
				status.Status = "failing"
				status.Message = "SMART self-assessment test returned FAILED!"

				// Trigger Healing
				if heal {
					if m.DryRun {
						status.Healed = true
					} else {
						// Patch nodes.longhorn.io
						patchCmd := fmt.Sprintf(`sudo k3s kubectl patch nodes.longhorn.io %s -n longhorn-system --type merge -p '{"spec": {"disks": {"%s": {"allowScheduling": false, "evictionRequested": true}}}}'`,
							lhNode.Metadata.Name, diskID)
						
						executor, err := m.getExecutor()
						if err == nil {
							_, err = executor.ExecuteCommand(masterNode.IP, m.Port, patchCmd)
							if err == nil {
								status.Healed = true
								status.Message += " [HEALED: Scheduling disabled and eviction requested]"
							} else {
								status.Message += fmt.Sprintf(" [HEALING FAILED: %v]", err)
							}
						} else {
							status.Message += fmt.Sprintf(" [HEALING FAILED: %v]", err)
						}
					}
				}
			} else if isUnsupported {
				status.Status = "unsupported"
				status.Message = fmt.Sprintf("SMART check is unsupported or failed to run: %v", checkErr)
			} else {
				status.Status = "healthy"
				status.Message = "SMART check passed."
			}

			results = append(results, status)
		}
	}

	return results, nil
}

func getRawBlockDevice(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	if strings.HasPrefix(source, "/dev/nvme") {
		idx := strings.LastIndex(source, "p")
		if idx > 0 {
			suffix := source[idx+1:]
			if isDigits(suffix) {
				return source[:idx]
			}
		}
		return source
	}
	return strings.TrimFunc(source, func(r rune) bool {
		return r >= '0' && r <= '9'
	})
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
