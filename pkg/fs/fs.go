// Package fs defines the file-system abstraction used by better-edit-tools.
//
// It provides a FileSystem interface so that callers can inject a sandboxed
// or in-memory implementation, plus a default OS-backed implementation.
package fs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// Sync flushes file or directory metadata to the underlying storage.
// On POSIX systems this calls fsync on the opened path; on Windows it is a
// no-op for directories because Windows does not support directory fsync.
func (OSFileSystem) Sync(name string) error {
	if runtime.GOOS == "windows" {
		info, err := os.Stat(name)
		if err == nil && info.IsDir() {
			return nil
		}
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// RootFileSystem wraps a FileSystem and restricts all operations to a single
// workspace root. Paths are resolved with filepath.Join/Clean and rejected when
// they escape the root via ".." or when they point outside the root.
type RootFileSystem struct {
	root string
	base FileSystem
}

// NewRootFileSystem creates a sandboxed FileSystem rooted at root. All paths
// passed to the returned FileSystem are resolved relative to root (or, if
// absolute, validated against root) and must stay inside root.
func NewRootFileSystem(root string) FileSystem {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = filepath.Clean(root)
	}
	return &RootFileSystem{root: abs, base: OSFileSystem{}}
}

func (r *RootFileSystem) resolve(name string) (string, error) {
	if name == "" {
		return "", errors.New("path is empty")
	}

	var resolved string
	if filepath.IsAbs(name) {
		resolved = filepath.Clean(name)
	} else {
		resolved = filepath.Join(r.root, name)
	}

	rel, err := filepath.Rel(r.root, resolved)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q escapes root %q", name, r.root)
	}
	return resolved, nil
}

func (r *RootFileSystem) ReadFile(name string) ([]byte, error) {
	p, err := r.resolve(name)
	if err != nil {
		return nil, err
	}
	return r.base.ReadFile(p)
}

func (r *RootFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	p, err := r.resolve(name)
	if err != nil {
		return err
	}
	return r.base.WriteFile(p, data, perm)
}

func (r *RootFileSystem) Stat(name string) (fs.FileInfo, error) {
	p, err := r.resolve(name)
	if err != nil {
		return nil, err
	}
	return r.base.Stat(p)
}

func (r *RootFileSystem) Rename(oldpath, newpath string) error {
	o, err := r.resolve(oldpath)
	if err != nil {
		return err
	}
	n, err := r.resolve(newpath)
	if err != nil {
		return err
	}
	return r.base.Rename(o, n)
}

func (r *RootFileSystem) Remove(name string) error {
	p, err := r.resolve(name)
	if err != nil {
		return err
	}
	return r.base.Remove(p)
}

func (r *RootFileSystem) Open(name string) (io.ReadCloser, error) {
	p, err := r.resolve(name)
	if err != nil {
		return nil, err
	}
	return r.base.Open(p)
}

func (r *RootFileSystem) Create(name string) (io.WriteCloser, error) {
	p, err := r.resolve(name)
	if err != nil {
		return nil, err
	}
	return r.base.Create(p)
}

// Ensure RootFileSystem implements FileSystem.
var _ FileSystem = (*RootFileSystem)(nil)

// ErrNotExist is a convenience alias so test code can check
// path errors from MemFS the same way as os.
var ErrNotExist = errors.New("file does not exist")
