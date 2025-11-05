# ZFS Module Management Scripts

Scripts to manage ZFS DKMS module builds and updates.

## Scripts

### check-zfs-version.sh

Check for version mismatch between userspace and kernel module.

```bash
./check-zfs-version.sh
```

Exit code 0 if versions match, 1 if mismatch detected.

### rebuild-zfs-dkms.sh

Rebuild ZFS DKMS module for current kernel.

```bash
sudo ./rebuild-zfs-dkms.sh
```

Installs kernel headers if needed and builds DKMS module.

### reload-zfs-module.sh

Reload ZFS kernel module (exports/imports pools).

```bash
sudo ./reload-zfs-module.sh
```

Requires confirmation before proceeding.

### update-zfs-module.sh

All-in-one: check, rebuild, and optionally reload.

```bash
sudo ./update-zfs-module.sh
```

Recommended for manual updates after kernel upgrades.

## Usage Scenarios

### After Kernel Update

When kernel is updated, DKMS should automatically rebuild. Verify:

```bash
./check-zfs-version.sh
```

If mismatch detected:

```bash
sudo ./update-zfs-module.sh
```

### Manual DKMS Rebuild

If DKMS auto-build failed or wrong kernel:

```bash
sudo ./rebuild-zfs-dkms.sh
```

Then reload module:

```bash
sudo ./reload-zfs-module.sh
```

### Remote Server

For remote servers, split rebuild and reload:

```bash
# Build first (safe, no disruption)
sudo ./rebuild-zfs-dkms.sh

# Verify build succeeded
dkms status

# Schedule reload during maintenance window
sudo ./reload-zfs-module.sh
```

## Automation

### System Integration

DKMS handles automatic rebuilds via APT hooks. Manual scripts are for:

- Debugging build failures
- Forcing rebuilds for specific kernels
- Controlled module reloads

### Monitoring

Add to system monitoring:

```bash
# Check daily for version mismatch
0 6 * * * /path/to/check-zfs-version.sh || echo "ZFS version mismatch" | mail -s "Alert" admin@example.com
```

### Post-Reboot Hook

Create systemd service to check after reboot:

```ini
[Unit]
Description=Check ZFS Module Version
After=zfs.target

[Service]
Type=oneshot
ExecStart=/path/to/check-zfs-version.sh

[Install]
WantedBy=multi-user.target
```

## Notes

- Module reload requires exporting all pools (brief downtime)
- DKMS builds can take several minutes
- Kernel headers must match running kernel exactly
- Module versions must match for full feature support
