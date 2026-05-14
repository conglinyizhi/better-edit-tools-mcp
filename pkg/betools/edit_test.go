package betools

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(data)
}

func TestShowAutoUsesFunctionRange(t *testing.T) {
	path := writeTempFile(t, "main.go", "package main\n\nfunc demo() {\n\tprintln(\"x\")\n}\n")
	res, _, err := Show(path, 3, -1, false)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if res.Start != 3 || res.End != 5 {
		t.Fatalf("unexpected range: %+v", res)
	}
	if !strings.Contains(res.Content, "3\tfunc demo() {") {
		t.Fatalf("unexpected content: %q", res.Content)
	}
}

func TestShowExplicitRange(t *testing.T) {
	path := writeTempFile(t, "main.go", "one\ntwo\nthree\n")
	res, _, err := Show(path, 2, 3, false)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if res.Start != 2 || res.End != 3 {
		t.Fatalf("unexpected range: %+v", res)
	}
	if !strings.Contains(res.Content, "2\ttwo") || !strings.Contains(res.Content, "3\tthree") {
		t.Fatalf("unexpected content: %q", res.Content)
	}
}

func TestReadAlias(t *testing.T) {
	path := writeTempFile(t, "alias.txt", "a\nb\n")
	showRes, showID, err := Show(path, 1, 1, false)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	readRes, readID, err := Read(path, 1, 1, false)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if showRes.Content != readRes.Content || showRes.Start != readRes.Start || showRes.End != readRes.End {
		t.Fatalf("read alias mismatch: show=%+v read=%+v", showRes, readRes)
	}
	if showID == "" || readID == "" {
		t.Fatalf("expected session ids from read-like calls")
	}
}

func TestReplacePreviewDoesNotWrite(t *testing.T) {
	path := writeTempFile(t, "a.txt", "a\nb\nc\n")
	old := "b\n"
	res, err := Replace(path, 2, 2, &old, "x", true, "plain", true, "", false)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !res.Preview {
		t.Fatalf("preview flag missing: %+v", res)
	}
	if got := readFile(t, path); got != "a\nb\nc\n" {
		t.Fatalf("file changed unexpectedly: %q", got)
	}
}

func TestReplaceRejectsOldMismatch(t *testing.T) {
	path := writeTempFile(t, "a.txt", "a\nb\nc\n")
	old := "x\n"
	if _, err := Replace(path, 2, 2, &old, "y", true, "plain", true, "", false); err == nil {
		t.Fatalf("expected mismatch error")
	}
	if got := readFile(t, path); got != "a\nb\nc\n" {
		t.Fatalf("file changed unexpectedly: %q", got)
	}
}

func TestInsertAndDeleteWriteFile(t *testing.T) {
	path := writeTempFile(t, "a.txt", "a\nb\n")
	if _, err := Insert(path, 1, "x", true, "plain", false, false); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := readFile(t, path); got != "a\nx\nb\n" {
		t.Fatalf("unexpected after insert: %q", got)
	}
	if _, err := Delete(path, 2, 2, 0, nil, "plain", false, false); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := readFile(t, path); got != "a\nb\n" {
		t.Fatalf("unexpected after delete: %q", got)
	}
}

func TestBatchSortsFromBottomUp(t *testing.T) {
	path := writeTempFile(t, "a.txt", "a\nb\nc\nd\n")
	spec := `{"file":"` + path + `","edits":[{"action":"delete-lines","start":2,"end":2},{"action":"insert-after","line":1,"content":"x"},{"action":"replace-lines","start":4,"end":4,"content":"z"}]}`
	res, err := Batch(spec, false, false)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if res.Files != 1 || len(res.Results) != 1 {
		t.Fatalf("unexpected batch result: %+v", res)
	}
	if got := readFile(t, path); got != "a\nx\nc\nz\n" {
		t.Fatalf("unexpected batch content: %q", got)
	}
}

func TestWriteDegradedParser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := `{"file":"` + path + `","content":"hello\nworld"}`
	res, err := Write(spec, false, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res.Degraded {
		t.Fatalf("unexpected degraded for valid json: %+v", res)
	}
	if got := readFile(t, path); got != "hello\nworld" {
		t.Fatalf("unexpected write content: %q", got)
	}
}

func TestWriteRawFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := `{"file":"` + path + `","content":"hello
world"}`
	res, err := Write(spec, false, false, false)
	if err != nil {
		t.Fatalf("write fallback: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag: %+v", res)
	}
	if got := readFile(t, path); !strings.Contains(got, "hello") {
		t.Fatalf("unexpected fallback content: %q", got)
	}
}

func TestShowRejectsBinaryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bin.dat")
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02, 0x7f, 0xff}, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	_, _, err := Show(path, 1, 1, false)
	if err == nil {
		t.Fatal("expected binary rejection")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestInjectedFileSystemIsUsed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sandbox.txt")
	fsys := sandboxFS{root: dir}
	if _, _, err := Show(path, 1, 1, false, WithFileSystem(fsys)); err == nil {
		t.Fatal("expected error for missing file inside injected fs")
	}
	if _, err := Write(`{"file":"sandbox.txt","content":"hello"}`, false, false, false, WithFileSystem(fsys)); err != nil {
		t.Fatalf("write with injected fs: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sandbox file: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("unexpected sandbox content: %q", string(got))
	}
}

func TestBalanceSimple(t *testing.T) {
	path := writeTempFile(t, "a.js", "function demo() { return [1, 2]; }\n")
	out, err := CheckStructureBalance(path, false)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	if v["mode"] != "unbalanced" {
		t.Fatalf("unexpected mode: %v", v["mode"])
	}
}

func TestTargetResolutionLine(t *testing.T) {
	path := writeTempFile(t, "a.txt", "one\ntwo\nthree\n")
	span, err := ResolveTargetSpan(path, ContentTarget{Kind: "line", Value: "2"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if span.Start != 2 || span.End != 2 {
		t.Fatalf("unexpected span: %+v", span)
	}
}
