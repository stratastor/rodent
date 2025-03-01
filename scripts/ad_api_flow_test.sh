#!/usr/bin/env bash

# Don't Panic: Active Directory API Testing
# ----------------------------------------------
# This is The Hitchhiker's Guide to our AD Galaxy

set -euo pipefail

# Error handling trap
trap 'echo -e "${RED}${BOLD}Error detected. Stopping execution.${RESET}"; exit 1' ERR

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
API_BASE="http://localhost:8042/api/v1/rodent/ad"
TIMESTAMP=$(date +%s)
DOMAIN_BASE="DC=ad,DC=strata,DC=internal"

# Ensure jq is installed
if ! command -v jq &> /dev/null; then
    echo -e "${RED}${BOLD}Error:${RESET} jq is required but not installed."
    echo "Please install it using: sudo apt install jq"
    exit 1
fi

echo -e "${BOLD}${CYAN}===========================================================${RESET}"
echo -e "${BOLD}${CYAN}  Don't Panic: The Ultimate Guide to AD API Testing${RESET}"
echo -e "${BOLD}${CYAN}  \"Time is an illusion. API testing time doubly so.\"${RESET}"
echo -e "${BOLD}${CYAN}===========================================================${RESET}\n"

echo -e "${YELLOW}Initializing test variables with a timestamp of $TIMESTAMP...${RESET}"
echo -e "${YELLOW}The Answer to the Ultimate Question of Life, the Universe, and Everything is not 42, it's successful API tests.${RESET}"
echo ""

# Generate unique names for our test objects
USER_NAME="arthur_dent_$TIMESTAMP"
GROUP_NAME="hitchhiker_$TIMESTAMP"
COMPUTER_NAME="heart_of_gold_$TIMESTAMP"

# Function to print the command and then execute it
execute_curl() {
    echo -e "${MAGENTA}$ $1${RESET}"
    eval $1
    echo ""
}

# Function to handle errors
handle_error() {
    if [ $? -ne 0 ]; then
        echo -e "${RED}${BOLD}Error:${RESET} The command failed. So long, and thanks for all the fish!"
        exit 1
    fi
}

echo -e "${BOLD}${BLUE}PART 1: User Operations${RESET}"
echo -e "${BLUE}\"In the beginning the Universe was created. This has made a lot of people very angry and been widely regarded as a bad move.\"${RESET}\n"

# 1. Create a User
echo -e "${BOLD}Creating a new user: $USER_NAME ${RESET}"
USER_JSON=$(cat <<EOF
{
  "cn": "$USER_NAME",
  "sam_account_name": "$USER_NAME",
  "user_principal_name": "$USER_NAME@ad.strata.internal",
  "given_name": "Arthur",
  "surname": "Dent",
  "description": "Earthman, survivor of Earth's demolition",
  "password": "DontPanic42!",
  "mail": "arthur@heartofgold.space",
  "display_name": "Arthur Dent",
  "title": "Galactic Hitchhiker",
  "department": "Earth Expatriates",
  "company": "Guide Research",
  "phone_number": "555-1234",
  "mobile": "555-4242",
  "employee_id": "HHGTTG-001",
  "enabled": true
}
EOF
)
echo "$USER_JSON" > user.json
execute_curl "curl -s -X POST $API_BASE/users -H 'Content-Type: application/json' -d @user.json | jq"
handle_error

# 2. List all users
echo -e "${BOLD}Listing all users:${RESET}"
execute_curl "curl -s -X GET $API_BASE/users | jq"
handle_error

# 3. Get specific user
echo -e "${BOLD}Getting details for user: $USER_NAME${RESET}"
execute_curl "curl -s -X GET $API_BASE/users/$USER_NAME | jq"
handle_error

# Store the user's DN for later use
USER_DN=$(curl -s -X GET $API_BASE/users/$USER_NAME | jq -r '.DN')
echo -e "${YELLOW}Stored user DN: $USER_DN for later use${RESET}\n"

# 4. Update user
echo -e "${BOLD}Updating user: $USER_NAME${RESET}"
USER_UPDATE_JSON=$(cat <<EOF
{
  "cn": "$USER_NAME",
  "sam_account_name": "$USER_NAME",
  "description": "Last human from Earth, tea enthusiast",
  "title": "Professional Hitchhiker"
}
EOF
)
echo "$USER_UPDATE_JSON" > user_update.json
execute_curl "curl -s -X PUT $API_BASE/users/$USER_NAME -H 'Content-Type: application/json' -d @user_update.json | jq"
handle_error

# 5. Verify the update
echo -e "${BOLD}Verifying user update:${RESET}"
execute_curl "curl -s -X GET $API_BASE/users/$USER_NAME | jq"
handle_error

echo -e "${BOLD}${BLUE}PART 2: Group Operations${RESET}"
echo -e "${BLUE}\"A common mistake that people make when trying to design something completely foolproof is to underestimate the ingenuity of complete fools.\"${RESET}\n"

# 1. Create a Group
echo -e "${BOLD}Creating a new group: $GROUP_NAME${RESET}"
GROUP_JSON=$(cat <<EOF
{
  "cn": "$GROUP_NAME",
  "sam_account_name": "$GROUP_NAME",
  "description": "Don't Panic!",
  "display_name": "Hitchhikers Guide to the Galaxy",
  "mail": "guide@galaxy.org",
  "group_type": 4,
  "scope": "Global",
  "managed": true
}
EOF
)
echo "$GROUP_JSON" > group.json
execute_curl "curl -s -X POST $API_BASE/groups -H 'Content-Type: application/json' -d @group.json | jq"
handle_error

# 2. List all groups
echo -e "${BOLD}Listing all groups:${RESET}"
execute_curl "curl -s -X GET $API_BASE/groups | jq"
handle_error

# 3. Get specific group
echo -e "${BOLD}Getting details for group: $GROUP_NAME${RESET}"
execute_curl "curl -s -X GET $API_BASE/groups/$GROUP_NAME | jq"
handle_error

# Store the group's DN for later use
GROUP_DN=$(curl -s -X GET $API_BASE/groups/$GROUP_NAME | jq -r '.DN')
echo -e "${YELLOW}Stored group DN: $GROUP_DN for later use${RESET}\n"

# 4. Update group
echo -e "${BOLD}Updating group: $GROUP_NAME${RESET}"
GROUP_UPDATE_JSON=$(cat <<EOF
{
  "cn": "$GROUP_NAME",
  "sam_account_name": "$GROUP_NAME",
  "description": "The most remarkable book ever to come out of the great publishing corporations of Ursa Minor",
  "display_name": "The Guide"
}
EOF
)
echo "$GROUP_UPDATE_JSON" > group_update.json
execute_curl "curl -s -X PUT $API_BASE/groups/$GROUP_NAME -H 'Content-Type: application/json' -d @group_update.json | jq"
handle_error

# 5. Add member to group
echo -e "${BOLD}Adding user $USER_NAME to group $GROUP_NAME:${RESET}"
ADD_MEMBER_JSON=$(cat <<EOF
{
  "members": ["$USER_DN"]
}
EOF
)
echo "$ADD_MEMBER_JSON" > add_member.json
execute_curl "curl -s -X POST $API_BASE/groups/$GROUP_NAME/members -H 'Content-Type: application/json' -d @add_member.json | jq"
handle_error

# 6. List group members
echo -e "${BOLD}Listing members of group: $GROUP_NAME${RESET}"
execute_curl "curl -s -X GET $API_BASE/groups/$GROUP_NAME/members | jq"
handle_error

# 7. List user's groups
echo -e "${BOLD}Listing groups for user: $USER_NAME${RESET}"
execute_curl "curl -s -X GET $API_BASE/users/$USER_NAME/groups | jq"
handle_error

echo -e "${BOLD}${BLUE}PART 3: Computer Operations${RESET}"
echo -e "${BLUE}\"It's not the fall that kills you; it's the sudden stop at the end.\"${RESET}\n"

# 1. Create a Computer
echo -e "${BOLD}Creating a new computer: $COMPUTER_NAME${RESET}"
COMPUTER_JSON=$(cat <<EOF
{
  "cn": "$COMPUTER_NAME",
  "sam_account_name": "${COMPUTER_NAME}\$",
  "description": "Infinite Improbability Drive Enabled",
  "dns_hostname": "$COMPUTER_NAME.ad.strata.internal",
  "os_name": "Sirius Cybernetics OS",
  "os_version": "42.0.1",
  "service_pack": "Genuine People Personality",
  "location": "Somewhere in the vicinity of Betelgeuse",
  "managed_by": "$USER_DN"
}
EOF
)
echo "$COMPUTER_JSON" > computer.json
execute_curl "curl -s -X POST $API_BASE/computers -H 'Content-Type: application/json' -d @computer.json | jq"
handle_error

# 2. Get computer
echo -e "${BOLD}Getting details for computer: ${COMPUTER_NAME}\$${RESET}"
execute_curl "curl -s -X GET $API_BASE/computers/${COMPUTER_NAME}\$ | jq"
handle_error

# 3. Update computer
echo -e "${BOLD}Updating computer: ${COMPUTER_NAME}\$${RESET}"
COMPUTER_UPDATE_JSON=$(cat <<EOF
{
  "cn": "$COMPUTER_NAME",
  "sam_account_name": "${COMPUTER_NAME}\$",
  "description": "Now with Genuine People Personalities technology",
  "os_version": "42.0.2"
}
EOF
)
echo "$COMPUTER_UPDATE_JSON" > computer_update.json
execute_curl "curl -s -X PUT $API_BASE/computers/${COMPUTER_NAME}\$ -H 'Content-Type: application/json' -d @computer_update.json | jq"
handle_error

# 4. Verify update
echo -e "${BOLD}Verifying computer update:${RESET}"
execute_curl "curl -s -X GET $API_BASE/computers/${COMPUTER_NAME}\$ | jq"
handle_error

echo -e "${BOLD}${BLUE}PART 4: Cleanup Operations${RESET}"
echo -e "${BLUE}\"So long, and thanks for all the fish!\"${RESET}\n"

# 1. Remove user from group
echo -e "${BOLD}Removing user $USER_NAME from group $GROUP_NAME:${RESET}"
REMOVE_MEMBER_JSON=$(cat <<EOF
{
  "members": ["$USER_DN"]
}
EOF
)
echo "$REMOVE_MEMBER_JSON" > remove_member.json
execute_curl "curl -s -X DELETE $API_BASE/groups/$GROUP_NAME/members -H 'Content-Type: application/json' -d @remove_member.json | jq"
handle_error

# 2. Delete Computer
echo -e "${BOLD}Deleting computer: ${COMPUTER_NAME}${RESET}"
execute_curl "curl -s -X DELETE $API_BASE/computers/${COMPUTER_NAME} | jq"
handle_error

# 3. Delete Group
echo -e "${BOLD}Deleting group: $GROUP_NAME${RESET}"
execute_curl "curl -s -X DELETE $API_BASE/groups/$GROUP_NAME | jq"
handle_error

# 4. Delete User
echo -e "${BOLD}Deleting user: $USER_NAME${RESET}"
execute_curl "curl -s -X DELETE $API_BASE/users/$USER_NAME | jq"
handle_error

# Cleanup temporary files
rm -f user.json user_update.json group.json group_update.json add_member.json remove_member.json computer.json computer_update.json

echo -e "${GREEN}${BOLD}===========================================================${RESET}"
echo -e "${GREEN}${BOLD}  All tests completed. But successfully? !!Check Manually!!${RESET}"
echo -e "${GREEN}${BOLD}  This script may look fancy, but it is a door knob.${RESET}"
echo -e "${GREEN}${BOLD}  !!...IMPROVE IT...!! The error handling is basic!${RESET}"
echo -e "${GREEN}${BOLD}  It's just a documentation to onboard new contributors.${RESET}"
echo -e "${GREEN}${BOLD}  !!So, DONT BOTHER!! But don't forget to bring your towel!${RESET}"
echo -e "${GREEN}${BOLD}===========================================================${RESET}\n"

echo -e "${CYAN}This tested:${RESET}"
echo -e "${CYAN}✔︎ User Creation, Listing, Retrieval, Update, and Deletion${RESET}"
echo -e "${CYAN}︎︎✔︎ Group Creation, Listing, Retrieval, Update, and Deletion${RESET}"
echo -e "${CYAN}︎︎✔︎ Computer Creation, Retrieval, Update, and Deletion${RESET}"
echo -e "${CYAN}︎︎✔︎ Group Membership Management${RESET}"
echo -e "${CYAN}︎︎✔︎ User-Group Relationships${RESET}\n"

echo -e "${YELLOW}Remember: DON'T PANIC and always know where your towel is.${RESET}"