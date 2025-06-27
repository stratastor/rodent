#!/usr/bin/env bash
# Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
# Copyright 2025 The StrataSTOR Authors and Contributors
# SPDX-License-Identifier: Apache-2.0

# ZFS Transfer Management API Demo Script
# This script demonstrates the ZFS Transfer REST API endpoints with curl commands
# Serves as interactive documentation and testing tool for managed transfers

set -e

# Configuration with defaults from transfer_manager_integration_test.go
API_BASE="${RODENT_API_BASE:-http://localhost:8042/api/v1/rodent/zfs/dataset/transfer}"
VERBOSE=${VERBOSE:-false}
DRY_RUN=${DRY_RUN:-false}

# Test configuration from environment variables or defaults
TARGET_USERNAME="${RODENT_TEST_TARGET_USERNAME:-rodent}"
TARGET_IP="${RODENT_TEST_TARGET_IP:-172.31.14.189}"
TARGET_FILESYSTEM="${RODENT_TEST_TARGET_FILESYSTEM:-store/newFS}"
SSH_KEY_PATH="${RODENT_TEST_SSH_KEY_PATH:-/home/rodent/.rodent/ssh/01978d99-b37f-7032-8f59-94d42795652f/id_ed25519}"
SOURCE_FILESYSTEM="${RODENT_TEST_SOURCE_FILESYSTEM:-tank/standardFS}"

# Pastel colors for output
SOFT_CORAL='\033[38;2;255;183;178m'    # Soft coral for errors
MINT_GREEN='\033[38;2;152;251;152m'    # Mint green for success
PEACH='\033[38;2;255;218;185m'         # Peach for warnings
LAVENDER='\033[38;2;230;230;250m'      # Lavender for info
SOFT_PINK='\033[38;2;255;182;193m'     # Soft pink for sections
POWDER_BLUE='\033[38;2;176;224;230m'   # Powder blue for commands
NC='\033[0m' # No Color

# Global variables for test state
TEST_SNAPSHOT=""
TEST_TRANSFER_ID=""
CLEANUP_FUNCTIONS=()

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

# Add cleanup function to queue
add_cleanup() {
    CLEANUP_FUNCTIONS+=("$1")
}

# Execute cleanup functions in reverse order
cleanup() {
    if [ ${#CLEANUP_FUNCTIONS[@]} -gt 0 ]; then
        section "Cleanup"
        for ((i=${#CLEANUP_FUNCTIONS[@]}-1; i>=0; i--)); do
            local cleanup_func="${CLEANUP_FUNCTIONS[i]}"
            log "Executing cleanup: $cleanup_func"
            eval "$cleanup_func" || warning "Cleanup failed: $cleanup_func"
        done
    fi
}

# Set trap for cleanup on exit
trap cleanup EXIT

# Print command and response
execute_curl() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    local description="$4"
    local expected_status="${5:-200}"
    
    echo -e "\n${POWDER_BLUE}Command:${NC} $description"
    
    local curl_cmd="curl -s -w \"\\n%{http_code}\" -X $method \"$API_BASE$endpoint\""
    
    if [ "$method" != "GET" ] && [ -n "$data" ]; then
        curl_cmd="$curl_cmd -H \"Content-Type: application/json\" -d '$data'"
    fi
    
    if [ "$VERBOSE" = "true" ]; then
        curl_cmd="$curl_cmd -v"
    fi
    
    echo -e "${PEACH}$ $curl_cmd${NC}"
    
    if [ "$DRY_RUN" = "true" ]; then
        echo -e "${PEACH}[DRY-RUN] Command not executed${NC}"
        return 0
    fi
    
    local response_with_code
    if [ "$method" != "GET" ] && [ -n "$data" ]; then
        response_with_code=$(curl -s -w "\\n%{http_code}" -X "$method" "$API_BASE$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data" 2>&1 || echo -e '{"success":false,"error":{"message":"cURL request failed"}}\n000')
    else
        response_with_code=$(curl -s -w "\\n%{http_code}" -X "$method" "$API_BASE$endpoint" 2>&1 || echo -e '{"success":false,"error":{"message":"cURL request failed"}}\n000')
    fi
    
    # Split response and HTTP code
    local response=$(echo "$response_with_code" | head -n -1)
    local http_code=$(echo "$response_with_code" | tail -n 1)
    
    echo -e "${MINT_GREEN}Response (HTTP $http_code):${NC}"
    echo "$response" | jq --color-output '.' 2>/dev/null || echo "$response"
    
    # Check HTTP status and response success
    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        local success_status=$(echo "$response" | jq -r '.success // false' 2>/dev/null || echo "false")
        if [ "$success_status" = "true" ]; then
            success "API call succeeded (HTTP $http_code)"
            echo "$response" # Return response for further processing
            return 0
        else
            error "API call returned error (HTTP $http_code)"
            return 1
        fi
    else
        error "API call failed with HTTP $http_code"
        return 1
    fi
}

# Extract transfer ID from response
extract_transfer_id() {
    local response="$1"
    echo "$response" | jq -r '.result.transfer_id // .result.id // empty' 2>/dev/null
}

# Extract result data from response
extract_result() {
    local response="$1"
    echo "$response" | jq -r '.result // empty' 2>/dev/null
}

# Wait for transfer to reach specific status
wait_for_transfer_status() {
    local transfer_id="$1"
    local target_statuses="$2"  # comma-separated list
    local timeout_seconds="${3:-300}"  # 5 minutes default
    local check_interval="${4:-2}"     # 2 seconds default
    
    log "Waiting for transfer $transfer_id to reach status: $target_statuses (timeout: ${timeout_seconds}s)"
    
    local elapsed=0
    while [ $elapsed -lt $timeout_seconds ]; do
        if [ "$DRY_RUN" = "true" ]; then
            log "DRY-RUN: Would check transfer status"
            return 0
        fi
        
        local response
        response=$(execute_curl "GET" "/$transfer_id" "" "Check transfer status" || echo "")
        
        if [ -n "$response" ]; then
            local status=$(echo "$response" | jq -r '.result.status // empty' 2>/dev/null)
            local error_msg=$(echo "$response" | jq -r '.result.error_message // empty' 2>/dev/null)
            
            if [ -n "$status" ]; then
                log "Transfer status: $status (elapsed: ${elapsed}s)"
                
                # Check if status matches any target status
                IFS=',' read -ra STATUSES <<< "$target_statuses"
                for target_status in "${STATUSES[@]}"; do
                    if [ "$status" = "$target_status" ]; then
                        success "Transfer reached target status: $status"
                        if [ "$status" = "failed" ] && [ -n "$error_msg" ]; then
                            error "Transfer error: $error_msg"
                        fi
                        return 0
                    fi
                done
                
                # Check for terminal failure states
                if [ "$status" = "failed" ]; then
                    error "Transfer failed: $error_msg"
                    return 1
                fi
            fi
        fi
        
        sleep $check_interval
        elapsed=$((elapsed + check_interval))
    done
    
    error "Timeout waiting for transfer to reach status: $target_statuses"
    return 1
}

# Create test snapshot
create_test_snapshot() {
    if [ "$DRY_RUN" = "true" ]; then
        TEST_SNAPSHOT="$SOURCE_FILESYSTEM@test-transfer-dryrun"
        log "DRY-RUN: Would create snapshot $TEST_SNAPSHOT"
        return 0
    fi
    
    local timestamp=$(date +"%Y%m%d-%H%M%S")
    local snapshot_name="test-transfer-$timestamp"
    TEST_SNAPSHOT="$SOURCE_FILESYSTEM@$snapshot_name"
    
    log "Creating test snapshot: $TEST_SNAPSHOT"
    
    if sudo zfs snapshot "$TEST_SNAPSHOT"; then
        success "Created snapshot: $TEST_SNAPSHOT"
        add_cleanup "cleanup_snapshot '$TEST_SNAPSHOT'"
        return 0
    else
        error "Failed to create snapshot: $TEST_SNAPSHOT"
        return 1
    fi
}

# Cleanup snapshot
cleanup_snapshot() {
    local snapshot="$1"
    if [ "$DRY_RUN" = "true" ]; then
        log "DRY-RUN: Would cleanup snapshot $snapshot"
        return 0
    fi
    
    log "Cleaning up snapshot: $snapshot"
    if sudo zfs destroy "$snapshot" 2>/dev/null; then
        success "Cleaned up snapshot: $snapshot"
    else
        warning "Failed to cleanup snapshot: $snapshot"
    fi
}

# Cleanup remote filesystem
cleanup_remote_filesystem() {
    local target_path="$1"
    if [ "$DRY_RUN" = "true" ]; then
        log "DRY-RUN: Would cleanup remote filesystem $target_path"
        return 0
    fi
    
    log "Cleaning up remote filesystem: $target_path"
    local cleanup_cmd="ssh -i $SSH_KEY_PATH $TARGET_USERNAME@$TARGET_IP 'sudo zfs destroy -r $target_path'"
    
    if eval "$cleanup_cmd" 2>/dev/null; then
        success "Cleaned up remote filesystem: $target_path"
    else
        warning "Failed to cleanup remote filesystem: $target_path"
    fi
}

# Print usage information
usage() {
    cat << EOF
ZFS Transfer Management API Demo Script

Usage: $0 [OPTIONS] [COMMANDS]

OPTIONS:
    -h, --help          Show this help message
    -v, --verbose       Enable verbose output
    -d, --dry-run       Show commands without executing
    -b, --base-url URL  Set API base URL (default: $API_BASE)

COMMANDS:
    all                 Run all demonstrations
    basic              Basic transfer lifecycle
    pause-resume       Pause and resume transfer operations
    list               Transfer listing and filtering
    logs               Transfer log operations
    comprehensive      Complete TransferConfig field demonstration
    cleanup            Manual cleanup operations

ENVIRONMENT VARIABLES:
    RODENT_API_BASE              API base URL (default: http://localhost:8042/api/v1/rodent/zfs/dataset/transfer)
    RODENT_TEST_TARGET_USERNAME  SSH username (default: rodent)
    RODENT_TEST_TARGET_IP        Target server IP (default: 172.31.14.189)
    RODENT_TEST_TARGET_FILESYSTEM Target filesystem (default: store/newFS)
    RODENT_TEST_SSH_KEY_PATH     SSH private key path
    RODENT_TEST_SOURCE_FILESYSTEM Source filesystem (default: tank/standardFS)
    VERBOSE                      Enable verbose output (true/false)
    DRY_RUN                      Show commands without executing (true/false)

EXAMPLES:
    # Run all demonstrations
    $0 all

    # Test specific functionality
    $0 basic logs

    # Use different server
    RODENT_API_BASE=http://server:8080/api/v1/dataset/transfer $0 basic

    # Dry run to see commands
    $0 --dry-run all

EOF
}

# Demonstrate basic transfer lifecycle
demo_basic_transfer() {
    section "Basic Transfer Lifecycle"
    
    # Create test snapshot
    if ! create_test_snapshot; then
        error "Failed to create test snapshot"
        return 1
    fi
    
    local timestamp=$(date +"%Y%m%d-%H%M%S")
    local target_path="$TARGET_FILESYSTEM/test-basic-$timestamp"
    add_cleanup "cleanup_remote_filesystem '$target_path'"
    
    # Minimal transfer configuration
    local transfer_config=$(jq -n \
        --arg snapshot "$TEST_SNAPSHOT" \
        --arg target "$target_path" \
        --arg host "$TARGET_IP" \
        --arg user "$TARGET_USERNAME" \
        --arg key "$SSH_KEY_PATH" \
        '{
            "send": {
                "snapshot": $snapshot,
                "verbose": true
            },
            "receive": {
                "target": $target,
                "force": true,
                "resumable": true,
                "remote_host": {
                    "host": $host,
                    "port": 22,
                    "user": $user,
                    "private_key": $key
                }
            }
        }')
    
    # Show transfer config for debugging
    if [ "$VERBOSE" = "true" ]; then
        log "Transfer configuration:"
        echo "$transfer_config" | jq --color-output '.' 2>/dev/null || echo "$transfer_config"
    fi
    
    # Start transfer
    local response
    response=$(execute_curl "POST" "/start" "$transfer_config" "Start basic transfer")
    if [ $? -ne 0 ]; then
        error "Failed to start transfer"
        error "Response was: $response"
        return 1
    fi
    
    TEST_TRANSFER_ID=$(extract_transfer_id "$response")
    if [ -z "$TEST_TRANSFER_ID" ]; then
        error "Failed to extract transfer ID from response"
        return 1
    fi
    
    success "Started transfer with ID: $TEST_TRANSFER_ID"
    add_cleanup "cleanup_transfer '$TEST_TRANSFER_ID'"
    
    # Get transfer details
    execute_curl "GET" "/$TEST_TRANSFER_ID" "" "Get transfer details"
    
    # Wait for completion (short timeout for demo)
    if wait_for_transfer_status "$TEST_TRANSFER_ID" "completed,failed" 60; then
        success "Transfer completed successfully"
    else
        warning "Transfer did not complete within timeout"
    fi
}

# Demonstrate pause and resume operations
demo_pause_resume() {
    section "Pause and Resume Operations"
    
    if ! create_test_snapshot; then
        error "Failed to create test snapshot"
        return 1
    fi
    
    local timestamp=$(date +"%Y%m%d-%H%M%S")
    local target_path="$TARGET_FILESYSTEM/test-pause-resume-$timestamp"
    add_cleanup "cleanup_remote_filesystem '$target_path'"
    
    local transfer_config=$(jq -n \
        --arg snapshot "$TEST_SNAPSHOT" \
        --arg target "$target_path" \
        --arg host "$TARGET_IP" \
        --arg user "$TARGET_USERNAME" \
        --arg key "$SSH_KEY_PATH" \
        '{
            "send": {
                "snapshot": $snapshot,
                "verbose": true
            },
            "receive": {
                "target": $target,
                "force": true,
                "resumable": true,
                "remote_host": {
                    "host": $host,
                    "port": 22,
                    "user": $user,
                    "private_key": $key
                }
            }
        }')
    
    # Start transfer
    local response
    response=$(execute_curl "POST" "/start" "$transfer_config" "Start transfer for pause/resume demo")
    if [ $? -ne 0 ]; then
        return 1
    fi
    
    local transfer_id=$(extract_transfer_id "$response")
    add_cleanup "cleanup_transfer '$transfer_id'"
    
    # Wait briefly for transfer to start, then pause (timing is critical)
    if [ "$DRY_RUN" != "true" ]; then
        sleep 1
    fi
    
    # Pause transfer
    execute_curl "POST" "/$transfer_id/pause" "" "Pause transfer"
    
    # Check paused status
    execute_curl "GET" "/$transfer_id" "" "Verify transfer is paused"
    
    # Wait a moment
    if [ "$DRY_RUN" != "true" ]; then
        sleep 2
    fi
    
    # Resume transfer
    execute_curl "POST" "/$transfer_id/resume" "" "Resume transfer"
    
    # Check resumed status
    execute_curl "GET" "/$transfer_id" "" "Verify transfer is resumed"
    
    # Stop transfer (cleanup)
    execute_curl "POST" "/$transfer_id/stop" "" "Stop transfer"
}

# Demonstrate transfer listing and filtering
demo_list_operations() {
    section "Transfer Listing and Filtering"
    
    # List all transfers
    execute_curl "GET" "/list" "" "List all transfers (default: active)"
    
    # List with different types
    execute_curl "GET" "/list?type=all" "" "List all transfers"
    execute_curl "GET" "/list?type=active" "" "List active transfers"
    execute_curl "GET" "/list?type=completed" "" "List completed transfers"
    execute_curl "GET" "/list?type=failed" "" "List failed transfers"
}

# Demonstrate log operations
demo_log_operations() {
    section "Transfer Log Operations"
    
    if [ -z "$TEST_TRANSFER_ID" ]; then
        warning "No active transfer ID, creating one for log demo"
        if ! create_test_snapshot; then
            return 1
        fi
        
        local timestamp=$(date +"%Y%m%d-%H%M%S")
        local target_path="$TARGET_FILESYSTEM/test-logs-$timestamp"
        add_cleanup "cleanup_remote_filesystem '$target_path'"
        
        local transfer_config=$(jq -n \
            --arg snapshot "$TEST_SNAPSHOT" \
            --arg target "$target_path" \
            --arg host "$TARGET_IP" \
            --arg user "$TARGET_USERNAME" \
            --arg key "$SSH_KEY_PATH" \
            '{
                "send": {
                    "snapshot": $snapshot,
                    "verbose": true
                },
                "receive": {
                    "target": $target,
                    "force": true,
                    "resumable": true,
                    "remote_host": {
                        "host": $host,
                        "port": 22,
                        "user": $user,
                        "private_key": $key
                    }
                },
                "log_config": {
                    "max_size_bytes": 5120,
                    "truncate_on_finish": false,
                    "retain_on_failure": true,
                    "header_lines": 10,
                    "footer_lines": 10
                }
            }')
        
        local response
        response=$(execute_curl "POST" "/start" "$transfer_config" "Start transfer for log demo")
        TEST_TRANSFER_ID=$(extract_transfer_id "$response")
        add_cleanup "cleanup_transfer '$TEST_TRANSFER_ID'"
        
        # Let transfer run briefly to generate logs
        if [ "$DRY_RUN" != "true" ]; then
            sleep 3
        fi
    fi
    
    # Get full log
    execute_curl "GET" "/$TEST_TRANSFER_ID/log" "" "Get full transfer log"
    
    # Get log gist (truncated)
    execute_curl "GET" "/$TEST_TRANSFER_ID/log/gist" "" "Get transfer log gist (truncated)"
    
    # Test non-existent transfer log
    execute_curl "GET" "/non-existent-id/log" "" "Get log for non-existent transfer (should fail)" 404
}

# Comprehensive TransferConfig demonstration
demo_comprehensive_config() {
    section "Comprehensive TransferConfig Field Demonstration"
    
    if ! create_test_snapshot; then
        return 1
    fi
    
    local timestamp=$(date +"%Y%m%d-%H%M%S")
    local target_path="$TARGET_FILESYSTEM/test-comprehensive-$timestamp"
    add_cleanup "cleanup_remote_filesystem '$target_path'"
    
    # Complete TransferConfig with all possible fields documented
    local comprehensive_config=$(jq -n \
        --arg snapshot "$TEST_SNAPSHOT" \
        --arg target "$target_path" \
        --arg host "$TARGET_IP" \
        --arg user "$TARGET_USERNAME" \
        --arg key "$SSH_KEY_PATH" \
        '{
            "send": {
                "snapshot": $snapshot,
                "from_snapshot": "",
                "replicate": false,
                "skip_missing": false,
                "properties": true,
                "raw": false,
                "large_blocks": true,
                "embed_data": false,
                "holds": false,
                "backup_stream": false,
                "intermediary": false,
                "incremental": false,
                "compressed": true,
                "dry_run": false,
                "verbose": true,
                "resume_token": "",
                "parsable": false,
                "timeout": "0s",
                "log_level": "debug"
            },
            "receive": {
                "target": $target,
                "force": true,
                "unmounted": false,
                "resumable": true,
                "properties": {
                    "compression": "lz4",
                    "mountpoint": "none"
                },
                "origin": "",
                "exclude_props": ["mounted"],
                "use_parent": false,
                "dry_run": false,
                "verbose": true,
                "remote_host": {
                    "host": $host,
                    "port": 22,
                    "user": $user,
                    "private_key": $key,
                    "options": "",
                    "skip_host_key_check": false
                }
            },
            "log_config": {
                "max_size_bytes": 10240,
                "truncate_on_finish": true,
                "retain_on_failure": true,
                "header_lines": 20,
                "footer_lines": 20
            }
        }')
    
    # Start comprehensive transfer
    local response
    response=$(execute_curl "POST" "/start" "$comprehensive_config" "Start transfer with comprehensive configuration")
    if [ $? -ne 0 ]; then
        return 1
    fi
    
    local transfer_id=$(extract_transfer_id "$response")
    add_cleanup "cleanup_transfer '$transfer_id'"
    
    # Show the created transfer details
    execute_curl "GET" "/$transfer_id" "" "Get comprehensive transfer details"
    
    # Stop for cleanup
    execute_curl "POST" "/$transfer_id/stop" "" "Stop comprehensive transfer"
}

# Cleanup transfer
cleanup_transfer() {
    local transfer_id="$1"
    if [ "$DRY_RUN" = "true" ]; then
        log "DRY-RUN: Would cleanup transfer $transfer_id"
        return 0
    fi
    
    log "Cleaning up transfer: $transfer_id"
    
    # Try to stop first (in case it's still running)
    execute_curl "POST" "/$transfer_id/stop" "" "Stop transfer (cleanup)" >/dev/null 2>&1 || true
    
    # Wait a moment
    sleep 1
    
    # Delete the transfer
    if execute_curl "DELETE" "/$transfer_id" "" "Delete transfer (cleanup)" >/dev/null 2>&1; then
        success "Cleaned up transfer: $transfer_id"
    else
        warning "Failed to cleanup transfer: $transfer_id"
    fi
}

# Manual cleanup operations
demo_cleanup() {
    section "Manual Cleanup Operations"
    
    log "This demonstrates cleanup operations that would normally be automatic"
    
    # List transfers to see what might need cleanup
    execute_curl "GET" "/list?type=all" "" "List all transfers for cleanup review"
    
    warning "Manual cleanup of specific transfers can be done with:"
    echo -e "${PEACH}curl -X POST $API_BASE/TRANSFER_ID/stop${NC}"
    echo -e "${PEACH}curl -X DELETE $API_BASE/TRANSFER_ID${NC}"
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
            all|basic|pause-resume|list|logs|comprehensive|cleanup)
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
    
    # Default to basic if no commands specified
    if [ ${#commands[@]} -eq 0 ]; then
        commands=("basic")
    fi
    
    # Print configuration
    echo -e "${LAVENDER}ZFS Transfer Management API Demo${NC}"
    echo -e "API Base URL: ${POWDER_BLUE}$API_BASE${NC}"
    echo -e "Target: ${POWDER_BLUE}$TARGET_USERNAME@$TARGET_IP${NC}"
    echo -e "Source FS: ${POWDER_BLUE}$SOURCE_FILESYSTEM${NC}"
    echo -e "Target FS: ${POWDER_BLUE}$TARGET_FILESYSTEM${NC}"
    echo -e "SSH Key: ${POWDER_BLUE}$SSH_KEY_PATH${NC}"
    echo -e "Verbose: ${POWDER_BLUE}$VERBOSE${NC}"
    echo -e "Dry Run: ${POWDER_BLUE}$DRY_RUN${NC}"
    
    if [ "$DRY_RUN" = "true" ]; then
        warning "DRY RUN MODE - Commands will be shown but not executed"
    fi
    
    # Validate configuration
    if [ "$DRY_RUN" != "true" ]; then
        log "Validating configuration..."
        
        if [ ! -f "$SSH_KEY_PATH" ]; then
            error "SSH key not found: $SSH_KEY_PATH"
            exit 1
        fi
        
        # Check if source filesystem exists
        if ! sudo zfs list "$SOURCE_FILESYSTEM" >/dev/null 2>&1; then
            error "Source filesystem not found: $SOURCE_FILESYSTEM"
            exit 1
        fi
        
        # Check API server connectivity  
        log "Checking API server connectivity at $API_BASE"
        if ! curl -s --connect-timeout 5 "$API_BASE/list" >/dev/null 2>&1; then
            error "Cannot reach API server at $API_BASE"
            error "Make sure the Rodent server is running and accessible"
            error "Try: curl -s $API_BASE/list"
            exit 1
        fi
        
        success "Configuration validated"
    fi
    
    # Execute requested demonstrations
    for cmd in "${commands[@]}"; do
        case $cmd in
            all)
                demo_basic_transfer
                demo_pause_resume  
                demo_list_operations
                demo_log_operations
                demo_comprehensive_config
                ;;
            basic)
                demo_basic_transfer
                ;;
            pause-resume)
                demo_pause_resume
                ;;
            list)
                demo_list_operations
                ;;
            logs)
                demo_log_operations
                ;;
            comprehensive)
                demo_comprehensive_config
                ;;
            cleanup)
                demo_cleanup
                ;;
        esac
    done
    
    section "Demo Complete"
    success "ZFS Transfer API demonstrations completed successfully"
    
    if [ "$DRY_RUN" != "true" ]; then
        log "All test resources will be cleaned up automatically"
        log "For production use, ensure proper authentication and authorization"
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
    
    if [ "$DRY_RUN" != "true" ]; then
        if ! command -v zfs >/dev/null 2>&1; then
            missing_deps+=("zfs")
        fi
        
        if ! command -v ssh >/dev/null 2>&1; then
            missing_deps+=("ssh")
        fi
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