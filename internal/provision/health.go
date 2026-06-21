package provision

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// HealthCheckResult contains the output of a specific health check
type HealthCheckResult struct {
	Name   string
	Icon   string
	Result string
	Error  error
}

// CheckDef defines a command to run and how to parse it
type CheckDef struct {
	Name   string
	Cmd    string
	Icon   string
	Parser func(string) string
}

// VerifyBootstrapResult contains the result of a bootstrap verification check
type VerifyBootstrapResult struct {
	Name    string
	Passed  bool
	Details string
}

// VerifyBootstrap validates that a node was correctly provisioned for K3s
func (e *Executor) VerifyBootstrap(ip string, port int) ([]VerifyBootstrapResult, error) {
	checks := []struct {
		Name string
		Cmd  string
		Want string // Expected substring in output for pass
	}{
		{
			Name: "Kernel Module: overlay",
			Cmd:  "lsmod | grep -q overlay && echo 'loaded' || echo 'missing'",
			Want: "loaded",
		},
		{
			Name: "Kernel Module: br_netfilter",
			Cmd:  "lsmod | grep -q br_netfilter && echo 'loaded' || echo 'missing'",
			Want: "loaded",
		},
		{
			Name: "Kernel Module: dm_crypt",
			Cmd:  "lsmod | grep -q dm_crypt && echo 'loaded' || echo 'missing'",
			Want: "loaded",
		},
		{
			Name: "iSCSI Service (iscsid)",
			Cmd:  "systemctl is-active iscsid",
			Want: "active",
		},
		{
			Name: "IP Forwarding Enabled",
			Cmd:  "sysctl -n net.ipv4.ip_forward",
			Want: "1",
		},
		{
			Name: "Swap Disabled",
			Cmd:  "swapon --show | wc -l",
			Want: "0",
		},
		{
			Name: "UFW Active",
			Cmd:  "sudo ufw status | head -1",
			Want: "active",
		},
		{
			Name: "UFW: K3s API (6443)",
			Cmd:  "sudo ufw status | grep -q '6443' && echo 'ok' || echo 'missing'",
			Want: "ok",
		},
		{
			Name: "User kgg-admin exists",
			Cmd:  "id kgg-admin &>/dev/null && echo 'exists' || echo 'missing'",
			Want: "exists",
		},
	}

	results := []VerifyBootstrapResult{}

	for _, check := range checks {
		out, err := e.ExecuteCommand(ip, port, check.Cmd)
		result := VerifyBootstrapResult{
			Name: check.Name,
		}

		if err != nil {
			result.Passed = false
			result.Details = fmt.Sprintf("Error: %v", err)
		} else {
			trimmed := strings.TrimSpace(out)
			result.Passed = strings.Contains(strings.ToLower(trimmed), strings.ToLower(check.Want))
			result.Details = trimmed
		}
		results = append(results, result)
	}

	return results, nil
}

// RunHealthCheck executes a comprehensive health check on a node
func (e *Executor) RunHealthCheck(ip string, port int) ([]HealthCheckResult, error) {
	checks := []CheckDef{
		{
			Name:   "NVMe SMART Status",
			Cmd:    "sudo nvme smart-log /dev/nvme0n1 2>/dev/null || echo 'No NVMe device found'",
			Icon:   "💾",
			Parser: parseNVMeOutput,
		},
		{
			Name: "CPU Temperature",
			Cmd:  "for z in /sys/class/thermal/thermal_zone*; do t=$(cat $z/type 2>/dev/null); if [ \"$t\" = \"x86_pkg_temp\" ] || [ \"$t\" = \"cpu-thermal\" ] || [ \"$t\" = \"soc-thermal\" ]; then cat $z/temp; exit; fi; done; cat /sys/class/thermal/thermal_zone0/temp 2>/dev/null || echo 'N/A'",
			Icon: "🌡️",
			Parser: func(out string) string {
				val := strings.TrimSpace(out)
				if val == "N/A" || val == "" {
					return "Unknown"
				}
				milli, err := strconv.Atoi(val)
				if err != nil {
					return val
				}
				return fmt.Sprintf("%.1f°C", float64(milli)/1000.0)
			},
		},
		{
			Name: "AppArmor Status",
			Cmd:  "sudo aa-status --enabled 2>/dev/null && echo 'Enabled' || echo 'Disabled'",
			Icon: "🛡️",
		},
		{
			Name: "iSCSI Service",
			Cmd:  "systemctl is-active iscsid 2>/dev/null || echo 'inactive'",
			Icon: "🔌",
		},
	}

	results := []HealthCheckResult{}

	for _, check := range checks {
		out, err := e.ExecuteCommand(ip, port, check.Cmd)
		res := HealthCheckResult{
			Name: check.Name,
			Icon: check.Icon,
		}

		if err != nil {
			res.Error = err
			res.Result = "Error"
		} else {
			val := strings.TrimSpace(out)
			if check.Parser != nil {
				val = check.Parser(out)
			}
			res.Result = val
		}
		results = append(results, res)
	}

	return results, nil
}

// RunNVMeVerbose returns the full NVMe SMART log
func (e *Executor) RunNVMeVerbose(ip string, port int) (string, error) {
	return e.ExecuteCommand(ip, port, "sudo nvme smart-log /dev/nvme0n1 2>/dev/null || echo 'No NVMe device'")
}

func parseNVMeOutput(output string) string {
	if strings.Contains(output, "No NVMe") {
		return "No NVMe device found"
	}

	// Extract key metrics
	lines := strings.Split(output, "\n")
	var percentUsed, temperatureStr, dataWritten string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "percentage_used") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				percentUsed = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "temperature") && temperatureStr == "" {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				temperatureStr = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "data_units_written") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				dataWritten = strings.TrimSpace(parts[1])
			}
		}
	}

	if percentUsed == "" {
		return "✅ Healthy"
	}

	// Normalize Temperature to Celsius
	// Input style: "42 C (315 K)", "107 F (315 K)", "315 K", etc.
	finalTemp := temperatureStr
	if temperatureStr != "" {
		// 1. Try to find Celsius value directly
		reC := regexp.MustCompile(`(\d+)\s*[Cc]`)
		matchC := reC.FindStringSubmatch(temperatureStr)
		if len(matchC) > 1 {
			finalTemp = matchC[1] + "°C"
		} else {
			// 2. Try to find Fahrenheit and convert
			reF := regexp.MustCompile(`(\d+)\s*[Ff]`)
			matchF := reF.FindStringSubmatch(temperatureStr)
			if len(matchF) > 1 {
				f, _ := strconv.ParseFloat(matchF[1], 64)
				c := (f - 32) * 5 / 9
				finalTemp = fmt.Sprintf("%.1f°C", c)
			} else {
				// 3. Try to find Kelvin and convert
				reK := regexp.MustCompile(`(\d+)\s*[Kk]`)
				matchK := reK.FindStringSubmatch(temperatureStr)
				if len(matchK) > 1 {
					k, _ := strconv.ParseFloat(matchK[1], 64)
					c := k - 273.15
					finalTemp = fmt.Sprintf("%.1f°C", c)
				}
			}
		}
	}

	// Format: "5% wear, 38.0°C, 12TB written"
	result := fmt.Sprintf("%s%% wear", percentUsed)
	if finalTemp != "" {
		result += ", " + finalTemp
	}
	if dataWritten != "" {
		result += ", " + dataWritten + " written"
	}
	return result
}
