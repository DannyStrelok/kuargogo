package discovery

import (
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// MergeDiscoveredNodes deduplicates nodes by Name, preferring discovered information when Name matches.
// This logic is shared between the CLI 'discover' command and the TUI discovery action.
func MergeDiscoveredNodes(existing, discovered []config.Node) []config.Node {
	nodeMap := make(map[string]config.Node)
	for _, n := range existing {
		nodeMap[n.Name] = n
	}
	for _, d := range discovered {
		if existingNode, ok := nodeMap[d.Name]; ok {
			// Update fields that are empty in existing with discovered values
			if existingNode.IP == "" && d.IP != "" {
				existingNode.IP = d.IP
			}
			if existingNode.MAC == "" && d.MAC != "" {
				existingNode.MAC = d.MAC
			}
			nodeMap[d.Name] = existingNode
		} else {
			nodeMap[d.Name] = d
		}
	}
	result := make([]config.Node, 0, len(nodeMap))
	for _, n := range nodeMap {
		result = append(result, n)
	}
	return result
}

// MDNSToNodes converts mDNS discovery results to config Node objects.
func MDNSToNodes(services []MDNSService, defaultUser string) []config.Node {
	var nodes []config.Node
	for _, s := range services {
		ip := ""
		if len(s.IPs) > 0 {
			ip = s.IPs[0]
		}
		mac := ResolveMAC(ip)
		node := config.Node{
			Name: s.Instance,
			IP:   ip,
			MAC:  mac,
			Arch: GuessArchitecture(s, mac),
			User: defaultUser,
			Role: "worker", // Default role
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// GuessArchitecture attempts to infer the CPU architecture of a node based on heuristics.
func GuessArchitecture(s MDNSService, mac string) string {
	name := strings.ToLower(s.Instance + s.HostName)

	// 1. Hostname hints
	if strings.Contains(name, "x64") || strings.Contains(name, "amd64") || strings.Contains(name, "x86_64") {
		return "amd64"
	}
	if strings.Contains(name, "arm64") || strings.Contains(name, "aarch64") {
		return "arm64"
	}
	if strings.Contains(name, "armv7") {
		return "armv7"
	}

	// 2. Raspberry Pi specific (usually ARM)
	if strings.Contains(name, "raspberry") || strings.Contains(name, "rpi") {
		return "arm64" // Default to arm64 for modern RPi
	}

	// 3. MAC OUI hints (Raspberry Pi Foundation)
	if mac != "" {
		// Clean common MAC delimiters (colons, hyphens, dots) to support diverse formats
		replacer := strings.NewReplacer(":", "", "-", "", ".", "")
		oui := replacer.Replace(strings.ToLower(mac))
		if len(oui) >= 6 {
			ouiPrefix := oui[:6]
			switch ouiPrefix {
			case "b827eb", "dca632", "e45f01":
				return "arm64"
			}
		}
	}

	return "" // Unknown
}
