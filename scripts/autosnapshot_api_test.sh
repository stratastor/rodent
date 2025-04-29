#!/usr/bin/env bash

# Auto-Snapshot API Test Script
# -----------------------------
# Tests the ZFS auto-snapshot API endpoints in the Rodent application
# Using different datasets to demonstrate various policy configurations

set -euo pipefail

# Color definitions (pastel versions)
RED='\033[38;5;217m'     # Pastel red/pink
GREEN='\033[38;5;158m'   # Pastel green
YELLOW='\033[38;5;222m'  # Pastel yellow
BLUE='\033[38;5;153m'    # Pastel blue
MAGENTA='\033[38;5;183m' # Pastel purple
CYAN='\033[38;5;159m'    # Pastel cyan
BOLD='\033[1m'           # Bold 
RESET='\033[0m'          # Reset 

# Configuration
BASE_URL="http://localhost:8042/api/v1/rodent/zfs/schedulers/autosnapshot"
TIMESTAMP=$(date +%s)

# Variables for dataset names (customize these for your environment)
DATASET_ROOT="tank"
DATASET_DATA="${DATASET_ROOT}/copyFS"
DATASET_BACKUPS="${DATASET_ROOT}/newFS"
DATASET_DEV="${DATASET_ROOT}/child"

# For storing policy IDs
HOURLY_POLICY_ID=""
COMPLEX_POLICY_ID=""
BALLMER_POLICY_ID=""

# Ensure jq is installed
if ! command -v jq &> /dev/null; then
    echo -e "${RED}${BOLD}Error:${RESET} jq is required but not installed."
    echo "Please install it using: brew install jq (macOS) or apt install jq (Linux)"
    exit 1
fi

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
echo -e "${BOLD}${CYAN}  ZFS AUTO-SNAPSHOT API TEST SCRIPT                        ${RESET}"
echo -e "${BOLD}${CYAN}  'CAP THEOREM EDITION: PICK TWO, OR MAYBE THREE'          ${RESET}"
echo -e "${BOLD}${CYAN}===========================================================${RESET}\n"

echo -e "${YELLOW}API SERVER: ${BASE_URL}${RESET}"
echo -e "${YELLOW}TIMESTAMP: ${TIMESTAMP}${RESET}"
echo -e "${YELLOW}DATASETS:${RESET}"
echo -e "${YELLOW}  - Data: ${DATASET_DATA}${RESET}"
echo -e "${YELLOW}  - Backups: ${DATASET_BACKUPS}${RESET}" 
echo -e "${YELLOW}  - Dev: ${DATASET_DEV}${RESET}\n"

# ---------------------------------------------------------------------------------
# PART 1: Create a basic hourly snapshot policy
# ---------------------------------------------------------------------------------

echo -e "${BOLD}${BLUE}PART 1: Creating 'BobbyTables' hourly snapshot policy${RESET}"
echo -e "${BOLD}Little Bobby Tables, we call him (xkcd #327)${RESET}\n"

HOURLY_POLICY=$(cat <<EOF
{
  "name": "bobby-tables-hourly-${TIMESTAMP}",
  "description": "Little Bobby Tables' hourly snapshots; we call him",
  "dataset": "${DATASET_DATA}",
  "schedules": [
    {
      "type": "hourly",
      "interval": 1,
      "enabled": true
    }
  ],
  "recursive": true,
  "snap_name_pattern": "bobby-%Y-%m-%d-%H%M",
  "retention_policy": {
    "count": 24,
    "force_destroy": true,
    "keep_named_snap": ["sanitized-input-snapshot"]
  },
  "properties": {
    "com.example:policy": "hourly",
    "com.example:creator": "autosnapshot-api",
    "com.example:xkcd": "327"
  },
  "enabled": true
}
EOF
)

# Ensure JSON is properly formatted
echo "$HOURLY_POLICY" | jq '.' > hourly_policy.json
execute_curl_with_data "curl -s -X POST ${BASE_URL}/policies -H 'Content-Type: application/json' -d @hourly_policy.json | jq" "hourly_policy.json"
handle_error

# Get the policy ID for later use
HOURLY_POLICY_NAME="bobby-tables-hourly-${TIMESTAMP}"
echo -e "${BOLD}Getting ID for policy: ${HOURLY_POLICY_NAME}${RESET}"
execute_curl "curl -s -X GET ${BASE_URL}/policies | jq '.policies[] | select(.name == \"${HOURLY_POLICY_NAME}\") | .id'"
HOURLY_POLICY_ID=$(curl -s -X GET ${BASE_URL}/policies | jq -r ".policies[] | select(.name == \"${HOURLY_POLICY_NAME}\") | .id")

if [ -z "$HOURLY_POLICY_ID" ]; then
    echo -e "${RED}${BOLD}Error:${RESET} Could not retrieve ID for hourly policy."
    echo -e "${YELLOW}Continuing with other tests...${RESET}\n"
fi

# ---------------------------------------------------------------------------------
# PART 2: Create a complex policy with multiple schedule types
# ---------------------------------------------------------------------------------

echo -e "${BOLD}${BLUE}PART 2: Creating 'Hard Problems in Computer Science' policy${RESET}"
echo -e "${BOLD}Cache invalidation, naming things, and off-by-one errors${RESET}\n"

COMPLEX_POLICY=$(cat <<EOF
{
  "name": "hard-problems-backup-${TIMESTAMP}",
  "description": "The hard problems in Computer Science: cache invalidation and naming things",
  "dataset": "${DATASET_BACKUPS}",
  "schedules": [
    {
      "type": "daily",
      "interval": 1,
      "at_time": "23:00",
      "enabled": true
    },
    {
      "type": "weekly",
      "interval": 1,
      "week_day": 0,
      "at_time": "22:00",
      "enabled": true
    },
    {
      "type": "monthly",
      "interval": 1,
      "day_of_month": 1,
      "at_time": "02:00",
      "enabled": true
    }
  ],
  "recursive": true,
  "snap_name_pattern": "cache-inval-%Y-%m-%d",
  "retention_policy": {
    "older_than": 2592000000000000,
    "count": 10,
    "force_destroy": true
  },
  "properties": {
    "com.example:policy": "complex-backup",
    "com.example:meme": "2-hard-problems"
  },
  "enabled": true
}
EOF
)

# Ensure JSON is properly formatted
echo "$COMPLEX_POLICY" | jq '.' > complex_policy.json
execute_curl_with_data "curl -s -X POST ${BASE_URL}/policies -H 'Content-Type: application/json' -d @complex_policy.json | jq" "complex_policy.json"
handle_error

# Get the policy ID for later use
COMPLEX_POLICY_NAME="hard-problems-backup-${TIMESTAMP}"
echo -e "${BOLD}Getting ID for policy: ${COMPLEX_POLICY_NAME}${RESET}"
execute_curl "curl -s -X GET ${BASE_URL}/policies | jq '.policies[] | select(.name == \"${COMPLEX_POLICY_NAME}\") | .id'"
COMPLEX_POLICY_ID=$(curl -s -X GET ${BASE_URL}/policies | jq -r ".policies[] | select(.name == \"${COMPLEX_POLICY_NAME}\") | .id")

if [ -z "$COMPLEX_POLICY_ID" ]; then
    echo -e "${RED}${BOLD}Error:${RESET} Could not retrieve ID for complex policy."
    echo -e "${YELLOW}Continuing with other tests...${RESET}\n"
fi

# ---------------------------------------------------------------------------------
# PART 3: Create a policy with advanced scheduling
# ---------------------------------------------------------------------------------

echo -e "${BOLD}${BLUE}PART 3: Creating 'The Ballmer Peak' policy${RESET}"
echo -e "${BOLD}Blood alcohol content between 0.129% and 0.138% for optimal programming (xkcd #323)${RESET}\n"

BALLMER_POLICY=$(cat <<EOF
{
  "name": "ballmer-peak-${TIMESTAMP}",
  "description": "The mythical BAC sweet spot for programming: 0.129%-0.138%",
  "dataset": "${DATASET_DEV}",
  "schedules": [
    {
      "type": "random",
      "min_duration": 1800000000000,
      "max_duration": 7200000000000,
      "enabled": true
    },
    {
      "type": "onetime",
      "start_time": "2025-12-31T23:59:59Z",
      "enabled": true
    }
  ],
  "recursive": true,
  "snap_name_pattern": "ballmer-%Y-%m-%d-%H%M",
  "retention_policy": {
    "count": 5,
    "force_destroy": true
  },
  "properties": {
    "com.example:policy": "ballmer-peak",
    "com.example:xkcd": "323",
    "com.example:optimal-bac": "0.1335%"
  },
  "enabled": true
}
EOF
)

# Ensure JSON is properly formatted
echo "$BALLMER_POLICY" | jq '.' > ballmer_policy.json
execute_curl_with_data "curl -s -X POST ${BASE_URL}/policies -H 'Content-Type: application/json' -d @ballmer_policy.json | jq" "ballmer_policy.json"
handle_error

# Get the policy ID for later use
BALLMER_POLICY_NAME="ballmer-peak-${TIMESTAMP}"
echo -e "${BOLD}Getting ID for policy: ${BALLMER_POLICY_NAME}${RESET}"
execute_curl "curl -s -X GET ${BASE_URL}/policies | jq '.policies[] | select(.name == \"${BALLMER_POLICY_NAME}\") | .id'"
BALLMER_POLICY_ID=$(curl -s -X GET ${BASE_URL}/policies | jq -r ".policies[] | select(.name == \"${BALLMER_POLICY_NAME}\") | .id")

if [ -z "$BALLMER_POLICY_ID" ]; then
    echo -e "${RED}${BOLD}Error:${RESET} Could not retrieve ID for ballmer policy."
    echo -e "${YELLOW}Continuing with other tests...${RESET}\n"
fi

# ---------------------------------------------------------------------------------
# PART 4: List all policies
# ---------------------------------------------------------------------------------

echo -e "${BOLD}${BLUE}PART 4: Listing all policies${RESET}"
echo -e "${BOLD}In Case of Fire: 1) git commit, 2) git push, 3) leave building${RESET}\n"

execute_curl "curl -s -X GET ${BASE_URL}/policies | jq '.'"
handle_error

# ---------------------------------------------------------------------------------
# PART 5: Get a specific policy by ID
# ---------------------------------------------------------------------------------

if [ -n "$HOURLY_POLICY_ID" ]; then
    echo -e "${BOLD}${BLUE}PART 5: Getting Bobby Tables policy details${RESET}\n"
    execute_curl "curl -s -X GET ${BASE_URL}/policies/$HOURLY_POLICY_ID | jq"
    handle_error
else
    echo -e "${YELLOW}${BOLD}Skipping part 5: No hourly policy ID available${RESET}\n"
fi

# ---------------------------------------------------------------------------------
# PART 6: Update an existing policy
# ---------------------------------------------------------------------------------

if [ -n "$HOURLY_POLICY_ID" ]; then
    echo -e "${BOLD}${BLUE}PART 6: Updating Bobby Tables policy${RESET}"
    echo -e "${BOLD}Have you tried turning it off and on again?${RESET}\n"
    
    UPDATED_POLICY=$(cat <<EOF
{
  "name": "sanitized-bobby-tables-${TIMESTAMP}",
  "description": "Updated: We now sanitize inputs to prevent SQL injection",
  "dataset": "${DATASET_DATA}",
  "schedules": [
    {
      "type": "hourly",
      "interval": 2,
      "enabled": true
    }
  ],
  "recursive": true,
  "snap_name_pattern": "sanitized-%Y-%m-%d-%H%M",
  "retention_policy": {
    "count": 12,
    "force_destroy": true
  },
  "properties": {
    "com.example:policy": "sanitized-inputs",
    "com.example:creator": "autosnapshot-api",
    "com.example:xkcd": "327-fixed"
  },
  "enabled": true
}
EOF
)
    
    # Ensure JSON is properly formatted
echo "$UPDATED_POLICY" | jq '.' > updated_policy.json
    execute_curl_with_data "curl -s -X PUT ${BASE_URL}/policies/$HOURLY_POLICY_ID -H 'Content-Type: application/json' -d @updated_policy.json | jq" "updated_policy.json"
    handle_error
else
    echo -e "${YELLOW}${BOLD}Skipping part 6: No hourly policy ID available${RESET}\n"
fi

# ---------------------------------------------------------------------------------
# PART 7: Run a policy immediately
# ---------------------------------------------------------------------------------

if [ -n "$BALLMER_POLICY_ID" ]; then
    echo -e "${BOLD}${BLUE}PART 7: Running Ballmer Peak policy immediately${RESET}"
    echo -e "${BOLD}It works on my machine!${RESET}\n"
    
    RUN_POLICY_PARAMS=$(cat <<EOF
{
  "schedule_index": 0,
  "dry_run": false
}
EOF
)
    
    # Ensure JSON is properly formatted
echo "$RUN_POLICY_PARAMS" | jq '.' > run_policy.json
    execute_curl_with_data "curl -s -X POST ${BASE_URL}/policies/$BALLMER_POLICY_ID/run -H 'Content-Type: application/json' -d @run_policy.json | jq" "run_policy.json"
    handle_error
else
    echo -e "${YELLOW}${BOLD}Skipping part 7: No ballmer policy ID available${RESET}\n"
fi

# ---------------------------------------------------------------------------------
# PART 8: Delete a policy without removing snapshots
# ---------------------------------------------------------------------------------

if [ -n "$COMPLEX_POLICY_ID" ]; then
    echo -e "${BOLD}${BLUE}PART 8: Deleting Hard Problems policy without removing snapshots${RESET}"
    echo -e "${BOLD}The 3rd hard problem: deleting without cascading${RESET}\n"
    
    execute_curl "curl -s -X DELETE ${BASE_URL}/policies/$COMPLEX_POLICY_ID | jq"
    handle_error
else
    echo -e "${YELLOW}${BOLD}Skipping part 8: No complex policy ID available${RESET}\n"
fi

# ---------------------------------------------------------------------------------
# PART 9: Delete a policy and remove all associated snapshots
# ---------------------------------------------------------------------------------

if [ -n "$BALLMER_POLICY_ID" ]; then
    echo -e "${BOLD}${BLUE}PART 9: Deleting Ballmer Peak policy WITH all snapshots${RESET}"
    echo -e "${BOLD}rm -rf /*: The nuclear option${RESET}\n"
    
    execute_curl "curl -s -X DELETE ${BASE_URL}/policies/$BALLMER_POLICY_ID?remove_snapshots=true | jq"
    handle_error
else
    echo -e "${YELLOW}${BOLD}Skipping part 9: No ballmer policy ID available${RESET}\n"
fi

# ---------------------------------------------------------------------------------
# REFERENCE: Schedule Types for Auto-Snapshots
# ---------------------------------------------------------------------------------

echo -e "${BOLD}${BLUE}REFERENCE: Schedule Types for Auto-Snapshots${RESET}"
echo -e "${BOLD}${BLUE}------------------------------------------------${RESET}\n"

echo -e "${MAGENTA}The 8 Fallacies of Distributed Computing:${RESET}"
echo -e "${MAGENTA}1. The network is reliable${RESET}"
echo -e "${MAGENTA}2. Latency is zero${RESET}"
echo -e "${MAGENTA}3. Bandwidth is infinite${RESET}"
echo -e "${MAGENTA}4. The network is secure${RESET}"
echo -e "${MAGENTA}5. Topology doesn't change${RESET}"
echo -e "${MAGENTA}6. There is one administrator${RESET}"
echo -e "${MAGENTA}7. Transport cost is zero${RESET}"
echo -e "${MAGENTA}8. The network is homogeneous${RESET}\n"

SCHEDULE_EXAMPLES=$(cat <<EOF
{
  "Secondly Schedule": {
    "type": "secondly",
    "interval": 30,
    "enabled": true
  },
  "Minutely Schedule": {
    "type": "minutely",
    "interval": 5,
    "enabled": true
  },
  "Hourly Schedule": {
    "type": "hourly",
    "interval": 2,
    "enabled": true
  },
  "Daily Schedule": {
    "type": "daily",
    "interval": 1,
    "at_time": "03:30",
    "enabled": true
  },
  "Weekly Schedule": {
    "type": "weekly",
    "interval": 1,
    "week_day": 1,
    "at_time": "02:00",
    "enabled": true
  },
  "Monthly Schedule": {
    "type": "monthly",
    "interval": 1,
    "day_of_month": 15,
    "at_time": "01:00",
    "enabled": true
  },
  "Yearly Schedule": {
    "type": "yearly",
    "day_of_month": 1,
    "month": 1,
    "at_time": "00:00",
    "enabled": true
  },
  "One-time Schedule": {
    "type": "onetime",
    "start_time": "2025-12-31T23:59:59Z",
    "enabled": true
  },
  "Duration Schedule": {
    "type": "duration",
    "duration": 3600000000000,
    "enabled": true
  },
  "Random Schedule": {
    "type": "random",
    "min_duration": 1800000000000,
    "max_duration": 7200000000000,
    "enabled": true
  }
}
EOF
)

# Ensure JSON is properly formatted
echo "$SCHEDULE_EXAMPLES" | jq '.' > schedule_examples.json
echo -e "${YELLOW}${BOLD}Available Schedule Types:${RESET}"
cat schedule_examples.json | jq

# Cleanup temporary files
echo -e "${BOLD}Cleaning up temporary files${RESET}"
rm -f hourly_policy.json complex_policy.json ballmer_policy.json updated_policy.json run_policy.json schedule_examples.json

echo -e "${GREEN}${BOLD}===========================================================${RESET}"
echo -e "${GREEN}${BOLD}  Auto-Snapshot API Test Script Completed Successfully      ${RESET}"
echo -e "${GREEN}${BOLD}===========================================================${RESET}"
echo -e "${CYAN}${BOLD}Remember: There are 10 types of people in the world...${RESET}"
echo -e "${CYAN}${BOLD}Those who understand binary, and those who don't.${RESET}"