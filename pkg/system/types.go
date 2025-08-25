// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"time"
)

// SystemInfo represents comprehensive system information
type SystemInfo struct {
	OS          OSInfo          `json:"os"`
	Hardware    HardwareInfo    `json:"hardware"`
	Performance PerformanceInfo `json:"performance"`
	Hostname    string          `json:"hostname"`
	Timezone    string          `json:"timezone"`
	Locale      string          `json:"locale"`
	Uptime      time.Duration   `json:"uptime"`
	Timestamp   time.Time       `json:"timestamp"`
}

// OSInfo represents operating system information
type OSInfo struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	ID           string `json:"id"`
	IDLike       string `json:"id_like"`
	VersionID    string `json:"version_id"`
	PrettyName   string `json:"pretty_name"`
	KernelName   string `json:"kernel_name"`
	KernelRelease string `json:"kernel_release"`
	KernelVersion string `json:"kernel_version"`
	Architecture  string `json:"architecture"`
	MachineID    string `json:"machine_id"`
	BootID       string `json:"boot_id"`
}

// HardwareInfo represents hardware information
type HardwareInfo struct {
	CPU    CPUInfo    `json:"cpu"`
	Memory MemoryInfo `json:"memory"`
	System SystemHW   `json:"system"`
}

// CPUInfo represents CPU information
type CPUInfo struct {
	ModelName    string  `json:"model_name"`
	Vendor       string  `json:"vendor"`
	Family       string  `json:"family"`
	Model        string  `json:"model"`
	Stepping     string  `json:"stepping"`
	Microcode    string  `json:"microcode"`
	CPUMHz       float64 `json:"cpu_mhz"`
	CacheSize    string  `json:"cache_size"`
	PhysicalID   string  `json:"physical_id"`
	Siblings     int     `json:"siblings"`
	CoreID       string  `json:"core_id"`
	CPUCores     int     `json:"cpu_cores"`
	ApicID       string  `json:"apic_id"`
	Flags        []string `json:"flags"`
	ProcessorCount int   `json:"processor_count"`
	ThreadsPerCore int   `json:"threads_per_core"`
	CoresPerSocket int   `json:"cores_per_socket"`
	Sockets        int   `json:"sockets"`
}

// MemoryInfo represents memory information
type MemoryInfo struct {
	Total          uint64  `json:"total"`           // Total memory in bytes
	Available      uint64  `json:"available"`       // Available memory in bytes
	Used           uint64  `json:"used"`            // Used memory in bytes
	Free           uint64  `json:"free"`            // Free memory in bytes
	Cached         uint64  `json:"cached"`          // Cached memory in bytes
	Buffers        uint64  `json:"buffers"`         // Buffered memory in bytes
	SwapTotal      uint64  `json:"swap_total"`      // Total swap in bytes
	SwapUsed       uint64  `json:"swap_used"`       // Used swap in bytes
	SwapFree       uint64  `json:"swap_free"`       // Free swap in bytes
	MemoryPercent  float64 `json:"memory_percent"`  // Memory usage percentage
	SwapPercent    float64 `json:"swap_percent"`    // Swap usage percentage
}

// SystemHW represents system hardware information
type SystemHW struct {
	BIOS       BIOSInfo         `json:"bios"`
	System     SystemHWInfo     `json:"system"`
	Baseboard  BaseboardInfo    `json:"baseboard"`
	Chassis    ChassisInfo      `json:"chassis"`
	Processor  ProcessorDMIInfo `json:"processor"`
}

// BIOSInfo represents BIOS information from DMI
type BIOSInfo struct {
	Vendor      string `json:"vendor"`
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
	Revision    string `json:"revision"`
}

// SystemHWInfo represents system information from DMI
type SystemHWInfo struct {
	Manufacturer string `json:"manufacturer"`
	ProductName  string `json:"product_name"`
	Version      string `json:"version"`
	SerialNumber string `json:"serial_number"`
	UUID         string `json:"uuid"`
	SKUNumber    string `json:"sku_number"`
	Family       string `json:"family"`
}

// BaseboardInfo represents baseboard/motherboard information from DMI
type BaseboardInfo struct {
	Manufacturer string `json:"manufacturer"`
	ProductName  string `json:"product_name"`
	Version      string `json:"version"`
	SerialNumber string `json:"serial_number"`
}

// ChassisInfo represents chassis information from DMI
type ChassisInfo struct {
	Manufacturer string `json:"manufacturer"`
	Type         string `json:"type"`
	Version      string `json:"version"`
	SerialNumber string `json:"serial_number"`
}

// ProcessorDMIInfo represents processor information from DMI
type ProcessorDMIInfo struct {
	Family       string `json:"family"`
	Manufacturer string `json:"manufacturer"`
	Version      string `json:"version"`
	Frequency    string `json:"frequency"`
}

// PerformanceInfo represents system performance metrics
type PerformanceInfo struct {
	LoadAverage    LoadAverage    `json:"load_average"`
	CPUUsage       CPUUsage       `json:"cpu_usage"`
	ProcessCount   ProcessCount   `json:"process_count"`
	UptimeSeconds  uint64         `json:"uptime_seconds"`
	BootTime       time.Time      `json:"boot_time"`
}

// LoadAverage represents system load averages
type LoadAverage struct {
	Load1  float64 `json:"load1"`   // 1-minute load average
	Load5  float64 `json:"load5"`   // 5-minute load average
	Load15 float64 `json:"load15"`  // 15-minute load average
}

// CPUUsage represents CPU usage statistics
type CPUUsage struct {
	User    float64 `json:"user"`     // User CPU time percentage
	System  float64 `json:"system"`   // System CPU time percentage
	Idle    float64 `json:"idle"`     // Idle CPU time percentage
	IOWait  float64 `json:"iowait"`   // IO wait time percentage
	IRQ     float64 `json:"irq"`      // Hardware interrupt time percentage
	SoftIRQ float64 `json:"softirq"`  // Software interrupt time percentage
	Steal   float64 `json:"steal"`    // Stolen time percentage (virtualization)
	Guest   float64 `json:"guest"`    // Guest time percentage (virtualization)
	Total   float64 `json:"total"`    // Total CPU usage percentage
}

// ProcessCount represents process statistics
type ProcessCount struct {
	Running  int `json:"running"`   // Running processes
	Sleeping int `json:"sleeping"`  // Sleeping processes
	Stopped  int `json:"stopped"`   // Stopped processes
	Zombie   int `json:"zombie"`    // Zombie processes
	Total    int `json:"total"`     // Total processes
}

// HostnameInfo represents hostname information
type HostnameInfo struct {
	Static      string `json:"static"`       // Static hostname
	Transient   string `json:"transient"`    // Transient hostname
	Pretty      string `json:"pretty"`       // Pretty hostname
	IconName    string `json:"icon_name"`    // Icon name
	Chassis     string `json:"chassis"`      // Chassis type
	MachineID   string `json:"machine_id"`   // Machine ID
	BootID      string `json:"boot_id"`      // Boot ID
}

// User represents a system user
type User struct {
	Username    string   `json:"username"`
	UID         int      `json:"uid"`
	GID         int      `json:"gid"`
	FullName    string   `json:"full_name"`
	HomeDir     string   `json:"home_dir"`
	Shell       string   `json:"shell"`
	Groups      []string `json:"groups"`
	Locked      bool     `json:"locked"`
	LastLogin   *time.Time `json:"last_login,omitempty"`
	CreateTime  *time.Time `json:"create_time,omitempty"`
}

// Group represents a system group
type Group struct {
	Name     string   `json:"name"`
	GID      int      `json:"gid"`
	Members  []string `json:"members"`
}

// CreateUserRequest represents a request to create a user
type CreateUserRequest struct {
	Username    string   `json:"username"    binding:"required"`
	FullName    string   `json:"full_name"`
	HomeDir     string   `json:"home_dir"`
	Shell       string   `json:"shell"`
	Groups      []string `json:"groups"`
	CreateHome  bool     `json:"create_home"`
	SystemUser  bool     `json:"system_user"`
	Password    string   `json:"password,omitempty"`
}

// CreateGroupRequest represents a request to create a group
type CreateGroupRequest struct {
	Name        string `json:"name"         binding:"required"`
	SystemGroup bool   `json:"system_group"`
}

// SetHostnameRequest represents a request to set hostname
type SetHostnameRequest struct {
	Hostname string `json:"hostname" binding:"required"`
	Pretty   string `json:"pretty"`
	Static   bool   `json:"static"`
}

// SetTimezoneRequest represents a request to set timezone
type SetTimezoneRequest struct {
	Timezone string `json:"timezone" binding:"required"`
}

// SetLocaleRequest represents a request to set locale
type SetLocaleRequest struct {
	Locale string `json:"locale" binding:"required"`
}

// PowerOperationRequest represents a power management request
type PowerOperationRequest struct {
	Force   bool `json:"force"`
	Message string `json:"message"`
}