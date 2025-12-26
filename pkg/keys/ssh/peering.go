// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"fmt"

	"github.com/stratastor/rodent/pkg/errors"
)

// AuthorizePeer adds a peer's public key to the authorized_keys file.
// This enables the peer to SSH into this node for ZFS transfers.
//
// NOTE: This function only manages authorized_keys (allowing incoming SSH connections).
// The known_hosts file (for outgoing SSH connections) is managed separately via
// AddRemoteHostKey, which requires the remote host's SSH server key, not the
// peer's authentication key.
func (m *SSHKeyManager) AuthorizePeer(ctx context.Context, peer PeerInfo) error {
	// Validate inputs
	if err := validatePeeringID(peer.PeeringID); err != nil {
		return err
	}

	if !isValidSSHPublicKey(peer.PublicKey) {
		return errors.New(errors.SSHKeyPairInvalidPublicKey, "Invalid public key format")
	}

	// Add to authorized_keys - this allows the peer to SSH into this machine
	if err := m.AddAuthorizedKey(peer.PublicKey, peer.PeeringID, peer.SSHOptions); err != nil {
		return err
	}

	m.logger.Debug("Peer authorized successfully",
		"peering_id", peer.PeeringID,
		"hostname", peer.Hostname)

	return nil
}

// DeauthorizePeer removes a peer's public key from the authorized_keys file
// and also attempts to remove it from the known_hosts file
// This prevents the peer from SSHing into this node and removes trust
func (m *SSHKeyManager) DeauthorizePeer(ctx context.Context, peeringID string) error {
	// Validate peering ID
	if err := validatePeeringID(peeringID); err != nil {
		return err
	}

	// Remove from authorized_keys (primary operation)
	if err := m.RemoveAuthorizedKey(peeringID); err != nil {
		return err
	}

	// Also try to remove from known_hosts, but don't fail if this doesn't work
	if err := m.RemoveKnownHost(ctx, peeringID, ""); err != nil {
		m.logger.Warn("Failed to remove peer from known_hosts after removing from authorized_keys",
			"peering_id", peeringID,
			"error", err)
	}

	return nil
}

// GetAuthorizedPeers returns a list of all peers authorized to connect to this node
func (m *SSHKeyManager) GetAuthorizedPeers(ctx context.Context) ([]PeerInfo, error) {
	// Get all authorized keys
	authKeys, err := m.ListAuthorizedKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to list authorized keys: %w", err)
	}

	// Convert to peer info
	var peers []PeerInfo
	for _, auth := range authKeys {
		if auth.Comment != "" {
			peers = append(peers, PeerInfo{
				PeeringID:  auth.Comment,
				PublicKey:  auth.PublicKey,
				SSHOptions: auth.Options,
			})
		}
	}

	return peers, nil
}
