package fs

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
	"time"
)

// MemFS is an in-memory implementation of FileSystem.
// All operations are O(1) amortized and safe for concurrent use.
// No real filesystem operations are performed — ideal for tests.
type MemFS struct {
	mu    sync.RWMutex
	files map[string]*memFile
}

type memFile struct {
	name    string
	data    []byte
	mode    fs.FileMode
	modTime time.Time
}

// NewMemFS creates a MemFS optionally pre-populated with the given files.
// Each key is a path, each value is file content.
func NewMemFS(initial map[string]string) *MemFS {
	m := &MemFS{files: make(map[string]*memFile)}
	if initial != nil {
		for path, content := range initial {
			m.files[clean(path)] = &memFile{
				name:    clean(path),
				data:    []byte(content),
				mode:    0o644,
				modTime: time.Now(),
			}
		}
	}
	return m
}

func clean(path string) string {
	return strings.TrimPrefix(path, "/")
}

func (m *MemFS) get(path string) *memFile {
	if m.files == nil {
		return nil
	}
	return m.files[clean(path)]
}

func (m *MemFS) ReadFile(name string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f := m.get(name)
	if f == nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	data := make([]byte, len(f.data))
	copy(data, f.data)
	return data, nil
}

func (m *MemFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.files == nil {
		m.files = make(map[string]*memFile)
	}
	p := clean(name)
	d := make([]byte, len(data))
	copy(d, data)
	m.files[p] = &memFile{
		name:    p,
		data:    d,
		mode:    perm,
		modTime: time.Now(),
	}
	return nil
}

func (m *MemFS) Stat(name string) (fs.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f := m.get(name)
	if f == nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return &memFileInfo{name: f.name, size: int64(len(f.data)), mode: f.mode, modTime: f.modTime}, nil
}

func (m *MemFS) Rename(oldpath, newpath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := clean(oldpath)
	np := clean(newpath)
	f := m.files[old]
	if f == nil {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: fs.ErrNotExist}
	}
	f.name = np
	f.modTime = time.Now()
	m.files[np] = f
	delete(m.files, old)
	return nil
}

func (m *MemFS) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := clean(name)
	if _, ok := m.files[p]; !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	delete(m.files, p)
	return nil
}

func (m *MemFS) Open(name string) (io.ReadCloser, error) {
	data, err := m.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *MemFS) Create(name string) (io.WriteCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.files == nil {
		m.files = make(map[string]*memFile)
	}
	p := clean(name)
	// Pre-allocate a buffer via the file struct; writes come via the WriteCloser.
	m.files[p] = &memFile{
		name:    p,
		data:    nil, // will be set on Close
		mode:    0o644,
		modTime: time.Now(),
	}
	return &memWriteCloser{fs: m, path: p, buf: new(bytes.Buffer)}, nil
}

// List returns a snapshot of all stored file paths.
// It is intended for tests that need to inspect the MemFS state.
func (m *MemFS) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.files))
	for name := range m.files {
		out = append(out, name)
	}
	return out
}

// ── memWriteCloser ─────────────────────────────────────

type memWriteCloser struct {
	fs   *MemFS
	path string
	buf  *bytes.Buffer
}

func (w *memWriteCloser) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *memWriteCloser) Close() error {
	w.fs.mu.Lock()
	defer w.fs.mu.Unlock()
	f, ok := w.fs.files[w.path]
	if !ok {
		// File was deleted (e.g. Remove called while writing) — discard.
		return fmt.Errorf("memfs: write %s: file was removed during write", w.path)
	}
	data := make([]byte, w.buf.Len())
	copy(data, w.buf.Bytes())
	f.data = data
	f.modTime = time.Now()
	return nil
}

// ── memFileInfo ────────────────────────────────────────

type memFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (i *memFileInfo) Name() string       { return i.name }
func (i *memFileInfo) Size() int64        { return i.size }
func (i *memFileInfo) Mode() fs.FileMode  { return i.mode }
func (i *memFileInfo) ModTime() time.Time { return i.modTime }
func (i *memFileInfo) IsDir() bool        { return false }
func (i *memFileInfo) Sys() any           { return nil }

// Ensure memFileInfo implements fs.FileInfo.
var _ fs.FileInfo = (*memFileInfo)(nil)

// Ensure MemFS implements FileSystem.
var _ FileSystem = (*MemFS)(nil)
