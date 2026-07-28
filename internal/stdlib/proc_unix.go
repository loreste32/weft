//go:build !windows

package stdlib

import (
	"strconv"
	"strings"
	"syscall"
)

func parseSignal(s string) syscall.Signal {
	switch strings.ToUpper(strings.TrimPrefix(s, "SIG")) {
	case "KILL", "9":
		return syscall.SIGKILL
	case "INT", "2":
		return syscall.SIGINT
	case "HUP", "1":
		return syscall.SIGHUP
	case "USR1", "10":
		return syscall.SIGUSR1
	case "USR2", "12":
		return syscall.SIGUSR2
	case "TERM", "15":
		return syscall.SIGTERM
	default:
		if n, e := strconv.Atoi(s); e == nil {
			return syscall.Signal(n)
		}
		return syscall.SIGTERM
	}
}
