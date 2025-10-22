#!/usr/bin/env bash
#
# Disk Management API - Configuration Operations Test
# Tests: config get, update, reload, monitoring config
#

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8042/api/v1/rodent/disks}"

echo "================================================================================"
echo "DISK MANAGEMENT API - CONFIGURATION OPERATIONS TEST"
echo "================================================================================"
echo "API Base: $API_BASE"
echo "Test Time: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo ""

# GET /disks/config - Get full configuration
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/config - Get Full Configuration"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/config"
echo ""
ORIGINAL_CONFIG=$(curl -s -X GET "$API_BASE/config")
echo "$ORIGINAL_CONFIG" | jq '.'
echo ""

# GET /disks/config/monitoring - Get monitoring configuration
echo "--------------------------------------------------------------------------------"
echo "TEST: GET /disks/config/monitoring - Get Monitoring Configuration"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: GET $API_BASE/config/monitoring"
echo ""
ORIGINAL_MONITORING=$(curl -s -X GET "$API_BASE/config/monitoring")
echo "$ORIGINAL_MONITORING" | jq '.'
echo ""

# PUT /disks/config/monitoring - Update monitoring configuration
echo "--------------------------------------------------------------------------------"
echo "TEST: PUT /disks/config/monitoring - Update Monitoring Configuration"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: PUT $API_BASE/config/monitoring"
echo "NOTE: Boldly changing temperature thresholds to test API robustness"
echo "BODY:"
cat <<EOF | jq '.'
{
  "enabled": true,
  "interval": 300000000000,
  "thresholds": {
    "temp_warning": 55,
    "temp_critical": 65,
    "reallocated_sectors_warning": 15,
    "reallocated_sectors_critical": 60,
    "pending_sectors_warning": 8,
    "pending_sectors_critical": 25,
    "power_on_hours_warning": 50000,
    "power_on_hours_critical": 60000,
    "nvme_percent_used_warning": 85,
    "nvme_percent_used_critical": 95,
    "media_errors_warning": 15,
    "media_errors_critical": 60
  },
  "metric_retention": 2592000000000000,
  "alert_on_warning": true,
  "alert_on_critical": true
}
EOF
echo ""
curl -s -X PUT "$API_BASE/config/monitoring" \
  -H "Content-Type: application/json" \
  -d '{"enabled":true,"interval":300000000000,"thresholds":{"temp_warning":55,"temp_critical":65,"reallocated_sectors_warning":15,"reallocated_sectors_critical":60,"pending_sectors_warning":8,"pending_sectors_critical":25,"power_on_hours_warning":50000,"power_on_hours_critical":60000,"nvme_percent_used_warning":85,"nvme_percent_used_critical":95,"media_errors_warning":15,"media_errors_critical":60},"metric_retention":2592000000000000,"alert_on_warning":true,"alert_on_critical":true}' | jq '.'
echo ""

# Verify monitoring config update
echo "Verifying monitoring config update (should show new threshold values)..."
UPDATED_CONFIG=$(curl -s -X GET "$API_BASE/config/monitoring")
echo "$UPDATED_CONFIG" | jq '.'
echo ""

# Verify specific threshold changes
echo "Confirming threshold updates:"
echo "  temp_warning: $(echo "$UPDATED_CONFIG" | jq -r '.result.thresholds.temp_warning') (expected: 55)"
echo "  temp_critical: $(echo "$UPDATED_CONFIG" | jq -r '.result.thresholds.temp_critical') (expected: 65)"
echo "  nvme_percent_used_warning: $(echo "$UPDATED_CONFIG" | jq -r '.result.thresholds.nvme_percent_used_warning') (expected: 85)"
echo ""

# PUT /disks/config - Update full configuration
echo "--------------------------------------------------------------------------------"
echo "TEST: PUT /disks/config - Update Full Configuration"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: PUT $API_BASE/config"
echo "NOTE: Using current configuration to avoid breaking changes"
echo ""

# Extract and re-submit current config
CURRENT_CONFIG=$(echo "$ORIGINAL_CONFIG" | jq '.result')
echo "BODY:"
echo "$CURRENT_CONFIG" | jq '.'
echo ""

curl -s -X PUT "$API_BASE/config" \
  -H "Content-Type: application/json" \
  -d "$CURRENT_CONFIG" | jq '.'
echo ""

# POST /disks/config/reload - Reload configuration
echo "--------------------------------------------------------------------------------"
echo "TEST: POST /disks/config/reload - Reload Configuration"
echo "--------------------------------------------------------------------------------"
echo "REQUEST: POST $API_BASE/config/reload"
echo ""
curl -s -X POST "$API_BASE/config/reload" | jq '.'
echo ""

echo "Waiting 2 seconds for config reload..."
sleep 2
echo ""

# Verify config after reload
echo "Verifying config after reload..."
curl -s -X GET "$API_BASE/config" | jq '{
  discovery_enabled: .result.discovery.enabled,
  monitoring_enabled: .result.monitoring.enabled,
  probing_enabled: .result.probing.enabled,
  topology_enabled: .result.topology.enabled
}'
echo ""

echo "================================================================================"
echo "CONFIGURATION OPERATIONS TEST COMPLETED"
echo "================================================================================"
