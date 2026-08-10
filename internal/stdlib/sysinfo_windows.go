//go:build windows && !js

package stdlib

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getTickCount64     = kernel32.NewProc("GetTickCount64")
	globalMemoryStatus = kernel32.NewProc("GlobalMemoryStatusEx")
	getDiskFreeSpace   = kernel32.NewProc("GetDiskFreeSpaceExW")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

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
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	r, _, err := globalMemoryStatus.Call(uintptr(unsafe.Pointer(&m)))
	if r == 0 {
		return memInfo{}, fmt.Errorf("GlobalMemoryStatusEx: %v", err)
	}
	used := m.TotalPhys - m.AvailPhys
	pct := 0.0
	if m.TotalPhys > 0 {
		pct = float64(used) / float64(m.TotalPhys) * 100
	}
	return memInfo{
		total:     m.TotalPhys,
		available: m.AvailPhys,
		used:      used,
		percent:   pct,
	}, nil
}

func sysDisk(path string) (diskInfo, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return diskInfo{}, err
	}
	var free, total, totalFree uint64
	r, _, callErr := getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&free)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if r == 0 {
		return diskInfo{}, fmt.Errorf("GetDiskFreeSpaceExW: %v", callErr)
	}
	used := total - totalFree
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return diskInfo{
		total:   total,
		free:    totalFree,
		used:    used,
		percent: pct,
	}, nil
}
