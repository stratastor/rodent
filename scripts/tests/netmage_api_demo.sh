#!/bin/bash
# Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
# Copyright 2025 The StrataSTOR Authors and Contributors
# SPDX-License-Identifier: Apache-2.0

# Network Management API Demo Script
# This script demonstrates the Netmage REST API endpoints with curl commands
# Serves as interactive documentation and testing tool

set -e

# Configuration
API_BASE="${NETMAGE_API_BASE:-http://localhost:8042/api/v1/rodent/network}"
VERBOSE=${VERBOSE:-false}
DRY_RUN=${DRY_RUN:-false}
USE_INTERFACE="${USE_INTERFACE:-enX0}"

# Pastel colors for output
SOFT_CORAL='\033[38;2;255;183;178m'    # Soft coral for errors
MINT_GREEN='\033[38;2;152;251;152m'    # Mint green for success
PEACH='\033[38;2;255;218;185m'         # Peach for warnings
LAVENDER='\033[38;2;230;230;250m'      # Lavender for info
SOFT_PINK='\033[38;2;255;182;193m'     # Soft pink for sections
POWDER_BLUE='\033[38;2;176;224;230m'   # Powder blue for commands
NC='\033[0m' # No Color

# Helper functions
log() {
    echo -e "${LAVENDER}[INFO]${NC} $1"
}

success() {
    echo -e "${MINT_GREEN}[SUCCESS]${NC} $1"
}

error() {
    echo -e "${SOFT_CORAL}[ERROR]${NC} $1"
}

warning() {
    echo -e "${PEACH}[WARNING]${NC} $1"
}

section() {
    echo -e "\n${SOFT_PINK}=== $1 ===${NC}"
}

# Print command and response
execute_curl() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    local description="$4"
    
    echo -e "\n${POWDER_BLUE}Command:${NC} $description"
    
    local curl_cmd="curl -s -X $method \"$API_BASE$endpoint\""
    
    if [ "$method" != "GET" ] && [ -n "$data" ]; then
        curl_cmd="$curl_cmd -H \"Content-Type: application/json\" -d '$data'"
    fi
    
    if [ "$VERBOSE" = "true" ]; then
        curl_cmd="$curl_cmd -v"
    fi
    
    echo -e "${PEACH}$ $curl_cmd${NC}"
    
    if [ "$DRY_RUN" = "true" ]; then
        echo -e "${PEACH}[DRY-RUN] Command not executed${NC}"
        return
    fi
    
    local response
    if [ "$method" != "GET" ] && [ -n "$data" ]; then
        response=$(curl -s -X "$method" "$API_BASE$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data" 2>/dev/null || echo '{"success":false,"error":{"message":"Request failed"}}')
    else
        response=$(curl -s -X "$method" "$API_BASE$endpoint" 2>/dev/null || echo '{"success":false,"error":{"message":"Request failed"}}')
    fi
    
    echo -e "${MINT_GREEN}Response:${NC}"
    echo "$response" | jq --color-output '.' 2>/dev/null || echo "$response"
    
    # Check if response indicates success
    local success_status=$(echo "$response" | jq -r '.success // false' 2>/dev/null || echo "false")
    if [ "$success_status" = "true" ]; then
        success "API call succeeded"
    else
        error "API call failed or returned error"
    fi
}

# Print usage information
usage() {
    cat << EOF
Network Management API Demo Script

Usage: $0 [OPTIONS] [COMMANDS]

OPTIONS:
    -h, --help          Show this help message
    -v, --verbose       Enable verbose output
    -d, --dry-run       Show commands without executing
    -b, --base-url URL  Set API base URL (default: $API_BASE)

COMMANDS:
    all                 Run all demonstrations
    system             System information endpoints
    interfaces         Interface management endpoints  
    config             Netplan configuration endpoints
    dns                DNS management endpoints
    validation         Validation endpoints
    backups            Backup management endpoints
    safe-apply         Safe configuration apply demo

ENVIRONMENT VARIABLES:
    NETMAGE_API_BASE   API base URL (default: http://localhost:8042/api/v1/rodent/network)
    VERBOSE            Enable verbose output (true/false)
    DRY_RUN            Show commands without executing (true/false)

EXAMPLES:
    # Run all demonstrations
    $0 all

    # Test specific functionality
    $0 interfaces validation

    # Use different server
    NETMAGE_API_BASE=http://server:8042/api/v1/rodent/network $0 system

    # Dry run to see commands
    $0 --dry-run all

EOF
}

# System information endpoints
demo_system() {
    section "System Information"
    
    execute_curl "GET" "/system" "" \
        "Get system network information"
}

# Interface management endpoints
demo_interfaces() {
    section "Interface Management"
    
    execute_curl "GET" "/interfaces" "" \
        "List all network interfaces"
    
    # Get first interface name for subsequent tests
    log "Getting first interface name for detailed tests..."
    local first_interface="$USE_INTERFACE"
    if [ "$DRY_RUN" != "true" ]; then
        first_interface=$(curl -s "$API_BASE/interfaces" | \
            jq -r '.result.interfaces[] | select(.type == "ethernet" and .name != "lo") | .name | select(.) | first // "enX0"' 2>/dev/null || echo "$USE_INTERFACE")
        log "Using interface: $first_interface"
    else
        first_interface=$USE_INTERFACE
    fi
    
    execute_curl "GET" "/interfaces/$first_interface" "" \
        "Get specific interface details"
    
    execute_curl "GET" "/interfaces/$first_interface/statistics" "" \
        "Get interface statistics"
    
    # Interface state management (commented out for safety)
    # execute_curl "PUT" "/interfaces/$first_interface/state" \
    #     '{"state": "up"}' \
    #     "Set interface state to UP"
}

# Address management endpoints  
demo_addresses() {
    section "IP Address Management"
    
    local test_interface="$USE_INTERFACE"
    
    execute_curl "GET" "/addresses/$test_interface" "" \
        "Get IP addresses for interface"
    
    # Address management (commented out for safety)
    # execute_curl "POST" "/addresses" \
    #     '{"interface": "'$test_interface'", "address": "192.168.100.10/24"}' \
    #     "Add IP address to interface"
    
    # execute_curl "DELETE" "/addresses" \
    #     '{"interface": "'$test_interface'", "address": "192.168.100.10/24"}' \
    #     "Remove IP address from interface"
}

# Route management endpoints
demo_routes() {
    section "Route Management"
    
    execute_curl "GET" "/routes" "" \
        "List all routes"
    
    execute_curl "GET" "/routes?table=main" "" \
        "List routes in main table"
    
    # Route management (commented out for safety)
    # execute_curl "POST" "/routes" \
    #     '{"to": "192.168.200.0/24", "via": "192.168.1.1", "device": "eth0"}' \
    #     "Add a route"
    
    # execute_curl "DELETE" "/routes" \
    #     '{"to": "192.168.200.0/24", "via": "192.168.1.1", "device": "eth0"}' \
    #     "Remove a route"
}

# Netplan configuration endpoints
demo_config() {
    section "Netplan Configuration"
    
    execute_curl "GET" "/netplan/config" "" \
        "Get current netplan configuration"
    
    execute_curl "GET" "/netplan/status" "" \
        "Get netplan status"
    
    execute_curl "GET" "/netplan/diff" "" \
        "Get netplan diff"
    
    # Configuration changes (commented out for safety)
    # execute_curl "PUT" "/netplan/config" \
    #     '{"config": {"network": {"version": 2, "renderer": "networkd"}}, "backup_description": "API test backup"}' \
    #     "Update netplan configuration with backup"
    
    # execute_curl "POST" "/netplan/apply" "" \
    #     "Apply netplan configuration"
}

# DNS management endpoints
demo_dns() {
    section "DNS Management"
    
    execute_curl "GET" "/dns/global" "" \
        "Get global DNS configuration"
    
    # DNS configuration changes (commented out for safety - requires root)
    # execute_curl "PUT" "/dns/global" \
    #     '{"addresses": ["8.8.8.8", "1.1.1.1"], "search": ["example.com"]}' \
    #     "Set global DNS configuration"
}

# Validation endpoints
demo_validation() {
    section "Validation"
    
    execute_curl "POST" "/validate/ip" \
        '{"address": "192.168.1.1"}' \
        "Validate IPv4 address"
    
    execute_curl "POST" "/validate/ip" \
        '{"address": "192.168.1.1/24"}' \
        "Validate IPv4 address with CIDR"
    
    execute_curl "POST" "/validate/ip" \
        '{"address": "2001:db8::1"}' \
        "Validate IPv6 address"
    
    execute_curl "POST" "/validate/ip" \
        '{"address": "invalid.ip"}' \
        "Validate invalid IP address"
    
    execute_curl "POST" "/validate/interface-name" \
        '{"name": "eth0"}' \
        "Validate interface name"
    
    execute_curl "POST" "/validate/interface-name" \
        '{"name": "invalid@name"}' \
        "Validate invalid interface name"
    
    execute_curl "POST" "/validate/netplan-config" \
        '{"config": {"network": {"version": 2, "renderer": "networkd"}}}' \
        "Validate netplan configuration"
}

# Backup management endpoints
demo_backups() {
    section "Backup Management"
    
    execute_curl "GET" "/backups" "" \
        "List configuration backups"
    
    execute_curl "POST" "/backups" "" \
        "Create configuration backup"
    
    # Backup restoration (commented out for safety)
    # execute_curl "POST" "/backups/backup-id/restore" "" \
    #     "Restore configuration from backup"
}

# Safe apply demonstration
demo_safe_apply() {
    section "Safe Configuration Apply"
    
    log "Safe apply requires current configuration as input"
    
    if [ "$DRY_RUN" != "true" ]; then
        log "Getting current configuration..."
        local current_config=$(curl -s "$API_BASE/netplan/config" | \
            jq -c '.result // {}' 2>/dev/null || echo '{}')
        
        if [ "$current_config" != "{}" ]; then
            local safe_apply_payload=$(jq -n \
                --argjson config "$current_config" \
                '{
                    "config": $config,
                    "options": {
                        "skip_pre_validation": true,
                        "skip_post_validation": true,
                        "validate_connectivity": false,
                        "auto_backup": true,
                        "auto_rollback": false,
                        "grace_period": "5s"
                    }
                }')
            execute_curl "POST" "/netplan/safe-apply" \
                "$safe_apply_payload" \
                "Safe apply current configuration (test mode)"
        else
            warning "Could not retrieve current configuration for safe apply demo"
        fi
    else
        execute_curl "POST" "/netplan/safe-apply" \
            '{"config": {"network": {"version": 2}}, "options": {"skip_pre_validation": true}}' \
            "Safe apply configuration (dry-run example)"
    fi
}

# Main execution
main() {
    local commands=()
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                usage
                exit 0
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -d|--dry-run)
                DRY_RUN=true
                shift
                ;;
            -b|--base-url)
                API_BASE="$2"
                shift 2
                ;;
            all|system|interfaces|addresses|routes|config|dns|validation|backups|safe-apply)
                commands+=("$1")
                shift
                ;;
            *)
                error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
    
    # Default to all if no commands specified
    if [ ${#commands[@]} -eq 0 ]; then
        commands=("all")
    fi
    
    # Print configuration
    echo -e "${LAVENDER}Network Management API Demo${NC}"
    echo -e "API Base URL: ${POWDER_BLUE}$API_BASE${NC}"
    echo -e "Verbose: ${POWDER_BLUE}$VERBOSE${NC}"
    echo -e "Dry Run: ${POWDER_BLUE}$DRY_RUN${NC}"
    
    if [ "$DRY_RUN" = "true" ]; then
        warning "DRY RUN MODE - Commands will be shown but not executed"
    fi
    
    # Check if server is reachable (skip in dry-run)
    if [ "$DRY_RUN" != "true" ]; then
        log "Checking API server connectivity..."
        if ! curl -s --connect-timeout 5 "$API_BASE/system" >/dev/null 2>&1; then
            error "Cannot reach API server at $API_BASE"
            error "Make sure the Rodent server is running and accessible"
            exit 1
        fi
        success "API server is reachable"
    fi
    
    # Execute requested demonstrations
    for cmd in "${commands[@]}"; do
        case $cmd in
            all)
                demo_system
                demo_interfaces  
                demo_addresses
                demo_routes
                demo_config
                demo_dns
                demo_validation
                demo_backups
                demo_safe_apply
                ;;
            system)
                demo_system
                ;;
            interfaces)
                demo_interfaces
                ;;
            addresses)
                demo_addresses
                ;;
            routes)
                demo_routes
                ;;
            config)
                demo_config
                ;;
            dns)
                demo_dns
                ;;
            validation)
                demo_validation
                ;;
            backups)
                demo_backups
                ;;
            safe-apply)
                demo_safe_apply
                ;;
        esac
    done
    
    section "Demo Complete"
    success "All demonstrations completed successfully"
    
    if [ "$DRY_RUN" != "true" ]; then
        log "For production use, ensure proper authentication and authorization"
        log "Many configuration changes require appropriate privileges"
    fi
}

# Check dependencies
check_dependencies() {
    local missing_deps=()
    
    if ! command -v curl >/dev/null 2>&1; then
        missing_deps+=("curl")
    fi
    
    if ! command -v jq >/dev/null 2>&1; then
        missing_deps+=("jq")
    fi
    
    if [ ${#missing_deps[@]} -gt 0 ]; then
        error "Missing required dependencies: ${missing_deps[*]}"
        error "Please install the missing tools and try again"
        exit 1
    fi
}

# Run dependency check and main function
check_dependencies
main "$@"