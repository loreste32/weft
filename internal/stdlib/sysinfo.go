//go:build !js

package stdlib

import (
	"fmt"
	"net"
	"os"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

var bootTime = time.Now() // fallback; overridden by OS-level uptime where available

// packageSysinfo — system metrics for ops scripts (CPU, memory, disk, uptime, interfaces).
func packageSysinfo() runtime.Value {
	p := pkg()

	// sysinfo.uptime() -> Result[map]  {seconds, human}
	set(p, "uptime", func(args []runtime.Value) (runtime.Value, error) {
		sec, err := sysUptime()
		if err != nil {
			return errRes(err.Error(), "sysinfo"), nil
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		put("seconds", runtime.Int(int64(sec)))
		put("human", runtime.Str(humanDuration(sec)))
		return runtime.Ok(m), nil
	}, 0)

	// sysinfo.loadavg() -> Result[[float]]  [1m, 5m, 15m]
	set(p, "loadavg", func(args []runtime.Value) (runtime.Value, error) {
		avgs, err := sysLoadAvg()
		if err != nil {
			return errRes(err.Error(), "sysinfo"), nil
		}
		items := make([]runtime.Value, len(avgs))
		for i, a := range avgs {
			items[i] = runtime.Float(a)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 0)

	// sysinfo.memory() -> Result[map]  byte counts plus human-readable fields.
	set(p, "memory", func(args []runtime.Value) (runtime.Value, error) {
		info, err := sysMemory()
		if err != nil {
			return errRes(err.Error(), "sysinfo"), nil
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		put("total", runtime.Int(int64(info.total)))
		put("available", runtime.Int(int64(info.available)))
		put("used", runtime.Int(int64(info.used)))
		put("percent", runtime.Float(info.percent))
		// Keep the original numeric keys for compatibility. These explicit names
		// make the unit and meaning unambiguous for new callers.
		put("unit", runtime.Str("bytes"))
		put("total_bytes", runtime.Int(int64(info.total)))
		put("available_bytes", runtime.Int(int64(info.available)))
		put("used_bytes", runtime.Int(int64(info.used)))
		put("total_human", runtime.Str(formatBytes(info.total)))
		put("available_human", runtime.Str(formatBytes(info.available)))
		put("used_human", runtime.Str(formatBytes(info.used)))
		put("percent_used", runtime.Float(info.percent))
		return runtime.Ok(m), nil
	}, 0)

	// sysinfo.disk(path?) -> Result[map]  filesystem byte counts and path.
	set(p, "disk", func(args []runtime.Value) (runtime.Value, error) {
		path := "/"
		if len(args) >= 1 && args[0].Kind != runtime.KindNull {
			path = args[0].String()
		}
		info, err := sysDisk(path)
		if err != nil {
			return errRes(err.Error(), "sysinfo"), nil
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		put("total", runtime.Int(int64(info.total)))
		put("free", runtime.Int(int64(info.free)))
		put("used", runtime.Int(int64(info.used)))
		put("percent", runtime.Float(info.percent))
		put("unit", runtime.Str("bytes"))
		put("total_bytes", runtime.Int(int64(info.total)))
		put("free_bytes", runtime.Int(int64(info.free)))
		put("used_bytes", runtime.Int(int64(info.used)))
		put("total_human", runtime.Str(formatBytes(info.total)))
		put("free_human", runtime.Str(formatBytes(info.free)))
		put("used_human", runtime.Str(formatBytes(info.used)))
		put("percent_used", runtime.Float(info.percent))
		put("free_scope", runtime.Str("available to the current user"))
		put("path", runtime.Str(path))
		return runtime.Ok(m), nil
	}, 1)

	// sysinfo.cpu_count() -> int
	set(p, "cpu_count", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Int(int64(goruntime.NumCPU())), nil
	}, 0)

	// sysinfo.net_interfaces() -> Result[[map]]
	set(p, "net_interfaces", func(args []runtime.Value) (runtime.Value, error) {
		ifaces, err := net.Interfaces()
		if err != nil {
			return errRes(err.Error(), "sysinfo"), nil
		}
		items := make([]runtime.Value, 0, len(ifaces))
		for _, iface := range ifaces {
			addrs, _ := iface.Addrs()
			addrList := make([]runtime.Value, 0, len(addrs))
			for _, a := range addrs {
				addrList = append(addrList, runtime.Str(a.String()))
			}
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			put := func(k string, v runtime.Value) {
				mo.Keys = append(mo.Keys, k)
				mo.Vals[k] = v
			}
			put("name", runtime.Str(iface.Name))
			put("mac", runtime.Str(iface.HardwareAddr.String()))
			put("mtu", runtime.Int(int64(iface.MTU)))
			flags := make([]runtime.Value, 0)
			for _, f := range strings.Split(iface.Flags.String(), "|") {
				if f != "" {
					flags = append(flags, runtime.Str(f))
				}
			}
			put("flags", runtime.List(flags...))
			put("addrs", runtime.List(addrList...))
			put("up", runtime.Bool(iface.Flags&net.FlagUp != 0))
			items = append(items, m)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 0)

	// sysinfo.env_summary() -> map  {os, arch, cpus, hostname, user, home, pid}
	set(p, "env_summary", func(args []runtime.Value) (runtime.Value, error) {
		h, _ := os.Hostname()
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		put("os", runtime.Str(goruntime.GOOS))
		put("arch", runtime.Str(goruntime.GOARCH))
		put("cpus", runtime.Int(int64(goruntime.NumCPU())))
		put("hostname", runtime.Str(h))
		u := os.Getenv("USER")
		if u == "" {
			u = os.Getenv("USERNAME")
		}
		put("user", runtime.Str(u))
		put("home", runtime.Str(os.Getenv("HOME")))
		put("pid", runtime.Int(int64(os.Getpid())))
		return m, nil
	}, 0)

	return p
}

type memInfo struct {
	total, available, used uint64
	percent                float64
}

type diskInfo struct {
	total, free, used uint64
	percent           float64
}

func humanDuration(sec float64) string {
	d := time.Duration(sec * float64(time.Second))
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func formatBytes(value uint64) string {
	const base = 1024.0
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	amount := float64(value)
	unit := 0
	for amount >= base && unit < len(units)-1 {
		amount /= base
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f %s", amount, units[unit])
	}
	return fmt.Sprintf("%.2f %s", amount, units[unit])
}

// parseMeminfo reads /proc/meminfo (Linux).
func parseMeminfo() (memInfo, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return memInfo{}, err
	}
	fields := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		val, parseErr := strconv.ParseUint(parts[1], 10, 64)
		if parseErr != nil {
			return memInfo{}, fmt.Errorf("invalid /proc/meminfo value for %s: %w", key, parseErr)
		}
		fields[key] = val * 1024 // kB to bytes
	}
	total := fields["MemTotal"]
	avail := fields["MemAvailable"]
	if avail == 0 {
		avail = fields["MemFree"] + fields["Buffers"] + fields["Cached"]
	}
	if total == 0 {
		return memInfo{}, fmt.Errorf("/proc/meminfo did not report total memory")
	}
	if avail > total {
		avail = total
	}
	used := total - avail
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return memInfo{total: total, available: avail, used: used, percent: pct}, nil
}

// parseLoadAvg reads /proc/loadavg (Linux).
func parseLoadAvg() ([]float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(string(data))
	if len(parts) < 3 {
		return nil, fmt.Errorf("unexpected /proc/loadavg format")
	}
	avgs := make([]float64, 3)
	for i := 0; i < 3; i++ {
		value, parseErr := strconv.ParseFloat(parts[i], 64)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid /proc/loadavg value %q: %w", parts[i], parseErr)
		}
		avgs[i] = value
	}
	return avgs, nil
}
