// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/keys/ssh"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterSSHKeyGRPCHandlers registers all SSH key command handlers with Toggle
func RegisterSSHKeyGRPCHandlers(handler *SSHKeyHandler) {
	// Key pair operations
	client.RegisterCommandHandler(proto.CmdSSHKeyPairGenerate, handleGenerateKeyPair(handler))
	client.RegisterCommandHandler(proto.CmdSSHKeyPairGet, handleGetKeyPair(handler))
	client.RegisterCommandHandler(proto.CmdSSHKeyPairList, handleListKeyPairs(handler))
	client.RegisterCommandHandler(proto.CmdSSHKeyPairRemove, handleRemoveKeyPair(handler))

	// Peer authorization operations
	client.RegisterCommandHandler(proto.CmdSSHPeerAuthorize, handleAuthorizePeer(handler))
	client.RegisterCommandHandler(proto.CmdSSHPeerDeauthorize, handleDeauthorizePeer(handler))
	client.RegisterCommandHandler(proto.CmdSSHPeerList, handleListAuthorizedPeers(handler))
}

// Helper function to parse JSON payload
func parseJSONPayload(cmd *proto.CommandRequest, out interface{}) error {
	if len(cmd.Payload) == 0 {
		return errors.New(errors.ServerRequestValidation, "empty payload")
	}
	return json.Unmarshal(cmd.Payload, out)
}

// Helper function to create a success response
func successResponse(
	requestID string,
	message string,
	data interface{},
) (*proto.CommandResponse, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, errors.Wrap(err, errors.ServerResponseError)
	}

	return &proto.CommandResponse{
		RequestId: requestID,
		Success:   true,
		Message:   message,
		Payload:   payload,
	}, nil
}

// handleGenerateKeyPair handles requests to generate a new SSH key pair
func handleGenerateKeyPair(handler *SSHKeyHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload ssh.GenerateKeyPairRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate input
		if payload.PeeringID == "" {
			return nil, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required")
		}

		// Generate key pair
		keyPair, err := handler.manager.GenerateKeyPair(
			context.Background(),
			payload.PeeringID,
			payload.Type,
		)
		if err != nil {
			return nil, err
		}

		// Create response
		resp := ssh.GenerateKeyPairResponse{
			PeeringID:      keyPair.PeeringID,
			PublicKey:      keyPair.PublicKey,
			PrivateKeyPath: keyPair.PrivateKeyPath,
			PublicKeyPath:  keyPair.PublicKeyPath,
			Type:           keyPair.Type,
		}

		return successResponse(req.RequestId, "SSH key pair generated successfully", resp)
	}
}

// handleGetKeyPair handles requests to get an existing SSH key pair
func handleGetKeyPair(handler *SSHKeyHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			PeeringID string `json:"peering_id"`
		}
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate input
		if payload.PeeringID == "" {
			return nil, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required")
		}

		// Get key pair
		keyPair, err := handler.manager.GetKeyPair(payload.PeeringID)
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "SSH key pair retrieved successfully", keyPair)
	}
}

// handleListKeyPairs handles requests to list all SSH key pairs
func handleListKeyPairs(handler *SSHKeyHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// List key pairs
		keyPairs, err := handler.manager.ListKeyPairs(context.Background())
		if err != nil {
			return nil, err
		}

		resp := ssh.KeyPairListResponse{
			KeyPairs: keyPairs,
		}

		return successResponse(req.RequestId, "SSH key pairs listed successfully", resp)
	}
}

// handleRemoveKeyPair handles requests to remove an SSH key pair
func handleRemoveKeyPair(handler *SSHKeyHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			PeeringID string `json:"peering_id"`
		}
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate input
		if payload.PeeringID == "" {
			return nil, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required")
		}

		// Remove key pair
		err := handler.manager.RemoveKeyPair(context.Background(), payload.PeeringID)
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "SSH key pair removed successfully", nil)
	}
}
