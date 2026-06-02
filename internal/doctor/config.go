package doctor

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/fatih/color"
)

// ValidateConfig runs advanced connectivity and environment checks against the active configuration.
// It supplements the static validation in config.Validate().
// Returns true if the configuration passes all connectivity checks, or false otherwise.
func ValidateConfig(out io.Writer) bool {
	cfg := config.GetConfig()
	passed := true

	_, _ = fmt.Fprintln(out, "\nValidating configuration topology & connectivity...")

	// 1. Static Validation (already covers Role, IPs, duplicate names, etc).
	err := cfg.Validate()
	if err != nil {
		_, _ = fmt.Fprintf(out, "%s\n", color.RedString("static Validation Failed:\n%s", err))
		passed = false
	} else {
		_, _ = fmt.Fprintf(out, "%s\n", color.GreenString("static Validation Passed"))
	}

	// 2. SSH Key Check
	keyPath, err := cfg.SSH.ExpandedKeyPath()
	if err == nil && keyPath != "" {
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			_, _ = fmt.Fprintf(out, "%s\n", color.YellowString("Warning: SSH Private Key not found at '%s'. Connections may fail.", keyPath))
		} else {
			_, _ = fmt.Fprintf(out, "%s\n", color.GreenString("SSH Key found at '%s'", keyPath))
		}
	} else if keyPath == "" {
		_, _ = fmt.Fprintf(out, "%s\n", color.YellowString("Warning: No SSH Private Key configured. Connections may rely on ssh-agent."))
	}

	// 3. Ping/Connectivity Check
	_, _ = fmt.Fprintln(out, "\nTesting node connectivity (Port 22)...")
	port := cfg.SSH.Port
	if port == 0 {
		port = 22
	}

	for _, n := range cfg.Nodes {
		if n.IP == "" {
			continue // Handled by static validation
		}
		if isReachable(n.IP, port) {
			_, _ = fmt.Fprintf(out, "%s\n", color.GreenString("%s (%s) is reachable on Port %d", n.Name, n.IP, port))
		} else {
			_, _ = fmt.Fprintf(out, "%s\n", color.YellowString("Warning: %s (%s) is unreachable on Port %d. It might be powered off or firewalled.", n.Name, n.IP, port))
			// We emit a warning, not a hard failure, because nodes might be powered off via WoL
		}
	}

	_, _ = fmt.Fprintln(out)
	if passed {
		_, _ = fmt.Fprintf(out, "%s\n", color.GreenString("Configuration Doctor completed successfully."))
	} else {
		_, _ = fmt.Fprintf(out, "%s\n", color.RedString("Configuration Doctor found critical issues (see above)."))
	}

	return passed
}

// isReachable attempts to open a TCP connection to the given IP and port with a short timeout.
func isReachable(ip string, port int) bool {
	timeout := 2 * time.Second
	target := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		_ = conn.Close()
		return true
	}
	return false
}
