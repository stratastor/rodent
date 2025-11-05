#!/bin/bash
# Check and update ZFS module if needed

set -e

if [ "$EUID" -ne 0 ]; then
    echo "ERROR: Must run as root (use sudo)"
    exit 1
fi

KERNEL=$(uname -r)

echo "=== ZFS Module Update Check ==="
echo ""

# Get versions
ZFS_VERSION=$(zfs version 2>/dev/null | head -1 | awk '{print $1}' | cut -d'-' -f2)
KMOD_VERSION=$(cat /sys/module/zfs/version 2>/dev/null || echo "not-loaded")

echo "ZFS Userspace:     ${ZFS_VERSION}"
echo "ZFS Kernel Module: ${KMOD_VERSION}"
echo "Current Kernel:    ${KERNEL}"
echo ""

# Normalize versions for comparison (remove trailing -N)
ZFS_VERSION_NORM=$(echo "$ZFS_VERSION" | cut -d'-' -f1)
KMOD_VERSION_NORM=$(echo "$KMOD_VERSION" | cut -d'-' -f1)

# Check if versions match (major.minor.patch)
if [ "$ZFS_VERSION_NORM" = "$KMOD_VERSION_NORM" ]; then
    echo "OK: Versions match, no update needed"
    exit 0
fi

echo "WARNING: Version mismatch detected"
echo ""

# Check DKMS status
ZFS_DKMS_VERSION=$(dkms status | grep -E '^zfs/' | head -1 | cut -d'/' -f2 | cut -d',' -f1)

if [ -z "$ZFS_DKMS_VERSION" ]; then
    echo "ERROR: No ZFS DKMS package found"
    exit 1
fi

echo "ZFS DKMS Package: ${ZFS_DKMS_VERSION}"

# Check if built for current kernel
if ! dkms status "zfs/${ZFS_DKMS_VERSION}" -k "${KERNEL}" 2>/dev/null | grep -q "installed"; then
    echo "DKMS module not built for current kernel"
    echo ""

    # Check kernel headers
    if [ ! -d "/lib/modules/${KERNEL}/build" ]; then
        echo "Installing kernel headers for ${KERNEL}..."
        apt-get update
        apt-get install -y "linux-headers-${KERNEL}" || {
            echo "ERROR: Failed to install kernel headers"
            exit 1
        }
    fi

    # Build DKMS
    echo "Building ZFS DKMS module for ${KERNEL}..."
    dkms install "zfs/${ZFS_DKMS_VERSION}" -k "${KERNEL}" || {
        echo "ERROR: DKMS build failed"
        exit 1
    }
    echo "SUCCESS: DKMS module built"
    echo ""
fi

# Offer to reload module
echo "DKMS module is ready. To apply the update:"
echo "1. All ZFS pools will be exported"
echo "2. ZFS module will be reloaded"
echo "3. Pools will be imported back"
echo ""

POOLS=$(zpool list -H -o name 2>/dev/null || true)
if [ -n "$POOLS" ]; then
    echo "Imported pools: ${POOLS}"
    echo ""
fi

read -p "Reload ZFS module now? (yes/no): " RELOAD
if [ "$RELOAD" = "yes" ]; then
    # Export pools
    if [ -n "$POOLS" ]; then
        echo "Exporting pools..."
        for POOL in $POOLS; do
            zpool export "$POOL" || {
                echo "ERROR: Failed to export ${POOL}"
                exit 1
            }
        done
    fi

    # Stop services
    systemctl stop zfs-zed.service zfs-mount.service zfs.target 2>/dev/null || true

    # Reload module
    modprobe -r zfs spl || {
        echo "ERROR: Failed to unload module"
        exit 1
    }
    modprobe zfs || {
        echo "ERROR: Failed to load module"
        exit 1
    }

    # Import all pools
    if [ -n "$POOLS" ]; then
        echo "Importing all pools..."
        zpool import -a
    fi

    # Start services
    systemctl start zfs.target zfs-mount.service zfs-zed.service 2>/dev/null || true

    NEW_VERSION=$(cat /sys/module/zfs/version)
    echo ""
    echo "SUCCESS: Module updated"
    echo "Old version: ${KMOD_VERSION}"
    echo "New version: ${NEW_VERSION}"
else
    echo "Skipped reload. Run later: sudo ./reload-zfs-module.sh"
fi
