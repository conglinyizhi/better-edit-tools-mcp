package betools

import (
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
)

func TestMemFS_ReadWriteFile(t *testing.T) {
	m := NewMemFS(nil)
	err := m.WriteFile("/tmp/test.txt", []byte("hello"), 0o644)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := m.ReadFile("/tmp/test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(data))
	}
}

func TestMemFS_ReadFile_NotExist(t *testing.T) {
	m := NewMemFS(nil)
	_, err := m.ReadFile("/nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestMemFS_Stat(t *testing.T) {
	m := NewMemFS(map[string]string{"/a.txt": "content"})
	info, err := m.Stat("/a.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Name() != "a.txt" {
		t.Fatalf("expected name 'a.txt', got %q", info.Name())
	}
	if info.Size() != 7 {
		t.Fatalf("expected size 7, got %d", info.Size())
	}
	if info.IsDir() {
		t.Fatal("expected IsDir=false")
	}
}

func TestMemFS_Stat_NotExist(t *testing.T) {
	m := NewMemFS(nil)
	_, err := m.Stat("/missing")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestMemFS_Rename(t *testing.T) {
	m := NewMemFS(map[string]string{"/old.txt": "data"})
	err := m.Rename("/old.txt", "/new.txt")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	// Old should not exist
	_, err = m.ReadFile("/old.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("expected old file to be gone after rename")
	}
	// New should have content
	data, err := m.ReadFile("/new.txt")
	if err != nil {
		t.Fatalf("ReadFile new: %v", err)
	}
	if string(data) != "data" {
		t.Fatalf("expected 'data', got %q", string(data))
	}
}

func TestMemFS_Rename_NotExist(t *testing.T) {
	m := NewMemFS(nil)
	err := m.Rename("/missing", "/new")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestMemFS_Remove(t *testing.T) {
	m := NewMemFS(map[string]string{"/del.txt": "x"})
	err := m.Remove("/del.txt")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_, err = m.ReadFile("/del.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("expected file to be removed")
	}
}

func TestMemFS_Remove_NotExist(t *testing.T) {
	m := NewMemFS(nil)
	err := m.Remove("/missing")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestMemFS_Open_Read(t *testing.T) {
	m := NewMemFS(map[string]string{"/hello.txt": "hello world"})
	rc, err := m.Open("/hello.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", string(data))
	}
}

func TestMemFS_Create_WriteThenRead(t *testing.T) {
	m := NewMemFS(nil)
	wc, err := m.Create("/new.txt")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = wc.Write([]byte("created content"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	err = wc.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	data, err := m.ReadFile("/new.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "created content" {
		t.Fatalf("expected 'created content', got %q", string(data))
	}
}

func TestMemFS_PathNormalization(t *testing.T) {
	m := NewMemFS(map[string]string{"/a/b.txt": "deep"})
	// Both with and without leading slash should work
	data1, err := m.ReadFile("/a/b.txt")
	if err != nil {
		t.Fatalf("ReadFile with slash: %v", err)
	}
	data2, err := m.ReadFile("a/b.txt")
	if err != nil {
		t.Fatalf("ReadFile without slash: %v", err)
	}
	if string(data1) != string(data2) {
		t.Fatal("path normalization mismatch")
	}
}

// ── MemFS integration with editing tools ────────────────────

func TestShowWithMemFS(t *testing.T) {
	m := NewMemFS(map[string]string{"f.txt": "a\nb\nc\n"})
	res, _, err := Show("f.txt", 1, 3, false, WithFileSystem(m))
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if res.Total != 3 {
		t.Fatalf("expected total=3, got %d", res.Total)
	}
	if !strings.Contains(res.Content, "a") || !strings.Contains(res.Content, "c") {
		t.Fatalf("unexpected content: %q", res.Content)
	}
}

func TestReplaceWithMemFS(t *testing.T) {
	m := NewMemFS(map[string]string{"r.txt": "keep\nold\nkeep\n"})
	_, err := Replace("r.txt", 2, 2, nil, "new\n", "plain", false, "", false, WithFileSystem(m))
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	data, err := m.ReadFile("r.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "keep\nnew\nkeep\n" {
		t.Fatalf("expected 'keep\\nnew\\nkeep\\n', got %q", string(data))
	}
}

func TestInsertWithMemFS(t *testing.T) {
	m := NewMemFS(map[string]string{"i.txt": "b\nc\n"})
	_, err := Insert("i.txt", 0, "a\n", "plain", false, false, WithFileSystem(m))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	data, err := m.ReadFile("i.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "a\nb\nc\n" {
		t.Fatalf("expected 'a\\nb\\nc\\n', got %q", string(data))
	}
}

func TestDeleteWithMemFS(t *testing.T) {
	m := NewMemFS(map[string]string{"d.txt": "a\nb\nc\n"})
	_, err := Delete("d.txt", 2, 2, "plain", false, false, WithFileSystem(m))
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	data, err := m.ReadFile("d.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "a\nc\n" {
		t.Fatalf("expected 'a\\nc\\n', got %q", string(data))
	}
}

func TestWriteWithMemFS(t *testing.T) {
	m := NewMemFS(nil)
	spec := `{"file":"w.txt","content":"written content"}`
	_, err := Write(spec, false, false, WithFileSystem(m))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := m.ReadFile("w.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "written content" {
		t.Fatalf("expected 'written content', got %q", string(data))
	}
}

func TestWriteAtomicWithMemFS(t *testing.T) {
	// This tests the full atomic write + backup + rename cycle
	m := NewMemFS(map[string]string{"target.txt": "original\n"})
	spec := `{"file":"target.txt","content":"replacement\n"}`
	_, err := Write(spec, false, false, WithFileSystem(m))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := m.ReadFile("target.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "replacement\n" {
		t.Fatalf("expected 'replacement\\n', got %q", string(data))
	}
	// No temp files should remain in MemFS
	for _, name := range m.List() {
		if strings.HasPrefix(name, ".fe-") {
			t.Fatalf("expected no temp files remaining in MemFS, found %q", name)
		}
	}
}

func TestFuncRangeWithMemFS(t *testing.T) {
	content := "package main\n\nfunc hello() {\n\treturn 1\n}\n"
	m := NewMemFS(map[string]string{"main.go": content})
	res, err := FuncRange("main.go", 3, WithFileSystem(m))
	if err != nil {
		t.Fatalf("FuncRange: %v", err)
	}
	if res.Start != 3 || res.End != 5 {
		t.Fatalf("expected range 3..5, got %d..%d", res.Start, res.End)
	}
}

func TestBalanceWithMemFS(t *testing.T) {
	content := "function demo() {\n\treturn 1;\n}\n"
	m := NewMemFS(map[string]string{"a.js": content})
	out, err := CheckStructureBalance("a.js", true, WithFileSystem(m))
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty balance output")
	}
}
