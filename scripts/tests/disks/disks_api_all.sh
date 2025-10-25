#!/usr/bin/env bash
#
# Disk Management API - Complete Test Suite
# Runs all disk API tests and captures outputs
#

set -euo pipefail

# Configuration
API_BASE="${API_BASE:-http://localhost:8042/api/v1/rodent/disks}"
OUTPUT_DIR="${OUTPUT_DIR:-/tmp/disks-api}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DISK="${TEST_DISK:-nvme4n1}"

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo "================================================================================"
echo "DISK MANAGEMENT API - COMPLETE TEST SUITE"
echo "================================================================================"
echo "API Base: $API_BASE"
echo "Test Disk: $TEST_DISK"
echo "Output Directory: $OUTPUT_DIR"
echo "Test Time: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo ""

# Test categories
TESTS=(
    "disks_api_basic:Basic Operations"
    "disks_api_state:State Management"
    "disks_api_discovery:Discovery Operations"
    "disks_api_probes:Probe Operations"
    "disks_api_topology:Topology Operations"
    "disks_api_statistics:Statistics Operations"
    "disks_api_config:Configuration Operations"
)

# Run each test
for test in "${TESTS[@]}"; do
    IFS=':' read -r script_name description <<< "$test"
    script_path="$SCRIPT_DIR/${script_name}.sh"
    output_file="$OUTPUT_DIR/${script_name}.out"

    if [ ! -f "$script_path" ]; then
        echo "WARNING: Test script not found: $script_path"
        continue
    fi

    echo "--------------------------------------------------------------------------------"
    echo "Running: $description"
    echo "Script: $script_name.sh"
    echo "Output: $output_file"
    echo "--------------------------------------------------------------------------------"

    # Make script executable
    chmod +x "$script_path"

    # Run test and capture output
    if API_BASE="$API_BASE" TEST_DISK="$TEST_DISK" bash "$script_path" > "$output_file" 2>&1; then
        echo "PASSED: $description"
    else
        echo "FAILED: $description (see $output_file for details)"
    fi
    echo ""
done

echo "================================================================================"
echo "TEST SUITE COMPLETED"
echo "================================================================================"
echo ""
echo "Test outputs saved to: $OUTPUT_DIR/"
echo ""
echo "Summary:"
ls -lh "$OUTPUT_DIR"/*.out 2>/dev/null || echo "No output files found"
echo ""
echo "To view a specific test output:"
echo "  cat $OUTPUT_DIR/disks_api_<test>.out"
echo ""
echo "To view all outputs:"
echo "  cat $OUTPUT_DIR/*.out"
echo ""
