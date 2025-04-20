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

		return successResponse(req.RequestId, "ZFS pools list", pools)
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

		return successResponse(req.RequestId, "Pool status", status)
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
			return nil, errors.New(errors.ServerRequestValidation, "at least one device specification is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Create method
		err := h.manager.Create(ctx, createConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolCreate)
		}

		return successResponse(req.RequestId, "Pool created successfully", nil)
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

		return successResponse(req.RequestId, "Pool destroyed successfully", nil)
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

		return successResponse(req.RequestId, "Pool imported successfully", nil)
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

		return successResponse(req.RequestId, "Pool exported successfully", nil)
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

		return successResponse(req.RequestId, "Pool properties", properties)
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
			return nil, errors.New(errors.ServerRequestValidation, "pool name and property name are required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's GetProperty method
		property, err := h.manager.GetProperty(ctx, propertyParam.Name, propertyParam.Property)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolGetProperty)
		}

		return successResponse(req.RequestId, "Pool property", property)
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
			return nil, errors.New(errors.ServerRequestValidation, "pool name and property name are required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's SetProperty method
		err := h.manager.SetProperty(ctx, setPropertyParam.Name, setPropertyParam.Property, setPropertyParam.Value)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolSetProperty)
		}

		return successResponse(req.RequestId, "Pool property set successfully", nil)
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

		return successResponse(req.RequestId, "Pool scrub "+action+" successfully", nil)
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

		return successResponse(req.RequestId, "Pool resilver started successfully", nil)
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
			return nil, errors.New(errors.ServerRequestValidation, "target and new device paths are required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's AttachDevice method
		err := h.manager.AttachDevice(ctx, attachParam.PoolName, attachParam.TargetDevice, attachParam.NewDevice)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successResponse(req.RequestId, "Device attached successfully", nil)
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

		return successResponse(req.RequestId, "Device detached successfully", nil)
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
			return nil, errors.New(errors.ServerRequestValidation, "old and new device paths are required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's ReplaceDevice method
		err := h.manager.ReplaceDevice(ctx, replaceParam.PoolName, replaceParam.OldDevice, replaceParam.NewDevice)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
		}

		return successResponse(req.RequestId, "Device replaced successfully", nil)
	}
}