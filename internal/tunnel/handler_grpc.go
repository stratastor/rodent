// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"encoding/json"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterTunnelGRPCHandlers registers tunnel command handlers with Toggle
func RegisterTunnelGRPCHandlers() {
	client.RegisterCommandHandler(proto.CmdTunnelHTTP, handleTunnelHTTP())
}

func handleTunnelHTTP() client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var tunnelReq TunnelRequest
		if err := json.Unmarshal(cmd.Payload, &tunnelReq); err != nil {
			return errors.ErrorResponse(
				req.RequestId,
				errors.Wrap(err, errors.ServerRequestValidation),
			), nil
		}

		respBytes, err := proxyHTTPRequest(&tunnelReq)
		if err != nil {
			return errors.ErrorResponse(req.RequestId, err), nil
		}

		return errors.SuccessResponse(req.RequestId, "tunnel proxy response", respBytes), nil
	}
}
