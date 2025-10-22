#!/usr/bin/env bash
#
# Disk Management API - Basic Operations Test
# Tests: inventory, disk details, health, SMART data
#

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8042/api/v1/rodent/disks}"
OUTPUT_DIR="${OUTPUT_DIR:-_gitignore/disks}"

echo "================================================================================"
echo "DISK MANAGEMENT API - BASIC OPERATIONS TEST"
echo "================================================================================"
echo "API Base: $API_BASE"
echo "Test Time: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo ""

# GET /disks/ - Get disk inventory
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/ - Get Disk Inventory"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/"
echo ""
curl -s -X GET "$API_BASE/" | jq '.'
echo ""

# Store first disk ID for subsequent tests
DISK_ID=$(curl -s -X GET "$API_BASE/" | jq -r '.result.disks[0].device_id // empty')

if [ -z "$DISK_ID" ]; then
    echo "ERROR: No disks found in inventory"
    exit 1
fi

echo "Using disk ID for tests: $DISK_ID"
echo ""

# GET /disks/:device_id - Get specific disk
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/:device_id - Get Specific Disk"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/$DISK_ID"
echo ""
curl -s -X GET "$API_BASE/$DISK_ID" | jq '.'
echo ""

# GET /disks/:device_id/health - Get disk health
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/:device_id/health - Get Disk Health"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/$DISK_ID/health"
echo ""
curl -s -X GET "$API_BASE/$DISK_ID/health" | jq '.'
echo ""

# GET /disks/:device_id/smart - Get SMART data
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/:device_id/smart - Get SMART Data"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/$DISK_ID/smart"
echo ""
SMART_RESPONSE=$(curl -s -X GET "$API_BASE/$DISK_ID/smart")
echo "$SMART_RESPONSE" | jq '.'
echo ""

# Check if SMART is available
if echo "$SMART_RESPONSE" | jq -e '.success == false' > /dev/null; then
    echo "NOTE: SMART data not available for this device"
fi
echo ""

# GET /disks/:device_id/statistics - Get device statistics
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/:device_id/statistics - Get Device Statistics"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/$DISK_ID/statistics"
echo ""
curl -s -X GET "$API_BASE/$DISK_ID/statistics" | jq '.'
echo ""

echo "================================================================================"
echo "BASIC OPERATIONS TEST COMPLETED"
echo "================================================================================"
