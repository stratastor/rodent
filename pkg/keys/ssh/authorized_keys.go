// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"fmt"
	"os"
	"strings"

	"github.com/stratastor/rodent/pkg/errors"
)

// AuthorizedKeysEntry represents an entry in the authorized_keys file
type AuthorizedKeysEntry struct {
	// PublicKey is the SSH public key in authorized_keys format
	PublicKey string `json:"public_key"`
	// Comment is an optional comment (often used to identify the key)
	Comment string `json:"comment,omitempty"`
	// Options are any SSH options associated with this key
	Options []string `json:"options,omitempty"`
}

// String returns the string representation of the entry for the authorized_keys file
func (e *AuthorizedKeysEntry) String() string {
	var result strings.Builder

	// Add options if present
	if len(e.Options) > 0 {
		result.WriteString(strings.Join(e.Options, ","))
		result.WriteString(" ")
	}

	// Add the key itself
	result.WriteString(e.PublicKey)

	// Add comment if present
	if e.Comment != "" && !strings.Contains(e.PublicKey, e.Comment) {
		result.WriteString(" ")
		result.WriteString(e.Comment)
	}

	return result.String()
}

// AddAuthorizedKey adds a public key to the authorized_keys file
// The key is typically the public key generated for a peering ID on another machine
func (m *SSHKeyManager) AddAuthorizedKey(
	publicKey string,
	peeringID string,
	options []string,
) error {
	// Format entry
	entry := &AuthorizedKeysEntry{
		PublicKey: publicKey,
		Comment:   peeringID,
		Options:   options,
	}

	// Read current authorized_keys content
	content, err := os.ReadFile(m.authorizedKeys)
	if err != nil {
		return errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", m.authorizedKeys)
	}

	// Check if entry with this comment/peering ID already exists
	entries := parseAuthorizedKeysContent(string(content))
	for _, existing := range entries {
		if existing.Comment == peeringID {
			return errors.New(errors.SSHKeyPairAlreadyExists,
				fmt.Sprintf("Entry for peering ID %s already exists in authorized_keys", peeringID))
		}
	}

	// Append the new entry
	f, err := os.OpenFile(m.authorizedKeys, os.O_APPEND|os.O_WRONLY, m.permissions)
	if err != nil {
		return errors.Wrap(err, errors.SSHKeyPairWriteFailed).
			WithMetadata("path", m.authorizedKeys)
	}
	defer f.Close()

	if _, err := f.WriteString(entry.String() + "\n"); err != nil {
		return errors.Wrap(err, errors.SSHKeyPairWriteFailed).
			WithMetadata("path", m.authorizedKeys)
	}

	return nil
}

// RemoveAuthorizedKey removes a key from the authorized_keys file by its peering ID
func (m *SSHKeyManager) RemoveAuthorizedKey(peeringID string) error {
	// Read current authorized_keys content
	content, err := os.ReadFile(m.authorizedKeys)
	if err != nil {
		return errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", m.authorizedKeys)
	}

	// Parse and filter entries
	entries := parseAuthorizedKeysContent(string(content))
	var newEntries []AuthorizedKeysEntry
	removed := false

	for _, entry := range entries {
		if entry.Comment == peeringID {
			removed = true
			continue
		}
		newEntries = append(newEntries, entry)
	}

	if !removed {
		return errors.New(errors.SSHKeyPairNotFound,
			fmt.Sprintf("No entry for peering ID %s found in authorized_keys", peeringID))
	}

	// Write back the filtered content
	var newContent strings.Builder
	for _, entry := range newEntries {
		newContent.WriteString(entry.String())
		newContent.WriteString("\n")
	}

	if err := os.WriteFile(m.authorizedKeys, []byte(newContent.String()), m.permissions); err != nil {
		return errors.Wrap(err, errors.SSHKeyPairWriteFailed).
			WithMetadata("path", m.authorizedKeys)
	}

	return nil
}

// ListAuthorizedKeys returns all entries in the authorized_keys file
func (m *SSHKeyManager) ListAuthorizedKeys() ([]AuthorizedKeysEntry, error) {
	// Read current authorized_keys content
	content, err := os.ReadFile(m.authorizedKeys)
	if err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", m.authorizedKeys)
	}

	return parseAuthorizedKeysContent(string(content)), nil
}

// FindAuthorizedKeyByPeeringID finds an entry in the authorized_keys file by its peering ID
func (m *SSHKeyManager) FindAuthorizedKeyByPeeringID(
	peeringID string,
) (*AuthorizedKeysEntry, error) {
	// Read current authorized_keys content
	content, err := os.ReadFile(m.authorizedKeys)
	if err != nil {
		return nil, errors.Wrap(err, errors.SSHKeyPairReadFailed).
			WithMetadata("path", m.authorizedKeys)
	}

	// Parse and find entry
	entries := parseAuthorizedKeysContent(string(content))
	for _, entry := range entries {
		if entry.Comment == peeringID {
			return &entry, nil
		}
	}

	return nil, errors.New(errors.SSHKeyPairNotFound,
		fmt.Sprintf("No entry for peering ID %s found in authorized_keys", peeringID))
}

// GetAuthorizedKeysPath returns the path to the authorized_keys file
func (m *SSHKeyManager) GetAuthorizedKeysPath() string {
	return m.authorizedKeys
}

// parseAuthorizedKeysContent parses the content of an authorized_keys file
func parseAuthorizedKeysContent(content string) []AuthorizedKeysEntry {
	var entries []AuthorizedKeysEntry
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entry := parseAuthorizedKeyLine(line)
		entries = append(entries, entry)
	}

	return entries
}

// parseAuthorizedKeyLine parses a single line from an authorized_keys file
func parseAuthorizedKeyLine(line string) AuthorizedKeysEntry {
	var entry AuthorizedKeysEntry

	// Handle options at the beginning (comma-separated, may be quoted)
	if strings.HasPrefix(line, "ssh-") || strings.HasPrefix(line, "ecdsa-") {
		// No options, just key and possibly comment
		parts := strings.SplitN(line, " ", 3)
		if len(parts) >= 2 {
			entry.PublicKey = parts[0] + " " + parts[1]
			if len(parts) > 2 {
				entry.Comment = parts[2]
			}
		} else {
			entry.PublicKey = line
		}
	} else {
		// Has options
		var optionEnd int
		inQuote := false
		for i, char := range line {
			if char == '"' {
				inQuote = !inQuote
			} else if char == ' ' && !inQuote {
				optionEnd = i
				break
			}
		}

		if optionEnd > 0 {
			options := line[:optionEnd]
			rest := strings.TrimSpace(line[optionEnd:])

			// Extract options (handling quoted values)
			entry.Options = parseSSHOptions(options)

			// Process the rest (key and comment)
			parts := strings.SplitN(rest, " ", 3)
			if len(parts) >= 2 {
				entry.PublicKey = parts[0] + " " + parts[1]
				if len(parts) > 2 {
					entry.Comment = parts[2]
				}
			} else {
				entry.PublicKey = rest
			}
		} else {
			// Couldn't find options delimiter, treat as just a key
			entry.PublicKey = line
		}
	}

	return entry
}

// parseSSHOptions parses SSH options from authorized_keys
func parseSSHOptions(options string) []string {
	var result []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(options); i++ {
		char := options[i]

		if char == '"' {
			inQuote = !inQuote
			current.WriteByte(char)
		} else if char == ',' && !inQuote {
			result = append(result, current.String())
			current.Reset()
		} else {
			current.WriteByte(char)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}
