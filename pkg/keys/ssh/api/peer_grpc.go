// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/keys/ssh"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

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