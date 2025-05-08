// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizePeer(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	// Create test peer
	peeringID := generateRandomID()
	hostname := "test-" + peeringID + ".example.com"
	publicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH"

	peer := PeerInfo{
		PeeringID:  peeringID,
		Hostname:   hostname,
		PublicKey:  publicKey,
		SSHOptions: []string{"no-port-forwarding"},
	}

	// Authorize the peer
	err := manager.AuthorizePeer(context.Background(), peer)
	require.NoError(t, err)

	// Verify it was added to authorized_keys
	authKeys, err := os.ReadFile(manager.authorizedKeys)
	require.NoError(t, err)
	assert.Contains(t, string(authKeys), publicKey)
	assert.Contains(t, string(authKeys), peeringID)
	assert.Contains(t, string(authKeys), "no-port-forwarding")

	// Verify it was added to known_hosts
	knownHosts, err := os.ReadFile(manager.knownHosts)
	require.NoError(t, err)
	assert.Contains(t, string(knownHosts), publicKey)
	assert.Contains(t, string(knownHosts), hostname)
	assert.Contains(t, string(knownHosts), peeringID)
}

func TestDeauthorizePeer(t *testing.T) {
	manager, tempDir, cleanup := setupTestManager(t)
	defer cleanup()

	// First authorize a peer
	peeringID := generateRandomID()
	hostname := "test-" + peeringID + ".example.com"
	publicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH"

	t.Logf("Test details - peeringID: %s, hostname: %s, tempDir: %s, knownHostsFile: %s",
		peeringID, hostname, tempDir, manager.knownHosts)

	peer := PeerInfo{
		PeeringID:  peeringID,
		Hostname:   hostname,
		PublicKey:  publicKey,
		SSHOptions: []string{"no-port-forwarding"},
	}

	err := manager.AuthorizePeer(context.Background(), peer)
	require.NoError(t, err)

	// Verify initial state - known_hosts has the entry
	knownHostsBefore, err := os.ReadFile(manager.knownHosts)
	require.NoError(t, err)
	t.Logf("Known hosts content after authorize: %s", string(knownHostsBefore))
	assert.Contains(t, string(knownHostsBefore), peeringID)

	// Now deauthorize the peer
	err = manager.DeauthorizePeer(context.Background(), peeringID)
	require.NoError(t, err)

	// Verify it was removed from authorized_keys
	authKeys, err := os.ReadFile(manager.authorizedKeys)
	require.NoError(t, err)
	t.Logf("Authorized keys after deauthorize: %s", string(authKeys))
	assert.NotContains(t, string(authKeys), peeringID)

	// Verify it was removed from known_hosts
	knownHostsAfter, err := os.ReadFile(manager.knownHosts)
	require.NoError(t, err)
	t.Logf("Known hosts content after deauthorize: %s", string(knownHostsAfter))

	// Check if peer ID is in the content
	if strings.Contains(string(knownHostsAfter), peeringID) {
		t.Errorf("Expected known_hosts to not contain %s, but it did", peeringID)

		// Debug the content character by character to detect issues
		content := string(knownHostsAfter)
		t.Logf("Content length: %d", len(content))
		for i, c := range content {
			t.Logf("Char[%d] = %q (byte: %d)", i, c, c)
		}
	} else {
		t.Logf("Peer ID successfully removed from known_hosts")
	}
}

func TestGetAuthorizedPeers(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	// Add some peers
	peeringID1 := generateRandomID()
	peeringID2 := generateRandomID()

	// Peer 1
	peer1 := PeerInfo{
		PeeringID:  peeringID1,
		Hostname:   "host-" + peeringID1 + ".example.com",
		PublicKey:  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH",
		SSHOptions: []string{"no-port-forwarding"},
	}

	// Peer 2
	peer2 := PeerInfo{
		PeeringID: peeringID2,
		Hostname:  "host-" + peeringID2 + ".example.com",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIL/7isI0E662Qi35Qalg4PYWEIlXJBbJ847WGzmWdFBg",
	}

	// Authorize peers
	err := manager.AuthorizePeer(context.Background(), peer1)
	require.NoError(t, err)
	err = manager.AuthorizePeer(context.Background(), peer2)
	require.NoError(t, err)

	// Get authorized peers
	peers, err := manager.GetAuthorizedPeers(context.Background())
	require.NoError(t, err)
	assert.Len(t, peers, 2)

	// Find specific peers in the list
	var foundPeer1, foundPeer2 bool
	for _, peer := range peers {
		if peer.PeeringID == peeringID1 {
			foundPeer1 = true
			assert.Equal(t, peer1.PublicKey, peer.PublicKey)
			assert.Contains(t, peer.SSHOptions, "no-port-forwarding")
		}
		if peer.PeeringID == peeringID2 {
			foundPeer2 = true
			assert.Equal(t, peer2.PublicKey, peer.PublicKey)
		}
	}

	assert.True(t, foundPeer1, "Peer 1 not found")
	assert.True(t, foundPeer2, "Peer 2 not found")
}

func TestPeerAuthorizationEdgeCases(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	// Test empty peering ID
	peer := PeerInfo{
		PeeringID: "",
		Hostname:  "host.example.com",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH",
	}

	err := manager.AuthorizePeer(context.Background(), peer)
	assert.Error(t, err)

	// Test invalid public key
	peer = PeerInfo{
		PeeringID: generateRandomID(),
		Hostname:  "host.example.com",
		PublicKey: "invalid-key",
	}

	err = manager.AuthorizePeer(context.Background(), peer)
	assert.Error(t, err)

	// Test no hostname (should still add to authorized_keys but not known_hosts)
	peeringID := generateRandomID()
	peer = PeerInfo{
		PeeringID: peeringID,
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH",
	}

	err = manager.AuthorizePeer(context.Background(), peer)
	require.NoError(t, err)

	// Verify it was added to authorized_keys but not known_hosts
	authKeys, err := os.ReadFile(manager.authorizedKeys)
	require.NoError(t, err)
	assert.Contains(t, string(authKeys), peeringID)

	knownHosts, err := os.ReadFile(manager.knownHosts)
	require.NoError(t, err)
	assert.NotContains(t, string(knownHosts), peeringID)
}
