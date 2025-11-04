# ZFS RAIDZ Expansion Support

## Requirements

RAIDZ expansion requires:

- ZFS 2.2.0 or newer (both userspace and kernel module)
- Pool feature `feature@raidz_expansion` enabled
- Matching versions between userspace tools and kernel module

## Identifying Support

### Check Userspace and Kernel Module Versions

```bash
zfs version
```

Example output showing version mismatch:

```text
zfs-2.3.4-1
zfs-kmod-2.2.2-0ubuntu9.4
```

Here, userspace is 2.3.4 but kernel module is 2.2.2 (mismatch).

### Check Loaded Kernel Module

```bash
cat /sys/module/zfs/version
```

Example output:

```text
2.2.2-0ubuntu9.4
```

### Check Feature Availability

```bash
zpool upgrade -v | grep -A 3 raidz_expansion
```

If feature is missing from output, the loaded module does not support it.

Example output with support:

```text
raidz_expansion
     Support for raidz expansion
fast_dedup                            (read-only compatible)
     Support for advanced deduplication
```

### Check Pool Feature Status

```bash
zpool get all <pool> | grep expansion
```

Example output:

```text
pool  feature@raidz_expansion        disabled                       local
```

Feature states:

- `disabled`: Feature available but not enabled on pool
- `enabled`: Feature active on pool
- Missing: Feature not supported by loaded module

## Spotting Version Mismatch

### Symptom

Attach operation fails:

```bash
sudo zpool attach tiny raidz1-0 /dev/disk/by-id/...
```

Error:

```text
cannot attach <device> to raidz1-0: the loaded zfs module doesn't support raidz expansion
```

### Root Cause

Check versions:

```bash
zfs version
```

```text
zfs-2.3.4-1                    # Userspace supports feature
zfs-kmod-2.2.2-0ubuntu9.4      # Kernel module does not
```

Kernel module version is older than userspace, lacks RAIDZ expansion support.

## Fixing the Build

### Step 1: Verify DKMS Package

```bash
dpkg -l | grep -E 'zfs|openzfs'
```

Example output:

```text
ii  openzfs-zfs-dkms     2.3.4-arm64-202508290245-ubuntu24.04    arm64
ii  openzfs-zfsutils     2.3.4-arm64-202508290245-ubuntu24.04    arm64
```

### Step 2: Check DKMS Build Status

```bash
dkms status
```

Example output:

```text
zfs/2.3.4-arm64-202508290245, 6.8.0-86-generic, aarch64: installed
```

Note: Built for `6.8.0-86-generic` but current kernel is `6.8.0-1040-raspi`.

### Step 3: Install Kernel Headers

```bash
sudo apt-get install linux-headers-$(uname -r)
```

### Step 4: Build Module for Current Kernel

```bash
sudo dkms install zfs/2.3.4-arm64-202508290245 -k $(uname -r)
```

Or let dpkg trigger the build:

```bash
sudo dpkg --configure -a
```

### Step 5: Reload Module

Export pools and stop services:

```bash
sudo zpool export <pool>
sudo systemctl stop zfs-zed.service zfs-mount.service zfs.target
```

Unload old module:

```bash
sudo modprobe -r zfs spl
```

Load new module:

```bash
sudo modprobe zfs
```

Verify version:

```bash
cat /sys/module/zfs/version
```

Expected output:

```text
2.3.4-1
```

Verify with zfs version:

```bash
zfs version
```

Expected output (matching versions):

```text
zfs-2.3.4-1
zfs-kmod-2.3.4-1
```

### Step 6: Import and Upgrade Pool

```bash
sudo zpool import <pool>
sudo zpool upgrade <pool>
```

Example output:

```text
This system supports ZFS pool feature flags.

Enabled the following features on 'pool':
  redaction_list_spill
  raidz_expansion
  fast_dedup
  longname
  large_microzap
```

Verify feature is enabled:

```bash
zpool get all <pool> | grep expansion
```

```text
pool  feature@raidz_expansion        enabled                        local
```

## Testing RAIDZ Expansion

After successful setup:

```bash
sudo zpool attach <pool> <raidz-vdev> /dev/disk/by-id/<disk-id>
```

Monitor expansion:

```bash
zpool status <pool>
```

## Common Build Issues

### Missing Kernel Headers

```text
Error! Your kernel headers for kernel X.X.X cannot be found
```

Install headers: `sudo apt-get install linux-headers-$(uname -r)`

### DKMS Built for Wrong Kernel

Check kernel: `uname -r`

Check DKMS: `dkms status`

Rebuild: `sudo dkms install zfs/<version> -k $(uname -r)`

### Module In Use

```text
modprobe: FATAL: Module zfs is in use
```

Export pools and stop services before unloading module.

### I/O Errors During Build

System storage issue. Reboot and verify filesystem with `fsck`.
