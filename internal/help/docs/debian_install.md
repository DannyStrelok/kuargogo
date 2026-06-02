# Debian 13 (Trixie) Installation Guide
## Hardware Targets: Lenovo m920q / HP 800 G4 Mini

This guide covers the installation of Debian 13 (Testing) on our standard homelab nodes.

### 1. Prerequisites
- **USB Drive**: At least 4GB.
- **ISO Image**: Download the latest Debian Testing "netinst" ISO (includes non-free firmware).
  - URL: `https://cdimage.debian.org/cdimage/weekly-builds/amd64/iso-cd/` (Look for `debian-testing-amd64-netinst.iso`)
- **Rufus (Windows)** or `dd` (Linux/Mac) to burn the ISO.

### 2. BIOS Settings (IMPORTANT)
Before booting, ensure BIOS settings are correct for headless operation and remote management.

**Lenovo m920q:**
- **F1** to enter BIOS.
- **Security > Secure Boot**: Disabled (recommended for custom kernel modules, else Enabled is fine).
- **Startup > Boot Priority**: Legacy First or UEFI First (UEFI recommended for modern setups).
- **Automatic Power On**: Power > After Power Loss > Power On.
- **Intel AMT**: Enable if you plan to use MeshCommander/MeshCentral. Set network access to "On".

**HP 800 G4 Mini:**
- **Esc** then **F10** to enter BIOS.
- **Advanced > Boot Options**: Disable Fast Boot.
- **Advanced > Built-in Device Options**: Enable Wake on LAN.
- **Power > After Power Loss**: Power On.

### 3. Installation Steps
1. Insert USB and boot (F12 for Boot Menu on Lenovo / F9 for HP).
2. Select **"Graphical Install"**.
3. **Language/Region**: English / your preference.
4. **Hostname**: Use the standard naming convention (e.g., `kgg-worker-0x`, `kgg-master`).
5. **Domain**: `local` or your homelab domain.
6. **User Setup**:
   - **Root Password**: *[REDACTED]* (Use the vault password).
   - **User**: `kgguser` (or your standard admin user).
   - **Password**: *[REDACTED]*.
7. **Partitioning**:
   - **Method**: "Guided - use entire disk and set up LVM" (Recommended for flexibility).
   - **Scheme**: "All files in one partition" (Simplest for these nodes).
   - **Swap**: Installer usually creates ~1GB.
8. **Package Manager**:
   - Mirror: `deb.debian.org`.
   - Proxy: Leave blank unless you have an `apt-cacher-ng`.
9. **Software Selection** (Crucial):
   - [ ] DMA (Debian Desktop Environment) - **UNCHECK** (Headless!)
   - [ ] GNOME - **UNCHECK**
   - [x] SSH server - **CHECK**
   - [x] Standard system utilities - **CHECK**

### 4. Post-Installation Configuration
After rebooting:

1. **Sudo Access**:
   `su -`
   `usermod -aG sudo kgguser`

2. **Network Setup (Static IP recommended)**:
   Edit `/etc/network/interfaces` or use `nmtui` if NetworkManager is installed.
   Example `/etc/network/interfaces`:
   ```
   auto lo
   iface lo inet loopback

   auto eno1
   iface eno1 inet static
       address 192.168.1.XX
       netmask 255.255.255.0
       gateway 192.168.1.1
       dns-nameservers 1.1.1.1 8.8.8.8
   ```

3. **Install Agents**:
   - QEMU Guest Agent (if VM): `apt install qemu-guest-agent`
   - K3s (managed by kuargogo later).

4. **Update System**:
   `apt update && apt full-upgrade -y`

### 5. Verification
- Verify SSH access: `ssh kgguser@<ip>`
- Verify simple command: `uptime`

### 6. Next Steps
Once your nodes are reachable via SSH with the initial user, the **very first thing** to do in `kuargogo` is to generate and install the cluster SSH keys:
1. `kgg ssh-keygen`
2. `kgg ssh-copy --node <IP> --user <user>`

This ensures all subsequent automated playbooks (Ansible) work correctly.
