/*
 * Copyright 2024-2025 Raamsri Kumar <raam@tinkershack.in> 
 * Copyright 2024-2025 The StrataSTOR Authors and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/rand"
)

type LoopDevice struct {
	File   *os.File
	Device string
	Number int
}

type TestEnv struct {
	Devices []*LoopDevice
}

const (
	// TestPoolPrefix is used as prefix for test pool names
	TestPoolPrefix = "test"

	// TestPoolNameLength is the length of random suffix
	TestPoolNameLength = 6

	// Chars used for random name generation
	poolNameChars = "abcdefghijklmnopqrstuvwxyz0123456789"

	LoopDeviceSize = 64 // required minimum size in MB
)

// GeneratePoolName creates a unique pool name for testing
func GeneratePoolName() string {
	rand.Seed(uint64(time.Now().UnixNano()))
	suffix := make([]byte, TestPoolNameLength)
	for i := range suffix {
		suffix[i] = poolNameChars[rand.Intn(len(poolNameChars))]
	}
	return fmt.Sprintf("%s-%s", TestPoolPrefix, string(suffix))
}

func CreateLoopDevice(t *testing.T, size int64) (*LoopDevice, error) {
	// Create temporary file
	f, err := os.CreateTemp("", "zfs-test-*.img")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}

	// Truncate to desired size
	if err := exec.Command("truncate", "-s", fmt.Sprintf("%dM", size), f.Name()).Run(); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("failed to create disk image: %v", err)
	}

	// Setup loopback device
	out, err := exec.Command("losetup", "-f", "--show", f.Name()).Output()
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("failed to setup loop device: %v", err)
	}

	device := strings.TrimSpace(string(out))
	number := -1
	fmt.Sscanf(device, "/dev/loop%d", &number)

	return &LoopDevice{
		File:   f,
		Device: device,
		Number: number,
	}, nil
}

func (l *LoopDevice) Cleanup() error {
	if l.Device != "" {
		if err := exec.Command("losetup", "-d", l.Device).Run(); err != nil {
			return fmt.Errorf("failed to detach loop device: %v", err)
		}
	}
	if l.File != nil {
		if err := l.File.Close(); err != nil {
			return fmt.Errorf("failed to close file: %v", err)
		}
		if err := os.Remove(l.File.Name()); err != nil {
			return fmt.Errorf("failed to remove file: %v", err)
		}
	}
	return nil
}

func NewTestEnv(t *testing.T, diskCount int) *TestEnv {
	env := &TestEnv{
		Devices: make([]*LoopDevice, diskCount),
	}

	for i := 0; i < diskCount; i++ {
		device, err := CreateLoopDevice(t, LoopDeviceSize) // 64MB
		if err != nil {
			// Cleanup already created devices
			for j := 0; j < i; j++ {
				if env.Devices[j] != nil {
					env.Devices[j].Cleanup()
				}
			}
			t.Fatalf("failed to create loop device %d: %v", i, err)
		}
		env.Devices[i] = device
	}

	return env
}

func (e *TestEnv) GetLoopDevices() []string {
	devices := make([]string, len(e.Devices))
	for i, d := range e.Devices {
		devices[i] = d.Device
	}
	return devices
}

func (e *TestEnv) Cleanup() {
	// Clean up all devices in reverse order
	for i := len(e.Devices) - 1; i >= 0; i-- {
		if e.Devices[i] != nil {
			// First detach the loop device
			if e.Devices[i].Device != "" {
				if err := exec.Command("losetup", "-d", e.Devices[i].Device).Run(); err != nil {
					// Log error but continue cleanup
					fmt.Printf("failed to detach loop device %s: %v\n", e.Devices[i].Device, err)
				}
			}

			// Then clean up the file
			if e.Devices[i].File != nil {
				e.Devices[i].File.Close()
				if err := os.Remove(e.Devices[i].File.Name()); err != nil {
					fmt.Printf("failed to remove file %s: %v\n", e.Devices[i].File.Name(), err)
				}
			}
		}
	}

	// Clear the devices slice
	e.Devices = nil
}

// checkSharingServices checks if NFS/SMB services are available
func CheckSharingServices(t *testing.T) (hasNFS, hasSMB bool) {
	// Check NFS
	if _, err := exec.Command("systemctl", "is-active", "nfs-server").Output(); err == nil {
		hasNFS = true
	}

	// Check SMB
	if _, err := exec.Command("which", "smbd").Output(); err == nil {
		// Also verify SMB service is running
		if _, err := exec.Command("systemctl", "is-active", "smbd").Output(); err == nil {
			hasSMB = true
		}
	}

	if !hasNFS && !hasSMB {
		t.Skip("Neither NFS nor SMB services are available")
	}

	return
}
