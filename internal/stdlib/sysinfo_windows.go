//go:build windows

package stdlib

import (
	"fmt"
	"time"
)

func sysUptime() (float64, error) {
	return time.Since(bootTime).Seconds(), nil
}

func sysLoadAvg() ([]float64, error) {
	return nil, fmt.Errorf("loadavg not available on windows")
}

func sysMemory() (memInfo, error) {
	return memInfo{}, fmt.Errorf("sysinfo.memory not implemented on windows")
}

func sysDisk(path string) (diskInfo, error) {
	return diskInfo{}, fmt.Errorf("sysinfo.disk not implemented on windows")
}
