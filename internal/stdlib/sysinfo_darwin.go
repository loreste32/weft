//go:build darwin && !js

package stdlib

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func sysUptime() (float64, error) {
	// sysctl kern.boottime
	tv := syscall.Timeval{}
	size := unsafe.Sizeof(tv)
	mib := [2]int32{1, 21} // CTL_KERN, KERN_BOOTTIME
	_, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		2,
		uintptr(unsafe.Pointer(&tv)),
		uintptr(unsafe.Pointer(&size)),
		0, 0,
	)
	if errno != 0 {
		return 0, fmt.Errorf("kern.boottime: %w", errno)
	}
	boot := time.Unix(tv.Sec, int64(tv.Usec)*1000)
	return time.Since(boot).Seconds(), nil
}

func sysLoadAvg() ([]float64, error) {
	// sysctl vm.loadavg
	out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return nil, fmt.Errorf("sysctl vm.loadavg: %w", err)
	}
	s := strings.TrimSpace(string(out))
	s = strings.Trim(s, "{ }")
	parts := strings.Fields(s)
	if len(parts) < 3 {
		return nil, fmt.Errorf("unexpected vm.loadavg format")
	}
	avgs := make([]float64, 3)
	for i := 0; i < 3 && i < len(parts); i++ {
		value, parseErr := strconv.ParseFloat(parts[i], 64)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid vm.loadavg value %q: %w", parts[i], parseErr)
		}
		avgs[i] = value
	}
	return avgs, nil
}

func sysMemory() (memInfo, error) {
	// Use sysctl for total memory
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return memInfo{}, err
	}
	total, parseErr := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if parseErr != nil || total == 0 {
		if parseErr == nil {
			parseErr = fmt.Errorf("reported zero memory")
		}
		return memInfo{}, fmt.Errorf("sysctl hw.memsize: %w", parseErr)
	}

	// vm_stat for page-level stats
	vmOut, err := exec.Command("vm_stat").Output()
	if err != nil {
		return memInfo{}, fmt.Errorf("vm_stat: %w", err)
	}
	pages := map[string]uint64{}
	for _, line := range strings.Split(string(vmOut), "\n") {
		if strings.HasPrefix(line, "Mach Virtual Memory Statistics") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		val := strings.TrimSpace(parts[1])
		val = strings.TrimSuffix(val, ".")
		n, parseErr := strconv.ParseUint(val, 10, 64)
		if parseErr != nil {
			return memInfo{}, fmt.Errorf("invalid vm_stat value %q: %w", val, parseErr)
		}
		pages[strings.TrimSpace(parts[0])] = n
	}
	pageSize := uint64(0)
	// Parse page size from header if present
	for _, line := range strings.Split(string(vmOut), "\n") {
		if strings.Contains(line, "page size of") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "of" && i+1 < len(parts) {
					ps, parseErr := strconv.ParseUint(parts[i+1], 10, 64)
					if parseErr != nil {
						return memInfo{}, fmt.Errorf("invalid vm_stat page size %q: %w", parts[i+1], parseErr)
					}
					if ps > 0 {
						pageSize = ps
					}
				}
			}
		}
	}
	if pageSize == 0 {
		return memInfo{}, fmt.Errorf("vm_stat did not report a page size")
	}
	free := pages["Pages free"] * pageSize
	inactive := pages["Pages inactive"] * pageSize
	avail := free + inactive
	if avail > total {
		avail = total
	}
	used := total - avail
	pct := float64(used) / float64(total) * 100
	return memInfo{total: total, available: avail, used: used, percent: pct}, nil
}
