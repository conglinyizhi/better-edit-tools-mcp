package betools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────
// Session 系统测试
// ──────────────────────────────────────────────

func TestCreateSession_ReturnsID(t *testing.T) {
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "sess.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	id := CreateSession(path, 1, 3)
	if id == "" {
		t.Fatal("expected non-empty session ID")
	}
	if len(id) != 36 {
		t.Fatalf("expected UUID length 36, got %d: %q", len(id), id)
	}
}

func TestGetSession_ReturnsNilForUnknown(t *testing.T) {
	s := GetSession("nonexistent-id")
	if s != nil {
		t.Fatalf("expected nil for unknown session, got %+v", s)
	}
}

func TestGetSession_ReturnsStoredSession(t *testing.T) {
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "sess2.txt")
	if err := os.WriteFile(path, []byte("x\ny\nz\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	id := CreateSession(path, 2, 3)
	s := GetSession(id)
	if s == nil {
		t.Fatal("expected session to be found")
	}
	if s.File != path {
		t.Fatalf("expected file %q, got %q", path, s.File)
	}
	if s.StartLine != 2 || s.EndLine != 3 {
		t.Fatalf("expected range 2..3, got %d..%d", s.StartLine, s.EndLine)
	}
	if s.LineCount != 2 {
		t.Fatalf("expected LineCount=2, got %d", s.LineCount)
	}
}

func TestSessionFromCache_FileUnchanged_NoWarning(t *testing.T) {
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "cache_ok.txt")
	if err := os.WriteFile(path, []byte("keep\nsame\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	id := CreateSession(path, 1, 2)
	_, warn := SessionFromCache(id)
	if warn != "" {
		t.Fatalf("expected no warning for unchanged file, got: %q", warn)
	}
}

func TestSessionFromCache_FileLineCountChanged_Warning(t *testing.T) {
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "cache_changed.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	id := CreateSession(path, 1, 3)
	if err := os.WriteFile(path, []byte("only2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, warn := SessionFromCache(id)
	if warn == "" {
		t.Fatal("expected warning for changed file size")
	}
	if !strings.Contains(warn, "file has changed") {
		t.Fatalf("expected 'file has changed' in warning, got: %q", warn)
	}
}

func TestSessionFromCache_FileDeleted_Error(t *testing.T) {
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "deleted.txt")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	id := CreateSession(path, 1, 2)
	os.Remove(path)
	_, warn := SessionFromCache(id)
	if warn == "" {
		t.Fatal("expected warning for deleted file")
	}
	if !strings.Contains(warn, "can't read file") {
		t.Fatalf("expected 'can't read file' warning, got: %q", warn)
	}
}

func TestSessionFromCache_InvalidID_ErrorMessage(t *testing.T) {
	_, warn := SessionFromCache("bogus-id")
	if warn == "" {
		t.Fatal("expected warning for invalid session ID")
	}
	if !strings.Contains(warn, "session not found") {
		t.Fatalf("expected 'session not found' message, got: %q", warn)
	}
}

func TestCleanupSession_RemovesSession(t *testing.T) {
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "cleanup.txt")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	id := CreateSession(path, 1, 2)
	CleanupSession(id)
	s := GetSession(id)
	if s != nil {
		t.Fatal("expected session to be removed after CleanupSession")
	}
}

// ──────────────────────────────────────────────
// Chip 系统测试
// ──────────────────────────────────────────────

func TestSaveChip_LargeArgs_CreatesChip(t *testing.T) {
	CommitSnapshots()
	args := map[string]any{
		"file":    "/tmp/large_test.txt",
		"content": strings.Repeat("x", 100),
	}
	id := SaveChip("be-write", args, "some error")
	if id == "" {
		t.Fatal("expected non-empty chip ID for large args")
	}
}

func TestSaveChip_SmallArgs_ReturnsEmpty(t *testing.T) {
	CommitSnapshots()
	args := map[string]any{"x": "y"}
	id := SaveChip("be-write", args, "")
	if id != "" {
		t.Fatalf("expected empty chip ID for small args, got %q", id)
	}
}

func TestGetChip_ValidID_ReturnsRecord(t *testing.T) {
	CommitSnapshots()
	args := map[string]any{"file": "/tmp/test.txt", "content": "hello"}
	id := SaveChip("be-write", args, "error msg")
	if id == "" {
		t.Skip("chip not saved (args too short)")
	}
	rec, err := GetChip(id)
	if err != nil {
		t.Fatalf("GetChip: %v", err)
	}
	if rec.Tool != "be-write" {
		t.Fatalf("expected tool 'be-write', got %q", rec.Tool)
	}
	if rec.ErrMsg != "error msg" {
		t.Fatalf("expected errMsg 'error msg', got %q", rec.ErrMsg)
	}
	if rec.Args["file"] != "/tmp/test.txt" {
		t.Fatalf("expected file arg, got %v", rec.Args["file"])
	}
}

func TestGetChip_InvalidID_Error(t *testing.T) {
	_, err := GetChip("nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid chip ID")
	}
}

func TestListChips_ReturnsIDs(t *testing.T) {
	CommitSnapshots()
	chipMu.Lock()
	chipStore = nil
	chipIDSet = nil
	chipMu.Unlock()
	id1 := SaveChip("be-write", map[string]any{"file": "/tmp/1.txt", "content": strings.Repeat("x", 100)}, "")
	id2 := SaveChip("be-replace", map[string]any{"file": "/tmp/2.txt", "content": strings.Repeat("y", 100)}, "")
	ids := ListChips()
	if len(ids) < 2 {
		t.Fatalf("expected at least 2 chips, got %d: %v", len(ids), ids)
	}
	if ids[0] != id1 {
		t.Fatalf("expected first chip ID %q, got %q", id1, ids[0])
	}
	if ids[1] != id2 {
		t.Fatalf("expected second chip ID %q, got %q", id2, ids[1])
	}
}

func TestSaveContentChip_ReturnsID(t *testing.T) {
	CommitSnapshots()
	chipMu.Lock()
	chipStore = nil
	chipIDSet = nil
	chipMu.Unlock()
	id, warn := SaveContentChip("be-delete", "deleted content here")
	if id == "" {
		t.Fatal("expected non-empty chip ID")
	}
	if warn != "" {
		t.Fatalf("unexpected overflow warning: %q", warn)
	}
	rec, err := GetChip(id)
	if err != nil {
		t.Fatalf("GetChip: %v", err)
	}
	content, ok := rec.Args["_content"].(string)
	if !ok {
		t.Fatalf("expected _content in args, got %v", rec.Args)
	}
	if content != "deleted content here" {
		t.Fatalf("expected content %q, got %q", "deleted content here", content)
	}
}

// ──────────────────────────────────────────────
// Snapshot 队列操作测试
// ──────────────────────────────────────────────

func TestListSnapshots_NewestFirst(t *testing.T) {
	CommitSnapshots()
	id1, _ := PushSnapshot(SnapshotRecord{Tool: "first", File: "/tmp/1.txt", Summary: "first"})
	id2, _ := PushSnapshot(SnapshotRecord{Tool: "second", File: "/tmp/2.txt", Summary: "second"})
	snapshots := ListSnapshots()
	if len(snapshots) < 2 {
		t.Fatalf("expected at least 2 snapshots, got %d", len(snapshots))
	}
	if snapshots[0].ID != id2 {
		t.Fatalf("expected newest first: got %q, want %q", snapshots[0].ID, id2)
	}
	if snapshots[1].ID != id1 {
		t.Fatalf("expected second newest: got %q, want %q", snapshots[1].ID, id1)
	}
	CommitSnapshots()
}

func TestSnapshotQueueStats_Accurate(t *testing.T) {
	CommitSnapshots()
	stats := SnapshotQueueStats()
	if stats.Used != 0 {
		t.Fatalf("expected 0 used after commit, got %d", stats.Used)
	}
	if stats.Max != MaxSnapshots {
		t.Fatalf("expected Max=%d, got %d", MaxSnapshots, stats.Max)
	}
	PushSnapshot(SnapshotRecord{Tool: "test", File: "/tmp/s.txt", Summary: "x"})
	stats = SnapshotQueueStats()
	if stats.Used != 1 {
		t.Fatalf("expected 1 used, got %d", stats.Used)
	}
	CommitSnapshots()
}

func TestRollbackSnapshots_Zero_Noop(t *testing.T) {
	CommitSnapshots()
	count, errs := RollbackSnapshots(0)
	if count != 0 || len(errs) > 0 {
		t.Fatalf("expected no-op rollback, got count=%d errs=%v", count, errs)
	}
}

// ──────────────────────────────────────────────
// Chip 持久化恢复测试
// ──────────────────────────────────────────────

func TestChipPersistence_LoadsFromDisk(t *testing.T) {
	// Save a chip
	args := map[string]any{"file": "/tmp/persist.txt", "content": strings.Repeat("data", 50)}
	id := SaveChip("be-write", args, "persist error")
	if id == "" {
		t.Skip("chip not saved")
	}

	// Verify it exists on disk
	path := filepath.Join(ChipDir(), fmt.Sprintf("chip-%s.json", id))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("chip file not found on disk: %v", err)
	}

	// Simulate restart: clear memory, reload from disk
	chipMu.Lock()
	chipStore = nil
	chipIDSet = nil
	chipMu.Unlock()

	loadChipsFromDisk()

	// Should be accessible via memory now
	ids := ListChips()
	found := false
	for _, cid := range ids {
		if cid == id {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("chip %s not found in ListChips after reload, got: %v", id, ids)
	}

	// Should be readable
	rec, err := GetChip(id)
	if err != nil {
		t.Fatalf("GetChip after reload: %v", err)
	}
	if rec.Tool != "be-write" {
		t.Fatalf("expected tool 'be-write', got %q", rec.Tool)
	}
	if rec.ErrMsg != "persist error" {
		t.Fatalf("expected errMsg 'persist error', got %q", rec.ErrMsg)
	}
}

func TestChipDir_NotTmp(t *testing.T) {
	dir := ChipDir()
	if dir == "" {
		t.Fatal("ChipDir() returned empty string")
	}
	if dir == "/tmp/bet-chips" {
		t.Fatal("ChipDir() should not return the old hardcoded /tmp/bet-chips path")
	}
}

func TestRollbackSnapshots_ExceedsQueue_Clamped(t *testing.T) {
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "rollback_clamp.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Replace(path, 2, 2, nil, "x\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	count, errs := RollbackSnapshots(100)
	if count < 1 {
		t.Fatalf("expected at least 1 rollback (clamped), got %d, errs=%v", count, errs)
	}
	_ = errs
	CommitSnapshots()
}

// ──────────────────────────────────────────────
// Target 解析完整分支测试
// ──────────────────────────────────────────────

func TestResolveTargetSpan_Line_Found(t *testing.T) {
	_, opt := withFS(map[string]string{"target_line.txt": "one\ntwo\nthree\n"})
	span, err := ResolveTargetSpan("target_line.txt", ContentTarget{Kind: "line", Value: "2"}, opt)
	if err != nil {
		t.Fatalf("resolve line target: %v", err)
	}
	if span.Start != 2 || span.End != 2 {
		t.Fatalf("expected line 2..2, got %d..%d", span.Start, span.End)
	}
}

func TestResolveTargetSpan_Line_OutOfRange(t *testing.T) {
	_, opt := withFS(map[string]string{"target_oor.txt": "only one\n"})
	_, err := ResolveTargetSpan("target_oor.txt", ContentTarget{Kind: "line", Value: "999"}, opt)
	if err == nil {
		t.Fatal("expected error for out-of-range line")
	}
}

func TestResolveTargetSpan_Marker_Found(t *testing.T) {
	_, opt := withFS(map[string]string{"target_marker.txt": "before\nTODO: fix this\nafter\n"})
	span, err := ResolveTargetSpan("target_marker.txt", ContentTarget{Kind: "marker", Value: "TODO"}, opt)
	if err != nil {
		t.Fatalf("resolve marker target: %v", err)
	}
	if span.Start != 2 || span.End != 2 {
		t.Fatalf("expected marker line 2..2, got %d..%d", span.Start, span.End)
	}
}

func TestResolveTargetSpan_Marker_NotFound(t *testing.T) {
	_, opt := withFS(map[string]string{"target_no_marker.txt": "a\nb\n"})
	_, err := ResolveTargetSpan("target_no_marker.txt", ContentTarget{Kind: "marker", Value: "missing"}, opt)
	if err == nil {
		t.Fatal("expected error for marker not found")
	}
}

func TestResolveTargetSpan_Marker_EmptyValue(t *testing.T) {
	_, opt := withFS(map[string]string{"target_empty_marker.txt": "a\nb\n"})
	_, err := ResolveTargetSpan("target_empty_marker.txt", ContentTarget{Kind: "marker", Value: "  "}, opt)
	if err == nil {
		t.Fatal("expected error for empty marker value")
	}
}

func TestResolveTargetSpan_Function_Found(t *testing.T) {
	_, opt := withFS(map[string]string{"target_func.go": "package main\n\nfunc hello() {\n\treturn 1\n}\n"})
	span, err := ResolveTargetSpan("target_func.go", ContentTarget{Kind: "function", Value: "hello"}, opt)
	if err != nil {
		t.Fatalf("resolve function target: %v", err)
	}
	if span.Start < 3 || span.End < span.Start {
		t.Fatalf("expected valid function range, got %d..%d", span.Start, span.End)
	}
}

func TestResolveTargetSpan_Function_NotFound(t *testing.T) {
	_, opt := withFS(map[string]string{"target_no_func.txt": "just text\n"})
	_, err := ResolveTargetSpan("target_no_func.txt", ContentTarget{Kind: "function", Value: "missing"}, opt)
	if err == nil {
		t.Fatal("expected error for function not found")
	}
}

func TestResolveTargetSpan_Tag_Found(t *testing.T) {
	_, opt := withFS(map[string]string{"target_tag.html": "<div>\n<span>text</span>\n</div>\n"})
	span, err := ResolveTargetSpan("target_tag.html", ContentTarget{Kind: "tag", Value: "span"}, opt)
	if err != nil {
		t.Fatalf("resolve tag target: %v", err)
	}
	if span.Start != 2 || span.End != 2 {
		t.Fatalf("expected span range 2..2, got %d..%d", span.Start, span.End)
	}
}

func TestResolveTargetSpan_Tag_NotFound(t *testing.T) {
	_, opt := withFS(map[string]string{"target_no_tag.txt": "just text\n"})
	_, err := ResolveTargetSpan("target_no_tag.txt", ContentTarget{Kind: "tag", Value: "missing"}, opt)
	if err == nil {
		t.Fatal("expected error for tag not found")
	}
}

func TestResolveTargetSpan_UnknownKind(t *testing.T) {
	_, opt := withFS(map[string]string{"target_unknown.txt": "a\nb\n"})
	_, err := ResolveTargetSpan("target_unknown.txt", ContentTarget{Kind: "unknownKind", Value: "x"}, opt)
	if err == nil {
		t.Fatal("expected error for unknown target kind")
	}
}

// ──────────────────────────────────────────────
// Diff 构建测试
// ──────────────────────────────────────────────

func TestBuildDiff_Plain(t *testing.T) {
	before := []string{"a\n", "b\n", "c\n"}
	after := []string{"a\n", "x\n", "c\n"}
	diff := buildDiff(before, after, 1, "plain")
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(diff, "a") || !strings.Contains(diff, "x") || !strings.Contains(diff, "c") {
		t.Fatalf("expected all content in diff, got: %q", diff)
	}
}

func TestBuildDiff_EmptyBefore(t *testing.T) {
	before := []string{}
	after := []string{"new\n", "content\n"}
	diff := buildDiff(before, after, 1, "plain")
	if diff == "" {
		t.Fatal("expected non-empty diff for insert into empty")
	}
}

func TestBuildDiff_EmptyAfter(t *testing.T) {
	before := []string{"a\n", "b\n"}
	after := []string{}
	diff := buildDiff(before, after, 1, "plain")
	if diff == "" {
		t.Fatal("expected non-empty diff for deletion")
	}
}

func TestBuildDiff_Unified(t *testing.T) {
	before := []string{"line1\n", "line2\n", "line3\n"}
	after := []string{"line1\n", "modified\n", "line3\n"}
	diff := buildDiff(before, after, 1, "unified")
	if diff == "" {
		t.Fatal("expected non-empty unified diff")
	}
	if !strings.Contains(diff, "---") || !strings.Contains(diff, "+++") {
		t.Fatalf("expected ---/+++ markers in unified diff, got: %q", diff)
	}
}

func TestQuickBalanceCheck_Balanced(t *testing.T) {
	result := quickBalanceCheck("func a() { return 1; }")
	if result == "" {
		t.Fatal("expected balanced OK result, got empty")
	}
	if !strings.Contains(result, "OK") {
		t.Fatalf("expected 'OK' in balanced result, got: %q", result)
	}
}

func TestQuickBalanceCheck_Unbalanced(t *testing.T) {
	result := quickBalanceCheck("func a() { return 1; ")
	if result == "" {
		t.Fatal("expected non-empty result for unbalanced braces")
	}
}

// ──────────────────────────────────────────────
// functionRangeRaw 边界测试
// ──────────────────────────────────────────────

func TestFuncRange_NestedBlocks(t *testing.T) {
	content := "package main\n\nfunc demo() {\n\tif true {\n\t\tfor i := 0; i < 10; i++ {\n\t\t\tdo()\n\t\t}\n\t}\n}\n"
	m, opt := withFS(map[string]string{"nested.go": content})
	res, err := FuncRange("nested.go", 3, opt)
	if err != nil {
		t.Fatalf("func-range nested: %v", err)
	}
	if res.Start != 3 {
		t.Fatalf("expected start at line 3 (func), got %d", res.Start)
	}
	if res.End != 9 {
		t.Fatalf("expected end at line 9 (closing of func), got %d", res.End)
	}
	_ = m
}

func TestFuncRange_TargetInsideInnerBlock(t *testing.T) {
	content := "package main\n\nfunc outer() {\n\tif true {\n\t\tinnerFunc()\n\t}\n}\n"
	m, opt := withFS(map[string]string{"inner.go": content})
	res, err := FuncRange("inner.go", 5, opt)
	if err != nil {
		t.Fatalf("func-range inside block: %v", err)
	}
	if res.Start != 3 {
		t.Fatalf("expected start at func (line 3), got %d", res.Start)
	}
	if res.End != 7 {
		t.Fatalf("expected end at func close (line 7), got %d", res.End)
	}
	_ = m
}

func TestFuncRange_TargetOutOfRange(t *testing.T) {
	m, opt := withFS(map[string]string{"oor_func.go": "package main\n\nfunc demo() {\n}\n"})
	_, _, err := Show("oor_func.go", 999, -1, false, opt)
	if err == nil {
		t.Fatal("expected error for out-of-range target line")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
	_ = m
}

// ──────────────────────────────────────────────
// Error 函数测试
// ──────────────────────────────────────────────

func TestError_SentinelValues(t *testing.T) {
	if ErrRead == nil || ErrWrite == nil || ErrInvalid == nil {
		t.Fatal("expected non-nil error sentinels")
	}
	if ErrRead.Error() == "" || ErrWrite.Error() == "" || ErrInvalid.Error() == "" {
		t.Fatal("expected non-empty error messages")
	}
}

func TestInvalidArg_WrapsErrInvalid(t *testing.T) {
	err := invalidArg("bad arg")
	if !errors.Is(err, ErrInvalid) {
		t.Fatal("expected invalidArg to wrap ErrInvalid")
	}
}

func TestNewWriteError_FormatsPath(t *testing.T) {
	err := newWriteError("/tmp/test", fmt.Errorf("disk full"))
	if err.Error() != "write /tmp/test: disk full" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestWritePath_FormatsPath(t *testing.T) {
	err := writePath("/tmp/test", fmt.Errorf("permission denied"))
	if !strings.Contains(err.Error(), "/tmp/test") {
		t.Fatalf("expected path in error message, got: %q", err.Error())
	}
}

func TestJSONParse_FormatsMessage(t *testing.T) {
	err := jsonParse(fmt.Errorf("syntax error"))
	if !strings.Contains(err.Error(), "parse JSON") {
		t.Fatalf("expected 'parse JSON' in error, got: %q", err.Error())
	}
}

// ──────────────────────────────────────────────
// Balance 输出格式测试
// ──────────────────────────────────────────────

func TestBalanceVerbose_HasMatchedAndUnbalanced(t *testing.T) {
	m, opt := withFS(map[string]string{"balance_v.js": "function demo() {\n\treturn 1;\n}\n"})
	out, err := CheckStructureBalance("balance_v.js", true, opt)
	if err != nil {
		t.Fatalf("balance verbose: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	if _, ok := v["matched"]; !ok {
		t.Fatalf("expected 'matched' in verbose output")
	}
	if _, ok := v["unbalanced"]; !ok {
		t.Fatalf("expected 'unbalanced' in verbose output")
	}
	_ = m
}

func keysOf(m map[string]any) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
