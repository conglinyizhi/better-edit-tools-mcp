package betools

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// sandboxFS is a FileSystem rooted at a temp directory, optionally
// blocking reads on specific paths.
// Deprecated: use MemFS + WithFileSystem instead.
type sandboxFS struct {
	root  string
	block map[string]bool
}

func (s sandboxFS) abs(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(s.root, name)
}

func (s sandboxFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(s.abs(name))
}

func (s sandboxFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(s.abs(name), data, perm)
}

func (s sandboxFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(s.abs(name))
}

func (s sandboxFS) Rename(oldpath, newpath string) error {
	return os.Rename(s.abs(oldpath), s.abs(newpath))
}

func (s sandboxFS) Remove(name string) error {
	return os.Remove(s.abs(name))
}

func (s sandboxFS) Open(name string) (io.ReadCloser, error) {
	if s.block != nil && s.block[name] {
		return nil, os.ErrPermission
	}
	return os.Open(s.abs(name))
}

func (s sandboxFS) Create(name string) (io.WriteCloser, error) {
	return os.Create(s.abs(name))
}

// withFS creates a MemFS with initial files and returns both the FS
// and the WithFileSystem option to pass to tool calls.
func withFS(files map[string]string) (*MemFS, Option) {
	m := NewMemFS(files)
	return m, WithFileSystem(m)
}

// readFS reads a file from a MemFS, failing the test on error.
func readFS(t *testing.T, m *MemFS, path string) string {
	t.Helper()
	data, err := m.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// writeTempFile writes content to a temp file and returns its path.
// Deprecated: use withFS instead.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// readFile reads a file from disk, failing the test on error.
// Deprecated: use readFS instead.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(data)
}
