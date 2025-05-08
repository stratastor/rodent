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
