#!/usr/bin/env bash

# System API Demo Script
# This script demonstrates the System API endpoints using curl commands
# with comprehensive JSON request/response documentation

# Pastel colors for output
SOFT_CORAL='\033[38;2;255;183;178m'    # Soft coral for errors
MINT_GREEN='\033[38;2;152;251;152m'    # Mint green for success
PEACH='\033[38;2;255;218;185m'         # Peach for warnings
LAVENDER='\033[38;2;230;230;250m'      # Lavender for info
SOFT_PINK='\033[38;2;255;182;193m'     # Soft pink for sections
POWDER_BLUE='\033[38;2;176;224;230m'   # Powder blue for commands
NC='\033[0m' # No Color

# Configuration variables
BASE_URL="${BASE_URL:-http://localhost:8042/api/v1/rodent/system}"
CONTENT_TYPE="Content-Type: application/json"

# Helper function to print section headers
print_section() {
    echo -e "\n${SOFT_PINK}================================${NC}"
    echo -e "${SOFT_PINK}$1${NC}"
    echo -e "${SOFT_PINK}================================${NC}\n"
}

# Helper function to print commands
print_command() {
    echo -e "${POWDER_BLUE}Command:${NC}"
    echo -e "${POWDER_BLUE}$1${NC}\n"
}

# Helper function to print JSON pretty
print_json() {
    echo -e "${LAVENDER}$1${NC}"
    echo "$2" | jq '.' 2>/dev/null || echo "$2"
    echo
}

# Helper function to execute curl and display results
execute_curl() {
    local description="$1"
    local curl_command="$2"
    
    echo -e "${MINT_GREEN}$description${NC}"
    print_command "$curl_command"
    
    echo -e "${LAVENDER}Response:${NC}"
    response=$(eval "$curl_command")
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    echo
    
    return 0
}

# Generate random suffix for test isolation
generate_random_suffix() {
    echo "$(date +%s)$(( RANDOM % 9000 + 1000 ))"
}

# Show usage information
show_usage() {
    echo -e "${SOFT_PINK}System API Demo Script${NC}"
    echo -e "${LAVENDER}This script demonstrates the System API endpoints with comprehensive JSON documentation.${NC}"
    echo -e "${LAVENDER}Base URL: $BASE_URL${NC}\n"
    
    echo -e "${POWDER_BLUE}Usage: $0 [test_name|all]${NC}"
    echo -e "${POWDER_BLUE}Available tests:${NC}"
    echo -e "  ${MINT_GREEN}info${NC}            - System information endpoints"
    echo -e "  ${MINT_GREEN}hostname${NC}        - Hostname management"
    echo -e "  ${MINT_GREEN}users${NC}           - User management"
    echo -e "  ${MINT_GREEN}groups${NC}          - Group management" 
    echo -e "  ${MINT_GREEN}power${NC}           - Power management"
    echo -e "  ${MINT_GREEN}config${NC}          - System configuration (timezone, locale)"
    echo -e "  ${MINT_GREEN}all${NC}             - Run all tests"
    echo -e "\n${PEACH}Examples:${NC}"
    echo -e "  $0 info"
    echo -e "  $0 users"
    echo -e "  $0 all"
    echo -e "\n${PEACH}Configuration:${NC}"
    echo -e "  Override base URL: BASE_URL=http://other-host:8042/api/v1/rodent/system $0 [test]"
    echo
}

# Test 1: System Information
test_info() {
    print_section "Test 1: System Information"
    
    echo -e "${MINT_GREEN}1.1: Get Complete System Info${NC}"
    GET_SYSINFO_CURL="curl -s -X GET \"$BASE_URL/info\" -H \"$CONTENT_TYPE\""
    execute_curl "Get Complete System Information" "$GET_SYSINFO_CURL"
    
    echo -e "${MINT_GREEN}1.2: Get OS Information${NC}"
    GET_OSINFO_CURL="curl -s -X GET \"$BASE_URL/info/os\" -H \"$CONTENT_TYPE\""
    execute_curl "Get OS Information" "$GET_OSINFO_CURL"
    
    echo -e "${MINT_GREEN}1.3: Get Hardware Information${NC}"
    GET_HWINFO_CURL="curl -s -X GET \"$BASE_URL/info/hardware\" -H \"$CONTENT_TYPE\""
    execute_curl "Get Hardware Information" "$GET_HWINFO_CURL"
    
    echo -e "${MINT_GREEN}1.4: Get Performance Information${NC}"
    GET_PERFINFO_CURL="curl -s -X GET \"$BASE_URL/info/performance\" -H \"$CONTENT_TYPE\""
    execute_curl "Get Performance Information" "$GET_PERFINFO_CURL"
    
    echo -e "${MINT_GREEN}1.5: Get System Health${NC}"
    GET_HEALTH_CURL="curl -s -X GET \"$BASE_URL/health\" -H \"$CONTENT_TYPE\""
    execute_curl "Get System Health" "$GET_HEALTH_CURL"
}

# Test 2: Hostname Management
test_hostname() {
    print_section "Test 2: Hostname Management"
    
    echo -e "${MINT_GREEN}2.1: Get Current Hostname${NC}"
    GET_HOSTNAME_CURL="curl -s -X GET \"$BASE_URL/hostname\" -H \"$CONTENT_TYPE\""
    response=$(eval "$GET_HOSTNAME_CURL")
    echo -e "${LAVENDER}Response:${NC}"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    echo
    
    # Store original hostname for restoration
    ORIGINAL_HOSTNAME=$(echo "$response" | jq -r '.result.hostname // "rodent-demo"' 2>/dev/null)
    
    SUFFIX=$(generate_random_suffix)
    NEW_HOSTNAME="rodent-demo-$SUFFIX"
    
    echo -e "${MINT_GREEN}2.2: Set New Hostname${NC}"
    cat << EOF > /tmp/set_hostname_request.json
{
  "hostname": "$NEW_HOSTNAME",
  "pretty": "Rodent Demo System $SUFFIX",
  "static": true
}
EOF

    print_json "Set Hostname Request JSON:" "$(cat /tmp/set_hostname_request.json)"
    
    SET_HOSTNAME_CURL="curl -s -X PUT \"$BASE_URL/hostname\" -H \"$CONTENT_TYPE\" -d @/tmp/set_hostname_request.json"
    execute_curl "Set New Hostname" "$SET_HOSTNAME_CURL"
    
    echo -e "${MINT_GREEN}2.3: Verify Hostname Change${NC}"
    execute_curl "Verify Hostname Change" "$GET_HOSTNAME_CURL"
    
    echo -e "${MINT_GREEN}2.4: Restore Original Hostname${NC}"
    cat << EOF > /tmp/restore_hostname_request.json
{
  "hostname": "$ORIGINAL_HOSTNAME",
  "static": true
}
EOF

    print_json "Restore Hostname Request JSON:" "$(cat /tmp/restore_hostname_request.json)"
    
    RESTORE_HOSTNAME_CURL="curl -s -X PUT \"$BASE_URL/hostname\" -H \"$CONTENT_TYPE\" -d @/tmp/restore_hostname_request.json"
    execute_curl "Restore Original Hostname" "$RESTORE_HOSTNAME_CURL"
    
    # Cleanup temp files
    rm -f /tmp/set_hostname_request.json
    rm -f /tmp/restore_hostname_request.json
}

# Test 3: User Management
test_users() {
    print_section "Test 3: User Management"
    
    SUFFIX=$(generate_random_suffix)
    NEW_USERNAME="demouser$SUFFIX"
    
    echo -e "${MINT_GREEN}3.1: List Existing Users${NC}"
    LIST_USERS_CURL="curl -s -X GET \"$BASE_URL/users\" -H \"$CONTENT_TYPE\""
    execute_curl "List System Users" "$LIST_USERS_CURL"
    
    echo -e "${MINT_GREEN}3.2: Create New User${NC}"
    cat << EOF > /tmp/create_user_request.json
{
  "username": "$NEW_USERNAME",
  "full_name": "Demo User $SUFFIX",
  "home_dir": "/home/$NEW_USERNAME",
  "shell": "/bin/bash",
  "groups": ["users"],
  "create_home": true,
  "system_user": false,
  "password": "DemoPassword123!"
}
EOF

    print_json "Create User Request JSON:" "$(cat /tmp/create_user_request.json)"
    
    CREATE_USER_CURL="curl -s -X POST \"$BASE_URL/users\" -H \"$CONTENT_TYPE\" -d @/tmp/create_user_request.json"
    response=$(eval "$CREATE_USER_CURL")
    echo -e "${LAVENDER}Response:${NC}"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    echo
    
    # Check if user was created successfully
    if echo "$response" | jq -e '.success == true' >/dev/null 2>&1; then
        echo -e "${MINT_GREEN}3.3: Get Created User Details${NC}"
        GET_USER_CURL="curl -s -X GET \"$BASE_URL/users/$NEW_USERNAME\" -H \"$CONTENT_TYPE\""
        execute_curl "Get User Details" "$GET_USER_CURL"
        
        echo -e "${MINT_GREEN}3.4: List Users After Creation${NC}"
        execute_curl "List Users After Creation" "$LIST_USERS_CURL"
        
        echo -e "${MINT_GREEN}3.5: Delete Created User (Cleanup)${NC}"
        DELETE_USER_CURL="curl -s -X DELETE \"$BASE_URL/users/$NEW_USERNAME\" -H \"$CONTENT_TYPE\""
        execute_curl "Delete Demo User" "$DELETE_USER_CURL"
        
        echo -e "${MINT_GREEN}3.6: Verify User Deletion${NC}"
        execute_curl "Verify User Deletion" "$LIST_USERS_CURL"
    else
        echo -e "${PEACH}User creation failed, skipping user operations${NC}"
    fi
    
    # Cleanup temp files
    rm -f /tmp/create_user_request.json
}

# Test 4: Group Management
test_groups() {
    print_section "Test 4: Group Management"
    
    SUFFIX=$(generate_random_suffix)
    NEW_GROUPNAME="demogroup$SUFFIX"
    
    echo -e "${MINT_GREEN}4.1: List Existing Groups${NC}"
    LIST_GROUPS_CURL="curl -s -X GET \"$BASE_URL/groups\" -H \"$CONTENT_TYPE\""
    execute_curl "List System Groups" "$LIST_GROUPS_CURL"
    
    echo -e "${MINT_GREEN}4.2: Create New Group${NC}"
    cat << EOF > /tmp/create_group_request.json
{
  "name": "$NEW_GROUPNAME",
  "system_group": false
}
EOF

    print_json "Create Group Request JSON:" "$(cat /tmp/create_group_request.json)"
    
    CREATE_GROUP_CURL="curl -s -X POST \"$BASE_URL/groups\" -H \"$CONTENT_TYPE\" -d @/tmp/create_group_request.json"
    response=$(eval "$CREATE_GROUP_CURL")
    echo -e "${LAVENDER}Response:${NC}"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    echo
    
    # Check if group was created successfully
    if echo "$response" | jq -e '.success == true' >/dev/null 2>&1; then
        echo -e "${MINT_GREEN}4.3: Get Created Group Details${NC}"
        GET_GROUP_CURL="curl -s -X GET \"$BASE_URL/groups/$NEW_GROUPNAME\" -H \"$CONTENT_TYPE\""
        execute_curl "Get Group Details" "$GET_GROUP_CURL"
        
        echo -e "${MINT_GREEN}4.4: List Groups After Creation${NC}"
        execute_curl "List Groups After Creation" "$LIST_GROUPS_CURL"
        
        echo -e "${MINT_GREEN}4.5: Delete Created Group (Cleanup)${NC}"
        DELETE_GROUP_CURL="curl -s -X DELETE \"$BASE_URL/groups/$NEW_GROUPNAME\" -H \"$CONTENT_TYPE\""
        execute_curl "Delete Demo Group" "$DELETE_GROUP_CURL"
        
        echo -e "${MINT_GREEN}4.6: Verify Group Deletion${NC}"
        execute_curl "Verify Group Deletion" "$LIST_GROUPS_CURL"
    else
        echo -e "${PEACH}Group creation failed, skipping group operations${NC}"
    fi
    
    # Cleanup temp files
    rm -f /tmp/create_group_request.json
}

# Test 5: Power Management
test_power() {
    print_section "Test 5: Power Management"
    
    echo -e "${MINT_GREEN}5.1: Get Power Status${NC}"
    GET_POWER_STATUS_CURL="curl -s -X GET \"$BASE_URL/power/status\" -H \"$CONTENT_TYPE\""
    execute_curl "Get Power Status" "$GET_POWER_STATUS_CURL"
    
    echo -e "${MINT_GREEN}5.2: Get Scheduled Shutdown Info${NC}"
    GET_SCHEDULED_CURL="curl -s -X GET \"$BASE_URL/power/scheduled\" -H \"$CONTENT_TYPE\""
    execute_curl "Get Scheduled Shutdown Info" "$GET_SCHEDULED_CURL"
    
    echo -e "${MINT_GREEN}5.3: Schedule Shutdown (Demo - Will be cancelled)${NC}"
    cat << EOF > /tmp/schedule_shutdown_request.json
{
  "delay_minutes": 60,
  "message": "System maintenance scheduled - Demo test"
}
EOF

    print_json "Schedule Shutdown Request JSON:" "$(cat /tmp/schedule_shutdown_request.json)"
    
    SCHEDULE_SHUTDOWN_CURL="curl -s -X POST \"$BASE_URL/power/schedule-shutdown\" -H \"$CONTENT_TYPE\" -d @/tmp/schedule_shutdown_request.json"
    execute_curl "Schedule Shutdown (Demo)" "$SCHEDULE_SHUTDOWN_CURL"
    
    echo -e "${MINT_GREEN}5.4: Verify Scheduled Shutdown${NC}"
    execute_curl "Verify Scheduled Shutdown" "$GET_SCHEDULED_CURL"
    
    echo -e "${MINT_GREEN}5.5: Cancel Scheduled Shutdown${NC}"
    CANCEL_SHUTDOWN_CURL="curl -s -X DELETE \"$BASE_URL/power/scheduled\" -H \"$CONTENT_TYPE\""
    execute_curl "Cancel Scheduled Shutdown" "$CANCEL_SHUTDOWN_CURL"
    
    echo -e "${MINT_GREEN}5.6: Verify Shutdown Cancelled${NC}"
    execute_curl "Verify Shutdown Cancelled" "$GET_SCHEDULED_CURL"
    
    echo -e "${PEACH}Note: Actual shutdown/reboot endpoints are available but not demonstrated${NC}"
    echo -e "${PEACH}for safety. Use with caution in production:${NC}"
    echo -e "${PEACH}  POST $BASE_URL/power/shutdown${NC}"
    echo -e "${PEACH}  POST $BASE_URL/power/reboot${NC}"
    
    # Cleanup temp files
    rm -f /tmp/schedule_shutdown_request.json
}

# Test 6: System Configuration
test_config() {
    print_section "Test 6: System Configuration"
    
    echo -e "${MINT_GREEN}6.1: Get Current Timezone${NC}"
    GET_TIMEZONE_CURL="curl -s -X GET \"$BASE_URL/config/timezone\" -H \"$CONTENT_TYPE\""
    response=$(eval "$GET_TIMEZONE_CURL")
    echo -e "${LAVENDER}Response:${NC}"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    echo
    
    # Store original timezone for restoration
    ORIGINAL_TIMEZONE=$(echo "$response" | jq -r '.result.timezone // "UTC"' 2>/dev/null)
    
    echo -e "${MINT_GREEN}6.2: Set Timezone (Demo)${NC}"
    cat << EOF > /tmp/set_timezone_request.json
{
  "timezone": "America/New_York"
}
EOF

    print_json "Set Timezone Request JSON:" "$(cat /tmp/set_timezone_request.json)"
    
    SET_TIMEZONE_CURL="curl -s -X PUT \"$BASE_URL/config/timezone\" -H \"$CONTENT_TYPE\" -d @/tmp/set_timezone_request.json"
    execute_curl "Set Timezone (Demo)" "$SET_TIMEZONE_CURL"
    
    echo -e "${MINT_GREEN}6.3: Verify Timezone Change${NC}"
    execute_curl "Verify Timezone Change" "$GET_TIMEZONE_CURL"
    
    echo -e "${MINT_GREEN}6.4: Restore Original Timezone${NC}"
    cat << EOF > /tmp/restore_timezone_request.json
{
  "timezone": "$ORIGINAL_TIMEZONE"
}
EOF

    print_json "Restore Timezone Request JSON:" "$(cat /tmp/restore_timezone_request.json)"
    
    RESTORE_TIMEZONE_CURL="curl -s -X PUT \"$BASE_URL/config/timezone\" -H \"$CONTENT_TYPE\" -d @/tmp/restore_timezone_request.json"
    execute_curl "Restore Original Timezone" "$RESTORE_TIMEZONE_CURL"
    
    echo -e "${MINT_GREEN}6.5: Get Current Locale${NC}"
    GET_LOCALE_CURL="curl -s -X GET \"$BASE_URL/config/locale\" -H \"$CONTENT_TYPE\""
    response=$(eval "$GET_LOCALE_CURL")
    echo -e "${LAVENDER}Response:${NC}"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    echo
    
    # Store original locale for restoration
    ORIGINAL_LOCALE=$(echo "$response" | jq -r '.result.locale // "en_US.UTF-8"' 2>/dev/null)
    
    echo -e "${MINT_GREEN}6.6: Set Locale (Demo)${NC}"
    cat << EOF > /tmp/set_locale_request.json
{
  "locale": "en_GB.UTF-8"
}
EOF

    print_json "Set Locale Request JSON:" "$(cat /tmp/set_locale_request.json)"
    
    SET_LOCALE_CURL="curl -s -X PUT \"$BASE_URL/config/locale\" -H \"$CONTENT_TYPE\" -d @/tmp/set_locale_request.json"
    execute_curl "Set Locale (Demo)" "$SET_LOCALE_CURL"
    
    echo -e "${MINT_GREEN}6.7: Verify Locale Change${NC}"
    execute_curl "Verify Locale Change" "$GET_LOCALE_CURL"
    
    echo -e "${MINT_GREEN}6.8: Restore Original Locale${NC}"
    cat << EOF > /tmp/restore_locale_request.json
{
  "locale": "$ORIGINAL_LOCALE"
}
EOF

    print_json "Restore Locale Request JSON:" "$(cat /tmp/restore_locale_request.json)"
    
    RESTORE_LOCALE_CURL="curl -s -X PUT \"$BASE_URL/config/locale\" -H \"$CONTENT_TYPE\" -d @/tmp/restore_locale_request.json"
    execute_curl "Restore Original Locale" "$RESTORE_LOCALE_CURL"
    
    # Cleanup temp files
    rm -f /tmp/set_timezone_request.json
    rm -f /tmp/restore_timezone_request.json
    rm -f /tmp/set_locale_request.json
    rm -f /tmp/restore_locale_request.json
}


# Main execution logic
main() {
    local test_name="$1"
    
    if [ -z "$test_name" ]; then
        show_usage
        exit 1
    fi
    
    case "$test_name" in
        "info")
            test_info
            ;;
        "hostname")
            test_hostname
            ;;
        "users")
            test_users
            ;;
        "groups")
            test_groups
            ;;
        "power")
            test_power
            ;;
        "config")
            test_config
            ;;
        "all")
            test_info
            test_hostname
            test_users
            test_groups
            test_power
            test_config
            ;;
        *)
            echo -e "${SOFT_CORAL}Unknown test: $test_name${NC}"
            show_usage
            exit 1
            ;;
    esac
    
    print_section "Demo Complete"
    echo -e "${LAVENDER}System API endpoint demonstration completed.${NC}"
    echo -e "${PEACH}Base URL: $BASE_URL${NC}"
    echo -e "${PEACH}All demo resources have been automatically cleaned up.${NC}"
}

# Run main function with all arguments
main "$@"