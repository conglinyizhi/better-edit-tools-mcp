package betools

import (
	"errors"
	"fmt"
)

var (
	ErrInvalid = errors.New("invalid argument")
	ErrRead    = errors.New("read error")
	ErrWrite   = errors.New("write error")
)

// ErrorKind returns a wrapped error with a contextual message.
// Callers can use errors.Is to check the kind.
//   - errors.Is(err, ErrInvalid) for invalid arguments
//   - errors.Is(err, ErrRead) for file read errors
//   - errors.Is(err, ErrWrite) for file write errors
func newReadError(path string, err error) error {
	return fmt.Errorf("read %s: %w", path, err)
}

func newWriteError(path string, err error) error {
	return fmt.Errorf("write %s: %w", path, err)
}

func invalidArg(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalid, msg)
}

func readPath(path string, err error) error {
	return newReadError(path, err)
}

func writePath(path string, err error) error {
	return newWriteError(path, err)
}

func jsonParse(err error) error {
	return fmt.Errorf("parse JSON: %w", err)
}
