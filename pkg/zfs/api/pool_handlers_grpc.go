// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/pool"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// Note: Inconsistent wrapping pattern: successPoolResponse vs successResponse
// gRPC: Uses successResponse() helper which wraps data in {"result": data}
// gRPC: Uses successPoolResponse() for operations that return pool data directly (no wrapping)
// TODO: Standardize on pattern with Result wrapper for all responses in the next major version

// handlePoolList returns a handler for listing ZFS pools
func handlePoolList(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's List method
		pools, err := h.manager.List(ctx)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolList)
		}

		return successPoolResponse(req.RequestId, "ZFS pools list", pools)
	}
}

// handlePoolGet returns a handler for getting a specific ZFS pool
func handlePoolGet(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameParam struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &nameParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if nameParam.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's List method with pool name
		pool, err := h.manager.List(ctx, nameParam.Name)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolList)
		}

		return successPoolResponse(req.RequestId, "Pool info", pool)
	}
}

// handlePoolImportList returns a handler for listing importable pools
func handlePoolImportList(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's ListImportable method
		result, err := h.manager.ListImportable(ctx)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolImport)
		}

		return successResponse(req.RequestId, "Importable pools", map[string]interface{}{
			"importable_pools": result,
		})
	}
}

// handlePoolStatus returns a handler for getting ZFS pool status
func handlePoolStatus(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameParam struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &nameParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if nameParam.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Status method
		status, err := h.manager.Status(ctx, nameParam.Name)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolStatus)
		}

		return successPoolResponse(req.RequestId, "Pool status", status)
	}
}

// handlePoolCreate returns a handler for creating a ZFS pool
func handlePoolCreate(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var createConfig pool.CreateConfig
		if err := parseJSONPayload(cmd, &createConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if createConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		if len(createConfig.VDevSpec) == 0 {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"at least one device specification is required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Create method
		err := h.manager.Create(ctx, createConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolCreate)
		}

		return successPoolResponse(req.RequestId, "Pool created successfully", nil)
	}
}

// handlePoolDestroy returns a handler for destroying a ZFS pool
func handlePoolDestroy(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var destroyParam struct {
			Name  string `json:"name"`
			Force bool   `json:"force,omitempty"`
		}

		if err := parseJSONPayload(cmd, &destroyParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if destroyParam.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Destroy method
		err := h.manager.Destroy(ctx, destroyParam.Name, destroyParam.Force)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDestroy)
		}

		return successPoolResponse(req.RequestId, "Pool destroyed successfully", nil)
	}
}

// handlePoolImport returns a handler for importing a ZFS pool
func handlePoolImport(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var importConfig pool.ImportConfig
		if err := parseJSONPayload(cmd, &importConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Import method
		err := h.manager.Import(ctx, importConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolImport)
		}

		return successPoolResponse(req.RequestId, "Pool imported successfully", nil)
	}
}

// handlePoolExport returns a handler for exporting a ZFS pool
func handlePoolExport(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var exportParam struct {
			Name  string `json:"name"`
			Force bool   `json:"force,omitempty"`
		}

		if err := parseJSONPayload(cmd, &exportParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if exportParam.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Export method
		err := h.manager.Export(ctx, exportParam.Name, exportParam.Force)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolExport)
		}

		return successPoolResponse(req.RequestId, "Pool exported successfully", nil)
	}
}

// handlePoolPropertyList returns a handler for listing ZFS pool properties
func handlePoolPropertyList(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameParam struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &nameParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if nameParam.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's GetProperties method
		properties, err := h.manager.GetProperties(ctx, nameParam.Name)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolGetProperty)
		}

		return successPoolResponse(req.RequestId, "Pool properties", properties)
	}
}

// handlePoolPropertyGet returns a handler for getting a ZFS pool property
func handlePoolPropertyGet(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var propertyParam struct {
			Name     string `json:"name"`
			Property string `json:"property"`
		}

		if err := parseJSONPayload(cmd, &propertyParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if propertyParam.Name == "" || propertyParam.Property == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"pool name and property name are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's GetProperty method
		property, err := h.manager.GetProperty(ctx, propertyParam.Name, propertyParam.Property)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolGetProperty)
		}

		return successPoolResponse(req.RequestId, "Pool property", property)
	}
}

// handlePoolPropertySet returns a handler for setting a ZFS pool property
func handlePoolPropertySet(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var setPropertyParam struct {
			Name     string `json:"name"`
			Property string `json:"property"`
			Value    string `json:"value"`
		}

		if err := parseJSONPayload(cmd, &setPropertyParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if setPropertyParam.Name == "" || setPropertyParam.Property == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"pool name and property name are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's SetProperty method
		err := h.manager.SetProperty(
			ctx,
			setPropertyParam.Name,
			setPropertyParam.Property,
			setPropertyParam.Value,
		)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolSetProperty)
		}

		return successPoolResponse(req.RequestId, "Pool property set successfully", nil)
	}
}

// handlePoolScrub returns a handler for starting a ZFS pool scrub
func handlePoolScrub(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var scrubParam struct {
			Name string `json:"name"`
			Stop bool   `json:"stop,omitempty"`
		}

		if err := parseJSONPayload(cmd, &scrubParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if scrubParam.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Scrub method
		err := h.manager.Scrub(ctx, scrubParam.Name, scrubParam.Stop)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolScrubFailed)
		}

		action := "started"
		if scrubParam.Stop {
			action = "stopped"
		}

		return successPoolResponse(req.RequestId, "Pool scrub "+action+" successfully", nil)
	}
}

// handlePoolResilver returns a handler for starting a ZFS pool resilver
func handlePoolResilver(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameParam struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &nameParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if nameParam.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Resilver method
		err := h.manager.Resilver(ctx, nameParam.Name)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolResilverFailed)
		}

		return successPoolResponse(req.RequestId, "Pool resilver started successfully", nil)
	}
}

// handlePoolDeviceAttach returns a handler for attaching a device to a ZFS pool
func handlePoolDeviceAttach(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var attachParam struct {
			PoolName     string `json:"pool_name"`
			TargetDevice string `json:"target_device"`
			NewDevice    string `json:"new_device"`
		}

		if err := parseJSONPayload(cmd, &attachParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if attachParam.PoolName == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		if attachParam.TargetDevice == "" || attachParam.NewDevice == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"target and new device paths are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's AttachDevice method
		err := h.manager.AttachDevice(
			ctx,
			attachParam.PoolName,
			attachParam.TargetDevice,
			attachParam.NewDevice,
		)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Device attached successfully", nil)
	}
}

// handlePoolDeviceDetach returns a handler for detaching a device from a ZFS pool
func handlePoolDeviceDetach(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var detachParam struct {
			PoolName string `json:"pool_name"`
			Device   string `json:"device"`
		}

		if err := parseJSONPayload(cmd, &detachParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if detachParam.PoolName == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		if detachParam.Device == "" {
			return nil, errors.New(errors.ServerRequestValidation, "device path is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's DetachDevice method
		err := h.manager.DetachDevice(ctx, detachParam.PoolName, detachParam.Device)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Device detached successfully", nil)
	}
}

// handlePoolDeviceReplace returns a handler for replacing a device in a ZFS pool
func handlePoolDeviceReplace(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var replaceParam struct {
			PoolName  string `json:"pool_name"`
			OldDevice string `json:"old_device"`
			NewDevice string `json:"new_device"`
		}

		if err := parseJSONPayload(cmd, &replaceParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if replaceParam.PoolName == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		if replaceParam.OldDevice == "" || replaceParam.NewDevice == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"old and new device paths are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's ReplaceDevice method
		err := h.manager.ReplaceDevice(
			ctx,
			replaceParam.PoolName,
			replaceParam.OldDevice,
			replaceParam.NewDevice,
		)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Device replaced successfully", nil)
	}
}

// handlePoolDeviceRemove returns a handler for removing devices from a ZFS pool
func handlePoolDeviceRemove(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var removeParam struct {
			PoolName string   `json:"pool_name"`
			Devices  []string `json:"devices"`
		}

		if err := parseJSONPayload(cmd, &removeParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if removeParam.PoolName == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		if len(removeParam.Devices) == 0 {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"at least one device is required",
			)
		}

		ctx := createHandlerContext(req)
		err := h.manager.Remove(ctx, removeParam.PoolName, removeParam.Devices)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Device(s) removed successfully", nil)
	}
}

// handlePoolDeviceOffline returns a handler for taking a device offline
func handlePoolDeviceOffline(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var cfg pool.OfflineConfig
		if err := parseJSONPayload(cmd, &cfg); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if cfg.Pool == "" || cfg.Device == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"pool name and device are required",
			)
		}

		ctx := createHandlerContext(req)
		err := h.manager.Offline(ctx, cfg)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Device taken offline successfully", nil)
	}
}

// handlePoolDeviceOnline returns a handler for bringing a device online
func handlePoolDeviceOnline(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var cfg pool.OnlineConfig
		if err := parseJSONPayload(cmd, &cfg); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if cfg.Pool == "" || cfg.Device == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"pool name and device are required",
			)
		}

		ctx := createHandlerContext(req)
		err := h.manager.Online(ctx, cfg)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Device brought online successfully", nil)
	}
}

// handlePoolAdd returns a handler for adding vdevs to a pool
func handlePoolAdd(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var cfg pool.AddConfig
		if err := parseJSONPayload(cmd, &cfg); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if cfg.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		if len(cfg.VDevSpec) == 0 {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"at least one vdev specification is required",
			)
		}

		ctx := createHandlerContext(req)
		err := h.manager.Add(ctx, cfg)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "VDevs added successfully", nil)
	}
}

// handlePoolClear returns a handler for clearing pool errors
func handlePoolClear(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var cfg pool.ClearConfig
		if err := parseJSONPayload(cmd, &cfg); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if cfg.Pool == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		ctx := createHandlerContext(req)
		err := h.manager.Clear(ctx, cfg)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Pool errors cleared successfully", nil)
	}
}

// handlePoolInitialize returns a handler for initializing pool devices
func handlePoolInitialize(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var cfg pool.InitializeConfig
		if err := parseJSONPayload(cmd, &cfg); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if cfg.Pool == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		ctx := createHandlerContext(req)
		err := h.manager.Initialize(ctx, cfg)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Pool initialization operation successful", nil)
	}
}

// handlePoolTrim returns a handler for trimming pool devices
func handlePoolTrim(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var cfg pool.TrimConfig
		if err := parseJSONPayload(cmd, &cfg); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if cfg.Pool == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		ctx := createHandlerContext(req)
		err := h.manager.Trim(ctx, cfg)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Pool trim operation successful", nil)
	}
}

// handlePoolCheckpoint returns a handler for pool checkpoint operations
func handlePoolCheckpoint(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var cfg pool.CheckpointConfig
		if err := parseJSONPayload(cmd, &cfg); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if cfg.Pool == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		ctx := createHandlerContext(req)
		err := h.manager.Checkpoint(ctx, cfg)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		action := "created"
		if cfg.Discard {
			action = "discarded"
		}

		return successPoolResponse(req.RequestId, "Pool checkpoint "+action+" successfully", nil)
	}
}

// handlePoolReguid returns a handler for regenerating pool GUID
func handlePoolReguid(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameParam struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &nameParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if nameParam.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		ctx := createHandlerContext(req)
		err := h.manager.Reguid(ctx, nameParam.Name)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Pool GUID regenerated successfully", nil)
	}
}

// handlePoolReopen returns a handler for reopening pool vdevs
func handlePoolReopen(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameParam struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &nameParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if nameParam.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		ctx := createHandlerContext(req)
		err := h.manager.Reopen(ctx, nameParam.Name)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Pool reopened successfully", nil)
	}
}

// handlePoolUpgrade returns a handler for upgrading a pool
func handlePoolUpgrade(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var upgradeParam struct {
			Name string `json:"name"`
			All  bool   `json:"all,omitempty"`
		}

		if err := parseJSONPayload(cmd, &upgradeParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		ctx := createHandlerContext(req)
		err := h.manager.Upgrade(ctx, upgradeParam.Name, upgradeParam.All)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Pool upgraded successfully", nil)
	}
}

// handlePoolHistory returns a handler for getting pool command history
func handlePoolHistory(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var historyParam struct {
			Name       string `json:"name"`
			Internal   bool   `json:"internal,omitempty"`
			LongFormat bool   `json:"long_format,omitempty"`
		}

		if err := parseJSONPayload(cmd, &historyParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		ctx := createHandlerContext(req)
		history, err := h.manager.History(
			ctx,
			historyParam.Name,
			historyParam.Internal,
			historyParam.LongFormat,
		)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successResponse(req.RequestId, "Pool history", map[string]interface{}{
			"history": history,
		})
	}
}

// handlePoolEvents returns a handler for getting pool events
func handlePoolEvents(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var eventsParam struct {
			Name    string `json:"name"`
			Verbose bool   `json:"verbose,omitempty"`
		}

		if err := parseJSONPayload(cmd, &eventsParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		ctx := createHandlerContext(req)
		events, err := h.manager.Events(ctx, eventsParam.Name, eventsParam.Verbose)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successResponse(req.RequestId, "Pool events", map[string]interface{}{
			"events": events,
		})
	}
}

// handlePoolIOStat returns a handler for getting pool I/O statistics
func handlePoolIOStat(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var iostatParam struct {
			Name    string `json:"name"`
			Verbose bool   `json:"verbose,omitempty"`
		}

		if err := parseJSONPayload(cmd, &iostatParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		ctx := createHandlerContext(req)
		iostat, err := h.manager.IOStat(ctx, iostatParam.Name, iostatParam.Verbose)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successResponse(req.RequestId, "Pool I/O statistics", map[string]interface{}{
			"iostat": iostat,
		})
	}
}

// handlePoolWait returns a handler for waiting on pool activities
func handlePoolWait(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var cfg pool.WaitConfig
		if err := parseJSONPayload(cmd, &cfg); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if cfg.Pool == "" {
			return nil, errors.New(errors.ServerRequestValidation, "pool name is required")
		}

		ctx := createHandlerContext(req)
		err := h.manager.Wait(ctx, cfg)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Pool wait completed successfully", nil)
	}
}

// handlePoolSplit returns a handler for splitting a mirrored pool
func handlePoolSplit(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var cfg pool.SplitConfig
		if err := parseJSONPayload(cmd, &cfg); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if cfg.Pool == "" || cfg.NewPool == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"pool name and new pool name are required",
			)
		}

		ctx := createHandlerContext(req)
		err := h.manager.Split(ctx, cfg)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Pool split successfully", nil)
	}
}

// handlePoolLabelClear returns a handler for clearing device labels
func handlePoolLabelClear(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var labelParam struct {
			Device string `json:"device"`
			Force  bool   `json:"force,omitempty"`
		}

		if err := parseJSONPayload(cmd, &labelParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if labelParam.Device == "" {
			return nil, errors.New(errors.ServerRequestValidation, "device is required")
		}

		ctx := createHandlerContext(req)
		err := h.manager.LabelClear(ctx, labelParam.Device, labelParam.Force)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Device label cleared successfully", nil)
	}
}

// handlePoolSync returns a handler for syncing pool transactions
func handlePoolSync(h *PoolHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameParam struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &nameParam); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		ctx := createHandlerContext(req)
		err := h.manager.Sync(ctx, nameParam.Name)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successPoolResponse(req.RequestId, "Pool synced successfully", nil)
	}
}
