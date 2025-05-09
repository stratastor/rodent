#!/bin/bash
# Setup script for rodent user and configuration

set -e

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo "This script must be run as root"
  exit 1
fi

# Create rodent user if it doesn't exist
if ! id rodent &>/dev/null; then
  echo "Creating rodent user..."
  useradd -m -s /bin/bash rodent
else
  echo "Rodent user already exists."
fi

# Create necessary directories
echo "Creating directory structure..."

# Base directories
mkdir -p /home/rodent/.rodent
mkdir -p /home/rodent/.ssh
mkdir -p /var/lib/rodent
mkdir -p /var/log/rodent

# Configuration directory structure
mkdir -p /home/rodent/.rodent/ssh
mkdir -p /home/rodent/.rodent/templates/traefik
mkdir -p /home/rodent/.rodent/state

# Set proper ownership
echo "Setting proper ownership..."
chown -R rodent:rodent /home/rodent/.rodent
chown -R rodent:rodent /home/rodent/.ssh
chown -R rodent:rodent /var/lib/rodent
chown -R rodent:rodent /var/log/rodent

# Set proper permissions
echo "Setting proper permissions..."
chmod 700 /home/rodent/.ssh
chmod 700 /home/rodent/.rodent/ssh
chmod 755 /var/lib/rodent
chmod 755 /var/log/rodent

# Copy default configuration
if [ -d "/etc/rodent" ]; then
  echo "Copying existing configuration from /etc/rodent..."
  cp -r /etc/rodent/* /home/rodent/.rodent/ 2>/dev/null || echo "No files to copy from /etc/rodent"
  chown -R rodent:rodent /home/rodent/.rodent
fi

# Create default config file if it doesn't exist
if [ ! -f "/home/rodent/.rodent/rodent.yml" ]; then
  echo "Creating default configuration file..."
  cat > /home/rodent/.rodent/rodent.yml <<EOL
# Rodent configuration file
server:
  port: 8042
  logLevel: info
logs:
  path: /var/log/rodent/rodent.log
  retention: 7d
  output: file
logger:
  logLevel: info
  enableSentry: false
keys:
  ssh:
    username: rodent
    dirPath: /home/rodent/.rodent/ssh
    algorithm: ed25519
    knownHostsFile: /home/rodent/.rodent/ssh/known_hosts
    authorizedKeysFile: /home/rodent/.ssh/authorized_keys
shares:
  smb:
    realm: AD.STRATA.INTERNAL
    workgroup: AD
EOL
  chown rodent:rodent /home/rodent/.rodent/rodent.yml
  chmod 600 /home/rodent/.rodent/rodent.yml
fi

# Create authorized_keys file if it doesn't exist
if [ ! -f "/home/rodent/.ssh/authorized_keys" ]; then
  echo "Creating authorized_keys file..."
  touch /home/rodent/.ssh/authorized_keys
  chown rodent:rodent /home/rodent/.ssh/authorized_keys
  chmod 600 /home/rodent/.ssh/authorized_keys
fi

# Install systemd service file
echo "Installing systemd service file..."
cp $(dirname "$0")/rodent.service /etc/systemd/system/
systemctl daemon-reload

# Install sudoers file
echo "Installing sudoers file..."
cp $(dirname "$0")/rodent.sudoers /etc/sudoers.d/rodent
chmod 440 /etc/sudoers.d/rodent

echo "Setup complete. To start the service, run: systemctl enable --now rodent.service"