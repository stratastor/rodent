#!/bin/bash
# Setup script for rodent user and configuration

set -e

# Default values
DEV_MODE=false
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --dev)
      DEV_MODE=true
      shift
      ;;
    --help)
      echo "Usage: $0 [--dev] [--help]"
      echo "  --dev    Enable development mode with relaxed sudo permissions"
      echo "  --help   Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Use --help for usage information"
      exit 1
      ;;
  esac
done

# Function to detect binary path with fallback
detect_binary() {
    local binary="$1"
    local fallback="$2"
    local path
    
    path=$(which "$binary" 2>/dev/null)
    if [ -n "$path" ]; then
        echo "$path"
    else
        echo "Warning: $binary not found, using fallback: $fallback" >&2
        echo "$fallback"
    fi
}

# Function to generate systemctl commands for services
generate_systemctl_commands() {
    local systemctl_path="$1"
    shift
    local services=("$@")
    local output=""
    
    for service in "${services[@]}"; do
        output+=", \\
    $systemctl_path start ${service}*, \\
    $systemctl_path stop ${service}*, \\
    $systemctl_path restart ${service}*, \\
    $systemctl_path status ${service}*, \\
    $systemctl_path enable ${service}*, \\
    $systemctl_path disable ${service}*, \\
    $systemctl_path is-enabled ${service}*"
    done
    
    echo "$output"
}

# Function to generate file operations commands
generate_file_operations() {
    local cat_path="$1"
    local cp_path="$2"
    local tee_path="$3"
    local rm_path="$4"
    local chmod_path="$5"
    local test_path="$6"
    
    local files=(
        "/etc/samba/smb.conf*"
        "/etc/samba/conf.d/*"
        "/etc/hosts"
        "/etc/resolv.conf"
        "/etc/krb5.conf"
        "/etc/systemd/network*"
        "/etc/systemd/resolve*"
        "/run/systemd/network*"
        "/run/systemd/resolve*"
        "/etc/netplan/*"
    )
    
    local output=""
    local first=true
    
    for file in "${files[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            output+=" \\"$'\n'"    "
        fi
        
        case "$file" in
            */conf.d/*)
                output+="$cat_path $file, \\
    $cp_path * $file, \\
    $tee_path $file, \\
    $tee_path -a $file, \\
    $rm_path -f $file, \\
    $chmod_path * $file, \\
    $test_path -e $file"
                ;;
            *network*|*resolve*|*netplan*)
                output+="$cat_path $file, \\
    $cp_path * $file, \\
    $tee_path $file, \\
    $tee_path -a $file, \\
    $test_path -e $file, \\
    $chmod_path * $file, \\
    $rm_path -f $file"
                ;;
            */smb.conf*)
                output+="$cat_path $file, \\
    $cp_path * $file, \\
    $tee_path $file, \\
    $tee_path -a $file, \\
    $test_path -e $file, \\
    $chmod_path * $file"
                ;;
            *)
                output+="$cat_path $file, \\
    $cp_path * $file, \\
    $tee_path $file, \\
    $tee_path -a $file, \\
    $test_path -e $file"
                ;;
        esac
        
        if [ "$file" != "${files[-1]}" ]; then
            output+=","
        fi
    done
    
    echo "$output"
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo "This script must be run as root"
  exit 1
fi

echo "Setting up Rodent user and environment..."
if [ "$DEV_MODE" = true ]; then
    echo "Development mode enabled"
fi

# Detect binary paths
echo "Detecting binary paths..."

# Core system binaries
SYSTEMCTL_PATH=$(detect_binary "systemctl" "/usr/bin/systemctl")
MOUNT_PATH=$(detect_binary "mount" "/bin/mount")
IP_PATH=$(detect_binary "ip" "/usr/bin/ip")
NETWORKCTL_PATH=$(detect_binary "networkctl" "/usr/bin/networkctl")
RESOLVECTL_PATH=$(detect_binary "resolvectl" "/usr/bin/resolvectl")
JOURNALCTL_PATH=$(detect_binary "journalctl" "/usr/bin/journalctl")
NETPLAN_PATH=$(detect_binary "netplan" "/usr/sbin/netplan")
WHICH_PATH=$(detect_binary "which" "/usr/bin/which")
PING_PATH=$(detect_binary "ping" "/usr/bin/ping")
HOSTNAMECTL_PATH=$(detect_binary "hostnamectl" "/usr/bin/hostnamectl")
TIMEDATECTL_PATH=$(detect_binary "timedatectl" "/usr/bin/timedatectl")
LOCALECTL_PATH=$(detect_binary "localectl" "/usr/bin/localectl")
LAST_PATH=$(detect_binary "last" "/usr/bin/last")
HOSTNAME_PATH=$(detect_binary "hostname" "/usr/bin/hostname")
UNAME_PATH=$(detect_binary "uname" "/usr/bin/uname")
UPTIME_PATH=$(detect_binary "uptime" "/usr/bin/uptime")
GROUPS_PATH=$(detect_binary "groups" "/usr/bin/groups")
PASSWD_PATH=$(detect_binary "passwd" "/usr/bin/passwd")
OPENSSL_PATH=$(detect_binary "openssl" "/usr/bin/openssl")
DMIDECODE_PATH=$(detect_binary "dmidecode" "/usr/sbin/dmidecode")
SYSTEMD_DETECT_VIRT_PATH=$(detect_binary "systemd-detect-virt" "/usr/bin/systemd-detect-virt")
REBOOT_PATH=$(detect_binary "reboot" "/usr/sbin/reboot")
SHUTDOWN_PATH=$(detect_binary "shutdown" "/usr/sbin/shutdown")
USERADD_PATH=$(detect_binary "useradd" "/usr/sbin/useradd")
USERDEL_PATH=$(detect_binary "userdel" "/usr/sbin/userdel")
USERMOD_PATH=$(detect_binary "usermod" "/usr/sbin/usermod")
GROUPADD_PATH=$(detect_binary "groupadd" "/usr/sbin/groupadd")
GROUPDEL_PATH=$(detect_binary "groupdel" "/usr/sbin/groupdel")
GROUPMOD_PATH=$(detect_binary "groupmod" "/usr/sbin/groupmod")
CHPASSWD_PATH=$(detect_binary "chpasswd" "/usr/sbin/chpasswd")


# File operation binaries
CAT_PATH=$(detect_binary "cat" "/bin/cat")
CP_PATH=$(detect_binary "cp" "/bin/cp")
TEE_PATH=$(detect_binary "tee" "/bin/tee")
RM_PATH=$(detect_binary "rm" "/bin/rm")
CHMOD_PATH=$(detect_binary "chmod" "/bin/chmod")
TEST_PATH=$(detect_binary "test" "/usr/bin/test")
MKDIR_PATH=$(detect_binary "mkdir" "/usr/bin/mkdir")

# ZFS binaries
ZFS_PATH=$(detect_binary "zfs" "/usr/local/sbin/zfs")
ZPOOL_PATH=$(detect_binary "zpool" "/usr/local/sbin/zpool")

# SMB binaries
SMBSTATUS_PATH=$(detect_binary "smbstatus" "/usr/bin/smbstatus")
SMBCONTROL_PATH=$(detect_binary "smbcontrol" "/usr/bin/smbcontrol")
SMBCLIENT_PATH=$(detect_binary "smbclient" "/usr/bin/smbclient")
TESTPARM_PATH=$(detect_binary "testparm" "/usr/bin/testparm")

# Docker binaries
DOCKER_PATH=$(detect_binary "docker" "/usr/bin/docker")
DOCKER_SNAP_PATH="/snap/bin/docker"

# File ACL binaries
GETFACL_PATH=$(detect_binary "getfacl" "/usr/bin/getfacl")
SETFACL_PATH=$(detect_binary "setfacl" "/usr/bin/setfacl")

# Build secure path
SECURE_PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# Generate dynamic command lists
SMB_SERVICES=("smbd" "winbind" "nmbd")
SMB_SYSTEMCTL_COMMANDS=$(generate_systemctl_commands "$SYSTEMCTL_PATH" "${SMB_SERVICES[@]}")

DOCKER_SYSTEMCTL_COMMANDS=$(generate_systemctl_commands "$SYSTEMCTL_PATH" "docker")

RODENT_SYSTEMCTL_COMMANDS=$(generate_systemctl_commands "$SYSTEMCTL_PATH" "rodent")

# Add Docker snap support if it exists
DOCKER_SNAP_COMMANDS=""
if [ -f "$DOCKER_SNAP_PATH" ]; then
    DOCKER_SNAP_COMMANDS=", \\
    $DOCKER_SNAP_PATH *"
fi

# Development mode: allow broader systemctl access
SYSTEMCTL_DEV_COMMANDS=""
if [ "$DEV_MODE" = true ]; then
    SYSTEMCTL_DEV_COMMANDS=", \\
    $SYSTEMCTL_PATH *"
fi

# Generate file operations
FILE_OPERATIONS_COMMANDS=$(generate_file_operations "$CAT_PATH" "$CP_PATH" "$TEE_PATH" "$RM_PATH" "$CHMOD_PATH" "$TEST_PATH")

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
mkdir -p /home/rodent/.rodent/services
mkdir -p /home/rodent/.rodent/templates/traefik
mkdir -p /home/rodent/.rodent/state
mkdir -p /home/rodent/.rodent/shares/smb
mkdir -p /home/rodent/.rodent/etc/rodent

# Create log directory in user's home as well for easier permissions
mkdir -p /home/rodent/.rodent/logs

# Set proper ownership
echo "Setting proper ownership..."
chown -R rodent:rodent /home/rodent/.rodent
chown -R rodent:rodent /home/rodent/.ssh
chown -R rodent:rodent /var/lib/rodent
chown -R rodent:rodent /var/log/rodent

# Set ACLs if available
if command -v setfacl >/dev/null 2>&1; then
    echo "Setting file ACLs..."
    setfacl -R -m d:u:rodent:rwx /var/lib/rodent 2>/dev/null || true
    setfacl -R -m d:u:rodent:rwx /var/log/rodent 2>/dev/null || true
    setfacl -R -m d:u:rodent:rw /etc/samba 2>/dev/null || true
    setfacl -m d:u:rodent:rw /etc/samba/smb.conf 2>/dev/null || true
    setfacl -m d:u:rodent:rw /etc/krb5.conf 2>/dev/null || true
    setfacl -m d:u:rodent:rw /etc/hosts 2>/dev/null || true
    setfacl -m d:u:rodent:rw /etc/hosts.allow 2>/dev/null || true
    setfacl -m d:u:rodent:rw /etc/hosts.deny 2>/dev/null || true
    setfacl -m d:u:rodent:rw /etc/resolv.conf 2>/dev/null || true
    setfacl -R -m d:u:rodent:rw /etc/systemd/network 2>/dev/null || true
    setfacl -R -m d:u:rodent:rw /etc/systemd/resolve 2>/dev/null || true
    setfacl -R -m d:u:rodent:rw /run/systemd/network 2>/dev/null || true
    setfacl -R -m d:u:rodent:rw /run/systemd/resolve 2>/dev/null || true
    setfacl -R -m d:u:rodent:rw /etc/netplan 2>/dev/null || true
fi

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

# Copy existing configuration from /etc/rodent if it exists
if [ -d "/etc/rodent" ]; then
  echo "Copying existing configuration from /etc/rodent..."
  cp -r /etc/rodent/* /home/rodent/.rodent/etc/rodent/ 2>/dev/null || echo "No files to copy from /etc/rodent"
  chown -R rodent:rodent /home/rodent/.rodent
fi

# Copy existing Samba configuration from /etc/samba/shares.d if it exists
if [ -d "/etc/rodent" ]; then
  echo "Copying existing configuration from /etc/samba/shares..."
  cp -r /etc/samba/shares.d/* /home/rodent/.rodent/shares/smb/ 2>/dev/null || echo "No files to copy from /etc/samba/shares.d"
  chown -R rodent:rodent /home/rodent/.rodent/shares
fi

# Create configuration file if it doesn't exist
CONFIG_FILE="/home/rodent/.rodent/rodent.yml"
if [ ! -f "$CONFIG_FILE" ]; then
  echo "Creating configuration file..."
  
  if [ -f "$SCRIPT_DIR/rodent.config.tmpl" ]; then
    # Set template variables based on mode
    if [ "$DEV_MODE" = true ]; then
      LOG_LEVEL="debug"
      TOGGLE_BASEURL="http://localhost:8142"
      TOGGLE_RPCADDR="localhost:8242"
      DEV_ENABLED="true"
      ENVIRONMENT="dev"
    else
      LOG_LEVEL="info"
      TOGGLE_BASEURL="https://toggle.strata.foo"
      TOGGLE_RPCADDR="tunnel.strata.foo:443"
      DEV_ENABLED="false"
      ENVIRONMENT="prod"
    fi
    
    # Replace placeholders in template
    sed \
        -e "s|{{LOG_LEVEL}}|$LOG_LEVEL|g" \
        -e "s|{{TOGGLE_BASEURL}}|$TOGGLE_BASEURL|g" \
        -e "s|{{TOGGLE_RPCADDR}}|$TOGGLE_RPCADDR|g" \
        -e "s|{{DEV_ENABLED}}|$DEV_ENABLED|g" \
        -e "s|{{ENVIRONMENT}}|$ENVIRONMENT|g" \
        "$SCRIPT_DIR/rodent.config.tmpl" > "$CONFIG_FILE"
    
    echo "Generated configuration from template"
  else
    echo "Error: Configuration template not found at $SCRIPT_DIR/rodent.config.tmpl"
    exit 1
  fi
  
  chown rodent:rodent "$CONFIG_FILE"
  chmod 600 "$CONFIG_FILE"
else
  echo "Configuration file already exists, keeping existing settings"
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
if [ -f "$SCRIPT_DIR/rodent.service" ]; then
    cp "$SCRIPT_DIR/rodent.service" /etc/systemd/system/
    systemctl daemon-reload
else
    echo "Warning: rodent.service not found in $SCRIPT_DIR"
fi

# Generate and install sudoers file
echo "Generating and installing sudoers file..."
SUDOERS_OUTPUT="/etc/sudoers.d/rodent"
TEMP_SUDOERS=$(mktemp)

# Generate sudoers file directly with detected paths
cat > "$TEMP_SUDOERS" <<EOF
# Allow rodent to run specific commands with sudo without password
# This file should be installed at /etc/sudoers.d/rodent

# Command aliases for ZFS operations
Cmnd_Alias ZFS_COMMANDS = \\
    $ZFS_PATH *, \\
    $ZPOOL_PATH *

# Command aliases for SMB operations
Cmnd_Alias SMB_COMMANDS = \\
    $SMBSTATUS_PATH *, \\
    $SMBCONTROL_PATH *, \\
    $SMBCLIENT_PATH *, \\
    $TESTPARM_PATH *, \\
    $SYSTEMCTL_PATH start smbd*, \\
    $SYSTEMCTL_PATH stop smbd*, \\
    $SYSTEMCTL_PATH restart smbd*, \\
    $SYSTEMCTL_PATH status smbd*, \\
    $SYSTEMCTL_PATH enable smbd*, \\
    $SYSTEMCTL_PATH disable smbd*, \\
    $SYSTEMCTL_PATH is-enabled smbd*, \\
    $SYSTEMCTL_PATH start winbind*, \\
    $SYSTEMCTL_PATH stop winbind*, \\
    $SYSTEMCTL_PATH restart winbind*, \\
    $SYSTEMCTL_PATH status winbind*, \\
    $SYSTEMCTL_PATH enable winbind*, \\
    $SYSTEMCTL_PATH disable winbind*, \\
    $SYSTEMCTL_PATH is-enabled winbind*, \\
    $SYSTEMCTL_PATH start nmbd*, \\
    $SYSTEMCTL_PATH stop nmbd*, \\
    $SYSTEMCTL_PATH restart nmbd*, \\
    $SYSTEMCTL_PATH status nmbd*, \\
    $SYSTEMCTL_PATH enable nmbd*, \\
    $SYSTEMCTL_PATH disable nmbd*, \\
    $SYSTEMCTL_PATH is-enabled nmbd*

Cmnd_Alias DOCKER_COMMANDS = \\
    $DOCKER_PATH *$([ -f "/snap/bin/docker" ] && echo ", \\
    /snap/bin/docker *"), \\
    $SYSTEMCTL_PATH start docker*, \\
    $SYSTEMCTL_PATH stop docker*, \\
    $SYSTEMCTL_PATH restart docker*, \\
    $SYSTEMCTL_PATH is-enabled docker*, \\
    $SYSTEMCTL_PATH enable docker*, \\
    $SYSTEMCTL_PATH disable docker*, \\
    $SYSTEMCTL_PATH status docker*

# Command aliases for file access control operations
Cmnd_Alias FACL_COMMANDS = \\
    $GETFACL_PATH *, \\
    $SETFACL_PATH *

# Command aliases for system commands
Cmnd_Alias SYSTEM_COMMANDS = \\
    $MOUNT_PATH *, \\
    $MOUNT_PATH -l, \\
    $WHICH_PATH *, \\
    $PING_PATH *, \\
    $IP_PATH *, \\
    $SYSTEMCTL_PATH start rodent*, \\
    $SYSTEMCTL_PATH stop rodent*, \\
    $SYSTEMCTL_PATH restart rodent*, \\
    $SYSTEMCTL_PATH enable rodent*, \\
    $SYSTEMCTL_PATH restart systemd-resolved*, \\
    $SYSTEMCTL_PATH status *, \\
    $SYSTEMCTL_PATH is-enabled *, \\
    $SYSTEMCTL_PATH is-system-running *, \\
    $SYSTEMCTL_PATH is-active *, \\
    $SYSTEMCTL_PATH is-failed *, \\
    $SYSTEMCTL_PATH daemon-reload *, \\
    $SYSTEMCTL_PATH list-*$([ "$DEV_MODE" = true ] && echo ", \\
    $SYSTEMCTL_PATH *"), \\
    $NETWORKCTL_PATH *, \\
    $RESOLVECTL_PATH *, \\
    $JOURNALCTL_PATH *, \\
    $HOSTNAMECTL_PATH *, \\
    $NETPLAN_PATH *, \\
    $TIMEDATECTL_PATH *, \\
    $LOCALECTL_PATH *, \\
    $LAST_PATH *, \\
    $HOSTNAME_PATH *, \\
    $UNAME_PATH *, \\
    $UPTIME_PATH *, \\
    $GROUPS_PATH *, \\
    $PASSWD_PATH *, \\
    $OPENSSL_PATH *, \\
    $DMIDECODE_PATH *, \\
    $SYSTEMD_DETECT_VIRT_PATH *, \\
    $REBOOT_PATH *, \\
    $SHUTDOWN_PATH *, \\
    $USERADD_PATH *, \\
    $USERDEL_PATH *, \\
    $USERMOD_PATH *, \\
    $GROUPADD_PATH *, \\
    $GROUPDEL_PATH *, \\
    $GROUPMOD_PATH *, \\
    $CHPASSWD_PATH *

# Command aliases for file operations
Cmnd_Alias FILE_OPERATIONS = \\
    $CAT_PATH /etc/samba/smb.conf*, \\
    $CAT_PATH /etc/samba/conf.d/*, \\
    $CP_PATH * /etc/samba/smb.conf*, \\
    $CP_PATH * /etc/samba/conf.d/*, \\
    $TEE_PATH /etc/samba/smb.conf*, \\
    $TEE_PATH -a /etc/samba/smb.conf*, \\
    $TEE_PATH /etc/samba/conf.d/*, \\
    $TEE_PATH -a /etc/samba/conf.d/*, \\
    $RM_PATH -f /etc/samba/conf.d/*, \\
    $CHMOD_PATH * /etc/samba/smb.conf*, \\
    $CHMOD_PATH * /etc/samba/conf.d/*, \\
    $TEST_PATH -e /etc/samba/smb.conf*, \\
    $TEST_PATH -e /etc/samba/conf.d/*, \\
    $CAT_PATH /etc/hosts, \\
    $CP_PATH * /etc/hosts, \\
    $TEE_PATH /etc/hosts, \\
    $TEE_PATH -a /etc/hosts, \\
    $TEST_PATH -e /etc/hosts, \\
    $CAT_PATH /etc/resolv.conf, \\
    $CP_PATH * /etc/resolv.conf, \\
    $TEE_PATH /etc/resolv.conf, \\
    $TEE_PATH -a /etc/resolv.conf, \\
    $TEST_PATH -e /etc/resolv.conf, \\
    $CAT_PATH /etc/krb5.conf, \\
    $CP_PATH * /etc/krb5.conf, \\
    $TEE_PATH /etc/krb5.conf, \\
    $TEE_PATH -a /etc/krb5.conf, \\
    $TEST_PATH -e /etc/krb5.conf, \\
    $CAT_PATH /etc/systemd/network*, \\
    $CP_PATH * /etc/systemd/network*, \\
    $TEE_PATH /etc/systemd/network*, \\
    $TEE_PATH -a /etc/systemd/network*, \\
    $TEST_PATH -e /etc/systemd/network*, \\
    $CHMOD_PATH * /etc/systemd/network*, \\
    $RM_PATH -f /etc/systemd/network*, \\
    $CAT_PATH /etc/systemd/resolve*, \\
    $CP_PATH * /etc/systemd/resolve*, \\
    $TEE_PATH /etc/systemd/resolve*, \\
    $TEE_PATH -a /etc/systemd/resolve*, \\
    $TEST_PATH -e /etc/systemd/resolve*, \\
    $CHMOD_PATH * /etc/systemd/resolve*, \\
    $RM_PATH -f /etc/systemd/resolve*, \\
    $CAT_PATH /run/systemd/network*, \\
    $CP_PATH * /run/systemd/network*, \\
    $TEE_PATH /run/systemd/network*, \\
    $TEE_PATH -a /run/systemd/network*, \\
    $TEST_PATH -e /run/systemd/network*, \\
    $CHMOD_PATH * /run/systemd/network*, \\
    $RM_PATH -f /run/systemd/network*, \\
    $CAT_PATH /run/systemd/resolve*, \\
    $CP_PATH * /run/systemd/resolve*, \\
    $TEE_PATH /run/systemd/resolve*, \\
    $TEE_PATH -a /run/systemd/resolve*, \\
    $TEST_PATH -e /run/systemd/resolve*, \\
    $CHMOD_PATH * /run/systemd/resolve*, \\
    $RM_PATH -f /run/systemd/resolve*, \\
    $MKDIR_PATH -p /etc/systemd/resolved.conf.d, \\
    $CAT_PATH /etc/netplan/*, \\
    $CP_PATH * /etc/netplan/*, \\
    $TEE_PATH /etc/netplan/*, \\
    $TEE_PATH -a /etc/netplan/*, \\
    $TEST_PATH -e /etc/netplan/*, \\
    $CHMOD_PATH * /etc/netplan/*, \\
    $RM_PATH -f /etc/netplan/*

# Grant permissions to the rodent user
rodent ALL=(ALL) NOPASSWD: ZFS_COMMANDS, SMB_COMMANDS, DOCKER_COMMANDS, FACL_COMMANDS, SYSTEM_COMMANDS, FILE_OPERATIONS

# Defaults specification for security
Defaults:rodent !requiretty
Defaults:rodent secure_path="$SECURE_PATH"
EOF

# Validate sudoers syntax
if visudo -c -f "$TEMP_SUDOERS" >/dev/null 2>&1; then
    cp "$TEMP_SUDOERS" "$SUDOERS_OUTPUT"
    chmod 440 "$SUDOERS_OUTPUT"
    echo "Sudoers file installed successfully"
else
    echo "Error: Generated sudoers file has syntax errors"
    echo "Temporary file: $TEMP_SUDOERS"
    exit 1
fi

# Clean up temporary file
rm -f "$TEMP_SUDOERS"

echo "Setup complete. To start the service, run: systemctl enable --now rodent.service"
echo "To switch to the rodent user, use: sudo -u rodent -i"

if [ "$DEV_MODE" = true ]; then
    echo ""
    echo "Development mode features enabled:"
    echo "  - Debug logging"
    echo "  - Local toggle endpoint"
    echo "  - Broader systemctl permissions"
    echo "  - Development environment settings"
fi