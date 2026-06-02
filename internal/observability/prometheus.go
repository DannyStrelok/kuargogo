package observability

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/provision"
)

var ErrPrometheusUnavailable = fmt.Errorf("prometheus service is unreachable")

// PrometheusResponse structs model the JSON returned by Prometheus API
type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"` // [unix_time, "value_string"]
		} `json:"result"`
	} `json:"data"`
}

// Client wraps an SSH executor to query Prometheus inside the cluster
type Client struct {
	Executor   *provision.Executor
	MasterIP   string
	MasterPort int
	cachedIP   string
}

// NewClient establishes a Prometheus query client routed through a master node
func NewClient(executor *provision.Executor, masterIP string, masterPort int) *Client {
	return &Client{
		Executor:   executor,
		MasterIP:   masterIP,
		MasterPort: masterPort,
	}
}

// DiscoverClusterIP attempts to find the stable IP for the prometheus service via kubectl
func (c *Client) DiscoverClusterIP() (string, error) {
	if c.cachedIP != "" {
		return c.cachedIP, nil
	}

	// Try to get ClusterIP from the master node where kubectl is available.
	// Requires sudo to access /etc/rancher/k3s/k3s.yaml
	cmd := `sudo kubectl get svc -n monitoring kube-prometheus-stack-prometheus -o jsonpath='{.spec.clusterIP}'`

	// Temporarily silence executor output to avoid cluttering the CLI
	oldStdout := c.Executor.Stdout
	c.Executor.Stdout = io.Discard // Send to black hole to silence ExecuteCommand
	defer func() { c.Executor.Stdout = oldStdout }()

	ip, err := c.Executor.ExecuteCommand(c.MasterIP, c.MasterPort, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to discover prometheus ClusterIP: %w", err)
	}
	c.cachedIP = strings.TrimSpace(ip)
	return c.cachedIP, nil
}

// Query performs a PromQL query with direct ClusterIP routing to avoid DNS timeouts
func (c *Client) Query(query string) (*PrometheusResponse, error) {
	// 1. Ensure we have the ClusterIP to avoid slow DNS resolution of .cluster.local on the host
	ip, err := c.DiscoverClusterIP()
	if err != nil {
		return nil, fmt.Errorf("failed to route prometheus query: %w", err)
	}

	encodedQuery := url.QueryEscape(query)
	// We use the ClusterIP directly to be fast
	fullURL := fmt.Sprintf("http://%s:9090/api/v1/query?query=%s", ip, encodedQuery)
	cmd := fmt.Sprintf(`curl -s "%s"`, fullURL)

	// Temporarily silence executor output to avoid printing raw JSON to the console
	oldStdout := c.Executor.Stdout
	c.Executor.Stdout = io.Discard
	defer func() { c.Executor.Stdout = oldStdout }()

	out, err := c.Executor.ExecuteCommand(c.MasterIP, c.MasterPort, cmd)
	if err != nil {
		return nil, fmt.Errorf("cURL execution failed: %w", err)
	}

	var resp PrometheusResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("%w: failed to parse prometheus JSON: %v. Raw output: %s", ErrPrometheusUnavailable, err, out)
	}

	if resp.Status != "success" {
		return nil, fmt.Errorf("%w: prometheus returned non-success status: %s", ErrPrometheusUnavailable, resp.Status)
	}

	return &resp, nil
}

// extractFloat parses the Prometheus value array to a float64
func extractFloat(result []interface{}) (float64, error) {
	if len(result) < 2 {
		return 0, fmt.Errorf("invalid prometheus value format")
	}
	valStr, ok := result[1].(string)
	if !ok {
		return 0, fmt.Errorf("prometheus value is not a string")
	}
	var val float64
	_, err := fmt.Sscanf(valStr, "%f", &val)
	return val, err
}

func (c *Client) GetNodeMetrics(nodeName, nodeIP string) ([]provision.HealthCheckResult, error) {
	results := []provision.HealthCheckResult{}

	// Flexible selector: match instance by IP or Name using regex
	selector := fmt.Sprintf(`instance=~"(%s|%s).*"`, nodeName, nodeIP)

	// 1. CPU & Load
	cpuQuery := fmt.Sprintf(`100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle", %s}[5m])) * 100)`, selector)
	cpuResp, err := c.Query(cpuQuery)
	if err != nil {
		return nil, fmt.Errorf("CPU query failed: %w", err)
	}
	if len(cpuResp.Data.Result) > 0 {
		val, _ := extractFloat(cpuResp.Data.Result[0].Value)
		results = append(results, provision.HealthCheckResult{
			Name:   "CPU Usage",
			Icon:   "⚡",
			Result: fmt.Sprintf("%.1f%%", val),
		})
	}

	loadQuery := fmt.Sprintf(`node_load1{%s}`, selector)
	if loadResp, err := c.Query(loadQuery); err == nil && len(loadResp.Data.Result) > 0 {
		l1, _ := extractFloat(loadResp.Data.Result[0].Value)
		results = append(results, provision.HealthCheckResult{
			Name:   "Load Avg (1m)",
			Icon:   "⚖️",
			Result: fmt.Sprintf("%.2f", l1),
		})
	}

	// 2. Memory
	memQuery := fmt.Sprintf(`(1 - (node_memory_MemAvailable_bytes{%s} / node_memory_MemTotal_bytes{%s})) * 100`, selector, selector)
	memResp, err := c.Query(memQuery)
	if err != nil {
		return nil, fmt.Errorf("memory query failed: %w", err)
	}
	if len(memResp.Data.Result) > 0 {
		val, _ := extractFloat(memResp.Data.Result[0].Value)
		results = append(results, provision.HealthCheckResult{
			Name:   "Memory Usage",
			Icon:   "🧠",
			Result: fmt.Sprintf("%.1f%%", val),
		})
	}

	// 3. Storage & Latency
	diskQuery := fmt.Sprintf(`100 - ((node_filesystem_avail_bytes{mountpoint="/", %s} * 100) / node_filesystem_size_bytes{mountpoint="/", %s})`, selector, selector)
	diskResp, err := c.Query(diskQuery)
	if err != nil {
		return nil, fmt.Errorf("disk query failed: %w", err)
	}
	if len(diskResp.Data.Result) > 0 {
		val, _ := extractFloat(diskResp.Data.Result[0].Value)
		results = append(results, provision.HealthCheckResult{
			Name:   "Disk Usage (/)",
			Icon:   "📁",
			Result: fmt.Sprintf("%.1f%%", val),
		})
	}

	latQuery := fmt.Sprintf(`rate(node_disk_read_time_seconds_total{%s}[5m]) / rate(node_disk_reads_completed_total{%s}[5m])`, selector, selector)
	if latResp, err := c.Query(latQuery); err == nil && len(latResp.Data.Result) > 0 {
		val, _ := extractFloat(latResp.Data.Result[0].Value)
		results = append(results, provision.HealthCheckResult{
			Name:   "Disk Latency (R)",
			Icon:   "⏳",
			Result: formatMS(val),
		})
	}

	// 4. Network Throughput
	rxQuery := fmt.Sprintf(`sum(rate(node_network_receive_bytes_total{%s, device!~"lo|veth.*"}[5m]))`, selector)
	if rxResp, err := c.Query(rxQuery); err == nil && len(rxResp.Data.Result) > 0 {
		val, _ := extractFloat(rxResp.Data.Result[0].Value)
		results = append(results, provision.HealthCheckResult{
			Name:   "Network Rx",
			Icon:   "⬇️",
			Result: formatMBps(val),
		})
	}
	txQuery := fmt.Sprintf(`sum(rate(node_network_transmit_bytes_total{%s, device!~"lo|veth.*"}[5m]))`, selector)
	if txResp, err := c.Query(txQuery); err == nil && len(txResp.Data.Result) > 0 {
		val, _ := extractFloat(txResp.Data.Result[0].Value)
		results = append(results, provision.HealthCheckResult{
			Name:   "Network Tx",
			Icon:   "⬆️",
			Result: formatMBps(val),
		})
	}

	// 5. GPU Metrics (Optional)
	gpuMemQuery := fmt.Sprintf(`(nvidia_gpu_memory_used_bytes{%s} / nvidia_gpu_memory_total_bytes{%s}) * 100`, selector, selector)
	if gpuMemResp, err := c.Query(gpuMemQuery); err == nil && len(gpuMemResp.Data.Result) > 0 {
		val, _ := extractFloat(gpuMemResp.Data.Result[0].Value)
		results = append(results, provision.HealthCheckResult{
			Name:   "GPU VRAM Usage",
			Icon:   "🎮",
			Result: fmt.Sprintf("%.1f%%", val),
		})

		gpuTempQuery := fmt.Sprintf(`nvidia_gpu_temperature_celsius{%s}`, selector)
		if gpuTempResp, err := c.Query(gpuTempQuery); err == nil && len(gpuTempResp.Data.Result) > 0 {
			val, _ := extractFloat(gpuTempResp.Data.Result[0].Value)
			results = append(results, provision.HealthCheckResult{
				Name:   "GPU Temp",
				Icon:   "🔥",
				Result: fmt.Sprintf("%.0f°C", val),
			})
		}
	}

	// 6. System Uptime
	upQuery := fmt.Sprintf(`time() - node_boot_time_seconds{%s}`, selector)
	if upResp, err := c.Query(upQuery); err == nil && len(upResp.Data.Result) > 0 {
		seconds, _ := extractFloat(upResp.Data.Result[0].Value)
		results = append(results, provision.HealthCheckResult{
			Name:   "Uptime",
			Icon:   "⏱",
			Result: formatUptime(seconds),
		})
	}

	return results, nil
}

func formatMBps(bytesPerSec float64) string {
	mbps := bytesPerSec / (1024 * 1024)
	if mbps < 1 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	}
	return fmt.Sprintf("%.1f MB/s", mbps)
}

func formatMS(seconds float64) string {
	if seconds == 0 {
		return "0ms"
	}
	return fmt.Sprintf("%.1fms", seconds*1000)
}

func formatUptime(seconds float64) string {
	d := int(seconds) / (24 * 3600)
	h := (int(seconds) % (24 * 3600)) / 3600
	m := (int(seconds) % 3600) / 60

	if d > 0 {
		return fmt.Sprintf("up %dd, %dh, %dm", d, h, m)
	}
	if h > 0 {
		return fmt.Sprintf("up %dh, %dm", h, m)
	}
	return fmt.Sprintf("up %dm", m)
}
