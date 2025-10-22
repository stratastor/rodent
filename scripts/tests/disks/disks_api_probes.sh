#!/usr/bin/env bash
#
# Disk Management API - Probe Operations Test
# Tests: probe start, get, cancel, list, history, schedules
#

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8042/api/v1/rodent/disks}"
TEST_DISK="${TEST_DISK:-nvme4n1}"

echo "================================================================================"
echo "DISK MANAGEMENT API - PROBE OPERATIONS TEST"
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

# GET /disks/probes - List active probes (before starting)
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/probes - List Active Probes (Before)"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/probes"
echo ""
curl -s -X GET "$API_BASE/probes" | jq '.'
echo ""

# POST /disks/probes/start - Start a quick probe
echo "--------------------------------------------------------------------------------"
echo "TEST: POST /disks/probes/start - Start Quick Probe"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: POST $API_BASE/probes/start"
echo "BODY:"
cat <<EOF | jq '.'
{
  "device_id": "$DISK_ID",
  "probe_type": "quick"
}
EOF
echo ""
PROBE_RESPONSE=$(curl -s -X POST "$API_BASE/probes/start" \
  -H "Content-Type: application/json" \
  -d "{\"device_id\":\"$DISK_ID\",\"probe_type\":\"quick\"}")
echo "$PROBE_RESPONSE" | jq '.'
echo ""

PROBE_ID=$(echo "$PROBE_RESPONSE" | jq -r '.result.probe_id // empty')

if [ -z "$PROBE_ID" ]; then
    echo "WARNING: Probe start may have failed or returned unexpected format"
    echo "Response: $PROBE_RESPONSE"
    # Continue anyway to test other endpoints
else
    echo "Probe ID: $PROBE_ID"
    echo ""

    # Wait a moment for probe to start
    sleep 2

    # GET /disks/probes/:probe_id - Get probe status
    echo "--------------------------------------------------------------------------------"
    echo "TEST: GET /disks/probes/:probe_id - Get Probe Status"
    echo "--------------------------------------------------------------------------------"
    echo "REQUEST: GET $API_BASE/probes/$PROBE_ID"
    echo ""
    curl -s -X GET "$API_BASE/probes/$PROBE_ID" | jq '.'
    echo ""

    # GET /disks/probes - List active probes (after starting)
    echo "--------------------------------------------------------------------------------"
    echo "TEST: GET /disks/probes - List Active Probes (After Start)"
    echo "--------------------------------------------------------------------------------"
    echo "REQUEST: GET $API_BASE/probes"
    echo ""
    curl -s -X GET "$API_BASE/probes" | jq '.'
    echo ""

    # POST /disks/probes/:probe_id/cancel - Cancel probe
    echo "--------------------------------------------------------------------------------"
    echo "TEST: POST /disks/probes/:probe_id/cancel - Cancel Probe"
    echo "--------------------------------------------------------------------------------"
    echo "REQUEST: POST $API_BASE/probes/$PROBE_ID/cancel"
    echo ""
    curl -s -X POST "$API_BASE/probes/$PROBE_ID/cancel" | jq '.'
    echo ""

    # Verify cancellation
    echo "Verifying probe cancellation..."
    curl -s -X GET "$API_BASE/probes/$PROBE_ID" | jq '.result.status'
    echo ""
fi

# GET /disks/:device_id/probes/history - Get probe history
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/:device_id/probes/history - Get Probe History"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/$DISK_ID/probes/history"
echo ""
curl -s -X GET "$API_BASE/$DISK_ID/probes/history" | jq '.'
echo ""

# GET /disks/:device_id/probes/history?limit=5 - Get limited probe history
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/:device_id/probes/history?limit=5 - Get Limited Probe History"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/$DISK_ID/probes/history?limit=5"
echo ""
curl -s -X GET "$API_BASE/$DISK_ID/probes/history?limit=5" | jq '.'
echo ""

# GET /disks/probes/schedules - List probe schedules
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/probes/schedules - List Probe Schedules"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/probes/schedules"
echo ""
SCHEDULES=$(curl -s -X GET "$API_BASE/probes/schedules")
echo "$SCHEDULES" | jq '.'
echo ""

# POST /disks/probes/schedules - Create probe schedule
echo "--------------------------------------------------------------------------------"
echo "TEST: POST /disks/probes/schedules - Create Probe Schedule"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: POST $API_BASE/probes/schedules"
echo "BODY:"
cat <<EOF | jq '.'
{
  "enabled": false,
  "type": "quick",
  "cron_expression": "0 4 * * *",
  "max_concurrent": 2,
  "timeout": 600,
  "retry_on_conflict": true,
  "retry_delay": 300,
  "max_retries": 3
}
EOF
echo ""
SCHEDULE_RESPONSE=$(curl -s -X POST "$API_BASE/probes/schedules" \
  -H "Content-Type: application/json" \
  -d '{"enabled":false,"type":"quick","cron_expression":"0 4 * * *","max_concurrent":2,"timeout":600,"retry_on_conflict":true,"retry_delay":300,"max_retries":3}')
echo "$SCHEDULE_RESPONSE" | jq '.'
echo ""

SCHEDULE_ID=$(echo "$SCHEDULE_RESPONSE" | jq -r '.result.id // empty')

if [ -n "$SCHEDULE_ID" ]; then
    echo "Schedule ID: $SCHEDULE_ID"
    echo ""

    # GET /disks/probes/schedules/:schedule_id - Get specific schedule
    echo "--------------------------------------------------------------------------------"
    echo "TEST: GET /disks/probes/schedules/:schedule_id - Get Specific Schedule"
    echo "--------------------------------------------------------------------------------"
    echo "REQUEST: GET $API_BASE/probes/schedules/$SCHEDULE_ID"
    echo ""
    curl -s -X GET "$API_BASE/probes/schedules/$SCHEDULE_ID" | jq '.'
    echo ""

    # PUT /disks/probes/schedules/:schedule_id - Update schedule
    echo "--------------------------------------------------------------------------------"
    echo "TEST: PUT /disks/probes/schedules/:schedule_id - Update Schedule"
    echo "--------------------------------------------------------------------------------"
    echo "REQUEST: PUT $API_BASE/probes/schedules/$SCHEDULE_ID"
    echo "BODY:"
    cat <<EOF | jq '.'
{
  "enabled": false,
  "type": "quick",
  "cron_expression": "0 5 * * *",
  "max_concurrent": 3,
  "timeout": 600,
  "retry_on_conflict": true,
  "retry_delay": 300,
  "max_retries": 3
}
EOF
    echo ""
    curl -s -X PUT "$API_BASE/probes/schedules/$SCHEDULE_ID" \
      -H "Content-Type: application/json" \
      -d '{"enabled":false,"type":"quick","cron_expression":"0 5 * * *","max_concurrent":3,"timeout":600,"retry_on_conflict":true,"retry_delay":300,"max_retries":3}' | jq '.'
    echo ""

    # POST /disks/probes/schedules/:schedule_id/enable - Enable schedule
    echo "--------------------------------------------------------------------------------"
    echo "TEST: POST /disks/probes/schedules/:schedule_id/enable - Enable Schedule"
    echo "--------------------------------------------------------------------------------"
    echo "REQUEST: POST $API_BASE/probes/schedules/$SCHEDULE_ID/enable"
    echo ""
    curl -s -X POST "$API_BASE/probes/schedules/$SCHEDULE_ID/enable" | jq '.'
    echo ""

    # Verify enabled
    echo "Verifying schedule enabled..."
    curl -s -X GET "$API_BASE/probes/schedules/$SCHEDULE_ID" | jq '.result.enabled'
    echo ""

    # POST /disks/probes/schedules/:schedule_id/disable - Disable schedule
    echo "--------------------------------------------------------------------------------"
    echo "TEST: POST /disks/probes/schedules/:schedule_id/disable - Disable Schedule"
    echo "--------------------------------------------------------------------------------"
    echo "REQUEST: POST $API_BASE/probes/schedules/$SCHEDULE_ID/disable"
    echo ""
    curl -s -X POST "$API_BASE/probes/schedules/$SCHEDULE_ID/disable" | jq '.'
    echo ""

    # DELETE /disks/probes/schedules/:schedule_id - Delete schedule
    echo "--------------------------------------------------------------------------------"
    echo "TEST: DELETE /disks/probes/schedules/:schedule_id - Delete Schedule"
    echo "--------------------------------------------------------------------------------"
    echo "REQUEST: DELETE $API_BASE/probes/schedules/$SCHEDULE_ID"
    echo ""
    curl -s -X DELETE "$API_BASE/probes/schedules/$SCHEDULE_ID" | jq '.'
    echo ""

    # Verify deletion
    echo "Verifying schedule deletion..."
    DELETE_CHECK=$(curl -s -X GET "$API_BASE/probes/schedules/$SCHEDULE_ID")
    echo "$DELETE_CHECK" | jq '.'
    echo ""
else
    echo "WARNING: Could not create schedule, skipping schedule-specific tests"
fi

echo "================================================================================"
echo "PROBE OPERATIONS TEST COMPLETED"
echo "================================================================================"
