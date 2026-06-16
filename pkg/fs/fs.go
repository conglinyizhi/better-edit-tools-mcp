// Package fs defines the file-system abstraction used by better-edit-tools.
//
// It provides a FileSystem interface so that callers can inject a sandboxed
// or in-memory implementation, plus a default OS-backed implementation.
package fs

import (
	"errors"
	"io"
	"io/fs"
	"os"
)

// FileSystem defines the file operations used by better-edit-tools.
//
// Callers embedding this library in sandboxed environments can provide an
// implementation that restricts access to a workspace root or a virtual FS.
type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	Stat(name string) (fs.FileInfo, error)
	Rename(oldpath, newpath string) error
	Remove(name string) error
	Open(name string) (io.ReadCloser, error)
	Create(name string) (io.WriteCloser, error)
}

// OSFileSystem is the default implementation backed by the host OS.
type OSFileSystem struct{}

func (OSFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (OSFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (OSFileSystem) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

func (OSFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (OSFileSystem) Remove(name string) error {
	return os.Remove(name)
}

func (OSFileSystem) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

func (OSFileSystem) Create(name string) (io.WriteCloser, error) {
	return os.Create(name)
}

// ErrNotExist is a convenience alias so test code can check
// path errors from MemFS the same way as os.
var ErrNotExist = errors.New("file does not exist")
