#!/bin/bash
# Rodent Full Installer Script
# Copyright 2025 The StrataSTOR Authors and Contributors
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

VERSION="1.0.0"

# Colors
RED='\033[38;2;255;183;178m'
GREEN='\033[38;2;152;251;152m'
YELLOW='\033[38;2;230;230;250m'
BLUE='\033[38;2;176;224;230m'
NC='\033[0m'

# Default values
INTERACTIVE=true
INSTALL_DEPS=true
INSTALL_ZFS=true
INSTALL_DOCKER=true
INSTALL_SAMBA=true
VERSION_TO_INSTALL="latest"
FORCE=false
DEV_MODE=false
SKIP_ZFS_VERSION_CHECK=false
VERBOSE=false
APT_QUIET="-q"

# Minimum versions
MIN_ZFS_VERSION="2.3.0"
REQUIRED_UBUNTU_VERSION="24.04"

# Default Kerberos values
DEFAULT_KRB_REALM="AD.STRATA.INTERNAL"
DEFAULT_KRB_ADMIN_SERVER="DC1.AD.STRATA.INTERNAL"

# Logging
LOG_FILE="/var/log/rodent-install.log"
INSTALL_ID=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || echo "unknown-$(date +%s)")
INSTALL_START_TIME=$(date +%s)

# Download URLs
PKG_DOMAIN="pkg.strata.host"
UTILS_DOMAIN="utils.strata.host"

# Detect script location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  Rodent Installer v${VERSION}${NC}"
    echo -e "${BLUE}  StrataSTOR Node Agent${NC}"
    echo -e "${BLUE}================================================${NC}"
    echo ""
}

log() {
    local level="$1"
    shift
    local message="$@"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "${timestamp} [${level}] ${message}" >> "$LOG_FILE" 2>/dev/null || true
}

log_info() {
    log "INFO" "$@"
    echo -e "${BLUE}ℹ${NC} $@"
}

log_success() {
    log "SUCCESS" "$@"
    echo -e "${GREEN}✓${NC} $@"
}

log_warn() {
    log "WARN" "$@"
    echo -e "${YELLOW}⚠${NC} $@"
}

log_error() {
    log "ERROR" "$@"
    echo -e "${RED}✗${NC} $@" >&2
}

log_step() {
    log "STEP" "$@"
    echo -e "\n${BLUE}▶${NC} $@"
}

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Rodent installer for Ubuntu 24.04+ systems.

OPTIONS:
    --help                  Show this help message
    --version               Show installer version
    --non-interactive       Run in non-interactive mode
    --install-version VER   Install specific version (default: latest)
    --skip-deps             Skip dependency installation
    --skip-zfs              Skip ZFS installation
    --skip-docker           Skip Docker installation
    --skip-samba            Skip Samba installation
    --skip-zfs-check        Skip ZFS version check (use with caution)
    --krb-realm REALM       Kerberos realm (default: ${DEFAULT_KRB_REALM})
    --krb-admin SERVER      Kerberos admin server (default: ${DEFAULT_KRB_ADMIN_SERVER})
    --dev                   Enable development mode
    --force                 Force installation even if checks fail
    --yes                   Assume yes to all prompts
    --verbose               Show detailed package installation output

EXAMPLES:
    # Interactive installation
    sudo ./install-rodent.sh

    # Non-interactive installation
    sudo ./install-rodent.sh --non-interactive --yes

    # Install specific version
    sudo ./install-rodent.sh --install-version v1.2.3

    # Custom Kerberos configuration
    sudo ./install-rodent.sh --krb-realm CORP.COM --krb-admin DC1.CORP.COM

EOF
    exit 0
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help) usage ;;
            --version) echo "Rodent Installer v${VERSION}"; exit 0 ;;
            --non-interactive|--yes) INTERACTIVE=false; shift ;;
            --install-version) VERSION_TO_INSTALL="$2"; shift 2 ;;
            --skip-deps) INSTALL_DEPS=false; shift ;;
            --skip-zfs) INSTALL_ZFS=false; shift ;;
            --skip-docker) INSTALL_DOCKER=false; shift ;;
            --skip-samba) INSTALL_SAMBA=false; shift ;;
            --skip-zfs-check) SKIP_ZFS_VERSION_CHECK=true; shift ;;
            --krb-realm) DEFAULT_KRB_REALM="$2"; shift 2 ;;
            --krb-admin) DEFAULT_KRB_ADMIN_SERVER="$2"; shift 2 ;;
            --dev) DEV_MODE=true; shift ;;
            --force) FORCE=true; shift ;;
            --verbose) VERBOSE=true; APT_QUIET=""; shift ;;
            *) log_error "Unknown option: $1"; usage ;;
        esac
    done
}

version_ge() {
    printf '%s\n%s\n' "$2" "$1" | sort -V -C
}

check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "This script must be run as root or with sudo"
        exit 1
    fi
}

check_os() {
    log_step "Checking operating system..."

    if [ ! -f /etc/lsb-release ]; then
        log_error "Not running on Ubuntu. Only Ubuntu 24.04+ is supported."
        exit 1
    fi

    source /etc/lsb-release

    if [ "$DISTRIB_ID" != "Ubuntu" ]; then
        log_error "Not running on Ubuntu. Found: $DISTRIB_ID"
        exit 1
    fi

    log_info "Detected: Ubuntu $DISTRIB_RELEASE ($DISTRIB_CODENAME)"

    if ! version_ge "$DISTRIB_RELEASE" "$REQUIRED_UBUNTU_VERSION"; then
        log_error "Ubuntu $REQUIRED_UBUNTU_VERSION or higher is required. Found: $DISTRIB_RELEASE"
        [ "$FORCE" != true ] && exit 1
        log_warn "Continuing due to --force flag..."
    fi

    export DISTRIB_CODENAME DISTRIB_RELEASE
    log_success "Operating system check passed"
}

check_architecture() {
    log_step "Checking system architecture..."

    ARCH=$(uname -m)
    case $ARCH in
        x86_64) ARCH_DEB="amd64"; ARCH_GO="amd64" ;;
        aarch64|arm64) ARCH_DEB="arm64"; ARCH_GO="arm64" ;;
        *)
            log_error "Unsupported architecture: $ARCH"
            log_error "Supported: x86_64 (amd64), aarch64 (arm64)"
            exit 1
            ;;
    esac

    log_success "Architecture: $ARCH ($ARCH_DEB)"
}

check_disk_space() {
    log_step "Checking disk space..."

    local required_mb=2048
    local available_mb=$(df /usr/local/bin 2>/dev/null | tail -1 | awk '{print int($4/1024)}')

    if [ $available_mb -lt $required_mb ]; then
        log_error "Insufficient disk space. Required: ${required_mb}MB, Available: ${available_mb}MB"
        [ "$FORCE" != true ] && exit 1
        log_warn "Continuing due to --force flag..."
    fi

    log_success "Disk space check passed (${available_mb}MB available)"
}

check_systemd() {
    log_step "Checking systemd..."

    if ! command -v systemctl &> /dev/null; then
        log_error "systemd is not available. Rodent requires systemd."
        exit 1
    fi

    # Check if systemd is actually running
    if ! systemctl is-system-running &> /dev/null && ! systemctl is-system-running --wait &> /dev/null; then
        log_error "systemd is not running or not fully operational"
        log_error "Rodent requires an active systemd-based system"
        [ "$FORCE" != true ] && exit 1
        log_warn "Continuing due to --force flag..."
    fi

    log_success "systemd check passed"
}

check_netplan() {
    log_step "Checking netplan..."

    if ! command -v netplan &> /dev/null; then
        log_error "netplan is not available. Rodent requires netplan for network management."
        [ "$FORCE" != true ] && exit 1
        log_warn "Continuing without netplan..."
    else
        log_success "netplan is available"
    fi
}

enable_systemd_resolved() {
    log_step "Checking systemd-resolved..."

    if ! systemctl is-active --quiet systemd-resolved; then
        log_info "systemd-resolved is not active, enabling..."
        systemctl enable systemd-resolved
        systemctl start systemd-resolved
        log_success "systemd-resolved enabled and started"
    else
        log_info "systemd-resolved is already active"
    fi

    # Ensure resolvectl is working
    if command -v resolvectl &> /dev/null; then
        log_success "resolvectl is available"
    else
        log_warn "resolvectl command not found"
    fi
}

check_existing_installation() {
    if [ -f /usr/local/bin/rodent ]; then
        RODENT_ALREADY_INSTALLED=true
        # Get current installed version
        CURRENT_RODENT_VERSION=$(/usr/local/bin/rodent version 2>/dev/null | grep -oP 'Rodent Version: \K.*' || echo "unknown")
        log_warn "Rodent is already installed"
        log_info "Current version: ${CURRENT_RODENT_VERSION}"
        log_info "Target version: ${VERSION_TO_INSTALL}"

        if [ "$INTERACTIVE" = true ]; then
            read -p "Do you want to continue? (y/N): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                log_info "Installation cancelled"
                exit 0
            fi
        fi
    else
        RODENT_ALREADY_INSTALLED=false
        CURRENT_RODENT_VERSION="none"
    fi
}

check_zfs_version() {
    if [ "$SKIP_ZFS_VERSION_CHECK" = true ]; then
        log_warn "Skipping ZFS version check as requested"
        return 0
    fi

    if ! command -v zfs &> /dev/null; then
        return 1
    fi

    # Parse version: "zfs-2.3.4-1" -> "2.3.4", "zfs-2.3.0-rc4" -> "2.3.0"
    local zfs_version=$(zfs version 2>/dev/null | head -1 | sed 's/^zfs-//' | cut -d'-' -f1)

    if [ -z "$zfs_version" ]; then
        return 1
    fi

    log_info "Found ZFS version: $zfs_version"

    if ! version_ge "$zfs_version" "$MIN_ZFS_VERSION"; then
        log_warn "ZFS version $MIN_ZFS_VERSION or higher is required. Found: $zfs_version"
        return 1
    fi

    log_success "ZFS version check passed"
    return 0
}

install_zfs() {
    log_step "Installing ZFS $MIN_ZFS_VERSION+ from Zabbly repository..."

    log_info "Setting up Zabbly repository for $DISTRIB_CODENAME..."
    mkdir -p /etc/apt/keyrings

    log_info "Downloading Zabbly GPG key..."
    rm -f /etc/apt/keyrings/zabbly.asc
    curl -fsSL https://pkgs.zabbly.com/key.asc -o /etc/apt/keyrings/zabbly.asc
    chmod 644 /etc/apt/keyrings/zabbly.asc

    log_info "Creating APT sources file..."
    cat > /etc/apt/sources.list.d/zabbly-kernel-stable.sources <<EOF
Enabled: yes
Types: deb
URIs: https://pkgs.zabbly.com/kernel/stable
Suites: ${DISTRIB_CODENAME}
Components: zfs
Architectures: ${ARCH_DEB}
Signed-By: /etc/apt/keyrings/zabbly.asc
EOF

    log_info "Updating package list..."
    apt-get update $APT_QUIET

    log_info "Installing ZFS packages (openzfs-zfsutils, openzfs-zfs-dkms, openzfs-zfs-initramfs)..."
    DEBIAN_FRONTEND=noninteractive apt-get install -y $APT_QUIET \
        openzfs-zfsutils \
        openzfs-zfs-dkms \
        openzfs-zfs-initramfs

    log_info "ZFS module will be loaded automatically"

    if check_zfs_version; then
        log_success "ZFS installed successfully"
    else
        log_error "ZFS installation verification failed"
        return 1
    fi
}

install_docker() {
    log_step "Installing Docker..."

    if command -v docker &> /dev/null; then
        log_info "Docker is already installed: $(docker --version)"
        return 0
    fi

    log_info "Adding Docker GPG key..."
    mkdir -p /etc/apt/keyrings
    rm -f /etc/apt/keyrings/docker.gpg
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg

    log_info "Adding Docker repository..."
    echo "deb [arch=${ARCH_DEB} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${DISTRIB_CODENAME} stable" \
        > /etc/apt/sources.list.d/docker.list

    log_info "Updating package list..."
    apt-get update $APT_QUIET

    log_info "Installing Docker packages..."
    DEBIAN_FRONTEND=noninteractive apt-get install -y $APT_QUIET \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin

    systemctl enable docker
    systemctl start docker

    log_success "Docker installed successfully: $(docker --version)"
}

install_samba() {
    log_step "Installing Samba with Kerberos client..."

    # Check if Samba is already installed
    if command -v smbd &> /dev/null; then
        local samba_version=$(smbd --version | head -1)
        log_info "Samba is already installed: ${samba_version}"

        # Check if Kerberos is configured
        if [ -f /etc/krb5.conf ]; then
            log_warn "Kerberos configuration already exists at /etc/krb5.conf"
            log_warn "Existing configuration will be preserved"
            log_info "If you need to reconfigure Kerberos, run: sudo dpkg-reconfigure krb5-config"
        fi

        return 0
    fi

    # Backup existing Kerberos config if it exists
    if [ -f /etc/krb5.conf ]; then
        local backup_file="/etc/krb5.conf.backup.$(date +%s)"
        log_warn "Existing Kerberos configuration found"
        log_info "Creating backup: ${backup_file}"
        cp /etc/krb5.conf "${backup_file}"
    fi

    log_info "Pre-configuring Kerberos client (non-interactive)..."
    log_info "  Realm: ${DEFAULT_KRB_REALM}"
    log_info "  KDC Server: ${DEFAULT_KRB_ADMIN_SERVER}"
    log_info "  Admin Server: ${DEFAULT_KRB_ADMIN_SERVER}"

    echo "krb5-config krb5-config/default_realm string ${DEFAULT_KRB_REALM}" | debconf-set-selections
    echo "krb5-config krb5-config/kerberos_servers string ${DEFAULT_KRB_ADMIN_SERVER}" | debconf-set-selections
    echo "krb5-config krb5-config/admin_server string ${DEFAULT_KRB_ADMIN_SERVER}" | debconf-set-selections

    log_info "Installing Samba and Kerberos packages..."
    DEBIAN_FRONTEND=noninteractive apt-get install -y $APT_QUIET \
        krb5-user \
        krb5-config \
        libpam-krb5 \
        samba \
        samba-common-bin \
        samba-vfs-modules \
        samba-ad-provision \
        winbind \
        libnss-winbind \
        libpam-winbind

    log_success "Samba installed successfully: $(smbd --version | head -1)"
    log_info "Kerberos configured with realm: ${DEFAULT_KRB_REALM}"
    log_info "Kerberos admin server: ${DEFAULT_KRB_ADMIN_SERVER}"
    log_info "Kerberos config file: /etc/krb5.conf"
}

install_utilities() {
    log_step "Installing system utilities..."

    log_info "Installing required packages..."
    DEBIAN_FRONTEND=noninteractive apt-get install -y $APT_QUIET \
        acl \
        attr \
        smartmontools \
        sg3-utils \
        lsscsi \
        netplan.io \
        systemd-resolved \
        curl \
        wget \
        gnupg \
        ca-certificates \
        jq \
        ripgrep \
        uuid-runtime

    log_success "System utilities installed successfully"
}

install_dependencies() {
    if [ "$INSTALL_DEPS" != true ]; then
        log_info "Skipping dependency installation"
        return 0
    fi

    log_step "Installing dependencies..."

    log_info "Updating package list..."
    apt-get update $APT_QUIET

    install_utilities

    if [ "$INSTALL_ZFS" = true ]; then
        if ! check_zfs_version; then
            install_zfs
        else
            log_info "ZFS is already installed with correct version"
        fi
    fi

    if [ "$INSTALL_DOCKER" = true ]; then
        install_docker
    fi

    if [ "$INSTALL_SAMBA" = true ]; then
        install_samba
    fi

    log_success "All dependencies installed successfully"
}

download_binary() {
    log_step "Downloading Rodent binary..."

    local download_url
    if [ "$VERSION_TO_INSTALL" = "latest" ]; then
        download_url="https://${PKG_DOMAIN}/rodent/latest/rodent-linux-${ARCH_GO}"
    else
        download_url="https://${PKG_DOMAIN}/rodent/${VERSION_TO_INSTALL}/rodent-linux-${ARCH_GO}"
    fi

    log_info "Download URL: $download_url"

    local temp_binary="/tmp/rodent-${INSTALL_ID}"

    if ! curl -fsSL -o "$temp_binary" "$download_url"; then
        log_error "Failed to download Rodent binary from $download_url"
        log_error "Please check your network connection and try again"
        return 1
    fi

    chmod +x "$temp_binary"

    # Backup existing binary if present
    if [ -f /usr/local/bin/rodent ]; then
        local backup_file="/usr/local/bin/rodent.backup.$(date +%Y%m%d-%H%M%S)"
        log_info "Backing up existing binary to $backup_file..."
        cp /usr/local/bin/rodent "$backup_file"
    fi

    mv "$temp_binary" /usr/local/bin/rodent

    log_success "Rodent binary installed to /usr/local/bin/rodent"

    # Try to get version
    if /usr/local/bin/rodent version &> /dev/null; then
        log_info "Version: $(/usr/local/bin/rodent version)"
    fi
}

setup_rodent_user() {
    log_step "Setting up Rodent user, directories, and permissions..."

    local setup_script="${SCRIPT_DIR}/setup_rodent_user.sh"

    if [ -f "$setup_script" ]; then
        log_info "Running user setup script..."
        bash "$setup_script" $([ "$DEV_MODE" = true ] && echo "--dev") || log_warn "User setup completed with warnings"
        log_success "Rodent user and permissions configured"
    else
        log_error "setup_rodent_user.sh not found at $setup_script"
        log_error "Cannot proceed without user setup script"
        exit 1
    fi
}

install_service() {
    log_step "Installing systemd service..."

    local service_file="${SCRIPT_DIR}/rodent.service"

    if [ ! -f "$service_file" ]; then
        log_error "Service file not found: $service_file"
        return 1
    fi

    cp "$service_file" /etc/systemd/system/rodent.service
    systemctl daemon-reload

    log_success "Systemd service installed"
    log_info "Service will not be started automatically"
}

collect_telemetry() {
    local install_end_time=$(date +%s)
    local duration=$((install_end_time - INSTALL_START_TIME))

    log_info "Collecting installation telemetry..."

    local zfs_installed="false"
    local docker_installed="false"
    local samba_installed="false"

    command -v zfs &> /dev/null && zfs_installed="true"
    command -v docker &> /dev/null && docker_installed="true"
    dpkg -l 2>/dev/null | grep -q "^ii  samba " && samba_installed="true"

    # Determine install type: fresh, reinstall, or upgrade
    local install_type="fresh"
    local previous_version="null"

    if [ "$RODENT_ALREADY_INSTALLED" = true ]; then
        previous_version="\"${CURRENT_RODENT_VERSION}\""

        # Compare versions to determine if upgrade or reinstall
        if [ "$CURRENT_RODENT_VERSION" = "$VERSION_TO_INSTALL" ] || [ "$CURRENT_RODENT_VERSION" = "unknown" ]; then
            install_type="reinstall"
        else
            install_type="upgrade"
        fi
    fi

    cat > /var/log/rodent-install-telemetry.json <<EOF
{
  "installation_id": "${INSTALL_ID}",
  "version": "${VERSION_TO_INSTALL}",
  "previous_version": ${previous_version},
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "duration_seconds": ${duration},
  "platform": {
    "os": "ubuntu",
    "version": "${DISTRIB_RELEASE:-unknown}",
    "codename": "${DISTRIB_CODENAME:-unknown}",
    "arch": "${ARCH_DEB}",
    "kernel": "$(uname -r)"
  },
  "install_type": "${install_type}",
  "install_mode": "$([ "$INTERACTIVE" = true ] && echo "interactive" || echo "automated")",
  "components": {
    "zfs": ${zfs_installed},
    "docker": ${docker_installed},
    "samba": ${samba_installed}
  },
  "dev_mode": $([ "$DEV_MODE" = true ] && echo "true" || echo "false"),
  "success": true
}
EOF

    log_info "Telemetry saved to /var/log/rodent-install-telemetry.json"
}

print_next_steps() {
    echo ""
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}  Installation completed successfully!${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo ""
    echo -e "${BLUE}Next steps:${NC}"
    echo ""
    echo -e "  1. Configure Rodent with your Toggle JWT:"
    echo -e "     ${YELLOW}sudo nano /home/rodent/.rodent/rodent.yml${NC}"
    echo -e "     (Edit the ${YELLOW}toggle.jwt${NC} field)"
    echo ""
    echo -e "  2. Start the Rodent service:"
    echo -e "     ${YELLOW}sudo systemctl enable --now rodent.service${NC}"
    echo ""
    echo -e "  3. Check service status:"
    echo -e "     ${YELLOW}sudo systemctl status rodent.service${NC}"
    echo ""
    echo -e "  4. View logs:"
    echo -e "     ${YELLOW}sudo journalctl -u rodent.service -f${NC}"
    echo ""
    echo -e "${BLUE}Important notes:${NC}"
    echo -e "  • Kerberos realm: ${YELLOW}${DEFAULT_KRB_REALM}${NC}"
    echo -e "  • Kerberos admin: ${YELLOW}${DEFAULT_KRB_ADMIN_SERVER}${NC}"
    echo -e "  • Config file: ${YELLOW}/home/rodent/.rodent/rodent.yml${NC}"
    echo -e "  • Installation log: ${YELLOW}${LOG_FILE}${NC}"
    echo ""
    echo -e "${BLUE}Documentation:${NC}"
    echo -e "  https://docs.strata.foo/rodent"
    echo ""
}

main() {
    parse_args "$@"

    touch "$LOG_FILE" 2>/dev/null || LOG_FILE="/tmp/rodent-install.log"
    chmod 644 "$LOG_FILE" 2>/dev/null || true

    print_header

    log_info "Installation ID: $INSTALL_ID"
    log_info "Installer version: $VERSION"
    log_info "Starting Rodent installation..."

    # Pre-flight checks
    check_root
    check_os
    check_architecture
    check_disk_space
    check_systemd
    check_netplan
    enable_systemd_resolved
    check_existing_installation

    # Install dependencies
    install_dependencies

    # Download and install binary
    download_binary

    # Setup user and environment
    setup_rodent_user
    install_service

    # Collect telemetry
    collect_telemetry

    # Done
    print_next_steps

    log_success "Installation completed successfully in $(($(date +%s) - INSTALL_START_TIME)) seconds"
}

trap 'log_error "Installation failed at line $LINENO. Check log: $LOG_FILE"; exit 1' ERR

main "$@"
