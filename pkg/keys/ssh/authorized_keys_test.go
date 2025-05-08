// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizedKeysEntryString(t *testing.T) {
	testCases := []struct {
		name     string
		entry    AuthorizedKeysEntry
		expected string
	}{
		{
			name: "PublicKeyOnly",
			entry: AuthorizedKeysEntry{
				PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH",
			},
			expected: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH",
		},
		{
			name: "WithComment",
			entry: AuthorizedKeysEntry{
				PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH",
				Comment:   "peer-123",
			},
			expected: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH peer-123",
		},
		{
			name: "WithOptions",
			entry: AuthorizedKeysEntry{
				PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH",
				Options:   []string{"no-port-forwarding", "no-X11-forwarding"},
			},
			expected: "no-port-forwarding,no-X11-forwarding ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH",
		},
		{
			name: "WithOptionsAndComment",
			entry: AuthorizedKeysEntry{
				PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH",
				Comment:   "peer-123",
				Options:   []string{"no-port-forwarding", "no-X11-forwarding"},
			},
			expected: "no-port-forwarding,no-X11-forwarding ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH peer-123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.entry.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestAddAndRemoveAuthorizedKey(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	peeringID := generateRandomID()
	publicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH"
	options := []string{"no-port-forwarding", "no-X11-forwarding"}

	// Add the key
	err := manager.AddAuthorizedKey(publicKey, peeringID, options)
	require.NoError(t, err)

	// Verify it was added
	content, err := os.ReadFile(manager.authorizedKeys)
	require.NoError(t, err)
	assert.Contains(t, string(content), publicKey)
	assert.Contains(t, string(content), peeringID)
	assert.Contains(t, string(content), "no-port-forwarding,no-X11-forwarding")

	// Try to add again - should error
	err = manager.AddAuthorizedKey(publicKey, peeringID, options)
	assert.Error(t, err)

	// Now remove the key
	err = manager.RemoveAuthorizedKey(peeringID)
	require.NoError(t, err)

	// Verify it was removed
	content, err = os.ReadFile(manager.authorizedKeys)
	require.NoError(t, err)
	assert.NotContains(t, string(content), peeringID)
}

func TestListAuthorizedKeys(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	// Add some keys
	peeringID1 := generateRandomID()
	peeringID2 := generateRandomID()
	publicKey1 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH"
	publicKey2 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIL/7isI0E662Qi35Qalg4PYWEIlXJBbJ847WGzmWdFBg"

	// Add keys
	err := manager.AddAuthorizedKey(publicKey1, peeringID1, nil)
	require.NoError(t, err)
	err = manager.AddAuthorizedKey(publicKey2, peeringID2, []string{"no-port-forwarding"})
	require.NoError(t, err)

	// List keys
	entries, err := manager.ListAuthorizedKeys()
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	// Find by peering ID
	entry, err := manager.FindAuthorizedKeyByPeeringID(peeringID1)
	require.NoError(t, err)
	assert.Equal(t, publicKey1, entry.PublicKey)
	assert.Equal(t, peeringID1, entry.Comment)

	entry, err = manager.FindAuthorizedKeyByPeeringID(peeringID2)
	require.NoError(t, err)
	assert.Equal(t, publicKey2, entry.PublicKey)
	assert.Equal(t, peeringID2, entry.Comment)
	assert.Contains(t, entry.Options, "no-port-forwarding")
}

func TestParseAuthorizedKeysContent(t *testing.T) {
	content := `# Comment line
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH peer1
no-port-forwarding,no-X11-forwarding ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIL/7isI0E662Qi35Qalg4PYWEIlXJBbJ847WGzmWdFBg peer2
from="192.168.1.1" ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC3k peer3
`

	entries := parseAuthorizedKeysContent(content)
	assert.Len(t, entries, 3)

	// Check first entry
	assert.Equal(
		t,
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKw3fGn15qQeXMUG/fMCGwvJ/QzZ9tsAEXkJD4x2V2JH",
		entries[0].PublicKey,
	)
	assert.Equal(t, "peer1", entries[0].Comment)
	assert.Empty(t, entries[0].Options)

	// Check second entry
	assert.Equal(
		t,
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIL/7isI0E662Qi35Qalg4PYWEIlXJBbJ847WGzmWdFBg",
		entries[1].PublicKey,
	)
	assert.Equal(t, "peer2", entries[1].Comment)
	assert.Len(t, entries[1].Options, 2)
	assert.Contains(t, entries[1].Options, "no-port-forwarding")
	assert.Contains(t, entries[1].Options, "no-X11-forwarding")

	// Check third entry
	assert.True(
		t,
		strings.HasPrefix(entries[2].PublicKey, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC3k"),
	)
	assert.Equal(t, "peer3", entries[2].Comment)
	assert.Len(t, entries[2].Options, 1)
	assert.Equal(t, `from="192.168.1.1"`, entries[2].Options[0])
}
