package infra

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/DannyStrelok/kuargogo/internal/ai"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/provision"
)

// NodeHealth represents the status of a single node
type NodeHealth struct {
	NodeName    string
	IP          string
	Status      string // ONLINE, OFFLINE, ERROR
	CPUUsage    string
	RAMUsage    string
	DiskUsage   string
	Services    map[string]string // service_name: status
	RecentLogs  string
	FailingPods []string // New: Pods not in Running state (only for servers)
	CPUTemp     string   // CPU Temperature in Celsius
	Error       error
}

// HealthReport represents the full cluster diagnostic
type HealthReport struct {
	Nodes         []NodeHealth
	Summary       string
	RepairActions []string // New: Structured actions for remediation
}

// RunHealthCheck performs a cluster-wide health check and optionally uses AI for diagnostics.
func (m *Manager) RunHealthCheck(aiEnabled bool) (*HealthReport, error) {
	cfg := config.GetConfig()
	report := &HealthReport{}

	var wg sync.WaitGroup
	results := make(chan NodeHealth, len(cfg.Nodes))

	executor, err := provision.NewExecutor(m.User, m.KeyPath, m.DryRun)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize executor: %w", err)
	}

	for _, node := range cfg.Nodes {
		wg.Add(1)
		go func(n config.Node) {
			defer wg.Done()
			results <- m.performNodeDiagnostic(executor, n)
		}(node)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		report.Nodes = append(report.Nodes, res)
	}

	if aiEnabled {
		_, _ = fmt.Fprintln(m.Output, "Analyzing health data with AI Performance Engine...")

		aiClient, err := ai.NewClient(cfg.AI, m.DryRun)
		if err != nil {
			// Fallback: Generate a deterministic summary if AI is not configured or fails
			report.Summary = m.generateBasicSummary(report)

			// Append the AI error as a note if it wasn't just a configuration issue
			if cfg.AI.Provider != "" {
				report.Summary += fmt.Sprintf("\n\n⚠️ (AI analysis failed: %v)", err)
			}
		} else {
			prompt := m.buildAIPrompt(report)

			var aiBuf strings.Builder
			multiWriter := io.MultiWriter(m.Output, &aiBuf)
			aiClient.SetOutput(multiWriter)

			if err := aiClient.Generate(prompt); err != nil {
				report.Summary = fmt.Sprintf("AI Generation failed: %v", err)
			} else {
				report.Summary = aiBuf.String()
				report.RepairActions = m.parseRepairActions(report.Summary)
			}
		}
	}

	return report, nil
}

func (m *Manager) performNodeDiagnostic(executor *provision.Executor, node config.Node) NodeHealth {
	health := NodeHealth{
		NodeName: node.Name,
		IP:       node.IP,
		Status:   "ONLINE",
		Services: make(map[string]string),
	}

	// Basic Connectivity & Stats
	// Collecting: Uptime Load | RAM usage | Disk usage | CPU Temp
	checkCmd := `TEMP=$(cat /sys/class/thermal/thermal_zone0/temp 2>/dev/null || echo "0"); echo "STATS|$(uptime | awk -F'load average:' '{ print $2 }' | xargs)|$(free -m | awk 'NR==2{printf "%d/%dMB (%.2f%%)", $3,$2,$3*100/$2 }')|$(df -h / | awk 'NR==2{print $5}')|$(echo "scale=1; $TEMP/1000" | bc -l 2>/dev/null || echo "N/A")°C"`

	out, err := executor.ExecuteCommand(node.IP, m.Port, checkCmd)
	if err != nil {
		health.Status = "OFFLINE"
		health.Error = err
		return health
	}

	parts := strings.Split(strings.TrimSpace(out), "|")
	if len(parts) >= 5 {
		health.CPUUsage = parts[1]
		health.RAMUsage = parts[2]
		health.DiskUsage = parts[3]
		health.CPUTemp = parts[4]
	}

	// Check Services
	services := []string{"kgg-agent", "k3s"}
	if node.Role == "infra-manager" {
		services = append(services, "mqtt")
	}

	for _, svc := range services {
		svcCmd := fmt.Sprintf("systemctl is-active %s || echo 'inactive'", svc)
		svcOut, _ := executor.ExecuteCommand(node.IP, m.Port, svcCmd)
		health.Services[svc] = strings.TrimSpace(svcOut)
	}

	// K3s Deep Diagnostics (only if k3s is active and node is master/infra)
	if health.Services["k3s"] == "active" && (node.Role == "master" || node.Role == "infra-manager") {
		podCmd := `kubectl get pods -A --field-selector=status.phase!=Running -o jsonpath='{range .items[*]}{.metadata.namespace}{"/"}{.metadata.name}{" ("}{.status.phase}{")\n"}{end}'`
		podOut, _ := executor.ExecuteCommand(node.IP, m.Port, podCmd)
		podOut = strings.TrimSpace(podOut)
		if podOut != "" {
			health.FailingPods = strings.Split(podOut, "\n")
		}
	}

	// Fetch recent critical logs
	logCmd := "tail -n 100 /var/log/syslog | grep -iE 'error|fail|exception' | tail -n 10 || echo 'No critical logs found'"
	logOut, _ := executor.ExecuteCommand(node.IP, m.Port, logCmd)
	health.RecentLogs = strings.TrimSpace(logOut)

	return health
}

func (m *Manager) buildAIPrompt(report *HealthReport) string {
	var sb strings.Builder
	sb.WriteString("You are a Senior SRE and Systems Architect auditing a mission-critical Kubernetes Homelab cluster.\n")
	sb.WriteString("Your task is to provide a 'Preventive Post-Mortem' analysis based on the following metrics and logs.\n\n")

	sb.WriteString("DIAGNOSTIC DATA:\n")
	for _, n := range report.Nodes {
		_, _ = fmt.Fprintf(&sb, "\nNODE: %s (%s) | Status: %s\n", n.NodeName, n.IP, n.Status)
		if n.Status == "ONLINE" {
			_, _ = fmt.Fprintf(&sb, "  - Resources: CPU Load [%s], Temp [%s], RAM [%s], Disk [%s]\n", n.CPUUsage, n.CPUTemp, n.RAMUsage, n.DiskUsage)
			_, _ = sb.WriteString("  - Services:\n")
			for s, st := range n.Services {
				_, _ = fmt.Fprintf(&sb, "    * %s: %s\n", s, st)
			}
			if len(n.FailingPods) > 0 {
				_, _ = sb.WriteString("  - ⚠️ FAILING K8S PODS:\n")
				for _, p := range n.FailingPods {
					_, _ = fmt.Fprintf(&sb, "    * %s\n", p)
				}
			}

			if n.RecentLogs != "" && n.RecentLogs != "No critical logs found" {
				_, _ = fmt.Fprintf(&sb, "  - Critical Log Snippet: %s\n", n.RecentLogs)
			}
		} else if n.Error != nil {
			_, _ = fmt.Fprintf(&sb, "  - Connection Error: %v\n", n.Error)
		}
		sb.WriteString("---\n")
	}

	sb.WriteString("\nINSTRUCTIONS:\n")
	sb.WriteString("1. **Analyze Failure Chains**: If a node is offline or a pod is failing, explain the likely impact on the rest of the cluster.\n")
	sb.WriteString("2. **Root Cause Identification**: Use the resource metrics and logs to identify potential bottlenecks (e.g., IO wait, memory pressure).\n")
	sb.WriteString("3. **Preventive Post-Mortem**: Suggest long-term architectural fixes to prevent these issues from recurring.\n")
	sb.WriteString("4. **KGG Commands**: Continue to suggest EXACT 'kgg' commands for immediate mitigation when applicable.\n\n")

	sb.WriteString("Output Format:\n")
	sb.WriteString("### [Node Name]: [Status Summary]\n")
	sb.WriteString("**Analysis**: [Technical explanation of health and issues]\n")
	sb.WriteString("**Mitigation**: [Immediate Fix (KGG command). MUST use the format: [REPAIR: kgg <command>]]\n")
	sb.WriteString("**Prevention**: [Strategic advice]\n\n")
	sb.WriteString("Final Conclusion: [Overall Cluster Health Score 0-10 and Strategic Summary]")

	return sb.String()
}

func (m *Manager) parseRepairActions(summary string) []string {
	re := regexp.MustCompile(`\[REPAIR: (kgg [^\]]+)\]`)
	matches := re.FindAllStringSubmatch(summary, -1)

	var actions []string
	for _, match := range matches {
		if len(match) > 1 {
			actions = append(actions, strings.TrimSpace(match[1]))
		}
	}
	return actions
}

// generateBasicSummary provides a rule-based analysis of the health report
// when AI is unavailable or fails.
func (m *Manager) generateBasicSummary(report *HealthReport) string {
	online := 0
	offline := 0
	failingPods := 0

	for _, n := range report.Nodes {
		if n.Status == "ONLINE" {
			online++
			failingPods += len(n.FailingPods)
		} else {
			offline++
		}
	}

	summary := "📋 **Deterministic Cluster Summary**\n"
	summary += fmt.Sprintf("Nodes: %d/%d Online", online, len(report.Nodes))

	if failingPods > 0 {
		summary += fmt.Sprintf(" | ⚠️ Pods: %d failing", failingPods)
	}

	if offline > 0 {
		summary += "\n\n🚨 **CRITICAL**: One or more nodes are unreachable. This may cause service degradation or high-availability loss."
	} else if failingPods > 0 {
		summary += "\n\n⚠️ **WARNING**: All nodes are online, but some Kubernetes pods are failing. Use 'kgg status' to investigate."
	} else {
		summary += "\n\n✅ Cluster is healthy and all services are nominal."
	}

	return summary
}
