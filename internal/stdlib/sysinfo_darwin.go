//go:build darwin

package stdlib

import (
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
		return time.Since(bootTime).Seconds(), nil
	}
	boot := time.Unix(tv.Sec, int64(tv.Usec)*1000)
	return time.Since(boot).Seconds(), nil
}

func sysLoadAvg() ([]float64, error) {
	// sysctl vm.loadavg
	out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return []float64{0, 0, 0}, nil
	}
	s := strings.TrimSpace(string(out))
	s = strings.Trim(s, "{ }")
	parts := strings.Fields(s)
	avgs := make([]float64, 3)
	for i := 0; i < 3 && i < len(parts); i++ {
		avgs[i], _ = strconv.ParseFloat(parts[i], 64)
	}
	return avgs, nil
}

func sysMemory() (memInfo, error) {
	// Use sysctl for total memory
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return memInfo{}, err
	}
	total, _ := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)

	// vm_stat for page-level stats
	vmOut, err := exec.Command("vm_stat").Output()
	if err != nil {
		used := total / 2 // rough fallback
		return memInfo{total: total, available: total - used, used: used, percent: 50}, nil
	}
	pages := map[string]uint64{}
	for _, line := range strings.Split(string(vmOut), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		val := strings.TrimSpace(parts[1])
		val = strings.TrimSuffix(val, ".")
		n, _ := strconv.ParseUint(val, 10, 64)
		pages[strings.TrimSpace(parts[0])] = n
	}
	pageSize := uint64(4096)
	// Parse page size from header if present
	for _, line := range strings.Split(string(vmOut), "\n") {
		if strings.Contains(line, "page size of") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "of" && i+1 < len(parts) {
					ps, _ := strconv.ParseUint(parts[i+1], 10, 64)
					if ps > 0 {
						pageSize = ps
					}
				}
			}
		}
	}
	free := pages["Pages free"] * pageSize
	inactive := pages["Pages inactive"] * pageSize
	avail := free + inactive
	used := total - avail
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return memInfo{total: total, available: avail, used: used, percent: pct}, nil
}
