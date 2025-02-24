package xerrors

import (
	"runtime"
)

type custom struct {
	err    error
	frames []uintptr
}

func Wrap(err error) error {
	return &custom{err, callers(0)}
}

func callers(skip int) []uintptr {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3+skip, pcs[:])
	return pcs[0:n]
}

func (e *custom) Error() string {
	return e.err.Error()
}

func (e *custom) StackFrames() []uintptr {
	return e.frames
}
