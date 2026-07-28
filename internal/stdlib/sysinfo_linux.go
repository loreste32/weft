//go:build linux

package stdlib

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func sysUptime() (float64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return 0, fmt.Errorf("unexpected /proc/uptime format")
	}
	return strconv.ParseFloat(parts[0], 64)
}

func sysLoadAvg() ([]float64, error) {
	return parseLoadAvg()
}

func sysMemory() (memInfo, error) {
	return parseMeminfo()
}
