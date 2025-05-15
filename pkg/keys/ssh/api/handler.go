// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/keys/ssh"
)

var APIError = common.APIError

// SSHKeyHandler handles HTTP requests for SSH key management
type SSHKeyHandler struct {
	manager *ssh.SSHKeyManager
	logger  logger.Logger
}

// Close cleans up resources
func (h *SSHKeyHandler) Close() {
	if h.manager != nil {
		h.manager.Close()
	}
}

// NewSSHKeyHandler creates a new SSH key manager handler
func NewSSHKeyHandler(logger logger.Logger) (*SSHKeyHandler, error) {
	manager, err := ssh.NewSSHKeyManager(logger)
	if err != nil {
		return nil, err
	}

	return &SSHKeyHandler{
		manager: manager,
		logger:  logger,
	}, nil
}

// RegisterRoutes registers SSH key management API routes
func (h *SSHKeyHandler) RegisterRoutes(router *gin.RouterGroup) {
	keyGroup := router.Group("")

	// Key pair operations
	keyGroup.POST("/keypair", h.generateKeyPair)
	keyGroup.GET("/keypair/:peering_id", h.getKeyPair)
	keyGroup.GET("/keypair", h.listKeyPairs)
	keyGroup.DELETE("/keypair/:peering_id", h.removeKeyPair)

	// Peer authorization operations
	keyGroup.POST("/peer", h.authorizePeer)
	keyGroup.DELETE("/peer/:peering_id", h.deauthorizePeer)
	keyGroup.GET("/peer", h.listAuthorizedPeers)
}

// generateKeyPair handles requests to generate a new SSH key pair
func (h *SSHKeyHandler) generateKeyPair(c *gin.Context) {
	var req ssh.GenerateKeyPairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	// Validate input
	if req.PeeringID == "" {
		APIError(c, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required"))
		return
	}

	// Generate key pair
	keyPair, err := h.manager.GenerateKeyPair(c.Request.Context(), req.PeeringID, req.Type)
	if err != nil {
		h.logger.Error("Failed to generate key pair", "error", err, "peering_id", req.PeeringID)
		APIError(c, err)
		return
	}

	// Create response
	resp := ssh.GenerateKeyPairResponse{
		PeeringID:      keyPair.PeeringID,
		PublicKey:      keyPair.PublicKey,
		PrivateKeyPath: keyPair.PrivateKeyPath,
		PublicKeyPath:  keyPair.PublicKeyPath,
		Type:           keyPair.Type,
	}

	c.JSON(http.StatusOK, resp)
}

// getKeyPair handles requests to get an existing SSH key pair
func (h *SSHKeyHandler) getKeyPair(c *gin.Context) {
	peeringID := c.Param("peering_id")
	if peeringID == "" {
		APIError(c, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required"))
		return
	}

	// Get key pair
	keyPair, err := h.manager.GetKeyPair(peeringID)
	if err != nil {
		h.logger.Error("Failed to get key pair", "error", err, "peering_id", peeringID)
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, keyPair)
}

// listKeyPairs handles requests to list all SSH key pairs
func (h *SSHKeyHandler) listKeyPairs(c *gin.Context) {
	keyPairs, err := h.manager.ListKeyPairs(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list key pairs", "error", err)
		APIError(c, err)
		return
	}

	resp := ssh.KeyPairListResponse{
		KeyPairs: keyPairs,
	}

	c.JSON(http.StatusOK, resp)
}

// removeKeyPair handles requests to remove an SSH key pair
func (h *SSHKeyHandler) removeKeyPair(c *gin.Context) {
	peeringID := c.Param("peering_id")
	if peeringID == "" {
		APIError(c, errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID is required"))
		return
	}

	// Remove key pair
	err := h.manager.RemoveKeyPair(c.Request.Context(), peeringID)
	if err != nil {
		h.logger.Error("Failed to remove key pair", "error", err, "peering_id", peeringID)
		APIError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

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
		"peer":    peerInfo,
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
		"message":    "Peer deauthorized successfully",
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
