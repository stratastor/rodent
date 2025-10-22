#!/usr/bin/env bash
#
# Disk Management API - Discovery Operations Test
# Tests: discovery, health check, SMART refresh
#

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8042/api/v1/rodent/disks}"

echo "================================================================================"
echo "DISK MANAGEMENT API - DISCOVERY OPERATIONS TEST"
echo "================================================================================"
echo "API Base: $API_BASE"
echo "Test Time: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo ""

# POST /disks/discovery/trigger - Trigger discovery
echo "--------------------------------------------------------------------------------"
echo "TEST: POST /disks/discovery/trigger - Trigger Disk Discovery"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: POST $API_BASE/discovery/trigger"
echo ""
echo "Triggering discovery..."
curl -s -X POST "$API_BASE/discovery/trigger" | jq '.'
echo ""

echo "Waiting 3 seconds for discovery to complete..."
sleep 3
echo ""

# Verify discovery results
echo "Checking inventory after discovery..."
DISK_COUNT=$(curl -s -X GET "$API_BASE/" | jq '.result.count')
echo "Total disks discovered: $DISK_COUNT"
echo ""

# POST /disks/health/check - Trigger health check
echo "--------------------------------------------------------------------------------"
echo "TEST: POST /disks/health/check - Trigger Health Check"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: POST $API_BASE/health/check"
echo ""
echo "Triggering health check..."
curl -s -X POST "$API_BASE/health/check" | jq '.'
echo ""

echo "Waiting 3 seconds for health check to complete..."
sleep 3
echo ""

# Verify health check results
echo "Checking health status after health check..."
curl -s -X GET "$API_BASE/" | jq '.result.disks[] | {device_id, health, health_reason}'
echo ""

# POST /disks/smart/refresh - Refresh SMART data
echo "--------------------------------------------------------------------------------"
echo "TEST: POST /disks/smart/refresh - Refresh SMART Data"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: POST $API_BASE/smart/refresh"
echo ""
echo "Triggering SMART refresh..."
curl -s -X POST "$API_BASE/smart/refresh" | jq '.'
echo ""

echo "Waiting 3 seconds for SMART refresh to complete..."
sleep 3
echo ""

# Verify SMART refresh results
echo "Checking SMART data after refresh..."
FIRST_DISK=$(curl -s -X GET "$API_BASE/" | jq -r '.result.disks[0].device_id')
curl -s -X GET "$API_BASE/$FIRST_DISK/smart" | jq '.result.overall_status // .error'
echo ""

echo "================================================================================"
echo "DISCOVERY OPERATIONS TEST COMPLETED"
echo "================================================================================"
