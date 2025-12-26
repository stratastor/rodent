// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package ssh

// KeyPairType represents the type of SSH key algorithm
type KeyPairType string

const (
	// KeyPairTypeED25519 represents an ED25519 key pair
	KeyPairTypeED25519 KeyPairType = "ed25519"
	// KeyPairTypeRSA represents an RSA key pair
	KeyPairTypeRSA KeyPairType = "rsa"
)

// KeyPair represents an SSH key pair
type KeyPair struct {
	// PeeringID identifies which Rodent peer this key pair belongs to
	PeeringID string `json:"peering_id"`
	// PublicKey is the public key in authorized_keys format
	PublicKey string `json:"public_key"`
	// PrivateKeyPath is the path to the private key file
	PrivateKeyPath string `json:"private_key_path"`
	// PublicKeyPath is the path to the public key file
	PublicKeyPath string `json:"public_key_path"`
	// Type is the key algorithm type
	Type KeyPairType `json:"type"`
}

// KnownHostEntry represents an entry in the known_hosts file
type KnownHostEntry struct {
	// Hostname is the hostname or IP address of the host
	Hostname string `json:"hostname"`
	// PublicKey is the public key in known_hosts format
	PublicKey string `json:"public_key"`
	// PeeringID identifies which Rodent peer this key belongs to
	PeeringID string `json:"peering_id"`
}

// GenerateKeyPairRequest represents a request to generate a new key pair
type GenerateKeyPairRequest struct {
	// PeeringID is a unique identifier for the peer
	PeeringID string `json:"peering_id"     binding:"required"`
	// Type is the key algorithm type (defaults to ed25519)
	Type KeyPairType `json:"type,omitempty"`
}

// GenerateKeyPairResponse represents the response to a key pair generation request
type GenerateKeyPairResponse struct {
	// PeeringID is the identifier for the peer
	PeeringID string `json:"peering_id"`
	// PublicKey is the generated public key in authorized_keys format
	PublicKey string `json:"public_key"`
	// PrivateKeyPath is the path to the private key file
	PrivateKeyPath string `json:"private_key_path"`
	// PublicKeyPath is the path to the public key file
	PublicKeyPath string `json:"public_key_path"`
	// Type is the key algorithm type
	Type KeyPairType `json:"type"`
}

// KeyPairListResponse represents a response listing all key pairs
type KeyPairListResponse struct {
	// KeyPairs is the list of key pairs
	KeyPairs []KeyPair `json:"key_pairs"`
}

// KnownHostListResponse represents a response listing all known hosts
type KnownHostListResponse struct {
	// KnownHosts is the list of known hosts entries
	KnownHosts []KnownHostEntry `json:"known_hosts"`
}

// AuthorizedKeysEntry represents an entry in the authorized_keys file
type AuthorizedKeysEntry struct {
	// PublicKey is the SSH public key in authorized_keys format
	PublicKey string `json:"public_key"`
	// Comment is an optional comment (often used to identify the key)
	Comment string `json:"comment,omitempty"`
	// Options are any SSH options associated with this key
	Options []string `json:"options,omitempty"`
}

// PeerInfo contains the information required for peering
type PeerInfo struct {
	// PeeringID is a unique identifier for the peer
	PeeringID string `json:"peering_id"`
	// Hostname is the hostname or IP address of the peer (not used for known_hosts anymore)
	Hostname string `json:"hostname,omitempty"`
	// PublicKey is the peer's public key
	PublicKey string `json:"public_key"`
	// SSHOptions are any additional SSH options to apply (for authorized_keys)
	SSHOptions []string `json:"ssh_options,omitempty"`
}

// HostKeyResponse represents the response when getting this machine's SSH host key
type HostKeyResponse struct {
	// HostKey is the SSH server's host public key (from /etc/ssh/ssh_host_*_key.pub)
	HostKey string `json:"host_key"`
	// KeyType is the type of the host key (e.g., "ssh-ed25519", "ssh-rsa")
	KeyType string `json:"key_type"`
}

// AddKnownHostRequest represents a request to add a remote host's key to known_hosts
type AddKnownHostRequest struct {
	// PeeringID is the unique identifier for the peering connection
	PeeringID string `json:"peering_id" binding:"required"`
	// Hostname is the hostname or IP address of the remote host
	Hostname string `json:"hostname" binding:"required"`
	// HostKey is the remote host's SSH server public key
	HostKey string `json:"host_key" binding:"required"`
}

// RemoveKnownHostRequest represents a request to remove a host entry from known_hosts
type RemoveKnownHostRequest struct {
	// PeeringID is the unique identifier for the peering connection
	PeeringID string `json:"peering_id" binding:"required"`
	// Hostname is optional; if provided, only removes the specific hostname entry
	Hostname string `json:"hostname,omitempty"`
}
