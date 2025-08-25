// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/stratastor/logger"
	generalCmd "github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/pkg/errors"
)

// CommandExecutor interface for executing system commands
type CommandExecutor interface {
	ExecuteCommand(ctx context.Context, command string, args ...string) (*CommandResult, error)
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	Stdout string
	Stderr string
}

// commandExecutorWrapper wraps the general command executor to match our interface
type commandExecutorWrapper struct {
	executor *generalCmd.CommandExecutor
}

// ExecuteCommand implements the CommandExecutor interface
func (w *commandExecutorWrapper) ExecuteCommand(
	ctx context.Context,
	command string,
	args ...string,
) (*CommandResult, error) {
	output, err := w.executor.ExecuteWithCombinedOutput(ctx, command, args...)
	return &CommandResult{
		Stdout: string(output),
		Stderr: "",
	}, err
}

// InfoCollector collects system information
type InfoCollector struct {
	executor CommandExecutor
	logger   logger.Logger
}

// NewInfoCollector creates a new info collector
func NewInfoCollector(logger logger.Logger) *InfoCollector {
	return &InfoCollector{
		executor: &commandExecutorWrapper{
			executor: generalCmd.NewCommandExecutor(false),
		},
		logger: logger,
	}
}

// GetSystemInfo collects comprehensive system information
func (ic *InfoCollector) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	info := &SystemInfo{
		Timestamp: time.Now(),
	}

	// Collect OS information
	osInfo, err := ic.getOSInfo(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get OS info", "error", err)
	} else {
		info.OS = *osInfo
	}

	// Collect hardware information
	hwInfo, err := ic.getHardwareInfo(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get hardware info", "error", err)
	} else {
		info.Hardware = *hwInfo
	}

	// Collect performance information
	perfInfo, err := ic.getPerformanceInfo(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get performance info", "error", err)
	} else {
		info.Performance = *perfInfo
	}

	// Get current hostname
	hostname, err := ic.getCurrentHostname(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get hostname", "error", err)
	} else {
		info.Hostname = hostname
	}

	// Get timezone and locale
	info.Timezone = ic.getTimezone(ctx)
	info.Locale = ic.getLocale(ctx)

	// Get uptime
	uptime, err := ic.getUptime(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get uptime", "error", err)
	} else {
		info.Uptime = uptime
	}

	return info, nil
}

// getOSInfo collects operating system information
func (ic *InfoCollector) getOSInfo(ctx context.Context) (*OSInfo, error) {
	info := &OSInfo{}

	// Read /etc/os-release
	if err := ic.parseOSRelease(info); err != nil {
		return nil, errors.New(
			errors.ServerInternalError,
			"Failed to parse OS release information: "+err.Error(),
		)
	}

	// Get kernel information
	if err := ic.getKernelInfo(ctx, info); err != nil {
		return nil, errors.New(
			errors.ServerInternalError,
			"Failed to get kernel information: "+err.Error(),
		)
	}

	// Get machine and boot ID
	ic.getMachineAndBootID(info)

	return info, nil
}

// parseOSRelease parses /etc/os-release file
func (ic *InfoCollector) parseOSRelease(info *OSInfo) error {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := parts[0]
			value := strings.Trim(parts[1], `"`)

			switch key {
			case "NAME":
				info.Name = value
			case "VERSION":
				info.Version = value
			case "ID":
				info.ID = value
			case "ID_LIKE":
				info.IDLike = value
			case "VERSION_ID":
				info.VersionID = value
			case "PRETTY_NAME":
				info.PrettyName = value
			}
		}
	}

	return scanner.Err()
}

// getKernelInfo gets kernel information using uname
func (ic *InfoCollector) getKernelInfo(ctx context.Context, info *OSInfo) error {
	// Get kernel name
	result, err := ic.executor.ExecuteCommand(ctx, "uname", "-s")
	if err == nil {
		info.KernelName = strings.TrimSpace(result.Stdout)
	}

	// Get kernel release
	result, err = ic.executor.ExecuteCommand(ctx, "uname", "-r")
	if err == nil {
		info.KernelRelease = strings.TrimSpace(result.Stdout)
	}

	// Get kernel version
	result, err = ic.executor.ExecuteCommand(ctx, "uname", "-v")
	if err == nil {
		info.KernelVersion = strings.TrimSpace(result.Stdout)
	}

	// Get architecture
	result, err = ic.executor.ExecuteCommand(ctx, "uname", "-m")
	if err == nil {
		info.Architecture = strings.TrimSpace(result.Stdout)
	}

	return nil
}

// getMachineAndBootID gets machine ID and boot ID
func (ic *InfoCollector) getMachineAndBootID(info *OSInfo) {
	// Machine ID
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		info.MachineID = strings.TrimSpace(string(data))
	}

	// Boot ID
	if data, err := os.ReadFile("/proc/sys/kernel/random/boot_id"); err == nil {
		info.BootID = strings.TrimSpace(string(data))
	}
}

// getHardwareInfo collects hardware information
func (ic *InfoCollector) getHardwareInfo(ctx context.Context) (*HardwareInfo, error) {
	info := &HardwareInfo{}

	// Get CPU info
	cpuInfo, err := ic.getCPUInfo(ctx)
	if err != nil {
		return nil, err
	}
	info.CPU = *cpuInfo

	// Get memory info
	memInfo, err := ic.getMemoryInfo(ctx)
	if err != nil {
		return nil, err
	}
	info.Memory = *memInfo

	// Get system hardware info from DMI
	sysInfo, err := ic.getSystemHWInfo(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get system hardware info", "error", err)
		info.System = SystemHW{} // Empty struct if DMI info not available
	} else {
		info.System = *sysInfo
	}

	return info, nil
}

// getCPUInfo parses /proc/cpuinfo
func (ic *InfoCollector) getCPUInfo(_ context.Context) (*CPUInfo, error) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info := &CPUInfo{}
	processors := make(map[string]bool)
	coreIDs := make(map[string]bool)
	physicalIDs := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
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
			case "processor":
				processors[value] = true
			}
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
	}

	return info, scanner.Err()
}

// getMemoryInfo parses /proc/meminfo
func (ic *InfoCollector) getMemoryInfo(_ context.Context) (*MemoryInfo, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info := &MemoryInfo{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		valueStr := fields[1]

		// Convert to bytes (values in /proc/meminfo are in kB)
		value, err := strconv.ParseUint(valueStr, 10, 64)
		if err != nil {
			continue
		}
		valueBytes := value * 1024

		switch key {
		case "MemTotal":
			info.Total = valueBytes
		case "MemAvailable":
			info.Available = valueBytes
		case "MemFree":
			info.Free = valueBytes
		case "Cached":
			info.Cached = valueBytes
		case "Buffers":
			info.Buffers = valueBytes
		case "SwapTotal":
			info.SwapTotal = valueBytes
		case "SwapFree":
			info.SwapFree = valueBytes
		}
	}

	// Calculate derived values
	info.Used = info.Total - info.Free - info.Buffers - info.Cached
	info.SwapUsed = info.SwapTotal - info.SwapFree

	if info.Total > 0 {
		info.MemoryPercent = float64(info.Used) / float64(info.Total) * 100
	}
	if info.SwapTotal > 0 {
		info.SwapPercent = float64(info.SwapUsed) / float64(info.SwapTotal) * 100
	}

	return info, scanner.Err()
}

// getSystemHWInfo gets comprehensive system hardware info from DMI
func (ic *InfoCollector) getSystemHWInfo(ctx context.Context) (*SystemHW, error) {
	info := &SystemHW{}

	// Get BIOS information
	biosInfo, err := ic.getBIOSInfo(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get BIOS info", "error", err)
	} else {
		info.BIOS = *biosInfo
	}

	// Get System information
	systemInfo, err := ic.getSystemDMIInfo(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get system DMI info", "error", err)
	} else {
		info.System = *systemInfo
	}

	// Get Baseboard information
	baseboardInfo, err := ic.getBaseboardInfo(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get baseboard info", "error", err)
	} else {
		info.Baseboard = *baseboardInfo
	}

	// Get Chassis information
	chassisInfo, err := ic.getChassisInfo(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get chassis info", "error", err)
	} else {
		info.Chassis = *chassisInfo
	}

	// Get Processor DMI information
	processorInfo, err := ic.getProcessorDMIInfo(ctx)
	if err != nil {
		ic.logger.Warn("Failed to get processor DMI info", "error", err)
	} else {
		info.Processor = *processorInfo
	}

	return info, nil
}

// getBIOSInfo gets BIOS information from DMI
func (ic *InfoCollector) getBIOSInfo(ctx context.Context) (*BIOSInfo, error) {
	info := &BIOSInfo{}

	dmiFields := map[string]*string{
		"bios-vendor":       &info.Vendor,
		"bios-version":      &info.Version,
		"bios-release-date": &info.ReleaseDate,
		"bios-revision":     &info.Revision,
	}

	for dmiKey, field := range dmiFields {
		result, err := ic.executor.ExecuteCommand(ctx, "dmidecode", "-s", dmiKey)
		if err == nil && result.Stdout != "" {
			value := strings.TrimSpace(result.Stdout)
			if ic.isValidDMIValue(value) {
				*field = value
			}
		}
	}

	return info, nil
}

// getSystemDMIInfo gets system information from DMI
func (ic *InfoCollector) getSystemDMIInfo(ctx context.Context) (*SystemHWInfo, error) {
	info := &SystemHWInfo{}

	dmiFields := map[string]*string{
		"system-manufacturer":  &info.Manufacturer,
		"system-product-name":  &info.ProductName,
		"system-version":       &info.Version,
		"system-serial-number": &info.SerialNumber,
		"system-uuid":          &info.UUID,
		"system-sku-number":    &info.SKUNumber,
		"system-family":        &info.Family,
	}

	for dmiKey, field := range dmiFields {
		result, err := ic.executor.ExecuteCommand(ctx, "dmidecode", "-s", dmiKey)
		if err == nil && result.Stdout != "" {
			value := strings.TrimSpace(result.Stdout)
			if ic.isValidDMIValue(value) {
				*field = value
			}
		}
	}

	return info, nil
}

// getBaseboardInfo gets baseboard information from DMI
func (ic *InfoCollector) getBaseboardInfo(ctx context.Context) (*BaseboardInfo, error) {
	info := &BaseboardInfo{}

	dmiFields := map[string]*string{
		"baseboard-manufacturer":  &info.Manufacturer,
		"baseboard-product-name":  &info.ProductName,
		"baseboard-version":       &info.Version,
		"baseboard-serial-number": &info.SerialNumber,
	}

	for dmiKey, field := range dmiFields {
		result, err := ic.executor.ExecuteCommand(ctx, "dmidecode", "-s", dmiKey)
		if err == nil && result.Stdout != "" {
			value := strings.TrimSpace(result.Stdout)
			if ic.isValidDMIValue(value) {
				*field = value
			}
		}
	}

	return info, nil
}

// getChassisInfo gets chassis information from DMI
func (ic *InfoCollector) getChassisInfo(ctx context.Context) (*ChassisInfo, error) {
	info := &ChassisInfo{}

	dmiFields := map[string]*string{
		"chassis-manufacturer":  &info.Manufacturer,
		"chassis-type":          &info.Type,
		"chassis-version":       &info.Version,
		"chassis-serial-number": &info.SerialNumber,
	}

	for dmiKey, field := range dmiFields {
		result, err := ic.executor.ExecuteCommand(ctx, "dmidecode", "-s", dmiKey)
		if err == nil && result.Stdout != "" {
			value := strings.TrimSpace(result.Stdout)
			if ic.isValidDMIValue(value) {
				*field = value
			}
		}
	}

	return info, nil
}

// getProcessorDMIInfo gets processor information from DMI
func (ic *InfoCollector) getProcessorDMIInfo(ctx context.Context) (*ProcessorDMIInfo, error) {
	info := &ProcessorDMIInfo{}

	dmiFields := map[string]*string{
		"processor-family":       &info.Family,
		"processor-manufacturer": &info.Manufacturer,
		"processor-version":      &info.Version,
		"processor-frequency":    &info.Frequency,
	}

	for dmiKey, field := range dmiFields {
		result, err := ic.executor.ExecuteCommand(ctx, "dmidecode", "-s", dmiKey)
		if err == nil && result.Stdout != "" {
			value := strings.TrimSpace(result.Stdout)
			if ic.isValidDMIValue(value) {
				*field = value
			}
		}
	}

	return info, nil
}

// isValidDMIValue checks if DMI value is valid (not placeholder text)
func (ic *InfoCollector) isValidDMIValue(value string) bool {
	invalidValues := []string{
		"Not Specified",
		"To be filled by O.E.M.",
		"Not Available",
		"Unknown",
		"",
	}

	for _, invalid := range invalidValues {
		if value == invalid {
			return false
		}
	}
	return true
}

// getPerformanceInfo collects performance metrics
func (ic *InfoCollector) getPerformanceInfo(ctx context.Context) (*PerformanceInfo, error) {
	info := &PerformanceInfo{}

	// Get load average
	loadAvg, err := ic.getLoadAverage()
	if err == nil {
		info.LoadAverage = *loadAvg
	}

	// Get CPU usage
	cpuUsage, err := ic.getCPUUsage(ctx)
	if err == nil {
		info.CPUUsage = *cpuUsage
	}

	// Get process count
	procCount, err := ic.getProcessCount()
	if err == nil {
		info.ProcessCount = *procCount
	}

	// Get uptime and boot time
	uptime, bootTime, err := ic.getUptimeAndBootTime()
	if err == nil {
		info.UptimeSeconds = uptime
		info.BootTime = bootTime
	}

	return info, nil
}

// getLoadAverage reads load average from /proc/loadavg
func (ic *InfoCollector) getLoadAverage() (*LoadAverage, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid loadavg format")
	}

	load1, err1 := strconv.ParseFloat(fields[0], 64)
	load5, err2 := strconv.ParseFloat(fields[1], 64)
	load15, err3 := strconv.ParseFloat(fields[2], 64)

	if err1 != nil || err2 != nil || err3 != nil {
		return nil, fmt.Errorf("failed to parse load averages")
	}

	return &LoadAverage{
		Load1:  load1,
		Load5:  load5,
		Load15: load15,
	}, nil
}

// getCPUUsage gets CPU usage from /proc/stat
func (ic *InfoCollector) getCPUUsage(_ context.Context) (*CPUUsage, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var cpuLine string
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			cpuLine = line
			break
		}
	}

	if cpuLine == "" {
		return nil, fmt.Errorf("cpu stats not found")
	}

	fields := strings.Fields(cpuLine)
	if len(fields) < 8 {
		return nil, fmt.Errorf("invalid cpu stat format")
	}

	// Parse CPU times
	times := make([]uint64, len(fields)-1)
	for i := 1; i < len(fields); i++ {
		time, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return nil, err
		}
		times[i-1] = time
	}

	// Calculate total time
	var total uint64
	for _, time := range times {
		total += time
	}

	if total == 0 {
		return nil, fmt.Errorf("zero total cpu time")
	}

	usage := &CPUUsage{}
	if len(times) >= 7 {
		usage.User = float64(times[0]) / float64(total) * 100
		// times[1] is nice, combine with user
		usage.User += float64(times[1]) / float64(total) * 100
		usage.System = float64(times[2]) / float64(total) * 100
		usage.Idle = float64(times[3]) / float64(total) * 100
		usage.IOWait = float64(times[4]) / float64(total) * 100
		usage.IRQ = float64(times[5]) / float64(total) * 100
		usage.SoftIRQ = float64(times[6]) / float64(total) * 100

		if len(times) >= 8 {
			usage.Steal = float64(times[7]) / float64(total) * 100
		}
		if len(times) >= 9 {
			usage.Guest = float64(times[8]) / float64(total) * 100
		}
	}

	usage.Total = 100 - usage.Idle

	return usage, nil
}

// getProcessCount gets process statistics from /proc/stat
func (ic *InfoCollector) getProcessCount() (*ProcessCount, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}

	count := &ProcessCount{}
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "processes ") {
			// This is total processes created since boot, not current count
			continue
		}
		if strings.HasPrefix(line, "procs_running ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if running, err := strconv.Atoi(fields[1]); err == nil {
					count.Running = running
				}
			}
		}
		if strings.HasPrefix(line, "procs_blocked ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if blocked, err := strconv.Atoi(fields[1]); err == nil {
					// Blocked processes are effectively sleeping
					count.Sleeping = blocked
				}
			}
		}
	}

	// Get more detailed process count by parsing /proc
	if entries, err := os.ReadDir("/proc"); err == nil {
		processRegex := regexp.MustCompile(`^\d+$`)
		for _, entry := range entries {
			if processRegex.MatchString(entry.Name()) {
				count.Total++
				// Could read /proc/[pid]/stat for more detailed state info
			}
		}
	}

	return count, nil
}

// getUptimeAndBootTime gets system uptime and boot time
func (ic *InfoCollector) getUptimeAndBootTime() (uint64, time.Time, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, time.Time{}, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, time.Time{}, fmt.Errorf("invalid uptime format")
	}

	uptimeFloat, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, time.Time{}, err
	}

	uptime := uint64(uptimeFloat)
	bootTime := time.Now().Add(-time.Duration(uptime) * time.Second)

	return uptime, bootTime, nil
}

// getCurrentHostname gets the current hostname
func (ic *InfoCollector) getCurrentHostname(ctx context.Context) (string, error) {
	result, err := ic.executor.ExecuteCommand(ctx, "hostname")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

// getTimezone gets the current timezone
func (ic *InfoCollector) getTimezone(ctx context.Context) string {
	result, err := ic.executor.ExecuteCommand(
		ctx,
		"timedatectl",
		"show",
		"--property=Timezone",
		"--value",
	)
	if err == nil && result.Stdout != "" {
		return strings.TrimSpace(result.Stdout)
	}

	// Fallback to reading /etc/timezone
	if data, err := os.ReadFile("/etc/timezone"); err == nil {
		return strings.TrimSpace(string(data))
	}

	return "Unknown"
}

// getLocale gets the current locale
func (ic *InfoCollector) getLocale(ctx context.Context) string {
	result, err := ic.executor.ExecuteCommand(ctx, "localectl", "status")
	if err == nil {
		lines := strings.Split(result.Stdout, "\n")
		for _, line := range lines {
			if strings.Contains(line, "LANG=") {
				parts := strings.Split(line, "=")
				if len(parts) >= 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// Fallback to environment variable
	if lang := os.Getenv("LANG"); lang != "" {
		return lang
	}

	return "Unknown"
}

// getUptime gets system uptime as duration
func (ic *InfoCollector) getUptime(_ context.Context) (time.Duration, error) {
	uptime, _, err := ic.getUptimeAndBootTime()
	if err != nil {
		return 0, err
	}
	return time.Duration(uptime) * time.Second, nil
}
