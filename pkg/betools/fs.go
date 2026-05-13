package betools

import (
	"io"
	"io/fs"
	"os"
)

// FileSystem defines the file operations used by betools.
//
// Callers embedding betools in sandboxed environments can provide an
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

// Option configures a betools operation.
type Option func(*callConfig)

type callConfig struct {
	fs FileSystem
}

func defaultCallConfig() callConfig {
	return callConfig{fs: OSFileSystem{}}
}

func withCallConfig(opts ...Option) callConfig {
	cfg := defaultCallConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

// WithFileSystem injects the filesystem used by the current operation.
func WithFileSystem(fsys FileSystem) Option {
	return func(cfg *callConfig) {
		if fsys != nil {
			cfg.fs = fsys
		}
	}
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
