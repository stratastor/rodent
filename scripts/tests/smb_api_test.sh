#!/usr/bin/env bash

# SMB Shares API Test Script
# --------------------------
# Tests the SMB shares API endpoints in the Rodent application
# Uses /tank/newFS as the share path and test-share as the user

set -euo pipefail

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE="\033[0;94m"
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

# Configuration
API_BASE="http://localhost:8042/api/v1/rodent/shares/smb"
TIMESTAMP=$(date +%s)
SHARE_NAME="test-share-${TIMESTAMP}"
SHARE_PATH="/tank/newFS"
USER_NAME="test-share"
HOSTNAME=$(hostname)
STATS_SHARE_NAME="tank-newFS"

# File paths
SMB_CONF="/etc/samba/smb.conf"
BACKUP_PATH="/tmp/smb.conf.backup.$(date +%s)"

# Ensure jq is installed
if ! command -v jq &> /dev/null; then
    echo -e "${RED}${BOLD}Error:${RESET} jq is required but not installed."
    echo "Please install it using: brew install jq"
    exit 1
fi

# Function to backup SMB configuration
backup_smb_config() {
  echo -e "${BOLD}Backing up SMB configuration...${RESET}"
  if [ -f "$SMB_CONF" ]; then
    echo -e "${YELLOW}Creating backup at ${BACKUP_PATH}${RESET}"
    sudo cp "$SMB_CONF" "$BACKUP_PATH"
    if [ $? -eq 0 ]; then
      echo -e "${GREEN}Backup created successfully.${RESET}"
    else
      echo -e "${RED}${BOLD}Error:${RESET} Failed to create backup. Aborting test."
      exit 1
    fi
  else
    echo -e "${YELLOW}No SMB configuration file found at ${SMB_CONF}${RESET}"
    touch "$BACKUP_PATH"  # Create empty backup file for restoring later
  fi
}

# Function to restore SMB configuration
restore_smb_config() {
  echo -e "${BOLD}Restoring SMB configuration...${RESET}"
  if [ -f "$BACKUP_PATH" ]; then
    echo -e "${YELLOW}Restoring from ${BACKUP_PATH} to ${SMB_CONF}${RESET}"
    sudo cp "$BACKUP_PATH" "$SMB_CONF"
    if [ $? -eq 0 ]; then
      echo -e "${GREEN}Restoration completed successfully.${RESET}"
      echo -e "${YELLOW}Removing backup file...${RESET}"
      sudo rm -f "$BACKUP_PATH"
    else
      echo -e "${RED}${BOLD}Error:${RESET} Failed to restore SMB configuration."
      echo -e "${YELLOW}Manual restoration may be required from: ${BACKUP_PATH}${RESET}"
      exit 1
    fi
  else
    echo -e "${RED}${BOLD}Error:${RESET} Backup file not found at ${BACKUP_PATH}"
  fi
}

# Function to handle interruptions (Ctrl+C)
handle_interrupt() {
  echo -e "\n${RED}${BOLD}Test interrupted. Restoring configuration...${RESET}"
  restore_smb_config
  exit 1
}

# Set up interrupt handler
trap handle_interrupt INT

backup_smb_config


# Function to print and execute a command
execute_curl() {
    echo -e "${MAGENTA}$ $1${RESET}"
    eval $1
    echo ""
}

# Function to print JSON data and execute curl with the data
execute_curl_with_data() {
    local command="$1"
    local json_file="$2"
    
    echo -e "${YELLOW}Request payload:${RESET}"
    cat "$json_file" | jq
    echo ""
    
    echo -e "${MAGENTA}$ $command${RESET}"
    eval $command
    echo ""
}

# Function to handle errors
handle_error() {
    if [ $? -ne 0 ]; then
        echo -e "${RED}${BOLD}Error:${RESET} The command failed."
        exit 1
    fi
}

echo -e "${BOLD}${CYAN}===========================================================${RESET}"
echo -e "${BOLD}${CYAN}  SMB Shares API Test Script${RESET}"
echo -e "${BOLD}${CYAN}===========================================================${RESET}\n"

echo -e "${YELLOW}Using timestamp: ${TIMESTAMP}${RESET}"
echo -e "${YELLOW}Share name: ${SHARE_NAME}${RESET}"
echo -e "${YELLOW}Share path: ${SHARE_PATH}${RESET}\n"

echo -e "${BOLD}${BLUE}PART 1: Service Operations${RESET}\n"

# 1. Get SMB service status
echo -e "${BOLD}Getting SMB service status:${RESET}"
execute_curl "curl -s -X GET ${API_BASE}/service/status | jq"
handle_error

# 2. Reload SMB configuration
echo -e "${BOLD}Reloading SMB configuration:${RESET}"
execute_curl "curl -s -X POST ${API_BASE}/service/reload | jq"
handle_error

echo -e "${BOLD}${BLUE}PART 2: Global SMB Configuration${RESET}\n"

# 3. Get global SMB configuration
echo -e "${BOLD}Getting global SMB configuration:${RESET}"
execute_curl "curl -s -X GET ${API_BASE}/global | jq"
handle_error

# 4. Update global SMB configuration
echo -e "${BOLD}Updating global SMB configuration:${RESET}"
GLOBAL_CONFIG=$(cat <<EOF
{                                                                                                                                 
  "workgroup": "AD",                                             
  "server_string": "Rodent TEST SMB Server",                                                                                           
  "security_mode": "ADS",                                        
  "realm": "AD.STRATA.INTERNAL",                                                                                                  
  "server_role": "member server",                                                                                                 
  "log_level": "1",                                              
  "max_log_size": 1000,                                          
  "winbind_use_default_domain": true,                            
  "winbind_offline_logon": true,                                 
  "idmap_config": {                                              
    "idmap config *:backend": "tdb",                             
    "idmap config *:range": "100000-199999",                                                                                      
    "idmap config AD:backend": "rid",                            
    "idmap config AD:range": "200000-999999"                                                                                      
  },                                                             
  "kerberos_method": "secrets and keytab",                                                                                        
  "custom_parameters": {                                         
    "dedicated keytab file": "/etc/krb5.keytab",                                                                                  
    "dns proxy": "no",                                                                                                            
    "map to guest": "Bad User",
    "unix charset": "UTF-8",
    "winbind enum groups": "yes",                                
    "winbind enum users": "yes",
    "winbind nested groups": "yes",                              
    "winbind refresh tickets": "yes"                             
  }                             
}
EOF
)
echo "$GLOBAL_CONFIG" > global_config.json
execute_curl_with_data "curl -s -X PUT ${API_BASE}/global -H 'Content-Type: application/json' -d @global_config.json | jq" "global_config.json"
handle_error

# 5. Verify global configuration update
echo -e "${BOLD}Verifying global configuration update:${RESET}"
execute_curl "curl -s -X GET ${API_BASE}/global | jq"
handle_error

echo -e "${BOLD}${BLUE}PART 3: Share Operations${RESET}\n"

# 6. List existing SMB shares
echo -e "${BOLD}Listing existing SMB shares:${RESET}"
execute_curl "curl -s -X GET ${API_BASE} | jq"
handle_error

# 7. Create a new SMB share
echo -e "${BOLD}Creating a new SMB share: ${SHARE_NAME}${RESET}"
SHARE_CONFIG=$(cat <<EOF
{
  "name": "${SHARE_NAME}",
  "path": "${SHARE_PATH}",
  "description": "Test share created by API test script",
  "enabled": true,
  "readOnly": false,
  "browsable": true,
  "guestOk": false,
  "validUsers": ["AD\\${USER_NAME}"],
  "inheritACLs": true,
  "mapACLInherit": true,
  "tags": {
    "purpose": "testing",
    "created_by": "api_test_script"
  },
  "customParameters": {
    "create mask": "0644",
    "directory mask": "0755",
    "vfs objects": "acl_xattr",
    "map archive": "no"
  }
}
EOF
)
echo "$SHARE_CONFIG" > share_config.json
execute_curl_with_data "curl -s -X POST ${API_BASE} -H 'Content-Type: application/json' -d @share_config.json | jq" "share_config.json"
handle_error

# 8. Verify share creation by listing shares
echo -e "${BOLD}Verifying share creation:${RESET}"
execute_curl "curl -s -X GET ${API_BASE} | jq"
handle_error

# 9. Get specific share details
echo -e "${BOLD}Getting details for share: ${SHARE_NAME}${RESET}"
execute_curl "curl -s -X GET ${API_BASE}/${SHARE_NAME} | jq"
handle_error

# 10. Get share statistics
echo -e "${BOLD}Getting share statistics: ${STATS_SHARE_NAME}${RESET}"
execute_curl "curl -s -X GET ${API_BASE}/${STATS_SHARE_NAME}/stats | jq"
handle_error

# 11. Get detailed share statistics
echo -e "${BOLD}Getting detailed share statistics: ${STATS_SHARE_NAME}${RESET}"
execute_curl "curl -s -X GET ${API_BASE}/${STATS_SHARE_NAME}/stats?detailed=true | jq"
handle_error

# 12. Update the share configuration
echo -e "${BOLD}Updating share: ${SHARE_NAME}${RESET}"
SHARE_UPDATE=$(cat <<EOF
{
  "name": "${SHARE_NAME}",
  "path": "${SHARE_PATH}",
  "description": "Updated test share description",
  "enabled": true,
  "readOnly": true,
  "browsable": true,
  "guestOk": false,
  "validUsers": ["AD\\${USER_NAME}","@AD\\ts-group"],
  "inheritACLs": true,
  "mapACLInherit": true,
  "tags": {
    "purpose": "testing",
    "created_by": "api_test_script",
    "updated": "true"
  },
  "customParameters": {
    "create mask": "0644",
    "directory mask": "0755",
    "vfs objects": "acl_xattr",
    "map archive": "no",
    "strict locking": "no"
  }
}
EOF
)
echo "$SHARE_UPDATE" > share_update.json
execute_curl_with_data "curl -s -X PUT ${API_BASE}/${SHARE_NAME} -H 'Content-Type: application/json' -d @share_update.json | jq" "share_update.json"
handle_error

# 13. Verify share update
echo -e "${BOLD}Verifying share update:${RESET}"
execute_curl "curl -s -X GET ${API_BASE}/${SHARE_NAME} | jq"
handle_error

echo -e "${BOLD}${BLUE}PART 4: Bulk Operations${RESET}\n"

# 14. Create a couple more test shares for bulk operations
for i in {1..2}; do
    NEW_SHARE="${SHARE_NAME}-bulk-${i}"
    echo -e "${BOLD}Creating additional test share: ${NEW_SHARE}${RESET}"
    BULK_SHARE=$(cat <<EOF
{
  "name": "${NEW_SHARE}",
  "path": "${SHARE_PATH}",
  "description": "Test share ${i} for bulk operations",
  "enabled": true,
  "readOnly": false,
  "browsable": true,
  "tags": {
    "purpose": "bulk-testing",
    "index": "${i}"
  }
}
EOF
    )
    echo "$BULK_SHARE" > bulk_share_${i}.json
    execute_curl_with_data "curl -s -X POST ${API_BASE} -H 'Content-Type: application/json' -d @bulk_share_${i}.json | jq" "bulk_share_${i}.json"
    handle_error
done

# 15. Perform a bulk update operation by share name
echo -e "${BOLD}Performing bulk update by share name:${RESET}"
BULK_UPDATE=$(cat <<EOF
{
  "shareNames": ["${SHARE_NAME}-bulk-1", "${SHARE_NAME}-bulk-2"],
  "parameters": {
    "hide dot files": "yes",
    "case sensitive": "no"
  }
}
EOF
)
echo "$BULK_UPDATE" > bulk_update.json
execute_curl_with_data "curl -s -X PUT ${API_BASE}/bulk-update -H 'Content-Type: application/json' -d @bulk_update.json | jq" "bulk_update.json"
handle_error

# 16. Perform a bulk update operation by tag
echo -e "${BOLD}Performing bulk update by tag:${RESET}"
BULK_UPDATE_TAG=$(cat <<EOF
{
  "tags": {
    "purpose": "bulk-testing",
    "index": "1"
  },
  "parameters": {
    "ea support": "yes",
    "level2 oplocks": "no"
  }
}
EOF
)
echo "$BULK_UPDATE_TAG" > bulk_update_tag.json
execute_curl_with_data "curl -s -X PUT ${API_BASE}/bulk-update -H 'Content-Type: application/json' -d @bulk_update_tag.json | jq" "bulk_update_tag.json"
handle_error

# 17. Perform a bulk update operation for all shares
echo -e "${BOLD}Performing bulk update for all shares:${RESET}"
BULK_UPDATE_ALL=$(cat <<EOF
{
  "all": true,
  "parameters": {
    "follow symlinks": "yes",
    "wide links": "no"
  }
}
EOF
)
echo "$BULK_UPDATE_ALL" > bulk_update_all.json
execute_curl_with_data "curl -s -X PUT ${API_BASE}/bulk-update -H 'Content-Type: application/json' -d @bulk_update_all.json | jq" "bulk_update_all.json"
handle_error

echo -e "${BOLD}${BLUE}PART 5: Cleanup${RESET}\n"

# 18. Delete all test shares
for share in "${SHARE_NAME}" "${SHARE_NAME}-bulk-1" "${SHARE_NAME}-bulk-2"; do
    echo -e "${BOLD}Deleting share: ${share}${RESET}"
    execute_curl "curl -s -X DELETE ${API_BASE}/${share}"
    handle_error
done

# 19. Verify shares have been deleted
echo -e "${BOLD}Verifying shares have been deleted:${RESET}"
execute_curl "curl -s -X GET ${API_BASE} | jq"
handle_error

# Cleanup temporary files
echo -e "${BOLD}Cleaning up temporary files${RESET}"
rm -f global_config.json share_config.json share_update.json bulk_share_*.json bulk_update*.json

echo -e "${GREEN}${BOLD}===========================================================${RESET}"
echo -e "${GREEN}${BOLD}  SMB Shares API Test Script Completed Successfully${RESET}"
echo -e "${GREEN}${BOLD}===========================================================${RESET}"

restore_smb_config