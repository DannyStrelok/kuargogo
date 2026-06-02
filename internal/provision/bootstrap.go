package provision

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
)

// BootstrapOptions holds configuration for the SSH key setup phase.
type BootstrapOptions struct {
	NodeIP   string
	User     string
	Password string
	KeyPath  string // Base path for the key pair (without .pub)
	SSHPort  int
	Output   io.Writer // Progress output
}

// FullBootstrapOptions holds configuration for the entire node lifecycle.
type FullBootstrapOptions struct {
	NodeName      string
	DHCP_IP       string
	StaticIP      string
	User          string
	Password      string
	KeyPath       string
	SSHPort       int
	CreateUser    bool
	SkipProvision bool
	Tags          []string
	Role          string
	DryRun        bool
	Output        io.Writer
}

// BootstrapResult captures the outcome of each bootstrap step.
type BootstrapResult struct {
	KeyGenerated bool
	KeyInstalled bool
	SSHVerified  bool
	Output       string
}

// FullBootstrap orchestrates the complete node preparation:
// 1. Network Bootstrap (DHCP -> Static)
// 2. SSH Identity Setup (Keys)
// 3. Final OS Provisioning
func FullBootstrap(opts FullBootstrapOptions) error {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	targetIP := opts.StaticIP
	if opts.DHCP_IP != "" {
		targetIP = opts.DHCP_IP
	}

	_, _ = fmt.Fprintf(out, "\n🚀 Starting Full Bootstrap for %s (Target: %s)...\n", opts.NodeName, opts.StaticIP)

	// Proactively clear existing host keys from both system and kgg_known_hosts
	// for both current DHCP IP and final Static IP. This prevents "HOST KEY CHANGED"
	// errors during the bootstrap process.
	ips := []string{opts.StaticIP}
	if opts.DHCP_IP != "" {
		ips = append(ips, opts.DHCP_IP)
	}

	for _, ip := range ips {
		_ = RemoveSystemHostKey(ip)
		_ = RemoveHostKey(ip)
		if opts.NodeName != ip {
			_ = RemoveSystemHostKey(opts.NodeName)
			_ = RemoveHostKey(opts.NodeName)
		}
	}

	// Step 1: Network Bootstrap (if DHCP provided)
	if opts.DHCP_IP != "" {
		_, _ = fmt.Fprintf(out, "🌐 Step 1: Configuring static IP %s via DHCP host %s...\n", opts.StaticIP, opts.DHCP_IP)
		_, err := ansible.RunBootstrap(opts.NodeName, opts.DHCP_IP, opts.StaticIP, opts.User, opts.Password, opts.DryRun, nil, out)
		if err != nil {
			return fmt.Errorf("network bootstrap failed: %w", err)
		}
		_, _ = fmt.Fprintln(out, "   ✅ Network bootstrap complete. Node will reboot.")
		_, _ = fmt.Fprintf(out, "   ⏳ Waiting for node to come back at %s...\n", opts.StaticIP)

		// Wait for the new IP to be reachable
		if err := WaitSSHPort(opts.StaticIP, opts.SSHPort, 180*time.Second); err != nil {
			return fmt.Errorf("node did not come back at %s: %w", opts.StaticIP, err)
		}
		_, _ = fmt.Fprintln(out, "   ✨ Node is online!")
		targetIP = opts.StaticIP
	}

	// Step 2: SSH Identity Flow
	_, _ = fmt.Fprintf(out, "\n🔑 Step 2: SSH Identity Flow (Target: %s)...\n", targetIP)

	// Proactively clear existing host key to prevent "HOST KEY CHANGED" errors
	// during bootstrap (common when reinstalling or reassigning IPs).
	_ = RemoveSystemHostKey(targetIP)
	_ = RemoveHostKey(targetIP)

	_, err := Bootstrap(BootstrapOptions{
		NodeIP:   targetIP,
		User:     opts.User,
		Password: opts.Password,
		KeyPath:  opts.KeyPath,
		SSHPort:  opts.SSHPort,
		Output:   out,
	})
	if err != nil {
		return fmt.Errorf("SSH bootstrap failed: %w", err)
	}

	// Step 3: Final Provisioning
	if !opts.SkipProvision {
		// AUDIT: Only run K3s provisioning for K3s nodes.
		// Infra-manager nodes follow a separate lifecycle (kgg infra init).
		if opts.Role == "infra-manager" {
			_, _ = fmt.Fprintln(out, "\nℹ️  Note: Infrastructure Manager provisioning (Watchdog, health-agent, etc.) is handled via 'kgg infra init'.")
			_, _ = fmt.Fprintln(out, "✅ SSH and Network bootstrap complete!")
			return nil
		}

		_, _ = fmt.Fprintf(out, "\n📦 Step 3: Running full provisioning on %s...\n", opts.NodeName)
		result, err := ansible.RunProvision(opts.NodeName, opts.CreateUser, opts.Password, opts.DryRun, opts.Tags, out)
		if err != nil {
			return fmt.Errorf("provisioning error: %w", err)
		}
		if !result.Success {
			return fmt.Errorf("provisioning failed (exit code: %d)", result.ExitCode)
		}
		_, _ = fmt.Fprintln(out, "\n✅ Node fully bootstrapped and provisioned!")
	} else {
		_, _ = fmt.Fprintln(out, "\n⏭️  Provisioning skipped per request.")
	}

	return nil
}

// Bootstrap executes the full SSH bootstrap flow:
//  1. Generate cluster key if missing (EnsureClusterKey)
//  2. Install public key via password (InstallKey with TOFU)
//  3. Verify SSH key authentication works (VerifySSHAccess)
//
// The caller (CLI or TUI) can then proceed with provisioning.
func Bootstrap(opts BootstrapOptions) (*BootstrapResult, error) {
	result := &BootstrapResult{}
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	port := opts.SSHPort
	if port == 0 {
		port = 22
	}

	// Step 1: Ensure cluster key exists
	if _, err := fmt.Fprintln(out, "🔑 Step 1/3: Checking cluster SSH key..."); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}
	generated, err := EnsureClusterKey(opts.KeyPath)
	if err != nil {
		return result, fmt.Errorf("key generation failed: %w", err)
	}
	result.KeyGenerated = generated
	if generated {
		if _, err := fmt.Fprintf(out, "   ✅ Generated new key pair at %s\n", opts.KeyPath); err != nil {
			log.Printf("Warning: failed to write status: %v", err)
		}
	} else {
		if _, err := fmt.Fprintf(out, "   ✅ Key already exists at %s\n", opts.KeyPath); err != nil {
			log.Printf("Warning: failed to write status: %v", err)
		}
	}

	// Step 2: Install public key via password auth (uses TOFU for host key)
	if _, err := fmt.Fprintf(out, "\n📤 Step 2/3: Installing key to %s@%s...\n", opts.User, opts.NodeIP); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}
	pubKeyPath := opts.KeyPath + ".pub"
	if err := InstallKey(opts.NodeIP, port, opts.User, opts.Password, pubKeyPath); err != nil {
		return result, fmt.Errorf("key installation failed: %w", err)
	}
	result.KeyInstalled = true
	if _, err := fmt.Fprintln(out, "   ✅ Public key installed"); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}

	// Step 3: Verify SSH key auth works
	if _, err := fmt.Fprintln(out, "\n🔒 Step 3/3: Verifying SSH key authentication..."); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}
	if err := VerifySSHAccess(opts.NodeIP, port, opts.User, opts.KeyPath, false); err != nil {
		return result, fmt.Errorf("SSH verification failed: %w", err)
	}
	result.SSHVerified = true
	if _, err := fmt.Fprintln(out, "   ✅ SSH key authentication verified"); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}

	if _, err := fmt.Fprintf(out, "\n🎉 Bootstrap complete! Node %s is ready for provisioning.\n", opts.NodeIP); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}
	return result, nil
}
