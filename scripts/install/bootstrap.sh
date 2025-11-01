#!/bin/bash
# Rodent Bootstrap Installer
# Copyright 2025 The StrataSTOR Authors and Contributors
# SPDX-License-Identifier: Apache-2.0
#
# Usage: curl -fsSL https://utils.strata.host/install.sh | sudo bash
# Or with options: curl -fsSL https://utils.strata.host/install.sh | sudo bash -s -- --dev

set -e
set -o pipefail

# Script version
BOOTSTRAP_VERSION="1.0.0"

# URLs
UTILS_DOMAIN="utils.strata.host"
INSTALLER_URL="https://${UTILS_DOMAIN}/rodent/install-rodent.sh"
SETUP_SCRIPT_URL="https://${UTILS_DOMAIN}/rodent/setup_rodent_user.sh"
SERVICE_FILE_URL="https://${UTILS_DOMAIN}/rodent/rodent.service"

# Colors
RED='\033[38;2;255;183;178m'
GREEN='\033[38;2;152;251;152m'
YELLOW='\033[38;2;255;218;185m'
BLUE='\033[38;2;176;224;230m'
NC='\033[0m'

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  Rodent Bootstrap Installer${NC}"
    echo -e "${BLUE}  StrataSTOR Node Agent${NC}"
    echo -e "${BLUE}================================================${NC}"
    echo ""
}

log_info() {
    echo -e "${BLUE}ℹ${NC} $@"
}

log_success() {
    echo -e "${GREEN}✓${NC} $@"
}

log_error() {
    echo -e "${RED}✗${NC} $@" >&2
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    log_error "This script must be run as root or with sudo"
    log_error "Usage: curl -fsSL https://utils.strata.host/install.sh | sudo bash"
    exit 1
fi

print_header

log_info "Bootstrap version: ${BOOTSTRAP_VERSION}"

# Quick preflight checks
log_info "Performing preflight checks..."

# Check if systemd
if ! command -v systemctl &> /dev/null; then
    log_error "systemd is not available. Rodent requires systemd."
    exit 1
fi

# Check if systemd is running and operational
SYSTEMD_STATE=$(systemctl is-system-running 2>&1 || true)
case "$SYSTEMD_STATE" in
    running|degraded)
        # System is operational (running = perfect, degraded = some units failed but systemd works)
        log_success "systemd is active (state: $SYSTEMD_STATE)"
        ;;
    offline|unknown)
        # systemd is not running or state cannot be determined
        log_error "systemd is not available (state: $SYSTEMD_STATE)"
        log_error "Rodent requires an active systemd-based system"
        exit 1
        ;;
    maintenance|stopping)
        # System is in rescue/emergency mode or shutting down
        log_error "systemd is not operational (state: $SYSTEMD_STATE)"
        log_error "System is in maintenance mode or shutting down"
        exit 1
        ;;
    initializing|starting)
        # System is still booting - wait for it to settle
        log_info "System is still booting (state: $SYSTEMD_STATE), waiting..."
        if SYSTEMD_STATE=$(systemctl is-system-running --wait 2>&1 || true); then
            case "$SYSTEMD_STATE" in
                running|degraded)
                    log_success "systemd is active (state: $SYSTEMD_STATE)"
                    ;;
                *)
                    log_error "systemd did not reach operational state (state: $SYSTEMD_STATE)"
                    exit 1
                    ;;
            esac
        else
            log_error "Failed to determine systemd state"
            exit 1
        fi
        ;;
    *)
        log_error "Unknown systemd state: $SYSTEMD_STATE"
        exit 1
        ;;
esac

# Check if netplan is available
if ! command -v netplan &> /dev/null; then
    log_error "netplan is not available. Rodent requires netplan for network management."
    exit 1
fi
log_success "netplan is available"

# Check if Ubuntu
if [ ! -f /etc/lsb-release ]; then
    log_error "Not running on Ubuntu. Only Ubuntu 24.04+ is supported."
    exit 1
fi

source /etc/lsb-release
if [ "$DISTRIB_ID" != "Ubuntu" ]; then
    log_error "Not running on Ubuntu. Found: $DISTRIB_ID"
    exit 1
fi

log_info "Detected: Ubuntu $DISTRIB_RELEASE"

# Version check
version_ge() {
    printf '%s\n%s\n' "$2" "$1" | sort -V -C
}

if ! version_ge "$DISTRIB_RELEASE" "24.04"; then
    log_error "Ubuntu 24.04 or higher is required. Found: $DISTRIB_RELEASE"
    exit 1
fi
log_success "Ubuntu version check passed"

# Create temporary directory
TEMP_DIR=$(mktemp -d -t rodent-install.XXXXXXXXXX)
trap "rm -rf $TEMP_DIR" EXIT

log_info "Downloading installer components..."

# Download main installer
if ! curl -fsSL -o "$TEMP_DIR/install-rodent.sh" "$INSTALLER_URL"; then
    log_error "Failed to download installer from $INSTALLER_URL"
    log_error "Please check your internet connection and try again"
    exit 1
fi

# Download setup script
if ! curl -fsSL -o "$TEMP_DIR/setup_rodent_user.sh" "$SETUP_SCRIPT_URL"; then
    log_error "Failed to download setup script from $SETUP_SCRIPT_URL"
    exit 1
fi

# Download service file
if ! curl -fsSL -o "$TEMP_DIR/rodent.service" "$SERVICE_FILE_URL"; then
    log_error "Failed to download service file from $SERVICE_FILE_URL"
    exit 1
fi

# Download sample config
if ! curl -fsSL -o "$TEMP_DIR/rodent.sample.yml" "https://${UTILS_DOMAIN}/rodent/rodent.sample.yml"; then
    log_error "Failed to download sample config"
    exit 1
fi

chmod +x "$TEMP_DIR/install-rodent.sh"
chmod +x "$TEMP_DIR/setup_rodent_user.sh"

log_success "Downloaded all components"

# Run the main installer with all passed arguments
log_info "Starting Rodent installation..."
echo ""

cd "$TEMP_DIR"
exec bash install-rodent.sh "$@"
