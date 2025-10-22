#!/usr/bin/env bash
#
# Disk Management API - Statistics Operations Test
# Tests: global statistics, device statistics
#

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8042/api/v1/rodent/disks}"

echo "================================================================================"
echo "DISK MANAGEMENT API - STATISTICS OPERATIONS TEST"
echo "================================================================================"
echo "API Base: $API_BASE"
echo "Test Time: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo ""

# GET /disks/statistics/global - Get global statistics
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/statistics/global - Get Global Statistics"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/statistics/global"
echo ""
curl -s -X GET "$API_BASE/statistics/global" | jq '.'
echo ""

# Get a disk ID for device statistics test
echo "Finding a disk for device statistics test..."
DISK_ID=$(curl -s -X GET "$API_BASE/" | jq -r '.result.disks[0].device_id // empty')

if [ -z "$DISK_ID" ]; then
    echo "ERROR: No disks found for statistics test"
    exit 1
fi

echo "Using disk ID: $DISK_ID"
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
echo "STATISTICS OPERATIONS TEST COMPLETED"
echo "================================================================================"
