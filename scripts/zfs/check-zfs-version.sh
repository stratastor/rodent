#!/bin/bash
# Check ZFS userspace vs kernel module version mismatch

set -e

# Get versions
ZFS_VERSION=$(zfs version 2>/dev/null | head -1 | awk '{print $1}' | cut -d'-' -f2)
KMOD_VERSION=$(cat /sys/module/zfs/version 2>/dev/null || echo "not-loaded")

echo "ZFS Userspace: zfs-${ZFS_VERSION}"
echo "ZFS Kernel Module: zfs-kmod-${KMOD_VERSION}"

# Normalize versions for comparison (remove trailing -N)
ZFS_VERSION_NORM=$(echo "$ZFS_VERSION" | cut -d'-' -f1)
KMOD_VERSION_NORM=$(echo "$KMOD_VERSION" | cut -d'-' -f1)

# Check if versions match (major.minor.patch)
if [ "$ZFS_VERSION_NORM" != "$KMOD_VERSION_NORM" ]; then
    echo ""
    echo "WARNING: Version mismatch detected!"
    echo "Userspace: ${ZFS_VERSION}"
    echo "Module:    ${KMOD_VERSION}"
    echo ""
    echo "Run: sudo ./rebuild-zfs-dkms.sh"
    exit 1
else
    echo ""
    echo "OK: Versions match"
    exit 0
fi
