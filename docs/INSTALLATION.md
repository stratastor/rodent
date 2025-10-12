# Rodent Installation Guide

## Overview

Rodent is a StrataSTOR node agent that manages ZFS-based storage systems. This guide covers the installation process for Ubuntu 24.04+ systems.

## Quick Start

For a quick installation with default settings:

```bash
curl -fsSL https://utils.strata.host/install.sh | sudo bash -s -- --non-interactive
```

## Installation Methods

### Method 1: One-Line Installer (Recommended)

The simplest method - downloads and runs the installer with all dependencies:

```bash
curl -fsSL https://utils.strata.host/install.sh | sudo bash 
```

### Method 2: Non-Interactive Installation

For automated deployments (CI/CD, provisioning tools):

```bash
curl -fsSL https://utils.strata.host/install.sh | sudo bash -s -- --non-interactive --yes
```

### Method 3: Custom Configuration

Install with custom Kerberos settings:

```bash
curl -fsSL https://utils.strata.host/install.sh | sudo bash -s -- \
  --krb-realm CORP.EXAMPLE.COM \
  --krb-admin DC1.CORP.EXAMPLE.COM
```

### Method 4: Selective Component Installation

Skip certain components if already installed:

```bash
curl -fsSL https://utils.strata.host/install.sh | sudo bash -s -- \
  --skip-docker \
  --skip-samba
```

## System Requirements

### Minimum Requirements

- **OS**: Ubuntu 24.04 LTS (Noble) or higher
- **Architecture**: x86_64 (amd64) or ARM64 (aarch64)
- **Disk Space**: 2GB free space
- **Memory**: 2GB RAM (recommended 4GB+)
- **Init System**: systemd
- **Network Manager**: netplan

### Required Packages (Installed Automatically)

The installer will automatically install:

1. **ZFS** (2.3.0+) - From Zabbly repository
   - openzfs-zfsutils
   - openzfs-zfs-dkms
   - openzfs-zfs-initramfs

2. **Docker** (latest) - From official Docker repository
   - docker-ce
   - docker-ce-cli
   - containerd.io
   - docker-buildx-plugin
   - docker-compose-plugin

3. **Samba & Kerberos**
   - samba
   - samba-common-bin
   - samba-vfs-modules
   - samba-ad-provision
   - winbind
   - krb5-user
   - krb5-config
   - libpam-krb5
   - libnss-winbind
   - libpam-winbind

4. **System Utilities**
   - acl, attr
   - smartmontools
   - sg3-utils, lsscsi
   - netplan.io
   - systemd-resolved
   - curl, wget, gnupg
   - jq, ripgrep
   - uuid-runtime

## Installation Process

The installer performs the following steps:

1. **Pre-flight Checks**
   - Verify root/sudo access
   - Check Ubuntu version (24.04+)
   - Verify architecture (amd64/arm64)
   - Check disk space (2GB+)
   - Verify systemd is active
   - Verify netplan is available

2. **Enable systemd-resolved**
   - Ensures DNS resolution service is active
   - Enables resolvectl for DNS management

3. **Install Dependencies**
   - System utilities (jq, ripgrep, acl, etc.)
   - ZFS 2.3+ from Zabbly repository
   - Docker from official repository
   - Samba with Kerberos client

4. **Download Rodent Binary**
   - Downloads from pkg.strata.host
   - Verifies binary integrity
   - Installs to /usr/local/bin/rodent

5. **Setup Rodent User**
   - Creates `rodent` system user
   - Sets up directory structure
   - Configures permissions
   - Adds user to docker group

6. **Configure Sudo Permissions**
   - Creates /etc/sudoers.d/rodent
   - Grants necessary elevated privileges
   - Configures secure execution paths

7. **Install Systemd Service**
   - Copies rodent.service to /etc/systemd/system/
   - Reloads systemd daemon
   - Service is NOT started automatically

8. **Collect Telemetry**
   - Saves installation metadata
   - Records installed components
   - Logs installation duration

## Post-Installation

After successful installation:

### 1. Configure Toggle JWT

Edit the configuration file:

```bash
sudo nano /home/rodent/.rodent/rodent.yml
```

Add your Toggle JWT:

```yaml
toggle:
  jwt: your-jwt-token-here
  baseurl: https://toggle.strata.foo
  rpcaddr: tunnel.strata.foo:443
stratasecure: true
```

### 2. Start Rodent Service

Enable and start the service:

```bash
sudo systemctl enable --now rodent.service
```

### 3. Verify Service Status

Check that the service is running:

```bash
sudo systemctl status rodent.service
```

### 4. View Logs

Monitor service logs in real-time:

```bash
sudo journalctl -u rodent.service -f
```

Or view installation log:

```bash
sudo cat /var/log/rodent-install.log
```

## Directory Structure

After installation, the following directories are created:

```sh
/usr/local/bin/
└── rodent                              # Rodent binary

/home/rodent/.rodent/
├── ssh/                                # SSH keys
├── services/                           # Service configurations
├── templates/traefik/                  # Traefik templates
├── state/                              # State files
├── shares/smb/                         # SMB share configurations
├── etc/rodent/                         # Additional configs
├── logs/                               # Application logs
└── rodent.yml                          # Main configuration

/var/lib/rodent/                        # Persistent data
/var/log/rodent/                        # Log files
/etc/systemd/system/
└── rodent.service                      # Systemd service file
/etc/sudoers.d/
└── rodent                              # Sudo permissions
```

## Configuration

### Default Kerberos Settings

- **Realm**: AD.STRATA.INTERNAL
- **Admin Server**: DC1.AD.STRATA.INTERNAL

To customize during installation:

```bash
curl -fsSL https://utils.strata.host/install.sh | sudo bash -s -- \
  --krb-realm YOUR.REALM.COM \
  --krb-admin dc1.your.realm.com
```

### Development Mode

For development environments with relaxed permissions:

```bash
curl -fsSL https://utils.strata.host/install.sh | sudo bash -s -- --dev
```

## Installer Options

```sh
--help                  Show help message
--version               Show installer version
--non-interactive       Run without prompts
--yes                   Assume yes to all prompts
--verbose               Show detailed package installation output
--install-version VER   Install specific version (default: latest)
--skip-deps             Skip all dependency installation
--skip-zfs              Skip ZFS installation
--skip-docker           Skip Docker installation
--skip-samba            Skip Samba installation
--skip-zfs-check        Skip ZFS version check
--krb-realm REALM       Set Kerberos realm
--krb-admin SERVER      Set Kerberos admin server
--dev                   Enable development mode
--force                 Force installation despite warnings
```

## Troubleshooting

### Installation Fails

Check the installation log:

```bash
sudo cat /var/log/rodent-install.log
```

### ZFS Not Found

Verify ZFS installation:

```bash
zfs version
```

If ZFS is not found, manually install:

```bash
curl -fsSL https://pkgs.zabbly.com/key.asc -o /etc/apt/keyrings/zabbly.asc
# Add repository and install as per installation script
```

### Docker Permission Issues

Add your user to the docker group:

```bash
sudo usermod -aG docker $USER
newgrp docker
```

### Service Won't Start

Check service status and logs:

```bash
sudo systemctl status rodent.service
sudo journalctl -u rodent.service -n 50
```

Verify configuration:

```bash
sudo -u rodent cat /home/rodent/.rodent/rodent.yml
```

### Binary Download Fails

Verify network connectivity:

```bash
curl -I https://pkg.strata.host/rodent/latest/rodent-linux-amd64
```

Or manually download:

```bash
curl -fsSL https://pkg.strata.host/rodent/latest/rodent-linux-amd64 -o /tmp/rodent
sudo chmod +x /tmp/rodent
sudo mv /tmp/rodent /usr/local/bin/rodent
```

## Uninstallation

### Quick Uninstall

Remove Rodent while keeping data and dependencies:

```bash
curl -fsSL https://utils.strata.host/rodent/uninstall-rodent.sh | sudo bash
```

### Complete Removal

Remove everything including data, logs, and dependencies:

```bash
curl -fsSL https://utils.strata.host/rodent/uninstall-rodent.sh | sudo bash -s -- \
  --yes --remove-data --remove-logs --remove-deps
```

### Uninstall Options

```sh
--yes              Non-interactive mode
--keep-user        Keep rodent user account
--remove-data      Remove /var/lib/rodent
--remove-logs      Remove /var/log/rodent
--remove-deps      Remove ZFS, Docker, Samba
--no-backup        Skip configuration backup
```

Configuration backups are saved to `/etc/rodent/backups/` by default.

## Upgrade

To upgrade to a new version:

```bash
# Re-run installer (it will detect existing installation)
curl -fsSL https://utils.strata.host/install.sh | sudo bash -s -- --non-interactive
```

Or install a specific version:

```bash
curl -fsSL https://utils.strata.host/install.sh | sudo bash -s -- \
  --non-interactive --install-version v1.2.3
```

## Support

- **Documentation**: <https://docs.stratastor.com/rodent>
- **Issues**: <https://github.com/stratastor/rodent/issues>
- **Installation Telemetry**: /var/log/rodent-install-telemetry.json

## Security Notes

- The installer requires root/sudo access
- All sensitive operations are logged
- Rodent runs as a dedicated system user
- Sudo permissions are scoped to necessary commands
- Kerberos credentials are pre-configured non-interactively
- Binary is downloaded over HTTPS
- All repositories use GPG-signed packages
