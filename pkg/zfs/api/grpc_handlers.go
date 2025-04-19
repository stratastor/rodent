// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterZFSGRPCHandlers registers all ZFS-related command handlers with Toggle
func RegisterZFSGRPCHandlers() {
	// Register handler for getting ZFS pool status
	client.RegisterCommandHandler("zfs.pool.status", handlePoolStatus)

	// Register handler for getting ZFS dataset information
	client.RegisterCommandHandler("zfs.dataset.info", handleDatasetInfo)

	// Add more handlers as needed
}

// handlePoolStatus returns the status of ZFS pools
func handlePoolStatus(
	req *proto.ToggleRequest,
	cmd *proto.CommandRequest,
) (*proto.CommandResponse, error) {
	// Example implementation - in a real setup this would call the actual ZFS manager
	poolsInfo := map[string]interface{}{
		"pools": []map[string]interface{}{
			{
				"name":   "tank",
				"status": "ONLINE",
				"size":   "1TB",
				"used":   "200GB",
			},
		},
		"timestamp": 1612345678,
	}

	// Marshal pool info to JSON
	payload, err := json.Marshal(poolsInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pool info: %w", err)
	}

	return &proto.CommandResponse{
		RequestId: req.RequestId, // Preserve the request ID for correlation
		Success:   true,
		Message:   "ZFS pools status",
		Payload:   payload,
	}, nil
}

// handleDatasetInfo returns information about ZFS datasets
func handleDatasetInfo(
	req *proto.ToggleRequest,
	cmd *proto.CommandRequest,
) (*proto.CommandResponse, error) {
	// Parse the dataset name from the request payload if provided
	var datasetName string
	if len(cmd.Payload) > 0 {
		var params map[string]string
		if err := json.Unmarshal(cmd.Payload, &params); err == nil {
			datasetName = params["name"]
		}
	}

	// Default to listing all datasets if no specific name provided
	if datasetName == "" {
		datasetsInfo := map[string]interface{}{
			"datasets": []map[string]interface{}{
				{
					"name":       "tank/home",
					"mountpoint": "/home",
					"used":       "100GB",
					"available":  "400GB",
				},
				{
					"name":       "tank/backup",
					"mountpoint": "/backup",
					"used":       "50GB",
					"available":  "450GB",
				},
			},
		}

		payload, err := json.Marshal(datasetsInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal datasets info: %w", err)
		}

		return &proto.CommandResponse{
			RequestId: req.RequestId,
			Success:   true,
			Message:   "ZFS datasets information",
			Payload:   payload,
		}, nil
	}

	// Example for a specific dataset
	datasetInfo := map[string]interface{}{
		"name":       datasetName,
		"mountpoint": fmt.Sprintf("/%s", datasetName),
		"used":       "50GB",
		"available":  "450GB",
		"properties": map[string]string{
			"compression": "lz4",
			"recordsize":  "128K",
		},
	}

	payload, err := json.Marshal(datasetInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dataset info: %w", err)
	}

	return &proto.CommandResponse{
		RequestId: req.RequestId,
		Success:   true,
		Message:   fmt.Sprintf("Information for dataset %s", datasetName),
		Payload:   payload,
	}, nil
}
