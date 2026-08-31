//go:build !js

package stdlib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/loreste/weft/internal/runtime"
)

// parseSignal is in proc_unix.go / proc_windows.go

// packageProc — process management for ops scripts (ps, kill, pid info).
func packageProc() runtime.Value {
	p := pkg()

	// proc.self() -> map  process IDs plus identity and location when available.
	set(p, "self", func(args []runtime.Value) (runtime.Value, error) {
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		put("pid", runtime.Int(int64(os.Getpid())))
		put("ppid", runtime.Int(int64(os.Getppid())))
		identity := currentProcessIdentity()
		if identity.uidOK {
			put("uid", runtime.Int(identity.uid))
		}
		if identity.gidOK {
			put("gid", runtime.Int(identity.gid))
		}
		if identity.user != "" {
			put("user", runtime.Str(identity.user))
		}
		if identity.group != "" {
			put("group", runtime.Str(identity.group))
		}
		if executable, err := os.Executable(); err == nil {
			put("executable", runtime.Str(executable))
		}
		if cwd, err := os.Getwd(); err == nil {
			put("cwd", runtime.Str(cwd))
		}
		return m, nil
	}, 0)

	// proc.kill(pid, signal?) -> Result[unit]  signal default: SIGTERM (15)
	set(p, "kill", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("proc.kill(pid, signal?)", "proc"), nil
		}
		pid64, err := runtime.AsInt(args[0])
		if err != nil {
			return errRes("pid must be int", "proc"), nil
		}
		if pid64 <= 0 {
			return errRes("pid must be a positive process id", "proc"), nil
		}
		pid, err := safeInt(pid64)
		if err != nil {
			return errRes("pid out of range", "proc"), nil
		}
		sig := syscall.SIGTERM
		if len(args) >= 2 {
			sig = parseSignal(args[1].String())
		}
		if sig < 0 {
			return errRes("signal must be non-negative", "proc"), nil
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return errRes(err.Error(), "proc"), nil
		}
		if err := proc.Signal(sig); err != nil {
			return errRes(err.Error(), "proc"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// proc.exists(pid) -> bool  check if process exists (signal 0)
	set(p, "exists", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		pid64, err := runtime.AsInt(args[0])
		if err != nil {
			return runtime.Bool(false), nil
		}
		if pid64 <= 0 {
			return runtime.Bool(false), nil
		}
		pid, err := safeInt(pid64)
		if err != nil {
			return runtime.Bool(false), nil
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return runtime.Bool(false), nil
		}
		err = proc.Signal(syscall.Signal(0))
		return runtime.Bool(err == nil), nil
	}, 1)

	// proc.list() -> Result[[map]]  list processes {pid, name, cmd}.
	set(p, "list", func(args []runtime.Value) (runtime.Value, error) {
		procs, err := listProcesses()
		if err != nil {
			return errRes(err.Error(), "proc"), nil
		}
		items := make([]runtime.Value, 0, len(procs))
		for _, pi := range procs {
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			put := func(k string, v runtime.Value) {
				mo.Keys = append(mo.Keys, k)
				mo.Vals[k] = v
			}
			put("pid", runtime.Int(int64(pi.pid)))
			put("name", runtime.Str(pi.name))
			put("cmd", runtime.Str(pi.cmd))
			items = append(items, m)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 0)

	// proc.find(name) -> Result[[map]]  find processes by name
	set(p, "find", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("proc.find(name)", "proc"), nil
		}
		name := strings.TrimSpace(args[0].String())
		if name == "" {
			return errRes("proc.find(name): name cannot be empty", "proc"), nil
		}
		name = strings.ToLower(name)
		procs, err := listProcesses()
		if err != nil {
			return errRes(err.Error(), "proc"), nil
		}
		items := make([]runtime.Value, 0)
		for _, pi := range procs {
			if strings.Contains(strings.ToLower(pi.name), name) ||
				strings.Contains(strings.ToLower(pi.cmd), name) {
				m := runtime.NewMap()
				mo := m.Obj.(*runtime.MapObj)
				put := func(k string, v runtime.Value) {
					mo.Keys = append(mo.Keys, k)
					mo.Vals[k] = v
				}
				put("pid", runtime.Int(int64(pi.pid)))
				put("name", runtime.Str(pi.name))
				put("cmd", runtime.Str(pi.cmd))
				items = append(items, m)
			}
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	return p
}

type procInfo struct {
	pid  int
	name string
	cmd  string
}

func listProcesses() ([]procInfo, error) {
	switch goruntime.GOOS {
	case "linux":
		return listProcessesLinux()
	case "darwin":
		return listProcessesPSCommand()
	default:
		return listProcessesPSCommand()
	}
}

// listProcessesLinux reads /proc directly.
func listProcessesLinux() ([]procInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return listProcessesPSCommand()
	}
	var procs []procInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		name := ""
		cmd := ""
		if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
			name = strings.TrimSpace(string(data))
		}
		if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
			cmd = strings.ReplaceAll(string(data), "\x00", " ")
			cmd = strings.TrimSpace(cmd)
		}
		procs = append(procs, procInfo{pid: pid, name: name, cmd: cmd})
	}
	return procs, nil
}

// listProcessesPSCommand uses ps for macOS and other unix.
func listProcessesPSCommand() ([]procInfo, error) {
	out, err := exec.Command("ps", "-eo", "pid=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}
	var procs []procInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "PID") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		pid, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		cmd := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if cmd == "" {
			continue
		}
		name := filepath.Base(strings.Fields(cmd)[0])
		procs = append(procs, procInfo{pid: pid, name: name, cmd: cmd})
	}
	return procs, nil
}
