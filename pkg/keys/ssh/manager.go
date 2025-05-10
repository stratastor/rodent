// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// SSHKeyManager handles SSH key operations
type SSHKeyManager struct {
	logger         logger.Logger
	dirPath        string
	knownHosts     string
	authorizedKeys string
	username       string
	algorithm      KeyPairType
	permissions    os.FileMode
}

// NewSSHKeyManager creates a new SSH key manager
func NewSSHKeyManager(logger logger.Logger) (*SSHKeyManager, error) {
	cfg := config.GetConfig()

	// Set key directory from config
	dirPath := cfg.Keys.SSH.DirPath
	if dirPath == "" {
		dirPath = config.GetSSHDir()
	}

	// Set known hosts file from config
	knownHostsFile := cfg.Keys.SSH.KnownHostsFile
	if knownHostsFile == "" {
		knownHostsFile = filepath.Join(dirPath, "known_hosts")
	}

	// Set username from config
	username := cfg.Keys.SSH.Username
	if username == "" {
		username = "rodent"
	}

	// Set authorized_keys file from config with home directory expansion
	authorizedKeysFile := cfg.Keys.SSH.AuthorizedKeysFile
	if authorizedKeysFile == "" {
		// Default to ~/.ssh/authorized_keys
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrap(err, errors.SSHKeyPairWriteFailed).
				WithMetadata("error", "Failed to determine user's home directory")
		}
		authorizedKeysFile = filepath.Join(homeDir, ".ssh", "authorized_keys")
	} else if strings.HasPrefix(authorizedKeysFile, "~") {
		// Expand tilde to home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrap(err, errors.SSHKeyPairWriteFailed).
				WithMetadata("error", "Failed to determine user's home directory")
		}
		authorizedKeysFile = filepath.Join(homeDir, authorizedKeysFile[1:])
	}

	// Set algorithm from config
	algorithm := KeyPairType(cfg.Keys.SSH.Algorithm)
	if algorithm == "" {
		algorithm = KeyPairTypeED25519
	}

	// Ensure SSH key directory exists with proper permissions
	if err := os.MkdirAll(dirPath, 0700); err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairWriteFailed).
			WithMetadata("path", dirPath)
	}

	// Ensure known_hosts file exists
	if _, err := os.Stat(knownHostsFile); os.IsNotExist(err) {
		if err := os.WriteFile(knownHostsFile, []byte{}, 0600); err != nil {
			return nil, errors.Wrap(err, errors.SSHKeyPairWriteFailed).
				WithMetadata("path", knownHostsFile)
		}
	}

	// Ensure authorized_keys directory exists
	authorizedKeysDir := filepath.Dir(authorizedKeysFile)
	if err := os.MkdirAll(authorizedKeysDir, 0700); err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairWriteFailed).
			WithMetadata("path", authorizedKeysDir)
	}

	// Ensure authorized_keys file exists
	if _, err := os.Stat(authorizedKeysFile); os.IsNotExist(err) {
		if err := os.WriteFile(authorizedKeysFile, []byte{}, 0600); err != nil {
			return nil, errors.Wrap(err, errors.SSHKeyPairWriteFailed).
				WithMetadata("path", authorizedKeysFile)
		}
	}

	return &SSHKeyManager{
		logger:         logger,
		dirPath:        dirPath,
		knownHosts:     knownHostsFile,
		authorizedKeys: authorizedKeysFile,
		username:       username,
		algorithm:      algorithm,
		permissions:    0600,
	}, nil
}

// Close cleans up any resources used by the manager
func (m *SSHKeyManager) Close() {
	// No resources to close currently
}

// GenerateKeyPair generates a new SSH key pair for a peering ID
func (m *SSHKeyManager) GenerateKeyPair(
	ctx context.Context,
	peeringID string,
	keyType KeyPairType,
) (*KeyPair, error) {
	// Validate peering ID
	if err := validatePeeringID(peeringID); err != nil {
		return nil, err
	}

	// Create peer directory
	peerDir := filepath.Join(m.dirPath, peeringID)
	if err := os.MkdirAll(peerDir, 0700); err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairWriteFailed).
			WithMetadata("path", peerDir)
	}

	// Use configured type if not specified
	if keyType == "" {
		keyType = m.algorithm
	}

	// Generate key pair
	var privateBytes, publicBytes []byte
	var err error

	switch keyType {
	case KeyPairTypeED25519:
		privateBytes, publicBytes, err = generateED25519KeyPair()
	case KeyPairTypeRSA:
		privateBytes, publicBytes, err = generateRSAKeyPair()
	default:
		return nil, errors.New(errors.SSHKeyPairInvalidType,
			fmt.Sprintf("Unsupported key algorithm: %s", keyType))
	}

	if err != nil {
		return nil, err
	}

	// Write private key
	privateKeyPath := filepath.Join(peerDir, "id_"+string(keyType))
	if err := os.WriteFile(privateKeyPath, privateBytes, m.permissions); err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairWriteFailed).
			WithMetadata("path", privateKeyPath)
	}

	// Write public key
	publicKeyPath := privateKeyPath + ".pub"
	if err := os.WriteFile(publicKeyPath, publicBytes, m.permissions); err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairWriteFailed).
			WithMetadata("path", publicKeyPath)
	}

	// Return KeyPair object
	return &KeyPair{
		PeeringID:      peeringID,
		PublicKey:      string(publicBytes),
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
		Type:           keyType,
	}, nil
}

// AddKnownHost adds a host public key to the known_hosts file
func (m *SSHKeyManager) AddKnownHost(
	ctx context.Context,
	hostname string,
	publicKey string,
	peeringID string,
) error {
	// Validate inputs
	if err := validateHostname(hostname); err != nil {
		return err
	}

	if err := validatePeeringID(peeringID); err != nil {
		return err
	}

	// Validate public key format (should start with key type like ssh-rsa, ssh-ed25519)
	if !isValidSSHPublicKey(publicKey) {
		return errors.New(errors.SSHKeyPairInvalidPublicKey, "Invalid public key format")
	}

	// Format entry for known_hosts file with peering ID as comment
	entry := fmt.Sprintf("%s %s %s\n", hostname, publicKey, peeringID)

	// Check if entry already exists
	knownHosts, err := os.ReadFile(m.knownHosts)
	if err != nil {
		return errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", m.knownHosts)
	}

	// Check for existing entries by hostname and peering ID
	entries := parseKnownHosts(string(knownHosts))
	for _, e := range entries {
		if e.Hostname == hostname && e.PeeringID == peeringID {
			return errors.New(errors.SSHKnownHostEntryAlreadyExists,
				fmt.Sprintf("Entry for %s with peering ID %s already exists", hostname, peeringID))
		}
	}

	// Append to the known_hosts file
	f, err := os.OpenFile(m.knownHosts, os.O_APPEND|os.O_WRONLY|os.O_CREATE, m.permissions)
	if err != nil {
		return errors.Wrap(err, errors.SSHKnownHostAddFailed).
			WithMetadata("path", m.knownHosts)
	}
	defer f.Close()

	if _, err := f.WriteString(entry); err != nil {
		return errors.Wrap(err, errors.SSHKnownHostAddFailed).
			WithMetadata("path", m.knownHosts)
	}

	return nil
}

// RemoveKnownHost removes a host entry from the known_hosts file by peering ID and optionally hostname
func (m *SSHKeyManager) RemoveKnownHost(
	ctx context.Context,
	peeringID string,
	hostname string,
) error {
	// Validate peering ID
	if err := validatePeeringID(peeringID); err != nil {
		return err
	}

	// Validate hostname if provided
	if hostname != "" {
		if err := validateHostname(hostname); err != nil {
			return err
		}
	}

	// Read known_hosts file
	knownHosts, err := os.ReadFile(m.knownHosts)
	if err != nil {
		return errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", m.knownHosts)
	}

	// Parse the file and filter entries
	entries := parseKnownHosts(string(knownHosts))
	var newEntries []KnownHostEntry
	entriesRemoved := 0

	for _, entry := range entries {
		// Skip entries matching peering ID and optionally hostname
		if entry.PeeringID == peeringID {
			if hostname == "" || entry.Hostname == hostname {
				entriesRemoved++
				continue
			}
		}
		newEntries = append(newEntries, entry)
	}

	if entriesRemoved == 0 {
		if hostname != "" {
			return errors.New(
				errors.SSHKnownHostEntryNotFound,
				fmt.Sprintf(
					"Entry for hostname %s with peering ID %s not found",
					hostname,
					peeringID,
				),
			)
		}
		return errors.New(errors.SSHKnownHostEntryNotFound,
			fmt.Sprintf("No entries found for peering ID %s", peeringID))
	}

	// Write back the file
	var newContent strings.Builder
	for _, entry := range newEntries {
		newContent.WriteString(fmt.Sprintf("%s %s %s\n",
			entry.Hostname, entry.PublicKey, entry.PeeringID))
	}

	if err := os.WriteFile(m.knownHosts, []byte(newContent.String()), m.permissions); err != nil {
		return errors.Wrap(err, errors.SSHKnownHostRemoveFailed).
			WithMetadata("path", m.knownHosts)
	}

	return nil
}

// RemoveKeyPair removes a key pair for a peering ID
func (m *SSHKeyManager) RemoveKeyPair(ctx context.Context, peeringID string) error {
	// Validate peering ID
	if err := validatePeeringID(peeringID); err != nil {
		return err
	}

	// Check if peering directory exists
	peerDir := filepath.Join(m.dirPath, peeringID)
	if _, err := os.Stat(peerDir); os.IsNotExist(err) {
		return errors.New(errors.SSHKeyPairNotFound,
			fmt.Sprintf("No key pair found for peering ID %s", peeringID))
	}

	// Remove the directory
	if err := os.RemoveAll(peerDir); err != nil {
		return errors.Wrap(err, errors.SSHKeyPairDeleteFailed).
			WithMetadata("path", peerDir)
	}

	return nil
}

// GetPublicKeyPath returns the path to the public key for a peering ID
func (m *SSHKeyManager) GetPublicKeyPath(peeringID string) (string, error) {
	// Validate peering ID
	if err := validatePeeringID(peeringID); err != nil {
		return "", err
	}

	// Construct the path
	publicKeyPath := filepath.Join(m.dirPath, peeringID, fmt.Sprintf("id_%s.pub", m.algorithm))

	// Check if the file exists
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		return "", errors.New(errors.SSHKeyPairNotFound,
			fmt.Sprintf("No public key found for peering ID %s", peeringID))
	}

	return publicKeyPath, nil
}

// GetPrivateKeyPath returns the path to the private key for a peering ID
func (m *SSHKeyManager) GetPrivateKeyPath(peeringID string) (string, error) {
	// Validate peering ID
	if err := validatePeeringID(peeringID); err != nil {
		return "", err
	}

	// Construct the path
	privateKeyPath := filepath.Join(m.dirPath, peeringID, fmt.Sprintf("id_%s", m.algorithm))

	// Check if the file exists
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return "", errors.New(errors.SSHKeyPairNotFound,
			fmt.Sprintf("No private key found for peering ID %s", peeringID))
	}

	return privateKeyPath, nil
}

// GetKnownHostsPath returns the path to the known_hosts file
func (m *SSHKeyManager) GetKnownHostsPath() string {
	return m.knownHosts
}

// GetKeyPair gets a key pair for a peering ID
func (m *SSHKeyManager) GetKeyPair(peeringID string) (*KeyPair, error) {
	// Validate peering ID
	if err := validatePeeringID(peeringID); err != nil {
		return nil, err
	}

	// Get paths
	privateKeyPath, err := m.GetPrivateKeyPath(peeringID)
	if err != nil {
		return nil, err
	}

	publicKeyPath, err := m.GetPublicKeyPath(peeringID)
	if err != nil {
		return nil, err
	}

	// Read public key content
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", publicKeyPath)
	}

	// Determine key type from filename
	keyType := m.algorithm
	if strings.HasSuffix(privateKeyPath, "_rsa") {
		keyType = KeyPairTypeRSA
	} else if strings.HasSuffix(privateKeyPath, "_ed25519") {
		keyType = KeyPairTypeED25519
	}

	return &KeyPair{
		PeeringID:      peeringID,
		PublicKey:      string(publicKeyBytes),
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
		Type:           keyType,
	}, nil
}

// HasKeyPair checks if a peering ID has a key pair
func (m *SSHKeyManager) HasKeyPair(peeringID string) (bool, error) {
	// Validate peering ID
	if err := validatePeeringID(peeringID); err != nil {
		return false, err
	}

	// Check if peering directory exists
	peerDir := filepath.Join(m.dirPath, peeringID)
	if _, err := os.Stat(peerDir); os.IsNotExist(err) {
		return false, nil
	}

	// Check if private key exists for any supported algorithm
	algorithms := []KeyPairType{KeyPairTypeED25519, KeyPairTypeRSA}
	for _, alg := range algorithms {
		privateKeyPath := filepath.Join(peerDir, "id_"+string(alg))
		publicKeyPath := privateKeyPath + ".pub"

		privateExists := false
		publicExists := false

		if _, err := os.Stat(privateKeyPath); !os.IsNotExist(err) {
			privateExists = true
		}

		if _, err := os.Stat(publicKeyPath); !os.IsNotExist(err) {
			publicExists = true
		}

		if privateExists && publicExists {
			return true, nil
		}
	}

	return false, nil
}

// FindKnownHostsByPeeringID finds known hosts entries by peering ID
func (m *SSHKeyManager) FindKnownHostsByPeeringID(peeringID string) ([]KnownHostEntry, error) {
	// Validate peering ID
	if err := validatePeeringID(peeringID); err != nil {
		return nil, err
	}

	// Read known_hosts file
	knownHosts, err := os.ReadFile(m.knownHosts)
	if err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", m.knownHosts)
	}

	// Parse and filter entries
	entries := parseKnownHosts(string(knownHosts))
	var results []KnownHostEntry

	for _, entry := range entries {
		if entry.PeeringID == peeringID {
			results = append(results, entry)
		}
	}

	return results, nil
}

// FindKnownHostsByHostname finds known hosts entries by hostname
func (m *SSHKeyManager) FindKnownHostsByHostname(hostname string) ([]KnownHostEntry, error) {
	// Validate hostname
	if err := validateHostname(hostname); err != nil {
		return nil, err
	}

	// Read known_hosts file
	knownHosts, err := os.ReadFile(m.knownHosts)
	if err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", m.knownHosts)
	}

	// Parse and filter entries
	entries := parseKnownHosts(string(knownHosts))
	var results []KnownHostEntry

	for _, entry := range entries {
		if entry.Hostname == hostname {
			results = append(results, entry)
		}
	}

	return results, nil
}

// ListKeyPairs lists all key pairs
func (m *SSHKeyManager) ListKeyPairs(ctx context.Context) ([]KeyPair, error) {
	// Read the ssh directory
	dirEntries, err := os.ReadDir(m.dirPath)
	if err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", m.dirPath)
	}

	var keyPairs []KeyPair
	for _, entry := range dirEntries {
		if entry.IsDir() {
			peeringID := entry.Name()

			// Check if it has a valid key pair
			hasKeyPair, err := m.HasKeyPair(peeringID)
			if err != nil {
				m.logger.Warn("Error checking key pair",
					"peering_id", peeringID, "error", err)
				continue
			}

			if hasKeyPair {
				keyPair, err := m.GetKeyPair(peeringID)
				if err != nil {
					m.logger.Warn("Error getting key pair",
						"peering_id", peeringID, "error", err)
					continue
				}
				keyPairs = append(keyPairs, *keyPair)
			}
		}
	}

	return keyPairs, nil
}

// ListKnownHosts lists all known hosts entries
func (m *SSHKeyManager) ListKnownHosts(ctx context.Context) ([]KnownHostEntry, error) {
	// Read known_hosts file
	knownHosts, err := os.ReadFile(m.knownHosts)
	if err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", m.knownHosts)
	}

	return parseKnownHosts(string(knownHosts)), nil
}

// parseKnownHosts parses a known_hosts file content and returns structured entries
func parseKnownHosts(content string) []KnownHostEntry {
	var entries []KnownHostEntry
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Trim any trailing whitespace (including \r for Windows CRLF)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// First split by space to get the hostname
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		hostname := parts[0]
		rest := parts[1]

		// Now split the rest by space, counting from the end to get the peering ID (last field)
		restParts := strings.Fields(rest)
		if len(restParts) < 2 {
			continue // Need at least a public key and peering ID
		}

		// Last field is the peering ID
		peeringID := restParts[len(restParts)-1]

		// Everything else is the public key (minus the last field)
		publicKey := strings.Join(restParts[:len(restParts)-1], " ")

		entries = append(entries, KnownHostEntry{
			Hostname:  hostname,
			PublicKey: publicKey,
			PeeringID: peeringID,
		})
	}

	return entries
}

// generateED25519KeyPair generates an ED25519 SSH key pair
func generateED25519KeyPair() ([]byte, []byte, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, errors.Wrap(err, errors.SSHKeyPairGenerationFailed)
	}

	// Convert to SSH format
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, errors.SSHKeyPairGenerationFailed)
	}

	// Format the private key in OpenSSH format
	privateKeyBytes, err := opensshPrivateKeyED25519(privateKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, errors.SSHKeyPairGenerationFailed)
	}

	// Get public key in authorized_keys format
	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)

	return privateKeyBytes, publicKeyBytes, nil
}

// opensshPrivateKeyED25519 creates an ed25519 private key in OpenSSH format
func opensshPrivateKeyED25519(privateKey ed25519.PrivateKey) ([]byte, error) {
	// Instead of trying to use ssh-keygen to convert a PEM to OpenSSH format,
	// we'll use the golang.org/x/crypto/ssh package to create the private key directly

	// // Convert the ed25519 private key to SSH private key format
	// sshPrivateKey, err := ssh.NewSignerFromKey(privateKey)
	// if err != nil {
	// 	return nil, err
	// }

	// Marshal the private key to OpenSSH format
	sshPrivateKeyBytes, err := ssh.MarshalPrivateKey(crypto.PrivateKey(privateKey), "")
	if err != nil {
		return nil, errors.New(errors.SSHKeyPairGenerationFailed,
			"Failed to marshal private key to OpenSSH format: "+err.Error())
	}

	// // Marshal to PEM format
	// pemKey := &pem.Block{
	// 	Type:  "OPENSSH PRIVATE KEY",
	// 	Bytes: edKeyToBinary(privateKey),
	// }

	// Encode to OpenSSH format
	privateKeyBytes := pem.EncodeToMemory(sshPrivateKeyBytes)

	return privateKeyBytes, nil
}

// generateRSAKeyPair generates an RSA SSH key pair
func generateRSAKeyPair() ([]byte, []byte, error) {
	// For RSA key generation, we'll use the ssh-keygen command directly
	// This ensures proper OpenSSH format

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "ssh-key-*")
	if err != nil {
		return nil, nil, errors.Wrap(err, errors.SSHKeyPairGenerationFailed)
	}
	defer os.RemoveAll(tempDir)

	// Path for the temporary key
	tmpPrivateKeyPath := filepath.Join(tempDir, "id_rsa")
	tmpPublicKeyPath := tmpPrivateKeyPath + ".pub"

	// Generate the key pair using ssh-keygen
	// Pass nil logger, ExecCommand will use common.Log
	_, err = command.ExecCommand(context.Background(), nil, "ssh-keygen",
		"-t", "rsa",
		"-b", "4096", // 4096 bits for better security
		"-f", tmpPrivateKeyPath,
		"-N", "", // Empty passphrase
		"-q") // Quiet mode

	if err != nil {
		return nil, nil, errors.Wrap(err, errors.SSHKeyPairGenerationFailed)
	}

	// Read the generated keys
	privateBytes, err := os.ReadFile(tmpPrivateKeyPath)
	if err != nil {
		return nil, nil, errors.Wrap(err, errors.SSHKeyPairReadFailed)
	}

	publicBytes, err := os.ReadFile(tmpPublicKeyPath)
	if err != nil {
		return nil, nil, errors.Wrap(err, errors.SSHKeyPairReadFailed)
	}

	return privateBytes, publicBytes, nil
}

// validatePeeringID validates a peering ID
func validatePeeringID(peeringID string) error {
	if peeringID == "" {
		return errors.New(errors.SSHKeyPairInvalidPeeringID, "Peering ID cannot be empty")
	}

	// Allow only alphanumeric characters, dash, and underscore
	validID := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validID.MatchString(peeringID) {
		return errors.New(errors.SSHKeyPairInvalidPeeringID,
			"Peering ID must contain only alphanumeric characters, dash, or underscore")
	}

	return nil
}

// validateHostname validates a hostname or IP address
func validateHostname(hostname string) error {
	if hostname == "" {
		return errors.New(errors.SSHKeyPairInvalidHostname, "Hostname cannot be empty")
	}

	// Simple hostname/IP validation
	// Allow hostnames, IPv4, IPv6, and wildcards
	validHost := regexp.MustCompile(
		`^([a-zA-Z0-9_-]+\.)*[a-zA-Z0-9_-]+$|^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$|^\[[0-9a-fA-F:]+\]$|^\*\.[a-zA-Z0-9_-]+(\.[a-zA-Z0-9_-]+)*$`,
	)
	if !validHost.MatchString(hostname) {
		return errors.New(errors.SSHKeyPairInvalidHostname,
			"Invalid hostname or IP address format")
	}

	return nil
}

// isValidSSHPublicKey checks if the provided string is a valid SSH public key
func isValidSSHPublicKey(key string) bool {
	// Valid SSH public keys start with the key type
	validKeyTypes := []string{"ssh-ed25519", "ssh-rsa", "ecdsa-sha2-nistp", "ssh-dss"}

	for _, keyType := range validKeyTypes {
		if strings.HasPrefix(key, keyType) {
			return true
		}
	}

	return false
}
