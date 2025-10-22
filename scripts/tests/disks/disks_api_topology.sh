#!/usr/bin/env bash
#
# Disk Management API - Topology Operations Test
# Tests: topology get, refresh, controllers, enclosures
#

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8042/api/v1/rodent/disks}"

echo "================================================================================"
echo "DISK MANAGEMENT API - TOPOLOGY OPERATIONS TEST"
echo "================================================================================"
echo "API Base: $API_BASE"
echo "Test Time: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo ""

# GET /disks/topology - Get complete topology
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/topology - Get Complete Topology"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/topology"
echo ""
curl -s -X GET "$API_BASE/topology" | jq '.'
echo ""

# GET /disks/topology/controllers - Get controllers
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/topology/controllers - Get Controllers"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/topology/controllers"
echo ""
CONTROLLERS=$(curl -s -X GET "$API_BASE/topology/controllers")
echo "$CONTROLLERS" | jq '.'
echo ""

CONTROLLER_COUNT=$(echo "$CONTROLLERS" | jq '.result.count // 0')
echo "Total controllers: $CONTROLLER_COUNT"
echo ""

# GET /disks/topology/enclosures - Get enclosures
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/topology/enclosures - Get Enclosures"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/topology/enclosures"
echo ""
ENCLOSURES=$(curl -s -X GET "$API_BASE/topology/enclosures")
echo "$ENCLOSURES" | jq '.'
echo ""

ENCLOSURE_COUNT=$(echo "$ENCLOSURES" | jq '.result.count // 0')
echo "Total enclosures: $ENCLOSURE_COUNT"
echo ""

# POST /disks/topology/refresh - Refresh topology
echo "--------------------------------------------------------------------------------"
echo "TEST: POST /disks/topology/refresh - Refresh Topology"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: POST $API_BASE/topology/refresh"
echo ""
echo "Triggering topology refresh..."
curl -s -X POST "$API_BASE/topology/refresh" | jq '.'
echo ""

echo "Waiting 3 seconds for topology refresh to complete..."
sleep 3
echo ""

# Verify refresh results
echo "Checking topology after refresh..."
curl -s -X GET "$API_BASE/topology" | jq '{
  controller_count: .result.controllers | length,
  enclosure_count: .result.enclosures | length,
  updated_at: .result.updated_at
}'
echo ""

echo "================================================================================"
echo "TOPOLOGY OPERATIONS TEST COMPLETED"
echo "================================================================================"
