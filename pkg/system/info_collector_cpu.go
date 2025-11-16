// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// getCPUInfo collects CPU information using lscpu (preferred) with /proc/cpuinfo fallback
func (ic *InfoCollector) getCPUInfo(ctx context.Context) (*CPUInfo, error) {
	// Try lscpu first (preferred method - architecture-agnostic, no hardcoded mappings)
	info, err := ic.getCPUInfoFromLscpu(ctx)
	if err == nil {
		return info, nil
	}

	// Log lscpu failure and fall back to /proc/cpuinfo
	ic.logger.Debug("lscpu failed, falling back to /proc/cpuinfo", "error", err)
	return ic.getCPUInfoFromProc(ctx)
}

// getCPUInfoFromLscpu parses standard lscpu output (preferred method)
func (ic *InfoCollector) getCPUInfoFromLscpu(ctx context.Context) (*CPUInfo, error) {
	result, err := ic.executor.ExecuteCommand(ctx, "lscpu")
	if err != nil {
		return nil, fmt.Errorf("lscpu command failed: %w", err)
	}

	info := &CPUInfo{}
	var caches []string // Collect cache info
	scanner := bufio.NewScanner(strings.NewReader(result.Stdout))

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Skip empty values and "-" (used for N/A fields)
		if value == "" || value == "-" {
			continue
		}

		switch key {
		case "Architecture":
			info.Architecture = value
		case "Vendor ID":
			info.Vendor = value
		case "Model name":
			info.ModelName = value
		case "CPU family":
			info.Family = value
		case "Model":
			info.Model = value
		case "Stepping":
			info.Stepping = value
		case "CPU(s)":
			if count, err := strconv.Atoi(value); err == nil {
				info.ProcessorCount = count
			}
		case "Thread(s) per core":
			if threads, err := strconv.Atoi(value); err == nil {
				info.ThreadsPerCore = threads
			}
		case "Core(s) per socket", "Core(s) per cluster":
			if cores, err := strconv.Atoi(value); err == nil {
				info.CoresPerSocket = cores
			}
		case "Socket(s)":
			if sockets, err := strconv.Atoi(value); err == nil {
				info.Sockets = sockets
			}
		case "CPU max MHz", "CPU MHz":
			if mhz, err := strconv.ParseFloat(value, 64); err == nil && info.CPUMHz == 0 {
				info.CPUMHz = mhz
			}
		case "BogoMIPS":
			// Use BogoMIPS if no MHz value found (ARM systems)
			if bogomips, err := strconv.ParseFloat(value, 64); err == nil && info.CPUMHz == 0 {
				info.CPUMHz = bogomips
			}
		case "Flags":
			info.Flags = strings.Fields(value)
		case "L1d cache", "L1i cache", "L2 cache", "L3 cache":
			// Collect cache info in format "L1d: 32 KiB, L2: 1 MiB"
			cacheName := strings.TrimSuffix(key, " cache")
			caches = append(caches, fmt.Sprintf("%s: %s", cacheName, value))
		}
	}

	// Combine cache info into single string
	if len(caches) > 0 {
		info.CacheSize = strings.Join(caches, ", ")
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error parsing lscpu output: %w", err)
	}

	// Calculate derived values
	if info.CPUCores == 0 && info.CoresPerSocket > 0 {
		info.CPUCores = info.CoresPerSocket
		if info.Sockets > 0 {
			info.CPUCores *= info.Sockets
		}
	}

	// For ARM systems without sockets (using clusters), set sockets to 1
	if info.Sockets == 0 {
		info.Sockets = 1
	}

	// Validate we got essential information
	if info.ModelName == "" && info.Vendor == "" {
		return nil, fmt.Errorf("lscpu output missing essential CPU information")
	}

	return info, nil
}

// getCPUInfoFromProc parses /proc/cpuinfo (fallback method for when lscpu unavailable)
func (ic *InfoCollector) getCPUInfoFromProc(_ context.Context) (*CPUInfo, error) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info := &CPUInfo{}
	processors := make(map[string]bool)
	coreIDs := make(map[string]bool)
	physicalIDs := make(map[string]bool)

	// ARM-specific fields (needed because /proc/cpuinfo uses codes instead of names)
	var cpuImplementer, cpuArchitecture, cpuPart, cpuVariant string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			// x86/x86_64 fields
			case "model name":
				if info.ModelName == "" {
					info.ModelName = value
				}
			case "vendor_id":
				if info.Vendor == "" {
					info.Vendor = value
				}
			case "cpu family":
				if info.Family == "" {
					info.Family = value
				}
			case "model":
				if info.Model == "" {
					info.Model = value
				}
			case "stepping":
				if info.Stepping == "" {
					info.Stepping = value
				}
			case "microcode":
				if info.Microcode == "" {
					info.Microcode = value
				}
			case "cpu MHz":
				if info.CPUMHz == 0 {
					if mhz, err := strconv.ParseFloat(value, 64); err == nil {
						info.CPUMHz = mhz
					}
				}
			case "cache size":
				if info.CacheSize == "" {
					info.CacheSize = value
				}
			case "physical id":
				if info.PhysicalID == "" {
					info.PhysicalID = value
				}
				physicalIDs[value] = true
			case "siblings":
				if info.Siblings == 0 {
					if siblings, err := strconv.Atoi(value); err == nil {
						info.Siblings = siblings
					}
				}
			case "core id":
				if info.CoreID == "" {
					info.CoreID = value
				}
				coreIDs[value] = true
			case "cpu cores":
				if info.CPUCores == 0 {
					if cores, err := strconv.Atoi(value); err == nil {
						info.CPUCores = cores
					}
				}
			case "apicid":
				if info.ApicID == "" {
					info.ApicID = value
				}
			case "flags":
				if len(info.Flags) == 0 {
					info.Flags = strings.Fields(value)
				}

			// ARM-specific fields
			case "CPU implementer":
				cpuImplementer = value
			case "CPU architecture":
				cpuArchitecture = value
			case "CPU part":
				cpuPart = value
			case "CPU variant":
				cpuVariant = value
			case "Features":
				// ARM equivalent of x86 flags
				if len(info.Flags) == 0 {
					info.Flags = strings.Fields(value)
				}
			case "BogoMIPS":
				// ARM systems use BogoMIPS instead of MHz
				if info.CPUMHz == 0 {
					if bogomips, err := strconv.ParseFloat(value, 64); err == nil {
						info.CPUMHz = bogomips
					}
				}

			// Common fields
			case "processor":
				processors[value] = true
			}
		}
	}

	// Process ARM-specific information if present
	if cpuImplementer != "" {
		info.Vendor = getARMVendor(cpuImplementer)
		info.ModelName = getARMModelName(cpuPart, cpuImplementer)
		info.Family = "ARMv" + cpuArchitecture
		info.Model = cpuPart
		if cpuVariant != "" {
			info.Stepping = cpuVariant
		}
	}

	// Calculate derived values
	info.ProcessorCount = len(processors)
	info.Sockets = len(physicalIDs)
	if info.Sockets == 0 {
		info.Sockets = 1
	}

	if info.CPUCores > 0 {
		info.CoresPerSocket = info.CPUCores
		if info.Siblings > 0 {
			info.ThreadsPerCore = info.Siblings / info.CPUCores
		}
	} else if info.ProcessorCount > 0 {
		// For ARM, assume each processor is a core if not specified
		info.CPUCores = info.ProcessorCount
		info.CoresPerSocket = info.ProcessorCount
		info.ThreadsPerCore = 1
	}

	return info, scanner.Err()
}

// getARMVendor maps ARM implementer codes to vendor names (fallback /proc/cpuinfo only)
func getARMVendor(implementer string) string {
	vendors := map[string]string{
		"0x41": "ARM",
		"0x42": "Broadcom",
		"0x43": "Cavium",
		"0x44": "DEC",
		"0x4e": "Nvidia",
		"0x50": "APM",
		"0x51": "Qualcomm",
		"0x53": "Samsung",
		"0x56": "Marvell",
		"0x61": "Apple",
		"0x66": "Faraday",
		"0x69": "Intel",
	}

	if vendor, ok := vendors[implementer]; ok {
		return vendor
	}
	return "ARM (unknown)"
}

// getARMModelName maps ARM CPU part codes to model names (fallback /proc/cpuinfo only)
func getARMModelName(part, implementer string) string {
	// ARM Ltd cores (implementer 0x41)
	if implementer == "0x41" {
		models := map[string]string{
			"0xd02": "Cortex-A34",
			"0xd04": "Cortex-A35",
			"0xd03": "Cortex-A53",
			"0xd05": "Cortex-A55",
			"0xd46": "Cortex-A510",
			"0xd07": "Cortex-A57",
			"0xd08": "Cortex-A72",
			"0xd09": "Cortex-A73",
			"0xd0a": "Cortex-A75",
			"0xd0b": "Cortex-A76",
			"0xd0d": "Cortex-A77",
			"0xd0e": "Cortex-A76AE",
			"0xd40": "Neoverse-V1",
			"0xd41": "Cortex-A78",
			"0xd42": "Cortex-A78AE",
			"0xd43": "Cortex-A65AE",
			"0xd44": "Cortex-X1",
			"0xd47": "Cortex-A710",
			"0xd48": "Cortex-X2",
			"0xd49": "Neoverse-N2",
			"0xd4a": "Neoverse-E1",
			"0xd4b": "Cortex-A78C",
			"0xd4c": "Cortex-X1C",
			"0xd4d": "Cortex-A715",
			"0xd4e": "Cortex-X3",
		}

		if model, ok := models[part]; ok {
			return model
		}
	}

	// Apple cores (implementer 0x61)
	if implementer == "0x61" {
		models := map[string]string{
			"0x020": "Icestorm-A14",
			"0x021": "Firestorm-A14",
			"0x022": "Icestorm-M1",
			"0x023": "Firestorm-M1",
			"0x024": "Icestorm-M1-Pro",
			"0x025": "Firestorm-M1-Pro",
			"0x028": "Icestorm-M1-Max",
			"0x029": "Firestorm-M1-Max",
			"0x030": "Blizzard-A15",
			"0x031": "Avalanche-A15",
			"0x032": "Blizzard-M2",
			"0x033": "Avalanche-M2",
		}

		if model, ok := models[part]; ok {
			return model
		}
	}

	return fmt.Sprintf("ARM CPU (part %s)", part)
}
