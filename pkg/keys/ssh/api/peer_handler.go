// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/keys/ssh"
)

// authorizePeer handles requests to authorize a peer by adding their public key to authorized_keys
// and optionally to known_hosts if hostname is provided
func (h *SSHKeyHandler) authorizePeer(c *gin.Context) {
	var peerInfo ssh.PeerInfo
	if err := c.ShouldBindJSON(&peerInfo); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	// Validate required fields
	if peerInfo.PeeringID == "" {
		APIError(c, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required"))
		return
	}
	if peerInfo.PublicKey == "" {
		APIError(c, errors.New(errors.SSHKeyPairInvalidPublicKey, "Public key is required"))
		return
	}

	// Authorize the peer (adds to authorized_keys and optionally to known_hosts)
	if err := h.manager.AuthorizePeer(c.Request.Context(), peerInfo); err != nil {
		h.logger.Error("Failed to authorize peer", 
			"error", err, 
			"peering_id", peerInfo.PeeringID)
		APIError(c, err)
		return
	}

	// Return success response with the peer info
	c.JSON(http.StatusOK, gin.H{
		"message": "Peer authorized successfully",
		"peer": peerInfo,
	})
}

// deauthorizePeer handles requests to deauthorize a peer by removing their public key 
// from authorized_keys and known_hosts
func (h *SSHKeyHandler) deauthorizePeer(c *gin.Context) {
	peeringID := c.Param("peering_id")
	if peeringID == "" {
		APIError(c, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required"))
		return
	}

	// Deauthorize the peer (removes from authorized_keys and known_hosts)
	if err := h.manager.DeauthorizePeer(c.Request.Context(), peeringID); err != nil {
		h.logger.Error("Failed to deauthorize peer", 
			"error", err, 
			"peering_id", peeringID)
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Peer deauthorized successfully",
		"peering_id": peeringID,
	})
}

// listAuthorizedPeers handles requests to list all peers authorized to connect to this node
func (h *SSHKeyHandler) listAuthorizedPeers(c *gin.Context) {
	peers, err := h.manager.GetAuthorizedPeers(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list authorized peers", "error", err)
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"peers": peers,
	})
}