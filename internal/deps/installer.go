package deps

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// InstallAnsible attempts to install Ansible based on the host OS
func InstallAnsible(out io.Writer) error {
	switch runtime.GOOS {
	case "windows":
		return installAnsibleWindows(out)
	case "darwin":
		return installBrewPackage("ansible", out)
	case "linux":
		return installAnsibleLinux(out)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// InstallK9s attempts to install K9s based on the host OS
func InstallK9s(out io.Writer) error {
	switch runtime.GOOS {
	case "windows":
		return installWingetPackage("derailed.k9s", out)
	case "darwin":
		return installBrewPackage("k9s", out)
	case "linux":
		return installK9sLinux(out)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func installAnsibleWindows(out io.Writer) error {
	_, _ = fmt.Fprintln(out, "Checking for WSL and Ubuntu...")
	if err := CheckWSLUbuntu(); err != nil {
		_, _ = fmt.Fprintln(out, "⚠️  WSL (Ubuntu) not found or not running.")
		_, _ = fmt.Fprintln(out, "Ansible requires WSL to run correctly on Windows.")
		_, _ = fmt.Fprintln(out, "⏳ Attempting automatic installation of WSL Ubuntu...")

		cmd := exec.Command("wsl", "--install", "-d", "Ubuntu")
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install WSL automatically. Open an Administrator terminal and run 'wsl --install -d Ubuntu', then restart: %w", err)
		}

		_, _ = fmt.Fprintln(out, "✅ WSL Ubuntu installed. ⚠️ You MUST restart your PC or terminal for WSL to activate.")
		return fmt.Errorf("wsl installation requires a restart before ansible can be installed")
	}

	_, _ = fmt.Fprintln(out, "✅ WSL Ubuntu detected.")
	_, _ = fmt.Fprintln(out, "Installing Ansible inside WSL via apt...")

	updateCmd := exec.Command("wsl", "-d", "Ubuntu", "--", "sudo", "apt-get", "update")
	updateCmd.Stdout = out
	updateCmd.Stderr = out
	_ = updateCmd.Run()

	installCmd := exec.Command("wsl", "-d", "Ubuntu", "--", "sudo", "apt-get", "install", "-y", "ansible")
	installCmd.Stdout = out
	installCmd.Stderr = out
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install ansible in WSL: %w", err)
	}

	_, _ = fmt.Fprintln(out, "✅ Ansible successfully installed in WSL Ubuntu.")
	return nil
}

// UpdateWinPath adds a directory to the User PATH environment variable on Windows using PowerShell.
func UpdateWinPath(newDir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	// PowerShell script to append to User PATH if not already present
	psScript := fmt.Sprintf(`
		$path = [Environment]::GetEnvironmentVariable("Path", "User")
		if ($path -notlike "*%s*") {
			[Environment]::SetEnvironmentVariable("Path", $path + ";%s", "User")
		}
	`, newDir, newDir)

	cmd := exec.Command("powershell", "-Command", psScript)
	return cmd.Run()
}

func installAnsibleLinux(out io.Writer) error {
	// 1. Try native package managers
	if err := CheckDependency("apt"); err == nil {
		_, _ = fmt.Fprintln(out, "Installing Ansible via apt (requires sudo)...")
		_ = exec.Command("sudo", "apt", "update").Run()
		cmd := exec.Command("sudo", "apt", "install", "-y", "ansible")
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	if err := CheckDependency("dnf"); err == nil {
		_, _ = fmt.Fprintln(out, "Installing Ansible via dnf (requires sudo)...")
		cmd := exec.Command("sudo", "dnf", "install", "-y", "ansible")
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	if err := CheckDependency("pacman"); err == nil {
		_, _ = fmt.Fprintln(out, "Installing Ansible via pacman (requires sudo)...")
		cmd := exec.Command("sudo", "pacman", "-S", "--noconfirm", "ansible")
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// 2. Fallback to PIP if package managers fail or are missing
	_, _ = fmt.Fprintln(out, "System package manager failed or not found. Attempting install via pip...")
	if err := CheckDependency("python3"); err == nil || CheckDependency("python") == nil {
		pyCmd := "python3"
		if CheckDependency("python3") != nil {
			pyCmd = "python"
		}
		cmd := exec.Command(pyCmd, "-m", "pip", "install", "--user", "ansible")
		cmd.Stdout = out
		cmd.Stderr = out
		return cmd.Run()
	}

	return fmt.Errorf("no supported package manager (apt, dnf, pacman) or python/pip found")
}

func installBrewPackage(pkg string, out io.Writer) error {
	if err := CheckDependency("brew"); err != nil {
		return fmt.Errorf("homebrew is not installed. Please install it from https://brew.sh")
	}
	_, _ = fmt.Fprintf(out, "Installing %s via brew...\n", pkg)
	cmd := exec.Command("brew", "install", pkg)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func installWingetPackage(pkg string, out io.Writer) error {
	if err := CheckDependency("winget"); err != nil {
		if pkg == "derailed.k9s" {
			_, _ = fmt.Fprintln(out, "⚠️ winget is missing. Attempting manual download of K9s...")
			return downloadK9sWindows(out)
		}
		return fmt.Errorf("winget is not installed. Please install App Installer from Microsoft Store")
	}
	_, _ = fmt.Fprintf(out, "Installing %s via winget...\n", pkg)
	cmd := exec.Command("winget", "install", "-e", "--id", pkg, "--accept-package-agreements", "--accept-source-agreements")
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		if pkg == "derailed.k9s" {
			_, _ = fmt.Fprintf(out, "⚠️ winget failed (error: %v). Attempting manual download of K9s...\n", err)
			return downloadK9sWindows(out)
		}
		return err
	}
	return nil
}

func downloadK9sWindows(out io.Writer) error {
	// 1. Determine download location (User Home / .kuargogo / bin)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	binDir := filepath.Join(home, ".kuargogo", "bin")
	_ = os.MkdirAll(binDir, 0755)

	zipPath := filepath.Join(os.TempDir(), "k9s.zip")
	// For simplicity, we hardcode a recent version or use the latest tag redirect if possible
	// However, GitHub direct download for 'latest' usually requires knowing the version or using a proxy.
	// We'll use a known good version or try a generic URL if available.
	downloadURL := "https://github.com/derailed/k9s/releases/download/v0.32.7/k9s_Windows_amd64.zip"

	_, _ = fmt.Fprintf(out, "Downloading K9s from %s...\n", downloadURL)
	curlCmd := exec.Command("curl", "-L", "-o", zipPath, downloadURL)
	curlCmd.Stdout = out
	curlCmd.Stderr = out
	if err := curlCmd.Run(); err != nil {
		return fmt.Errorf("failed to download K9s via curl: %w", err)
	}

	_, _ = fmt.Fprintln(out, "Extracting K9s...")
	powershellCmd := exec.Command("powershell", "-Command", fmt.Sprintf("Expand-Archive -Path '%s' -DestinationPath '%s' -Force", zipPath, binDir))
	powershellCmd.Stdout = out
	powershellCmd.Stderr = out
	if err := powershellCmd.Run(); err != nil {
		return fmt.Errorf("failed to extract K9s: %w", err)
	}

	// 2. Add to PATH
	_, _ = fmt.Fprintln(out, "Adding K9s to User PATH...")
	if err := UpdateWinPath(binDir); err != nil {
		return fmt.Errorf("k9s downloaded to %s but failed to update PATH: %w", binDir, err)
	}

	_, _ = fmt.Fprintln(out, "✅ K9s manual install complete!")
	return nil
}

func installK9sLinux(out io.Writer) error {
	// 1. Try apt (if repo is configured)
	if err := CheckDependency("apt"); err == nil {
		_, _ = fmt.Fprintln(out, "Attempting K9s install via apt (requires sudo)...")
		cmd := exec.Command("sudo", "apt", "install", "-y", "k9s")
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// 2. Try brew (Linuxbrew)
	if err := CheckDependency("brew"); err == nil {
		return installBrewPackage("k9s", out)
	}

	// 3. Try snap
	if err := CheckDependency("snap"); err == nil {
		_, _ = fmt.Fprintln(out, "Installing K9s via snap (requires sudo)...")
		cmd := exec.Command("sudo", "snap", "install", "k9s", "--classic")
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// 4. Try pacman
	if err := CheckDependency("pacman"); err == nil {
		_, _ = fmt.Fprintln(out, "Installing K9s via pacman (requires sudo)...")
		cmd := exec.Command("sudo", "pacman", "-S", "--noconfirm", "k9s")
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("could not find apt, brew, snap, or pacman to install K9s. Please install manually: https://k9scli.io/topics/install/")
}
