// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
)

// DeploymentEnvironment represents the type of deployment environment
type DeploymentEnvironment string

const (
	// DeploymentPhysical indicates bare metal deployment
	DeploymentPhysical DeploymentEnvironment = "physical"
	// DeploymentVirtualRestricted indicates virtualized environment with networking restrictions (e.g., AWS EC2)
	DeploymentVirtualRestricted DeploymentEnvironment = "virtual-restricted"
	// DeploymentVirtualOpen indicates virtualized environment with MACVLAN support (e.g., VMware with promiscuous mode)
	DeploymentVirtualOpen DeploymentEnvironment = "virtual-open"
	// DeploymentUnknown indicates unknown environment
	DeploymentUnknown DeploymentEnvironment = "unknown"
)

// EnvironmentInfo contains detailed information about the deployment environment
type EnvironmentInfo struct {
	Type                   DeploymentEnvironment `json:"type"`
	IsVirtualized          bool                  `json:"is_virtualized"`
	Hypervisor             string                `json:"hypervisor,omitempty"`
	SupportsMACVLAN        bool                  `json:"supports_macvlan"`
	KernelVersion          string                `json:"kernel_version"`
	KernelVersionMajor     int                   `json:"kernel_version_major"`
	KernelVersionMinor     int                   `json:"kernel_version_minor"`
	CloudProvider          string                `json:"cloud_provider,omitempty"`
	RecommendedNetworkMode string                `json:"recommended_network_mode"` // "host" or "macvlan"
}

// EnvironmentDetector detects deployment environment characteristics
type EnvironmentDetector struct {
	executor CommandExecutor
	logger   logger.Logger
}

// NewEnvironmentDetector creates a new environment detector
func NewEnvironmentDetector(logger logger.Logger) *EnvironmentDetector {
	return &EnvironmentDetector{
		executor: &commandExecutorWrapper{
			executor: NewInfoCollector(logger).executor.(*commandExecutorWrapper).executor,
		},
		logger: logger,
	}
}

// DetectEnvironment detects the deployment environment and its capabilities
func (ed *EnvironmentDetector) DetectEnvironment(ctx context.Context) (*EnvironmentInfo, error) {
	info := &EnvironmentInfo{}

	// Detect virtualization
	isVirt, hypervisor, err := ed.detectVirtualization(ctx)
	if err != nil {
		ed.logger.Warn("Failed to detect virtualization", "error", err)
	}
	info.IsVirtualized = isVirt
	info.Hypervisor = hypervisor

	// Get kernel version for MACVLAN support check
	kernelVer, major, minor, err := ed.getKernelVersion(ctx)
	if err != nil {
		ed.logger.Warn("Failed to get kernel version", "error", err)
	}
	info.KernelVersion = kernelVer
	info.KernelVersionMajor = major
	info.KernelVersionMinor = minor

	// Detect cloud provider
	cloudProvider := ed.detectCloudProvider(ctx)
	info.CloudProvider = cloudProvider

	// Determine MACVLAN support
	info.SupportsMACVLAN = ed.determineMACVLANSupport(major, minor, cloudProvider, hypervisor)

	// Determine environment type and recommendation
	info.Type = ed.determineEnvironmentType(isVirt, cloudProvider, hypervisor, info.SupportsMACVLAN)
	info.RecommendedNetworkMode = ed.recommendNetworkMode(info.Type, info.SupportsMACVLAN)

	ed.logger.Info("Environment detected",
		"type", info.Type,
		"virtualized", info.IsVirtualized,
		"hypervisor", info.Hypervisor,
		"cloud_provider", info.CloudProvider,
		"supports_macvlan", info.SupportsMACVLAN,
		"recommended_mode", info.RecommendedNetworkMode)

	return info, nil
}

// detectVirtualization checks if the system is virtualized and identifies the hypervisor
func (ed *EnvironmentDetector) detectVirtualization(ctx context.Context) (bool, string, error) {
	// Try systemd-detect-virt first (most reliable)
	result, err := ed.executor.ExecuteCommand(ctx, "systemd-detect-virt")
	if err == nil {
		virtType := strings.TrimSpace(result.Stdout)
		if virtType != "none" && virtType != "" {
			return true, virtType, nil
		}
		return false, "", nil
	}

	// Fallback: Check DMI/SMBIOS
	result, err = ed.executor.ExecuteCommand(ctx, "sudo", "dmidecode", "-s", "system-product-name")
	if err == nil {
		productName := strings.TrimSpace(strings.ToLower(result.Stdout))

		// Check for known virtualization products
		virtProducts := map[string]string{
			"vmware":           "vmware",
			"virtualbox":       "virtualbox",
			"kvm":              "kvm",
			"qemu":             "qemu",
			"xen":              "xen",
			"microsoft":        "hyperv",
			"amazon ec2":       "amazon",
			"google compute":   "google",
			"azure":            "azure",
		}

		for key, hypervisor := range virtProducts {
			if strings.Contains(productName, key) {
				return true, hypervisor, nil
			}
		}
	}

	// Check /proc/cpuinfo for hypervisor flag
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		if strings.Contains(string(data), "hypervisor") {
			return true, "unknown", nil
		}
	}

	return false, "", nil
}

// getKernelVersion gets the kernel version and parses major/minor version numbers
func (ed *EnvironmentDetector) getKernelVersion(ctx context.Context) (string, int, int, error) {
	result, err := ed.executor.ExecuteCommand(ctx, "uname", "-r")
	if err != nil {
		return "", 0, 0, errors.Wrap(err, errors.SystemInfoCollectionFailed)
	}

	kernelVer := strings.TrimSpace(result.Stdout)

	// Parse version (e.g., "5.15.0-91-generic" -> major=5, minor=15)
	parts := strings.Split(kernelVer, ".")
	if len(parts) < 2 {
		return kernelVer, 0, 0, fmt.Errorf("invalid kernel version format: %s", kernelVer)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return kernelVer, 0, 0, fmt.Errorf("failed to parse major version: %w", err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return kernelVer, 0, 0, fmt.Errorf("failed to parse minor version: %w", err)
	}

	return kernelVer, major, minor, nil
}

// detectCloudProvider attempts to identify the cloud provider
func (ed *EnvironmentDetector) detectCloudProvider(ctx context.Context) string {
	// Check DMI product name
	result, err := ed.executor.ExecuteCommand(ctx, "sudo", "dmidecode", "-s", "system-product-name")
	if err == nil {
		productName := strings.TrimSpace(strings.ToLower(result.Stdout))

		if strings.Contains(productName, "amazon") || strings.Contains(productName, "ec2") {
			return "aws"
		}
		if strings.Contains(productName, "google") {
			return "gcp"
		}
		if strings.Contains(productName, "microsoft") || strings.Contains(productName, "azure") {
			return "azure"
		}
	}

	// Check for cloud-init or cloud metadata
	if _, err := os.Stat("/run/cloud-init"); err == nil {
		// Try to determine which cloud
		if data, err := os.ReadFile("/sys/class/dmi/id/product_name"); err == nil {
			productName := strings.TrimSpace(strings.ToLower(string(data)))
			if strings.Contains(productName, "amazon") {
				return "aws"
			}
		}
	}

	return ""
}

// determineMACVLANSupport checks if MACVLAN is supported in this environment
func (ed *EnvironmentDetector) determineMACVLANSupport(
	kernelMajor, kernelMinor int,
	cloudProvider, hypervisor string,
) bool {
	// Kernel version check (need >= 3.9, recommended >= 4.0)
	if kernelMajor < 3 {
		return false
	}
	if kernelMajor == 3 && kernelMinor < 9 {
		return false
	}

	// Cloud providers that restrict MACVLAN
	restrictedProviders := []string{"aws", "gcp", "azure"}
	for _, provider := range restrictedProviders {
		if cloudProvider == provider {
			ed.logger.Debug("MACVLAN not supported on cloud provider", "provider", cloudProvider)
			return false
		}
	}

	// VMware and KVM support MACVLAN with proper configuration
	// VirtualBox is unreliable with MACVLAN
	if hypervisor == "virtualbox" {
		ed.logger.Debug("MACVLAN unreliable on VirtualBox")
		return false
	}

	// If virtualized but not a restricted cloud provider, assume MACVLAN is possible
	// (e.g., VMware, KVM on-premise)
	return true
}

// determineEnvironmentType determines the overall environment type
func (ed *EnvironmentDetector) determineEnvironmentType(
	isVirtualized bool,
	cloudProvider, hypervisor string,
	supportsMACVLAN bool,
) DeploymentEnvironment {
	if !isVirtualized {
		return DeploymentPhysical
	}

	// Restricted cloud providers
	if cloudProvider == "aws" || cloudProvider == "gcp" || cloudProvider == "azure" {
		return DeploymentVirtualRestricted
	}

	// Virtualized but supports MACVLAN (VMware, KVM, etc.)
	if supportsMACVLAN {
		return DeploymentVirtualOpen
	}

	// Virtualized but unknown MACVLAN support - be conservative
	return DeploymentVirtualRestricted
}

// recommendNetworkMode recommends the network mode based on environment type
func (ed *EnvironmentDetector) recommendNetworkMode(
	envType DeploymentEnvironment,
	supportsMACVLAN bool,
) string {
	switch envType {
	case DeploymentPhysical:
		return "macvlan"
	case DeploymentVirtualOpen:
		return "macvlan"
	case DeploymentVirtualRestricted:
		return "host"
	default:
		// Conservative default
		return "host"
	}
}
