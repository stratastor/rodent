#!/bin/bash
# Rebuild ZFS DKMS module for current kernel

set -e

if [ "$EUID" -ne 0 ]; then
    echo "ERROR: Must run as root (use sudo)"
    exit 1
fi

KERNEL=$(uname -r)
echo "Current kernel: ${KERNEL}"

# Check if kernel headers are installed
if [ ! -d "/lib/modules/${KERNEL}/build" ]; then
    echo "ERROR: Kernel headers not found for ${KERNEL}"
    echo "Installing headers..."
    apt-get update
    apt-get install -y "linux-headers-${KERNEL}" || {
        echo "ERROR: Failed to install kernel headers"
        exit 1
    }
fi

# Find ZFS DKMS version
ZFS_DKMS_VERSION=$(dkms status | grep -E '^zfs/' | head -1 | cut -d'/' -f2 | cut -d',' -f1)

if [ -z "$ZFS_DKMS_VERSION" ]; then
    echo "ERROR: No ZFS DKMS package found"
    echo "Available DKMS modules:"
    dkms status
    exit 1
fi

echo "ZFS DKMS version: ${ZFS_DKMS_VERSION}"

# Check if already built for current kernel
if dkms status "zfs/${ZFS_DKMS_VERSION}" -k "${KERNEL}" 2>/dev/null | grep -q "installed"; then
    echo "OK: DKMS already built for current kernel"
    exit 0
fi

# Build DKMS module
echo "Building ZFS DKMS module for ${KERNEL}..."
dkms install "zfs/${ZFS_DKMS_VERSION}" -k "${KERNEL}" || {
    echo "ERROR: DKMS build failed"
    echo "Check logs: /var/lib/dkms/zfs/${ZFS_DKMS_VERSION}/build/make.log"
    exit 1
}

echo ""
echo "SUCCESS: DKMS module built for ${KERNEL}"
echo ""
echo "To load the new module, you need to:"
echo "1. Export all ZFS pools: sudo zpool export <pool>"
echo "2. Unload old module: sudo modprobe -r zfs spl"
echo "3. Load new module: sudo modprobe zfs"
echo "4. Import pools: sudo zpool import <pool>"
echo ""
echo "Or run: sudo ./reload-zfs-module.sh"
