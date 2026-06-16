package betools

import (
	"github.com/conglinyizhi/better-edit-tools-mcp/pkg/fs"
)

// FileSystem defines the file operations used by betools.
//
// Callers embedding betools in sandboxed environments can provide an
// implementation that restricts access to a workspace root or a virtual FS.
//
// This is an alias to pkg/fs.FileSystem; new code may import that package
// directly.
type FileSystem = fs.FileSystem

// OSFileSystem is the default implementation backed by the host OS.
//
// This is an alias to pkg/fs.OSFileSystem.
type OSFileSystem = fs.OSFileSystem

// MemFS is an in-memory implementation of FileSystem.
//
// This is an alias to pkg/fs.MemFS.
type MemFS = fs.MemFS

// ErrNotExist is a convenience alias so test code can check
// path errors from MemFS the same way as os.
var ErrNotExist = fs.ErrNotExist

// NewMemFS creates a MemFS optionally pre-populated with the given files.
//
// This is a forward to pkg/fs.NewMemFS.
var NewMemFS = fs.NewMemFS

// Option configures a betools operation.
type Option func(*callConfig)

type callConfig struct {
	fs fs.FileSystem
}

func defaultCallConfig() callConfig {
	return callConfig{fs: fs.OSFileSystem{}}
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
func WithFileSystem(fsys fs.FileSystem) Option {
	return func(cfg *callConfig) {
		if fsys != nil {
			cfg.fs = fsys
		}
	}
}
