// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	// "google.golang.org/grpc/codes"
	// "google.golang.org/grpc/status"
)

// GRPCHandler handles gRPC requests for auto-snapshot operations
type GRPCHandler struct {
	manager *Manager
}

// NewGRPCHandler creates a new snapshot gRPC handler
func NewGRPCHandler(dsManager *dataset.Manager) (*GRPCHandler, error) {
	manager, err := NewManager(dsManager, "")
	if err != nil {
		return nil, err
	}

	return &GRPCHandler{
		manager: manager,
	}, nil
}

// StartManager starts the snapshot manager scheduler
func (h *GRPCHandler) StartManager() error {
	return h.manager.Start()
}

// StopManager stops the snapshot manager scheduler
func (h *GRPCHandler) StopManager() error {
	return h.manager.Stop()
}

// TODO: Implement gRPC service methods for snapshot management
// This would typically include methods like:
// - ListPolicies
// - GetPolicy
// - CreatePolicy
// - UpdatePolicy
// - DeletePolicy
// - RunPolicy

// These methods will be implemented once the gRPC protobuf definitions are available.
// The implementation will follow a similar pattern to the REST API handlers,
// but adapted for gRPC:
//
// For example:
//
// func (h *GRPCHandler) CreatePolicy(ctx context.Context, req *pb.CreatePolicyRequest) (*pb.Policy, error) {
//     params := EditPolicyParams{
//         Name: req.Name,
//         Description: req.Description,
//         Dataset: req.Dataset,
//         // ... map other fields from the request
//     }
//
//     policyID, err := h.manager.AddPolicy(params)
//     if err != nil {
//         // Convert error to gRPC status error
//         return nil, status.Errorf(codes.Internal, "Failed to create policy: %v", err)
//     }
//
//     // Get the created policy to return
//     policy, err := h.manager.GetPolicy(policyID)
//     if err != nil {
//         return nil, status.Errorf(codes.Internal, "Failed to retrieve created policy: %v", err)
//     }
//
//     // Convert policy to protobuf response
//     response := &pb.Policy{
//         Id: policy.ID,
//         Name: policy.Name,
//         // ... map other fields to the response
//     }
//
//     return response, nil
// }
