#!/usr/bin/env bash
#
# Disk Management API - State Management Test
# Tests: state operations, validation, quarantine, tags, notes
#

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8042/api/v1/rodent/disks}"
TEST_DISK="${TEST_DISK:-nvme4n1}"

echo "================================================================================"
echo "DISK MANAGEMENT API - STATE MANAGEMENT TEST"
echo "================================================================================"
echo "API Base: $API_BASE"
echo "Test Disk: $TEST_DISK"
echo "Test Time: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo ""

# Find the device_id for the test disk
echo "Finding device_id for $TEST_DISK..."
DISK_ID=$(curl -s -X GET "$API_BASE/" | jq -r ".result.disks[] | select(.device_path | contains(\"$TEST_DISK\")) | .device_id" | head -n1)

if [ -z "$DISK_ID" ]; then
    echo "ERROR: Could not find device_id for $TEST_DISK"
    exit 1
fi

echo "Using disk ID: $DISK_ID"
echo ""

# GET /disks/:device_id/state - Get current state
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/:device_id/state - Get Device State"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/$DISK_ID/state"
echo ""
ORIGINAL_STATE=$(curl -s -X GET "$API_BASE/$DISK_ID/state")
echo "$ORIGINAL_STATE" | jq '.'
echo ""

CURRENT_STATE=$(echo "$ORIGINAL_STATE" | jq -r '.result.state')
echo "Current state: $CURRENT_STATE"
echo ""

# PUT /disks/:device_id/state - Set device state
echo "--------------------------------------------------------------------------------"
echo "TEST: PUT /disks/:device_id/state - Set Device State"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: PUT $API_BASE/$DISK_ID/state"
echo "BODY:"
cat <<EOF | jq '.'
{
  "state": "AVAILABLE",
  "reason": "API test - setting to AVAILABLE"
}
EOF
echo ""
curl -s -X PUT "$API_BASE/$DISK_ID/state" \
  -H "Content-Type: application/json" \
  -d '{"state":"AVAILABLE","reason":"API test - setting to AVAILABLE"}' | jq '.'
echo ""

# Verify state change
echo "Verifying state change..."
NEW_STATE=$(curl -s -X GET "$API_BASE/$DISK_ID/state" | jq -r '.result.state')
echo "New state: $NEW_STATE"
echo ""

# PUT /disks/:device_id/tags - Set disk tags
echo "--------------------------------------------------------------------------------"
echo "TEST: PUT /disks/:device_id/tags - Set Disk Tags"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: PUT $API_BASE/$DISK_ID/tags"
echo "BODY:"
cat <<EOF | jq '.'
{
  "tags": {
    "test": "api-test",
    "environment": "development",
    "purpose": "testing"
  }
}
EOF
echo ""
curl -s -X PUT "$API_BASE/$DISK_ID/tags" \
  -H "Content-Type: application/json" \
  -d '{"tags":{"test":"api-test","environment":"development","purpose":"testing"}}' | jq '.'
echo ""

# Verify tags
echo "Verifying tags..."
curl -s -X GET "$API_BASE/$DISK_ID" | jq '.result.tags'
echo ""

# PUT /disks/:device_id/notes - Set disk notes
echo "--------------------------------------------------------------------------------"
echo "TEST: PUT /disks/:device_id/notes - Set Disk Notes"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: PUT $API_BASE/$DISK_ID/notes"
echo "BODY:"
cat <<EOF | jq '.'
{
  "notes": "Test disk for API validation - DO NOT USE IN PRODUCTION"
}
EOF
echo ""
curl -s -X PUT "$API_BASE/$DISK_ID/notes" \
  -H "Content-Type: application/json" \
  -d '{"notes":"Test disk for API validation - DO NOT USE IN PRODUCTION"}' | jq '.'
echo ""

# Verify notes
echo "Verifying notes..."
curl -s -X GET "$API_BASE/$DISK_ID" | jq '.result.notes'
echo ""

# POST /disks/:device_id/validate - Validate disk
echo "--------------------------------------------------------------------------------"
echo "TEST: POST /disks/:device_id/validate - Validate Disk"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: POST $API_BASE/$DISK_ID/validate"
echo ""
curl -s -X POST "$API_BASE/$DISK_ID/validate" | jq '.'
echo ""

# DELETE /disks/:device_id/tags - Delete specific tags
echo "--------------------------------------------------------------------------------"
echo "TEST: DELETE /disks/:device_id/tags - Delete Disk Tags"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: DELETE $API_BASE/$DISK_ID/tags"
echo "BODY:"
cat <<EOF | jq '.'
{
  "tag_keys": ["test", "purpose"]
}
EOF
echo ""
curl -s -X DELETE "$API_BASE/$DISK_ID/tags" \
  -H "Content-Type: application/json" \
  -d '{"tag_keys":["test","purpose"]}' | jq '.'
echo ""

# Verify tag deletion
echo "Verifying tag deletion (should only have 'environment' left)..."
curl -s -X GET "$API_BASE/$DISK_ID" | jq '.result.tags'
echo ""

# POST /disks/:device_id/quarantine - Quarantine disk
echo "--------------------------------------------------------------------------------"
echo "TEST: POST /disks/:device_id/quarantine - Quarantine Disk"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: POST $API_BASE/$DISK_ID/quarantine"
echo "BODY:"
cat <<EOF | jq '.'
{
  "reason": "API test - temporarily quarantining for testing"
}
EOF
echo ""
curl -s -X POST "$API_BASE/$DISK_ID/quarantine" \
  -H "Content-Type: application/json" \
  -d '{"reason":"API test - temporarily quarantining for testing"}' | jq '.'
echo ""

# Verify quarantine state
echo "Verifying quarantine state..."
QUARANTINE_STATE=$(curl -s -X GET "$API_BASE/$DISK_ID/state" | jq -r '.result.state')
echo "State after quarantine: $QUARANTINE_STATE"
echo ""

# Restore original state
echo "Restoring original state ($CURRENT_STATE)..."
curl -s -X PUT "$API_BASE/$DISK_ID/state" \
  -H "Content-Type: application/json" \
  -d "{\"state\":\"$CURRENT_STATE\",\"reason\":\"API test - restoring original state\"}" | jq '.'
echo ""

# Clean up tags
echo "Cleaning up remaining tags..."
curl -s -X DELETE "$API_BASE/$DISK_ID/tags" \
  -H "Content-Type: application/json" \
  -d '{"tag_keys":["environment"]}' | jq '.'
echo ""

# Clean up notes
echo "Cleaning up notes..."
curl -s -X PUT "$API_BASE/$DISK_ID/notes" \
  -H "Content-Type: application/json" \
  -d '{"notes":""}' | jq '.'
echo ""

echo "================================================================================"
echo "STATE MANAGEMENT TEST COMPLETED"
echo "================================================================================"
