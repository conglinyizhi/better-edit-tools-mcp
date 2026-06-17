package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOSFileSystemSync_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	fs := OSFileSystem{}
	if err := fs.Sync(path); err != nil {
		t.Fatalf("Sync file failed: %v", err)
	}
}

func TestOSFileSystemSync_Directory(t *testing.T) {
	dir := t.TempDir()
	fs := OSFileSystem{}
	if err := fs.Sync(dir); err != nil {
		t.Fatalf("Sync directory failed: %v", err)
	}
}

func TestOSFileSystemSync_MissingPath(t *testing.T) {
	fs := OSFileSystem{}
	err := fs.Sync(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}
