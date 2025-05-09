#!/bin/bash
# Cleanup script to remove rodent user and configuration

set -e

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo "This script must be run as root"
  exit 1
fi

# Ask for confirmation
echo "WARNING: This will remove the rodent user, service, and all associated configurations."
read -p "Are you sure you want to continue? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Operation cancelled."
  exit 0
fi

# Stop and disable the service
echo "Stopping and disabling rodent service..."
systemctl stop rodent.service 2>/dev/null || true
systemctl disable rodent.service 2>/dev/null || true

# Remove systemd service file
echo "Removing systemd service file..."
rm -f /etc/systemd/system/rodent.service
systemctl daemon-reload

# Remove sudoers file
echo "Removing sudoers file..."
rm -f /etc/sudoers.d/rodent

# Backup configuration if requested
read -p "Do you want to backup rodent configuration before removal? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  BACKUP_DIR="/root/rodent-backup-$(date +%Y%m%d-%H%M%S)"
  echo "Creating backup at $BACKUP_DIR..."
  mkdir -p "$BACKUP_DIR"
  
  # Backup rodent home directory
  if [ -d "/home/rodent" ]; then
    cp -r /home/rodent "$BACKUP_DIR/home_rodent"
  fi
  
  # Backup data and logs
  if [ -d "/var/lib/rodent" ]; then
    cp -r /var/lib/rodent "$BACKUP_DIR/var_lib_rodent"
  fi
  
  if [ -d "/var/log/rodent" ]; then
    cp -r /var/log/rodent "$BACKUP_DIR/var_log_rodent"
  fi
  
  echo "Backup created successfully at $BACKUP_DIR"
fi

# Remove rodent user's files
echo "Removing rodent user's files..."
rm -rf /home/rodent/.rodent 2>/dev/null || true

# Remove service data and logs
read -p "Do you want to remove all rodent data and logs? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  echo "Removing service data and logs..."
  rm -rf /var/lib/rodent 2>/dev/null || true
  rm -rf /var/log/rodent 2>/dev/null || true
else
  echo "Keeping service data and logs."
fi

# Remove rodent user
read -p "Do you want to remove the rodent user? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  echo "Removing rodent user..."
  userdel -r rodent 2>/dev/null || true
else
  echo "Keeping rodent user but removing service configuration."
fi

echo "Cleanup completed successfully."