package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type LoopDevice struct {
	File   *os.File
	Device string
	Number int
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

type TestEnv struct {
	Devices []*LoopDevice
}

func NewTestEnv(t *testing.T, diskCount int) *TestEnv {
	env := &TestEnv{
		Devices: make([]*LoopDevice, diskCount),
	}

	for i := 0; i < diskCount; i++ {
		device, err := CreateLoopDevice(t, 50) // 50MB size
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
	for _, d := range e.Devices {
		if d != nil {
			d.Cleanup()
		}
	}
}
