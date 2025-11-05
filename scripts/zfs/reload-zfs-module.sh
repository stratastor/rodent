#!/bin/bash
# Reload ZFS kernel module (requires pool export/import)

set -e

if [ "$EUID" -ne 0 ]; then
    echo "ERROR: Must run as root (use sudo)"
    exit 1
fi

# Get current version
OLD_VERSION=$(cat /sys/module/zfs/version 2>/dev/null || echo "not-loaded")
echo "Current loaded module: zfs-kmod-${OLD_VERSION}"

# Check what version DKMS has for current kernel
KERNEL=$(uname -r)
DKMS_VERSION=$(dkms status | grep -E "^zfs/.*, ${KERNEL}.*: installed" | cut -d'/' -f2 | cut -d',' -f1)

if [ -z "$DKMS_VERSION" ]; then
    echo "ERROR: No DKMS module built for kernel ${KERNEL}"
    echo "Run: sudo ./rebuild-zfs-dkms.sh"
    exit 1
fi

echo "DKMS module available: zfs-${DKMS_VERSION}"
echo ""

# Get list of imported pools
POOLS=$(zpool list -H -o name 2>/dev/null || true)

if [ -n "$POOLS" ]; then
    echo "The following pools will be exported:"
    echo "$POOLS"
    echo ""
fi

# Confirm
read -p "This will export all pools and reload the ZFS module. Continue? (yes/no): " CONFIRM
if [ "$CONFIRM" != "yes" ]; then
    echo "Aborted"
    exit 0
fi

# Export all pools
if [ -n "$POOLS" ]; then
    echo "Exporting pools..."
    for POOL in $POOLS; do
        echo "  Exporting ${POOL}..."
        zpool export "$POOL" || {
            echo "ERROR: Failed to export ${POOL}"
            echo "Check for processes using the pool: lsof | grep ${POOL}"
            exit 1
        }
    done
fi

# Stop ZFS services
echo "Stopping ZFS services..."
systemctl stop zfs-zed.service zfs-mount.service zfs.target 2>/dev/null || true

# Unload old module
echo "Unloading old ZFS module..."
modprobe -r zfs spl || {
    echo "ERROR: Failed to unload ZFS module"
    echo "Check: lsmod | grep zfs"
    exit 1
}

# Load new module
echo "Loading new ZFS module..."
modprobe zfs || {
    echo "ERROR: Failed to load ZFS module"
    exit 1
}

# Verify new version
NEW_VERSION=$(cat /sys/module/zfs/version)
echo ""
echo "New module loaded: zfs-kmod-${NEW_VERSION}"

# Import all pools
if [ -n "$POOLS" ]; then
    echo ""
    echo "Importing all pools..."
    zpool import -a || {
        echo "WARNING: Failed to import pools"
        echo "You may need to manually import: sudo zpool import -a"
    }
fi

# Start ZFS services
echo "Starting ZFS services..."
systemctl start zfs.target zfs-mount.service zfs-zed.service 2>/dev/null || true

echo ""
echo "SUCCESS: ZFS module reloaded"
echo "Old version: ${OLD_VERSION}"
echo "New version: ${NEW_VERSION}"
echo ""

# Verify pool status
if [ -n "$POOLS" ]; then
    echo "Pool status:"
    zpool list
fi
