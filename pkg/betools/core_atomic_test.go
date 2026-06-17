package betools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func noFeFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".fe-") {
			t.Fatalf("found orphan temp/backup file: %s", e.Name())
		}
	}
}

func TestWriteFileAtomic_NoTempLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	if err := writeFileAtomic(path, "replaced\n", WithFileSystem(OSFileSystem{})); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "replaced\n" {
		t.Fatalf("expected 'replaced\\n', got %q", string(got))
	}
	noFeFiles(t, dir)
}

func TestWriteFilesAtomic_AllCommitted(t *testing.T) {
	dir := t.TempDir()
	writes := []WriteSpecItem{
		{File: filepath.Join(dir, "a.txt"), Content: "alpha\n"},
		{File: filepath.Join(dir, "b.txt"), Content: "beta\n"},
	}

	if err := writeFilesAtomic(writes, WithFileSystem(OSFileSystem{})); err != nil {
		t.Fatalf("writeFilesAtomic: %v", err)
	}

	for _, w := range writes {
		got, err := os.ReadFile(w.File)
		if err != nil {
			t.Fatalf("read %s: %v", w.File, err)
		}
		if string(got) != w.Content {
			t.Fatalf("%s: expected %q, got %q", w.File, w.Content, string(got))
		}
	}
	noFeFiles(t, dir)
}

func TestWriteFilesAtomic_RollbackCrashSafe(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(a, []byte("original-a\n"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}

	// The second target points into a non-existent directory, so its rename
	// will fail after the first file has already been committed.
	writes := []WriteSpecItem{
		{File: a, Content: "modified-a\n"},
		{File: filepath.Join(dir, "missing-dir", "b.txt"), Content: "b\n"},
	}

	err := writeFilesAtomic(writes, WithFileSystem(OSFileSystem{}))
	if err == nil {
		t.Fatal("expected error for partial multi-file write")
	}

	got, err := os.ReadFile(a)
	if err != nil {
		t.Fatalf("read a: %v", err)
	}
	if string(got) != "original-a\n" {
		t.Fatalf("expected a to be rolled back to original, got %q", string(got))
	}
	noFeFiles(t, dir)
}

func TestWriteFilesAtomic_KeepsFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mode.txt")
	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}

	writes := []WriteSpecItem{{File: path, Content: "y\n"}}
	if err := writeFilesAtomic(writes, WithFileSystem(OSFileSystem{})); err != nil {
		t.Fatalf("writeFilesAtomic: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %o", info.Mode().Perm())
	}
}

func TestRollbackSnapshots_AtomicRestore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", dir)
	resetSnapshotStore(t)
	CommitSnapshots()

	path := filepath.Join(t.TempDir(), "rollback.txt")
	if err := os.WriteFile(path, []byte("original\ncontent\n"), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	if _, err := Replace(path, 1, 1, nil, "modified\n", "plain", false, "", false); err != nil {
		t.Fatalf("replace: %v", err)
	}

	count, errs := RollbackSnapshots(1)
	if len(errs) > 0 {
		t.Fatalf("rollback errors: %v", errs)
	}
	if count != 1 {
		t.Fatalf("expected 1 rollback, got %d", count)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "original\ncontent\n" {
		t.Fatalf("expected original content, got %q", string(got))
	}

	parent := filepath.Dir(path)
	noFeFiles(t, parent)
	CommitSnapshots()
}
