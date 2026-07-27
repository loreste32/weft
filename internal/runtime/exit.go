package runtime

import "fmt"

// ExitSignal is returned by builtins like cli.exit to end the process with a code.
type ExitSignal struct {
	Code    int
	Message string
}

func (e *ExitSignal) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("exit %d", e.Code)
}

// IsExit reports whether err is an ExitSignal and returns the code.
func IsExit(err error) (code int, ok bool) {
	if err == nil {
		return 0, false
	}
	if es, ok := err.(*ExitSignal); ok {
		return es.Code, true
	}
	return 0, false
}
