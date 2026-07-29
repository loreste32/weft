package weft

import "os/exec"

func init() {
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command(name, args...)
	}
	execLookPath = exec.LookPath
}

func defaultExecCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func defaultExecLookPath(name string) (string, error) {
	return exec.LookPath(name)
}
