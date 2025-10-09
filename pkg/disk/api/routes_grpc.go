// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterDiskGRPCHandlers registers all disk management command handlers with Toggle
func RegisterDiskGRPCHandlers(handler *DiskHandler) {
	// Inventory operations
	client.RegisterCommandHandler(proto.CmdDiskList, handleDiskList(handler))
	client.RegisterCommandHandler(proto.CmdDiskGet, handleDiskGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskDiscover, handleDiskDiscover(handler))
	client.RegisterCommandHandler(proto.CmdDiskRefresh, handleDiskRefresh(handler))

	// Health and SMART operations
	client.RegisterCommandHandler(proto.CmdDiskHealthGet, handleDiskHealthGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskSMARTGet, handleDiskSMARTGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskSMARTRefresh, handleDiskSMARTRefresh(handler))

	// TODO: Add more command registrations as implementation progresses
}
