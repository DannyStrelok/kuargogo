package ansible

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/deps"
)

// Result captures the outcome of an Ansible playbook execution.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Success  bool
	Playbook string
}

// VaultPassFileOverride allows CLI flags to override the config-based vault password file.
// Set this before calling any Run* helper to override the value from kuargogo.yaml.
var VaultPassFileOverride string

// Runner executes Ansible playbooks with dynamic inventory.
type Runner struct {
	PlaybookDir         string
	Output              io.Writer
	DryRun              bool
	Tags                []string // Ansible --tags filter
	VaultPassFile       string   // Path to Ansible Vault password file
	SkipHostKeyChecking bool     // If true, disables ANSIBLE_HOST_KEY_CHECKING
}

// NewRunner creates a new Ansible runner.
// playbookDir is the base directory containing playbook YAML files.
func NewRunner(playbookDir string) *Runner {
	r := &Runner{
		PlaybookDir: playbookDir,
		Output:      os.Stdout,
	}

	// Auto-resolve vault password file: CLI override > config
	if VaultPassFileOverride != "" {
		r.VaultPassFile = VaultPassFileOverride
	} else {
		cfg := config.GetConfig()
		r.VaultPassFile = cfg.Ansible.VaultPasswordFile
	}

	return r
}

// Run executes a playbook and returns the result.
// playbookName should be the filename (e.g., "update.yml").
// limit restricts execution to specific hosts (passed as -l). Use "" for all.
// extraVars are passed as -e key=value arguments.
func (r *Runner) Run(playbookName string, limit string, extraVars map[string]string) (*Result, error) {
	// Check ansible-playbook is installed (WSL or Native)
	if !r.DryRun {
		if runtime.GOOS == "windows" {
			if err := deps.CheckWSLUbuntu(); err != nil {
				return nil, err
			}
			if err := deps.CheckWSLCommand("ansible-playbook"); err != nil {
				return nil, fmt.Errorf("ansible is not installed in WSL. Run 'kgg setup' to install it: %w", err)
			}
		} else {
			if err := deps.CheckDependency("ansible-playbook"); err != nil {
				return nil, err
			}
		}
	}

	// Generate dynamic inventory from config
	inventoryPath, keyPath, err := GenerateInventory(r.DryRun)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.Remove(inventoryPath)
	}()

	// Automatically inject public key path as extra-var if it exists.
	if keyPath != "" {
		localKeyPath, err := config.ResolveKeyPath("")
		if err == nil && localKeyPath != "" {
			if _, err := os.Stat(localKeyPath + ".pub"); err == nil {
				if extraVars == nil {
					extraVars = make(map[string]string)
				}
				extraVars["kgg_cluster_pubkey"] = keyPath + ".pub"
			}
		}
	}

	// Automatically inject the Master Passphrase if configured.
	// This allows remote services (like the Telegram bot) to decrypt secrets.
	if mKey, err := config.GetMasterKey(); err == nil && mKey != "" {
		if extraVars == nil {
			extraVars = make(map[string]string)
		}
		extraVars["kgg_master_passphrase"] = mKey
	}

	// General path autoconversion for Windows/WSL compatibility
	if runtime.GOOS == "windows" {
		for k, v := range extraVars {
			if deps.IsWindowsPath(v) {
				if wslV, err := deps.ConvertToWSLPath(v); err == nil {
					extraVars[k] = wslV
				}
			}
		}
	}

	playbookPath := fmt.Sprintf("%s/%s", r.PlaybookDir, playbookName)

	baseCmd := "ansible-playbook"
	var cmdArgs []string

	if runtime.GOOS == "windows" {
		baseCmd = "wsl"
		// Use `env` to inject Ansible config vars INSIDE the WSL Linux environment.
		// Setting cmd.Env only affects the Windows wsl.exe process, not the Linux side.
		cmdArgs = append(cmdArgs, "-d", deps.GetWSLDistro(), "-e", "env",
			"ANSIBLE_INVENTORY_ENABLED=host_list,yaml,constructed,ini,toml",
			"ANSIBLE_FORCE_COLOR=True",
			fmt.Sprintf("ANSIBLE_HOST_KEY_CHECKING=%t", !r.SkipHostKeyChecking),
			"ANSIBLE_PYTHON_INTERPRETER=auto_silent",
			"ANSIBLE_DEPRECATION_WARNINGS=False",
			"ansible-playbook",
		)

		inventoryPath, _ = deps.ConvertToWSLPath(inventoryPath)
		playbookPath, _ = deps.ConvertToWSLPath(playbookPath)
		if r.VaultPassFile != "" {
			r.VaultPassFile, _ = deps.ConvertToWSLPath(r.VaultPassFile)
		}
	}

	// Build command arguments
	cmdArgs = append(cmdArgs, "-i", inventoryPath, playbookPath)

	if limit != "" {
		limit = SanitizeAnsibleHostname(limit)
		cmdArgs = append(cmdArgs, "-l", limit)
	}

	// AUDIT: Ensure kgg_user is passed correctly.
	// If limit is a single host, we can find its user.
	if limit != "" && !strings.ContainsAny(limit, ":,") {
		cfg := config.GetConfig()
		for _, n := range cfg.Nodes {
			if n.Name == limit || n.IP == limit {
				if extraVars == nil {
					extraVars = make(map[string]string)
				}
				extraVars["kgg_user"] = n.User
				break
			}
		}
	}

	// SECURE EXTRA-VARS PASSING:
	// To prevent secrets (like master_passphrase and API tokens) from appearing
	// in the OS process list (ps aux), we serialize them to a temporary JSON file
	// and pass it via -e @file.json
	if len(extraVars) > 0 {
		tempFile, err := os.CreateTemp("", "kgg-extravars-*.json")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp vars file: %w", err)
		}
		varsPath := tempFile.Name()
		defer func() { _ = os.Remove(varsPath) }() // Ensure cleanup happens after Ansible finishes

		encoder := json.NewEncoder(tempFile)
		if err := encoder.Encode(extraVars); err != nil {
			_ = tempFile.Close()
			return nil, fmt.Errorf("failed to encode extra vars: %w", err)
		}
		_ = tempFile.Close() // Close before ansible reads it

		wslVarsPath := varsPath
		if runtime.GOOS == "windows" {
			wslVarsPath, _ = deps.ConvertToWSLPath(varsPath)
		}

		cmdArgs = append(cmdArgs, "-e", "@"+wslVarsPath)
	}

	if len(r.Tags) > 0 {
		cmdArgs = append(cmdArgs, "--tags", strings.Join(r.Tags, ","))
	}

	if r.VaultPassFile != "" {
		cmdArgs = append(cmdArgs, "--vault-password-file", r.VaultPassFile)
	}

	// AUDIT: Point Ansible to the kgg_known_hosts file for security consistency.
	// This ensures that 'ANSIBLE_HOST_KEY_CHECKING=True' works with our managed host keys.
	knownHostsWSLPath := "~/.ssh/kgg_known_hosts"
	sshArgs := "-o ConnectTimeout=30 -o ServerAliveInterval=15 -o ServerAliveCountMax=4"
	if r.SkipHostKeyChecking {
		sshArgs += " -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"
	} else {
		sshArgs += fmt.Sprintf(" -o UserKnownHostsFile=%s", knownHostsWSLPath)
	}

	if runtime.GOOS == "windows" {
		// Injected via env command for WSL
		newArgs := make([]string, 0, len(cmdArgs)+1)
		// Insert before the last element (ansible-playbook)
		for i, arg := range cmdArgs {
			if arg == "ansible-playbook" {
				newArgs = append(newArgs, "ANSIBLE_SCP_IF_SSH=True", "ANSIBLE_SSH_ARGS="+sshArgs, "ansible-playbook")
				newArgs = append(newArgs, cmdArgs[i+1:]...)
				break
			}
			newArgs = append(newArgs, arg)
		}
		cmdArgs = newArgs
	}

	// DryRun: print the full command and return without executing
	if r.DryRun {
		_, err := fmt.Fprintln(r.Output, "[DRY RUN] Would execute: ", baseCmd, cmdArgs)
		if err != nil {
			return nil, err
		}
		return &Result{
			Playbook: playbookName,
			Success:  true,
		}, nil
	}

	// Capture output while also streaming to Output
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(baseCmd, cmdArgs...)

	// Pass parent environment plus overrides for native Linux/macOS execution.
	// (On Windows, these are already injected via the 'env' command above, but setting them here doesn't hurt).
	env := os.Environ()
	env = append(env, "ANSIBLE_INVENTORY_ENABLED=host_list,yaml,constructed,ini,toml")
	env = append(env, "ANSIBLE_FORCE_COLOR=True")
	env = append(env, fmt.Sprintf("ANSIBLE_HOST_KEY_CHECKING=%t", !r.SkipHostKeyChecking))
	env = append(env, "ANSIBLE_PYTHON_INTERPRETER=auto_silent")
	env = append(env, "ANSIBLE_DEPRECATION_WARNINGS=False")

	if runtime.GOOS != "windows" {
		env = append(env, "ANSIBLE_SSH_ARGS="+sshArgs)
	}
	cmd.Env = env

	cmd.Stdout = io.MultiWriter(r.Output, &stdout)
	cmd.Stderr = io.MultiWriter(r.Output, &stderr)

	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		Playbook: playbookName,
		Success:  err == nil,
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else if err != nil {
		return result, fmt.Errorf("failed to run playbook: %w", err)
	}

	return result, nil
}
