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
	client.RegisterCommandHandler(proto.CmdDiskListAvailable, handleDiskListAvailable(handler))
	client.RegisterCommandHandler(proto.CmdDiskDiscover, handleDiskDiscover(handler))

	// Health and SMART operations
	client.RegisterCommandHandler(proto.CmdDiskHealthCheck, handleDiskHealthCheck(handler))
	client.RegisterCommandHandler(proto.CmdDiskHealthGet, handleDiskHealthGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskSMARTGet, handleDiskSMARTGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskSMARTRefresh, handleDiskSMARTRefresh(handler))

	// Probe operations
	client.RegisterCommandHandler(proto.CmdDiskProbeStart, handleDiskProbeStart(handler))
	client.RegisterCommandHandler(proto.CmdDiskProbeCancel, handleDiskProbeCancel(handler))
	client.RegisterCommandHandler(proto.CmdDiskProbeGet, handleDiskProbeGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskProbeList, handleDiskProbeList(handler))
	client.RegisterCommandHandler(proto.CmdDiskProbeHistory, handleDiskProbeHistory(handler))

	// Probe schedule operations
	client.RegisterCommandHandler(
		proto.CmdDiskProbeScheduleList,
		handleDiskProbeScheduleList(handler),
	)
	client.RegisterCommandHandler(
		proto.CmdDiskProbeScheduleGet,
		handleDiskProbeScheduleGet(handler),
	)
	client.RegisterCommandHandler(
		proto.CmdDiskProbeScheduleCreate,
		handleDiskProbeScheduleCreate(handler),
	)
	client.RegisterCommandHandler(
		proto.CmdDiskProbeScheduleUpdate,
		handleDiskProbeScheduleUpdate(handler),
	)
	client.RegisterCommandHandler(
		proto.CmdDiskProbeScheduleDelete,
		handleDiskProbeScheduleDelete(handler),
	)
	client.RegisterCommandHandler(
		proto.CmdDiskProbeScheduleEnable,
		handleDiskProbeScheduleEnable(handler),
	)
	client.RegisterCommandHandler(
		proto.CmdDiskProbeScheduleDisable,
		handleDiskProbeScheduleDisable(handler),
	)

	// Topology operations
	client.RegisterCommandHandler(proto.CmdDiskTopologyGet, handleDiskTopologyGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskTopologyRefresh, handleDiskTopologyRefresh(handler))
	client.RegisterCommandHandler(
		proto.CmdDiskTopologyControllers,
		handleDiskTopologyControllers(handler),
	)
	client.RegisterCommandHandler(
		proto.CmdDiskTopologyEnclosures,
		handleDiskTopologyEnclosures(handler),
	)

	// State management operations
	client.RegisterCommandHandler(proto.CmdDiskStateGet, handleDiskStateGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskStateSet, handleDiskStateSet(handler))
	client.RegisterCommandHandler(proto.CmdDiskValidate, handleDiskValidate(handler))
	client.RegisterCommandHandler(proto.CmdDiskQuarantine, handleDiskQuarantine(handler))

	// Metadata operations
	client.RegisterCommandHandler(proto.CmdDiskTagsSet, handleDiskTagsSet(handler))
	client.RegisterCommandHandler(proto.CmdDiskTagsDelete, handleDiskTagsDelete(handler))
	client.RegisterCommandHandler(proto.CmdDiskNotesSet, handleDiskNotesSet(handler))

	// Statistics operations
	client.RegisterCommandHandler(proto.CmdDiskStatsGet, handleDiskStatsGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskStatsGlobal, handleDiskStatsGlobal(handler))

	// Monitoring operations
	client.RegisterCommandHandler(proto.CmdDiskMonitoringGet, handleDiskMonitoringGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskMonitoringSet, handleDiskMonitoringSet(handler))

	// Configuration operations
	client.RegisterCommandHandler(proto.CmdDiskConfigGet, handleDiskConfigGet(handler))
	client.RegisterCommandHandler(proto.CmdDiskConfigUpdate, handleDiskConfigUpdate(handler))
	client.RegisterCommandHandler(proto.CmdDiskConfigReload, handleDiskConfigReload(handler))
}
