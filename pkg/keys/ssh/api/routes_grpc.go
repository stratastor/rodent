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

	// Peer authorization operations (manages authorized_keys on destination)
	client.RegisterCommandHandler(proto.CmdSSHPeerAuthorize, handleAuthorizePeer(handler))
	client.RegisterCommandHandler(proto.CmdSSHPeerDeauthorize, handleDeauthorizePeer(handler))
	client.RegisterCommandHandler(proto.CmdSSHPeerList, handleListAuthorizedPeers(handler))

	// Host key operations (manages known_hosts on source)
	client.RegisterCommandHandler(proto.CmdSSHHostKeyGet, handleGetHostKey(handler))
	client.RegisterCommandHandler(proto.CmdSSHKnownHostAdd, handleAddKnownHost(handler))
	client.RegisterCommandHandler(proto.CmdSSHKnownHostRemove, handleRemoveKnownHost(handler))
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

// handleAuthorizePeer handles requests to authorize a peer
func handleAuthorizePeer(handler *SSHKeyHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var peerInfo ssh.PeerInfo
		if err := parseJSONPayload(cmd, &peerInfo); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if peerInfo.PeeringID == "" {
			return nil, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required")
		}
		if peerInfo.PublicKey == "" {
			return nil, errors.New(errors.SSHKeyPairInvalidPublicKey, "Public key is required")
		}

		// Authorize the peer (adds to authorized_keys and optionally to known_hosts)
		if err := handler.manager.AuthorizePeer(context.Background(), peerInfo); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"message": "Peer authorized successfully",
			"peer":    peerInfo,
		}

		return successResponse(req.RequestId, "Peer authorized successfully", response)
	}
}

// handleDeauthorizePeer handles requests to deauthorize a peer
func handleDeauthorizePeer(handler *SSHKeyHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			PeeringID string `json:"peering_id"`
		}
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate peering ID
		if payload.PeeringID == "" {
			return nil, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required")
		}

		// Deauthorize the peer (removes from authorized_keys and known_hosts)
		if err := handler.manager.DeauthorizePeer(context.Background(), payload.PeeringID); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"message":    "Peer deauthorized successfully",
			"peering_id": payload.PeeringID,
		}

		return successResponse(req.RequestId, "Peer deauthorized successfully", response)
	}
}

// handleListAuthorizedPeers handles requests to list all authorized peers
func handleListAuthorizedPeers(handler *SSHKeyHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// List authorized peers
		peers, err := handler.manager.GetAuthorizedPeers(context.Background())
		if err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"peers": peers,
		}

		return successResponse(req.RequestId, "Listed authorized peers successfully", response)
	}
}

// handleGetHostKey handles requests to get this machine's SSH host public key
func handleGetHostKey(handler *SSHKeyHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Get the host public key
		hostKeyResp, err := handler.manager.GetHostPublicKey()
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "Host key retrieved successfully", hostKeyResp)
	}
}

// handleAddKnownHost handles requests to add a remote host's key to known_hosts
func handleAddKnownHost(handler *SSHKeyHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload ssh.AddKnownHostRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if payload.PeeringID == "" {
			return nil, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required")
		}
		if payload.Hostname == "" {
			return nil, errors.New(errors.SSHKeyPairInvalidHostname, "Hostname is required")
		}
		if payload.HostKey == "" {
			return nil, errors.New(errors.SSHKeyPairInvalidPublicKey, "Host key is required")
		}

		// Add the remote host's key to known_hosts
		if err := handler.manager.AddRemoteHostKey(
			context.Background(),
			payload.Hostname,
			payload.HostKey,
			payload.PeeringID,
		); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"message":    "Known host added successfully",
			"peering_id": payload.PeeringID,
			"hostname":   payload.Hostname,
		}

		return successResponse(req.RequestId, "Known host added successfully", response)
	}
}

// handleRemoveKnownHost handles requests to remove a host entry from known_hosts
func handleRemoveKnownHost(handler *SSHKeyHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload ssh.RemoveKnownHostRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate peering ID
		if payload.PeeringID == "" {
			return nil, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required")
		}

		// Remove the host entry from known_hosts
		if err := handler.manager.RemoveKnownHost(
			context.Background(),
			payload.PeeringID,
			payload.Hostname,
		); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"message":    "Known host removed successfully",
			"peering_id": payload.PeeringID,
		}

		return successResponse(req.RequestId, "Known host removed successfully", response)
	}
}
