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
mkdir -p /home/rodent/.rodent/shares/smb

# Create log directory in user's home as well for easier permissions
mkdir -p /home/rodent/.rodent/logs

# Set proper ownership
echo "Setting proper ownership..."
chown -R rodent:rodent /home/rodent/.rodent
chown -R rodent:rodent /home/rodent/.ssh
chown -R rodent:rodent /var/lib/rodent
chown -R rodent:rodent /var/log/rodent
# Don't fail if these directories don't exist
setfacl -R -m d:u:rodent:rwx /var/lib/rodent || true
setfacl -R -m d:u:rodent:rwx /var/log/rodent || true
setfacl -m d:u:rodent:rw /etc/samba/smb.conf || true
setfacl -m d:u:rodent:rw /etc/krb5.conf || true
setfacl -m d:u:rodent:rw /etc/hosts || true
setfacl -m d:u:rodent:rw /etc/hosts.allow || true
setfacl -m d:u:rodent:rw /etc/hosts.deny || true
setfacl -m d:u:rodent:rw /etc/resolv.conf || true

# Set proper permissions
echo "Setting proper permissions..."
chmod 700 /home/rodent/.ssh
chmod 700 /home/rodent/.rodent/ssh
chmod 755 /var/lib/rodent
chmod 755 /var/log/rodent

# Add rodent user to docker group if it exists
if getent group docker > /dev/null; then
  echo "Adding rodent user to docker group..."
  usermod -aG docker rodent
else
  echo "Docker group does not exist. Skipping adding rodent to docker group."
fi

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
  loglevel: debug
  daemonize: false
health:
  interval: 30s
  endpoint: /health
ad:
  adminpassword: Passw0rd
  ldapurl: ldaps://DC1.ad.strata.internal:636
  basedn: CN=Users,DC=ad,DC=strata,DC=internal
  admindn: CN=Administrator,CN=Users,DC=ad,DC=strata,DC=internal
logs:
  path: /home/rodent/.rodent/logs/rodent.log
  retention: 7d
  output: file
logger:
  loglevel: debug
  enablesentry: false
  sentrydsn: ""
toggle:
  jwt: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTg3NzQxODgsImlhdCI6MTc0NTM4ODU4OCwicHJ2Ijp0cnVlLCJyaWQiOiIydzdOS2FLaTVjNHdHOHphRW5URW04QWdkMzUiLCJzdWIiOiJjNzUwMGNjOC02M2UxLTRjMmItYWU4NS02MmFkOTA0YTdmNmIiLCJ0aWQiOiIydzdOS1o4QTloSEduTTJ1UjRRWGd0aEVBREkifQ.gwLIbSB7GlGEcBQeDSjIdlnZ4vhUCIQWj9WvOthxG6o
  baseurl: http://localhost:8142
  rpcaddr: localhost:8242
stratasecure: true
shares:
  smb:
    realm: AD.STRATA.INTERNAL
    workgroup: AD
keys:
  ssh:
    username: rodent
    dirPath: /home/rodent/.rodent/ssh
    algorithm: ed25519
    knownHostsFile: /home/rodent/.rodent/ssh/known_hosts
    authorizedKeysFile: /home/rodent/.ssh/authorized_keys
development:
  enabled: false
environment: dev

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

# Use sudo -u rodent -i instead of sudo su rodent to switch to the rodent user directly without a password(empty) prompt
echo "To switch to the rodent user, use: sudo -u rodent -i"