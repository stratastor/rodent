#!/bin/bash
# Rodent Uninstall Script
# Copyright 2025 The StrataSTOR Authors and Contributors
# SPDX-License-Identifier: Apache-2.0
#
# Usage: curl -fsSL https://utils.strata.host/rodent/uninstall-rodent.sh | sudo bash
# Or with options: curl -fsSL https://utils.strata.host/rodent/uninstall-rodent.sh | sudo bash -s -- --yes

set -e
set -o pipefail

# Colors
RED='\033[38;2;255;183;178m'
GREEN='\033[38;2;152;251;152m'
YELLOW='\033[38;2;230;230;250m'
BLUE='\033[38;2;176;224;230m'
NC='\033[0m'

# Default options
NON_INTERACTIVE=false
REMOVE_USER=true
REMOVE_DATA=false
REMOVE_LOGS=false
BACKUP_CONFIG=true
REMOVE_DEPS=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --yes|-y)
            NON_INTERACTIVE=true
            shift
            ;;
        --keep-user)
            REMOVE_USER=false
            shift
            ;;
        --remove-data)
            REMOVE_DATA=true
            shift
            ;;
        --remove-logs)
            REMOVE_LOGS=true
            shift
            ;;
        --no-backup)
            BACKUP_CONFIG=false
            shift
            ;;
        --remove-deps)
            REMOVE_DEPS=true
            shift
            ;;
        --help|-h)
            cat <<EOF
Rodent Uninstall Script

Usage: $0 [OPTIONS]

Options:
  --yes, -y           Run without prompts (use with caution)
  --keep-user         Keep the rodent user account
  --remove-data       Remove all rodent data (/var/lib/rodent)
  --remove-logs       Remove all rodent logs (/var/log/rodent)
  --no-backup         Skip configuration backup
  --remove-deps       Remove installed dependencies (ZFS, Docker, Samba)
  --help, -h          Show this help message

Examples:
  # Interactive uninstall (recommended)
  sudo $0

  # Non-interactive uninstall, keep data and logs
  sudo $0 --yes

  # Complete removal including data and logs
  sudo $0 --yes --remove-data --remove-logs

  # Uninstall but keep the rodent user
  sudo $0 --yes --keep-user

  # Remote uninstall via curl
  curl -fsSL https://utils.strata.host/rodent/uninstall-rodent.sh | sudo bash
EOF
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Run with --help for usage information"
            exit 1
            ;;
    esac
done

log_info() {
    echo -e "${BLUE}ℹ${NC} $@"
}

log_success() {
    echo -e "${GREEN}✓${NC} $@"
}

log_warn() {
    echo -e "${YELLOW}⚠${NC} $@"
}

log_error() {
    echo -e "${RED}✗${NC} $@" >&2
}

confirm() {
    if [ "$NON_INTERACTIVE" = true ]; then
        return 0
    fi

    local prompt="$1"
    local default="${2:-n}"

    if [ "$default" = "y" ]; then
        prompt="$prompt (Y/n)"
    else
        prompt="$prompt (y/N)"
    fi

    read -p "$prompt " -n 1 -r
    echo

    if [ "$default" = "y" ]; then
        [[ $REPLY =~ ^[Nn]$ ]] && return 1 || return 0
    else
        [[ $REPLY =~ ^[Yy]$ ]] && return 0 || return 1
    fi
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    log_error "This script must be run as root"
    exit 1
fi

# Show warning
echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}  Rodent Uninstall Script${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

if [ "$NON_INTERACTIVE" = false ]; then
    log_warn "This will remove Rodent from your system"
    if ! confirm "Are you sure you want to continue?" "n"; then
        log_info "Uninstall cancelled"
        exit 0
    fi
    echo ""
fi

# Stop and disable the service
log_info "Stopping rodent service..."
if systemctl is-active --quiet rodent.service 2>/dev/null; then
    systemctl stop rodent.service
    log_success "Service stopped"
else
    log_info "Service not running"
fi

if systemctl is-enabled --quiet rodent.service 2>/dev/null; then
    systemctl disable rodent.service
    log_success "Service disabled"
fi

# Backup configuration
if [ "$BACKUP_CONFIG" = true ] && [ -d "/home/rodent/.rodent" ]; then
    BACKUP_DIR="/etc/rodent/backups/$(date +%Y%m%d-%H%M%S)"
    log_info "Creating configuration backup at $BACKUP_DIR..."
    mkdir -p "$BACKUP_DIR"

    if [ -f "/home/rodent/.rodent/rodent.yml" ]; then
        cp /home/rodent/.rodent/rodent.yml "$BACKUP_DIR/"
        log_success "Backed up rodent.yml"
    fi

    if [ -f "/home/rodent/.rodent/rodent.db" ]; then
        cp /home/rodent/.rodent/rodent.db "$BACKUP_DIR/"
        log_success "Backed up rodent.db"
    fi

    if [ -d "/home/rodent/.ssh" ]; then
        cp -r /home/rodent/.ssh "$BACKUP_DIR/"
        log_success "Backed up SSH keys"
    fi

    log_success "Backup created at $BACKUP_DIR"
fi

# Remove systemd service file
log_info "Removing systemd service..."
rm -f /etc/systemd/system/rodent.service
systemctl daemon-reload
log_success "Systemd service removed"

# Remove sudoers file
log_info "Removing sudoers configuration..."
rm -f /etc/sudoers.d/rodent
log_success "Sudoers file removed"

# Remove binary
log_info "Removing rodent binary..."
rm -f /usr/local/bin/rodent
rm -f /usr/local/bin/rodent.backup.*
log_success "Binary removed"

# Remove data
if [ "$REMOVE_DATA" = true ]; then
    log_info "Removing rodent data..."
    rm -rf /var/lib/rodent
    rm -rf /home/rodent/.rodent
    log_success "Data removed"
else
    log_info "Keeping rodent data (use --remove-data to delete)"
fi

# Remove logs
if [ "$REMOVE_LOGS" = true ]; then
    log_info "Removing rodent logs..."
    rm -rf /var/log/rodent
    rm -f /var/log/rodent-install.log
    rm -f /var/log/rodent-install-telemetry.json
    log_success "Logs removed"
else
    log_info "Keeping rodent logs (use --remove-logs to delete)"
fi

# Remove user
if [ "$REMOVE_USER" = true ]; then
    if id "rodent" &>/dev/null; then
        log_info "Removing rodent user..."
        userdel -r rodent 2>/dev/null || userdel rodent 2>/dev/null || true
        log_success "User removed"
    else
        log_info "Rodent user does not exist"
    fi
else
    log_info "Keeping rodent user (use --keep-user to preserve)"
fi

# Remove dependencies (optional, dangerous!)
if [ "$REMOVE_DEPS" = true ]; then
    log_warn "Removing dependencies (ZFS, Docker, Samba)..."
    log_warn "This may affect other services on your system!"

    if ! confirm "Are you absolutely sure you want to remove dependencies?" "n"; then
        log_info "Skipping dependency removal"
    else
        # Check and handle ZFS pools
        if command -v zpool &>/dev/null; then
            ACTIVE_POOLS=$(zpool list -H 2>/dev/null | awk '{print $1}' || true)
            if [ -n "$ACTIVE_POOLS" ]; then
                log_warn "Active ZFS pools detected:"
                zpool list 2>/dev/null || true
                echo ""

                if confirm "Do you want to export all ZFS pools before removing ZFS?" "y"; then
                    log_info "Exporting ZFS pools..."
                    for pool in $ACTIVE_POOLS; do
                        log_info "Exporting pool: $pool"
                        zpool export "$pool" 2>/dev/null || log_warn "Failed to export $pool (may be in use)"
                    done
                    log_success "ZFS pools exported"
                else
                    log_warn "Skipping ZFS pool export - pools will remain imported"
                fi
            fi
        fi

        # Check Docker containers
        if command -v docker &>/dev/null; then
            RUNNING_CONTAINERS=$(docker ps -q 2>/dev/null || true)
            if [ -n "$RUNNING_CONTAINERS" ]; then
                log_warn "Running Docker containers detected:"
                docker ps --format "table {{.Names}}\t{{.Status}}" 2>/dev/null || true
                echo ""

                if confirm "Do you want to stop all containers before removing Docker?" "y"; then
                    log_info "Stopping Docker containers..."
                    docker stop $(docker ps -q) 2>/dev/null || true
                    log_success "Docker containers stopped"
                fi
            fi
        fi

        # Check Samba shares
        if command -v smbstatus &>/dev/null; then
            ACTIVE_SHARES=$(smbstatus -S 2>/dev/null | grep -v "^-" | grep -v "Service" | wc -l || echo "0")
            if [ "$ACTIVE_SHARES" -gt 0 ]; then
                log_warn "Active Samba shares detected"
                smbstatus -S 2>/dev/null || true
                echo ""
            fi
        fi

        # Remove Docker
        log_info "Removing Docker..."
        systemctl stop docker 2>/dev/null || true
        apt-get remove -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin 2>/dev/null || true
        log_success "Docker removed"

        # Remove Samba
        log_info "Removing Samba..."
        systemctl stop smbd nmbd winbind 2>/dev/null || true
        apt-get remove -y samba samba-common-bin winbind krb5-user 2>/dev/null || true
        log_success "Samba removed"

        # Remove ZFS
        log_info "Removing ZFS..."
        apt-get remove -y openzfs-zfsutils openzfs-zfs-dkms 2>/dev/null || true
        log_success "ZFS removed"

        log_success "All dependencies removed"
    fi
fi

echo ""
log_success "Rodent has been uninstalled"

if [ "$BACKUP_CONFIG" = true ] && [ -d "$BACKUP_DIR" ]; then
    echo ""
    log_info "Configuration backup: $BACKUP_DIR"
fi

if [ "$REMOVE_DATA" = false ]; then
    echo ""
    log_info "Data preserved:"
    [ -d "/var/lib/rodent" ] && echo "  - /var/lib/rodent"
    [ -d "/home/rodent/.rodent" ] && echo "  - /home/rodent/.rodent"
fi

if [ "$REMOVE_LOGS" = false ]; then
    echo ""
    log_info "Logs preserved:"
    [ -d "/var/log/rodent" ] && echo "  - /var/log/rodent"
    [ -f "/var/log/rodent-install.log" ] && echo "  - /var/log/rodent-install.log"
fi

echo ""
