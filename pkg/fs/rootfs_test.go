package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootFileSystemImplementsFileSystem(t *testing.T) {
	var _ FileSystem = (*RootFileSystem)(nil)
}

func TestRootFileSystem_AllowsRelativePath(t *testing.T) {
	dir := t.TempDir()
	rfs := NewRootFileSystem(dir).(*RootFileSystem)

	if err := rfs.WriteFile("foo.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile relative path: %v", err)
	}
	data, err := rfs.ReadFile("foo.txt")
	if err != nil {
		t.Fatalf("ReadFile relative path: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected hello, got %q", string(data))
	}

	// Absolute path inside the root should also work.
	absPath := filepath.Join(dir, "abs.txt")
	if err := rfs.WriteFile(absPath, []byte("world"), 0o644); err != nil {
		t.Fatalf("WriteFile absolute inside root: %v", err)
	}
	data, err = rfs.ReadFile(absPath)
	if err != nil {
		t.Fatalf("ReadFile absolute inside root: %v", err)
	}
	if string(data) != "world" {
		t.Fatalf("expected world, got %q", string(data))
	}
}

func TestRootFileSystem_BlocksDotDotEscape(t *testing.T) {
	dir := t.TempDir()
	rfs := NewRootFileSystem(dir).(*RootFileSystem)

	err := rfs.WriteFile("../escape.txt", []byte("bad"), 0o644)
	if err == nil {
		t.Fatalf("expected error for .. escape, got nil")
	}
	if !strings.Contains(err.Error(), "escapes root") {
		t.Fatalf("expected escapes root error, got %v", err)
	}

	// Ensure the file was not written outside the root.
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.txt")); err == nil {
		t.Fatalf("escape.txt should not exist outside root")
	}
}

func TestRootFileSystem_BlocksAbsolutePathAttack(t *testing.T) {
	dir := t.TempDir()
	rfs := NewRootFileSystem(dir).(*RootFileSystem)

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("setup outside file: %v", err)
	}

	_, err := rfs.ReadFile(outsideFile)
	if err == nil {
		t.Fatalf("expected error for absolute path outside root, got nil")
	}
	if !strings.Contains(err.Error(), "escapes root") {
		t.Fatalf("expected escapes root error, got %v", err)
	}
}

func TestRootFileSystem_BlocksEmptyPath(t *testing.T) {
	dir := t.TempDir()
	rfs := NewRootFileSystem(dir).(*RootFileSystem)

	_, err := rfs.ReadFile("")
	if err == nil {
		t.Fatalf("expected error for empty path, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty path error, got %v", err)
	}
}
