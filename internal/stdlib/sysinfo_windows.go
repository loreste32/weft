//go:build windows && !js

package stdlib

import (
	"fmt"
	"syscall"
)

var getTickCount64 = syscall.NewLazyDLL("kernel32.dll").NewProc("GetTickCount64")

func sysUptime() (float64, error) {
	millis, _, _ := getTickCount64.Call()
	if millis == 0 {
		return 0, fmt.Errorf("GetTickCount64 returned no uptime")
	}
	return float64(millis) / 1000, nil
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
