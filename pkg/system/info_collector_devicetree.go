// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"os"
	"strings"
)

const deviceTreeBasePath = "/proc/device-tree"

// getSystemInfoFromDeviceTree reads system information from Device Tree (ARM systems)
func (ic *InfoCollector) getSystemInfoFromDeviceTree(_ context.Context) (*SystemHW, error) {
	info := &SystemHW{}

	// Check if Device Tree is available
	if _, err := os.Stat(deviceTreeBasePath); os.IsNotExist(err) {
		return nil, err
	}

	// Read model (provides product name and version info)
	if model, err := ic.readDeviceTreeFile("model"); err == nil {
		info.System.ProductName = model
		info.Chassis.Type = "Embedded" // ARM systems are typically embedded
	}

	// Read serial number
	if serial, err := ic.readDeviceTreeFile("serial-number"); err == nil {
		info.System.SerialNumber = serial
	}

	// Read compatible string (provides manufacturer and chip info)
	if compatible, err := ic.readDeviceTreeFile("compatible"); err == nil {
		// Parse compatible string (format: "vendor,model\0chip-vendor,chip")
		parts := strings.Split(compatible, "\x00")
		if len(parts) > 0 && parts[0] != "" {
			// Extract vendor from first compatible entry (e.g., "raspberrypi,5-model-b")
			vendorModel := strings.SplitN(parts[0], ",", 2)
			if len(vendorModel) > 0 {
				vendor := ic.normalizeVendorName(vendorModel[0])
				info.System.Manufacturer = vendor
				info.Baseboard.Manufacturer = vendor
				info.Chassis.Manufacturer = vendor
			}
		}

		// If we have multiple compatible entries, use the last one for chip info
		if len(parts) > 1 && parts[1] != "" {
			chipInfo := strings.SplitN(parts[1], ",", 2)
			if len(chipInfo) == 2 {
				// Store chip info in version field (e.g., "brcm,bcm2712")
				info.Baseboard.Version = chipInfo[1]
			}
		}
	}

	return info, nil
}

// readDeviceTreeFile reads a file from the Device Tree
func (ic *InfoCollector) readDeviceTreeFile(relativePath string) (string, error) {
	filePath := deviceTreeBasePath + "/" + relativePath
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// For files that we know should be text, validate and clean
	// Device Tree strings are null-terminated and may contain null separators
	value := string(data)

	// Validate that the content looks like valid text (not pure binary)
	// Check if at least 80% of the content is printable ASCII
	if !ic.looksLikeText(data) {
		return "", nil // Skip binary data
	}

	// Trim trailing null bytes and whitespace
	value = strings.TrimRight(value, "\x00\n\r\t ")
	value = strings.TrimSpace(value)

	return value, nil
}

// looksLikeText performs a heuristic check to determine if data looks like text
func (ic *InfoCollector) looksLikeText(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Count printable characters (including null bytes which are valid separators in DT)
	printableCount := 0
	for _, b := range data {
		// Printable ASCII (32-126), null (0), or common whitespace
		if (b >= 32 && b <= 126) || b == 0 || b == '\n' || b == '\r' || b == '\t' {
			printableCount++
		}
	}

	// If less than 80% is "text-like", treat it as binary
	threshold := float64(len(data)) * 0.8
	return float64(printableCount) >= threshold
}

// normalizeVendorName converts device tree vendor prefixes to readable names
func (ic *InfoCollector) normalizeVendorName(vendor string) string {
	// Map of common Device Tree vendor prefixes to full names
	vendorMap := map[string]string{
		"raspberrypi": "Raspberry Pi",
		"broadcom":    "Broadcom",
		"brcm":        "Broadcom",
		"nvidia":      "NVIDIA",
		"qcom":        "Qualcomm",
		"samsung":     "Samsung",
		"ti":          "Texas Instruments",
		"rockchip":    "Rockchip",
		"amlogic":     "Amlogic",
		"allwinner":   "Allwinner",
		"marvell":     "Marvell",
		"mediatek":    "MediaTek",
		"apple":       "Apple",
	}

	if fullName, ok := vendorMap[strings.ToLower(vendor)]; ok {
		return fullName
	}

	// Capitalize first letter if not in map
	if len(vendor) > 0 {
		return strings.ToUpper(vendor[:1]) + vendor[1:]
	}

	return vendor
}
