package discovery

import (
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

var macRegex = regexp.MustCompile(`([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})`)

// ResolveMAC attempts to find the MAC address for a given IP address using the system's ARP table.
// It supports Windows (arp -a), Linux (ip neigh), and macOS (arp -n).
func ResolveMAC(ip string) string {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("arp", "-a", ip)
	case "linux":
		// 'ip neigh' is more modern than 'arp' on Linux
		cmd = exec.Command("ip", "neigh", "show", ip)
	case "darwin":
		cmd = exec.Command("arp", "-n", ip)
	default:
		return ""
	}

	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Find the first MAC address in the output
	match := macRegex.FindString(string(output))
	if match != "" {
		// Standardize to colon format
		return strings.ReplaceAll(strings.ToLower(match), "-", ":")
	}

	return ""
}
