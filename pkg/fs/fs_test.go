package fs

import (
	"errors"
	iofs "io/fs"
	"testing"
)

func TestOSFileSystemIsFileSystem(t *testing.T) {
	var _ FileSystem = OSFileSystem{}
}

func TestMemFSImplementsFileSystem(t *testing.T) {
	var _ FileSystem = (*MemFS)(nil)
}

func TestMemFSReadWriteFile(t *testing.T) {
	m := NewMemFS(nil)
	if err := m.WriteFile("a.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := m.ReadFile("a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected hello, got %q", string(data))
	}
}

func TestMemFSReadFileNotExist(t *testing.T) {
	m := NewMemFS(nil)
	_, err := m.ReadFile("missing")
	if !errors.Is(err, iofs.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestMemFSList(t *testing.T) {
	m := NewMemFS(map[string]string{"a.txt": "a", "b.txt": "b"})
	list := m.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 files, got %d", len(list))
	}
}
