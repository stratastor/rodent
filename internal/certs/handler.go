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

// writeSudo writes content to a file via sudo tee.
func writeSudo(path, content string) (string, error) {
	cmd := exec.Command("sudo", "tee", path)
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

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

		// Write certs via sudo (Rodent runs as 'rodent' user, sudoers allows /etc/dremio/tls)
		if err := exec.Command("sudo", "mkdir", "-p", payload.DestDir).Run(); err != nil {
			return nil, errors.Wrap(err, errors.ServerInternalError)
		}

		certPath := filepath.Join(payload.DestDir, "cert.pem")
		keyPath := filepath.Join(payload.DestDir, "key.pem")

		if out, err := writeSudo(certPath, payload.CertPEM); err != nil {
			return nil, errors.New(errors.ServerInternalError, fmt.Sprintf("write cert failed: %s", out))
		}
		if out, err := writeSudo(keyPath, payload.KeyPEM); err != nil {
			return nil, errors.New(errors.ServerInternalError, fmt.Sprintf("write key failed: %s", out))
		}
		_ = exec.Command("sudo", "chmod", "0600", keyPath).Run()

		// Signal Traefik to reload (watches file changes, but SIGHUP as safety net)
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
