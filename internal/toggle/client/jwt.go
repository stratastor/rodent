// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// ParseJWT splits a JWT token into its three parts
func ParseJWT(tokenString string) []string {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil
	}
	return parts
}

// DecodeJWTPayload decodes the JWT payload (second part) and returns the claims
func DecodeJWTPayload(encodedPayload string) (map[string]interface{}, error) {
	payload, err := decodeBase64UrlSafe(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return claims, nil
}

// decodeBase64UrlSafe decodes base64url-encoded data
func decodeBase64UrlSafe(s string) ([]byte, error) {
	// Add padding if necessary
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}

	// Replace URL-safe chars with standard base64 chars
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")

	return base64.StdEncoding.DecodeString(s)
}

// ExtractRodentIDFromJWT extracts the 'rid' claim from a JWT
func ExtractRodentIDFromJWT(tokenString string) (string, error) {
	parts := ParseJWT(tokenString)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid JWT format")
	}

	// Decode the payload (second part)
	payload, err := decodeBase64UrlSafe(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Extract the 'rid' claim
	rid, ok := claims["rid"].(string)
	if !ok {
		return "", fmt.Errorf("rid claim not found or not a string in JWT")
	}

	return rid, nil
}

// ExtractSubFromJWT extracts the 'sub' claim from a JWT
func ExtractSubFromJWT(tokenString string) (string, error) {
	parts := ParseJWT(tokenString)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid JWT format")
	}

	// Decode the payload (second part)
	payload, err := decodeBase64UrlSafe(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Extract the 'sub' claim
	sub, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("sub claim not found or not a string in JWT")
	}

	return sub, nil
}

// IsPrivateNetwork determines if the JWT token indicates a private network
func IsPrivateNetwork(jwt string) (bool, error) {
	parts := ParseJWT(jwt)
	if parts == nil {
		return false, fmt.Errorf("invalid JWT format")
	}

	claims, err := DecodeJWTPayload(parts[1])
	if err != nil {
		return false, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Check for the "prv" claim indicating a private network
	prvValue, ok := claims["prv"]
	if !ok {
		return false, nil
	}

	// Convert to boolean
	prvBool, ok := prvValue.(bool)
	if !ok {
		return false, fmt.Errorf("prv claim is not a boolean")
	}

	return prvBool, nil
}
