// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

// Package certs handles TLS certificate delivery from Toggle.
// Toggle issues certs via ACME (DNS-01/Route53) and pushes them to nodes
// via gRPC. This handler writes cert files to disk and reloads Traefik.
package certs

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterCertGRPCHandlers registers cert delivery command handlers with Toggle.
func RegisterCertGRPCHandlers() {
	client.RegisterCommandHandler(proto.CmdCertDeliver, handleCertDeliver())
	client.RegisterCommandHandler(proto.CmdCertStatus, handleCertStatus())
}

// certDeliverPayload is the JSON payload for certs.deliver commands.
type certDeliverPayload struct {
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
	Domain  string `json:"domain"`
	DestDir string `json:"dest_dir"` // e.g., "/etc/dremio/tls"
}

// handleCertDeliver writes cert and key PEM files to the specified directory
// and signals Traefik to reload (Traefik watches the file directory).
func handleCertDeliver() client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload certDeliverPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if payload.CertPEM == "" || payload.KeyPEM == "" {
			return nil, errors.New(errors.ServerRequestValidation, "cert_pem and key_pem are required")
		}
		if payload.DestDir == "" {
			payload.DestDir = "/etc/dremio/tls"
		}

		// Ensure destination directory exists (Rodent runs as 'rodent' user, need sudo)
		if err := exec.Command("sudo", "mkdir", "-p", payload.DestDir).Run(); err != nil {
			return nil, errors.Wrap(err, errors.ServerInternalError)
		}

		certPath := filepath.Join(payload.DestDir, "cert.pem")
		keyPath := filepath.Join(payload.DestDir, "key.pem")

		// Write via sudo tee (Rodent user can't write to /etc/dremio/tls directly)
		writePEM := func(path, content, mode string) error {
			cmd := exec.Command("sudo", "tee", path)
			cmd.Stdin = strings.NewReader(content)
			cmd.Stdout = nil // discard tee's stdout copy
			if err := cmd.Run(); err != nil {
				return err
			}
			return exec.Command("sudo", "chmod", mode, path).Run()
		}

		if err := writePEM(certPath, payload.CertPEM, "0644"); err != nil {
			return nil, errors.Wrap(err, errors.ServerInternalError)
		}
		if err := writePEM(keyPath, payload.KeyPEM, "0600"); err != nil {
			return nil, errors.Wrap(err, errors.ServerInternalError)
		}

		// Traefik watches the file directory and auto-reloads when certs change.
		// Signal as a safety net — Docker volume mount picks up changes automatically.
		_ = exec.Command("sudo", "docker", "kill", "--signal=HUP", "traefik-traefik-1").Run()

		resp := map[string]interface{}{
			"domain":    payload.Domain,
			"cert_path": certPath,
			"key_path":  keyPath,
			"message":   "certificate delivered successfully",
		}
		respBytes, _ := json.Marshal(resp)

		return &proto.CommandResponse{
			RequestId: req.RequestId,
			Success:   true,
			Message:   fmt.Sprintf("Certificate for %s delivered to %s", payload.Domain, payload.DestDir),
			Payload:   respBytes,
		}, nil
	}
}

// handleCertStatus returns info about the current cert on disk.
func handleCertStatus() client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			DestDir string `json:"dest_dir"`
		}
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}
		if payload.DestDir == "" {
			payload.DestDir = "/etc/dremio/tls"
		}

		certPath := filepath.Join(payload.DestDir, "cert.pem")
		info, err := os.Stat(certPath)

		resp := map[string]interface{}{
			"dest_dir": payload.DestDir,
			"exists":   err == nil,
		}
		if err == nil {
			resp["modified_at"] = info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
			resp["size_bytes"] = info.Size()
		}

		respBytes, _ := json.Marshal(resp)
		return &proto.CommandResponse{
			RequestId: req.RequestId,
			Success:   true,
			Message:   "Certificate status retrieved",
			Payload:   respBytes,
		}, nil
	}
}
