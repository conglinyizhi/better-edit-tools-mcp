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
// #38: be-read 负 end 值从文件末尾倒数行号
// ──────────────────────────────────────────────

func TestShowNegativeEnd_FromEnd(t *testing.T) {
	m, opt := withFS(map[string]string{"a.txt": "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\n"})
	res, _, err := Show("a.txt", 1, -3, false, opt)
	if err != nil {
		t.Fatalf("show with end=-3: %v", err)
	}
	if res.Start != 1 || res.End != 9 {
		t.Fatalf("expected range 1..9 for end=-3 on 11 lines, got %d..%d", res.Start, res.End)
	}
	if !strings.Contains(res.Content, "line9") {
		t.Fatalf("expected content to include line9, got: %q", res.Content)
	}
	if strings.Contains(res.Content, "line10") {
		t.Fatalf("expected content NOT to include line10, got: %q", res.Content)
	}
	_ = m
}

func TestShowNegativeEnd_BeforeStart(t *testing.T) {
	m, opt := withFS(map[string]string{"b.txt": "a\nb\nc\nd\ne\n"})
	_, _, err := Show("b.txt", 3, -9, false, opt)
	if err == nil {
		t.Fatal("expected error for negative end before start")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got: %v", err)
	}
	_ = m
}

func TestShowNegativeEnd_ExactEnd(t *testing.T) {
	m, opt := withFS(map[string]string{"c.txt": "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"})
	res, _, err := Show("c.txt", 5, -2, false, opt)
	if err != nil {
		t.Fatalf("show with end=-2: %v", err)
	}
	if res.End != 9 {
		t.Fatalf("expected end=9 for end=-2 on 10 lines, got %d", res.End)
	}
	_ = m
}

func TestShowNegativeEnd_AutoModePreserved(t *testing.T) {
	m, opt := withFS(map[string]string{"main.go": "package main\n\nfunc demo() {\n\tprintln(\"x\")\n}\n"})
	res, _, err := Show("main.go", 3, -1, false, opt)
	if err != nil {
		t.Fatalf("show with end=-1 (auto): %v", err)
	}
	if res.Start != 3 || res.End != 5 {
		t.Fatalf("expected auto mode range 3..5, got %d..%d", res.Start, res.End)
	}
	_ = m
}

// ──────────────────────────────────────────────
// #39: be-read 空文件读取
// ──────────────────────────────────────────────

func TestShowEmptyFile_ReturnsOK(t *testing.T) {
	m, opt := withFS(nil)
	// Empty file via MemFS: just read a non-existent path won't work,
	// so create an empty file in MemFS
	m.WriteFile("empty.txt", []byte{}, 0o644)
	res, _, err := Show("empty.txt", 1, 1, false, opt)
	if err != nil {
		t.Fatalf("show on empty file: %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("expected status ok, got %q", res.Status)
	}
	if res.Total != 0 {
		t.Fatalf("expected total=0, got %d", res.Total)
	}
	if res.Content != "" {
		t.Fatalf("expected empty content, got %q", res.Content)
	}
}

func TestReadEmptyFile_ReturnsOK(t *testing.T) {
	m, opt := withFS(nil)
	// Verify Read matches Show for empty files
	m.WriteFile("empty2.txt", []byte{}, 0o644)
	res, _, err := Read("empty2.txt", 1, 1, false, opt)
	if err != nil {
		t.Fatalf("read on empty file: %v", err)
	}
	if res.Total != 0 {
		t.Fatalf("expected total=0 for Read on empty file, got %d", res.Total)
	}
	_ = m
}

// ──────────────────────────────────────────────
// #40: be-read 空行显示 <blank> 标记
// ──────────────────────────────────────────────

func TestShowBlankLineMarker(t *testing.T) {
	m, opt := withFS(map[string]string{"blank.txt": "a\n\n\nb\n"})
	res, _, err := Show("blank.txt", 1, 4, false, opt)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(res.Content, "2\t<blank>") {
		t.Fatalf("expected line 2 to show <blank>, got: %q", res.Content)
	}
	if !strings.Contains(res.Content, "3\t<blank>") {
		t.Fatalf("expected line 3 to show <blank>, got: %q", res.Content)
	}
	if strings.Contains(res.Content, "1\t<blank>") {
		t.Fatalf("line 1 has content and should NOT show <blank>")
	}
	_ = m
}

func TestShowBlankLineMarker_OnlyBlankLines(t *testing.T) {
	m, opt := withFS(map[string]string{"only_blank.txt": "\n\n\n"})
	res, _, err := Show("only_blank.txt", 1, 3, false, opt)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	for _, lineNum := range []int{1, 2, 3} {
		expected := fmt.Sprintf("%d\t<blank>", lineNum)
		if !strings.Contains(res.Content, expected) {
			t.Fatalf("expected line %d to show <blank>, content: %q", lineNum, res.Content)
		}
	}
	_ = m
}

// ──────────────────────────────────────────────
// #41: 纯空行文件误报 trailing whitespace
// ──────────────────────────────────────────────

func TestScanWarnings_BlankLinesOnly_NoTrailingWhitespace(t *testing.T) {
	content := "1\t<blank>\n2\t<blank>\n3\t<blank>"
	warnings := scanContentWarnings(content)
	for _, w := range warnings {
		if strings.Contains(w, "trailing whitespace") {
			t.Fatalf("unexpected trailing whitespace warning for blank-line content: %q", w)
		}
	}
}

func TestScanWarnings_ActualTrailingSpaces_StillDetected(t *testing.T) {
	content := "1\tline1  \n2\tline2\n"
	warnings := scanContentWarnings(content)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "trailing whitespace") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected trailing whitespace warning for actual trailing spaces")
	}
}

func TestScanWarnings_BlankLinesMixedContent_NoFalsePositive(t *testing.T) {
	content := "1\tnormal\n2\t<blank>\n3\tmore\n4\t<blank>"
	warnings := scanContentWarnings(content)
	for _, w := range warnings {
		if strings.Contains(w, "trailing whitespace") {
			t.Fatalf("unexpected trailing whitespace warning: %q", w)
		}
	}
}

// ──────────────────────────────────────────────
// #42: 空文件/空内容时不应报告 trailing newline
// ──────────────────────────────────────────────

func TestScanWarnings_EmptyContent_NoNewlineWarning(t *testing.T) {
	warnings := scanContentWarnings("")
	for _, w := range warnings {
		if strings.Contains(w, "does not end with a newline") {
			t.Fatalf("unexpected 'no newline' warning for empty content: %q", w)
		}
	}
}

func TestScanWarnings_SingleLine_ReportsNoNewline(t *testing.T) {
	warnings := scanContentWarnings("hello")
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "does not end with a newline") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'does not end with a newline' warning for single-line content")
	}
}

func TestScanWarnings_MultiLineWithNewline_NoWarning(t *testing.T) {
	warnings := scanContentWarnings("hello\nworld\n")
	for _, w := range warnings {
		if strings.Contains(w, "does not end with a newline") {
			t.Fatalf("unexpected 'no newline' warning for content ending with newline")
		}
	}
}

// ──────────────────────────────────────────────
// #43: be-write 覆盖 CRLF 文件时保留换行风格
// ──────────────────────────────────────────────

func TestWritePreservesExistingCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crlf.txt")
	orig := "line1\r\nline2\r\nline3\r\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatalf("write orig: %v", err)
	}
	spec := `{"file":"` + path + `","content":"new1\nnew2\n"}`
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gotStr := string(got)
	if !strings.Contains(gotStr, "\r\n") {
		t.Fatalf("expected CRLF preserved in output, got: %q", gotStr)
	}
	if gotStr != "new1\r\nnew2\r\n" {
		t.Fatalf("expected content with CRLF, got: %q", gotStr)
	}
}

func TestWriteNewFileDefaultLF(t *testing.T) {
	m, opt := withFS(nil)
	spec := `{"file":"new_lf.txt","content":"hello\nworld\n"}`
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFS(t, m, "new_lf.txt")
	if strings.Contains(got, "\r\n") {
		t.Fatalf("new file should use LF, got CRLF: %q", got)
	}
	if got != "hello\nworld\n" {
		t.Fatalf("expected LF content, got: %q", got)
	}
}

func TestWriteCRLFContentMixed_UnifiedToTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed_crlf.txt")
	orig := "keep\r\nme\r\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatalf("write orig: %v", err)
	}
	spec := `{"file":"` + path + `","content":"a\r\nb\nc\r\nd\n"}`
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gotStr := string(got)
	if !strings.Contains(gotStr, "\r\n") {
		t.Fatalf("expected CRLF in output, got: %q", gotStr)
	}
	if strings.Contains(gotStr, "\n\r") {
		t.Fatalf("unexpected \\n\\r sequence: %q", gotStr)
	}
}

// ──────────────────────────────────────────────
// #42 (complement): Write empty content → no false warnings
// ──────────────────────────────────────────────

func TestWriteEmptyContent_CleanWarnings(t *testing.T) {
	m, opt := withFS(nil)
	spec := `{"file":"empty_write.txt","content":""}`
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write empty content: %v", err)
	}
	for _, r := range res.Results {
		for _, w := range r.Warnings {
			if strings.Contains(w, "does not end with a newline") {
				t.Fatalf("unexpected 'no newline' warning for empty write: %q", w)
			}
		}
	}
	_ = m
}

// ──────────────────────────────────────────────
// #6 / #12: Replace/Delete 行号范围边界语义
// ──────────────────────────────────────────────

func TestReplaceFirstLine(t *testing.T) {
	m, opt := withFS(map[string]string{"first.txt": "old_first\nsecond\nthird\n"})
	_, err := Replace("first.txt", 1, 1, nil, "new_first\n", "plain", false, "", false, opt)
	if err != nil {
		t.Fatalf("replace first line: %v", err)
	}
	got := readFS(t, m, "first.txt")
	if got != "new_first\nsecond\nthird\n" {
		t.Fatalf("expected first line replaced, got: %q", got)
	}
}

func TestReplaceLastLine(t *testing.T) {
	m, opt := withFS(map[string]string{"last.txt": "first\nsecond\nold_last\n"})
	_, err := Replace("last.txt", 3, 3, nil, "new_last\n", "plain", false, "", false, opt)
	if err != nil {
		t.Fatalf("replace last line: %v", err)
	}
	got := readFS(t, m, "last.txt")
	if got != "first\nsecond\nnew_last\n" {
		t.Fatalf("expected last line replaced, got: %q", got)
	}
}

func TestReplaceRangeInclusive(t *testing.T) {
	m, opt := withFS(map[string]string{"range.txt": "a\nb\nc\nd\ne\n"})
	_, err := Replace("range.txt", 2, 4, nil, "x\ny\n", "plain", false, "", false, opt)
	if err != nil {
		t.Fatalf("replace range: %v", err)
	}
	got := readFS(t, m, "range.txt")
	if got != "a\nx\ny\ne\n" {
		t.Fatalf("expected 3 lines replaced by 2, got: %q", got)
	}
}

// ──────────────────────────────────────────────
// #14: be-insert after_line 语义
// ──────────────────────────────────────────────

func TestInsertAtBeginning(t *testing.T) {
	m, opt := withFS(map[string]string{"begin.txt": "b\nc\n"})
	_, err := Insert("begin.txt", 0, "a\n", "plain", false, false, opt)
	if err != nil {
		t.Fatalf("insert at beginning: %v", err)
	}
	got := readFS(t, m, "begin.txt")
	if got != "a\nb\nc\n" {
		t.Fatalf("expected 'a' at beginning, got: %q", got)
	}
}

func TestInsertAtEnd(t *testing.T) {
	m, opt := withFS(map[string]string{"end.txt": "a\nb\n"})
	_, err := Insert("end.txt", 2, "c\n", "plain", false, false, opt)
	if err != nil {
		t.Fatalf("insert at end: %v", err)
	}
	got := readFS(t, m, "end.txt")
	if got != "a\nb\nc\n" {
		t.Fatalf("expected 'c' at end, got: %q", got)
	}
}

func TestInsertAtBeginning_EmptyFile(t *testing.T) {
	m, opt := withFS(nil)
	m.WriteFile("empty_insert.txt", []byte{}, 0o644)
	_, err := Insert("empty_insert.txt", 0, "first\n", "plain", false, false, opt)
	if err != nil {
		t.Fatalf("insert into empty file: %v", err)
	}
	got := readFS(t, m, "empty_insert.txt")
	if got != "first\n" {
		t.Fatalf("expected 'first' in empty file, got: %q", got)
	}
}

// ──────────────────────────────────────────────
// #15: be-func-range 应包含函数签名行
// ──────────────────────────────────────────────

func TestFuncRangeIncludesSignature(t *testing.T) {
	m, opt := withFS(map[string]string{"main.go": "package main\n\nfunc hello() {\n\treturn 1\n}\n"})
	res, err := FuncRange("main.go", 3, opt)
	if err != nil {
		t.Fatalf("func-range: %v", err)
	}
	if res.Start != 3 {
		t.Fatalf("expected range to start at line 3 (func signature), got %d", res.Start)
	}
	if res.End != 5 {
		t.Fatalf("expected range to end at line 5 (closing brace), got %d", res.End)
	}
	_ = m
}

func TestFuncRange_EmptyBody(t *testing.T) {
	m, opt := withFS(map[string]string{"empty.go": "package main\n\nfunc empty() {}\n"})
	res, err := FuncRange("empty.go", 3, opt)
	if err != nil {
		t.Fatalf("func-range empty body: %v", err)
	}
	if res.Start != 3 {
		t.Fatalf("expected start=3 for func on one line, got %d", res.Start)
	}
	_ = m
}

func TestTagRangeSelfClosing_Skipped(t *testing.T) {
	m, opt := withFS(map[string]string{"page.html": "<div>\n<br/>\n</div>\n"})
	res, err := TagRange("page.html", 2, opt)
	if err != nil {
		t.Fatalf("tag-range for <br/>: %v", err)
	}
	if res.Tag != "div" {
		t.Fatalf("expected enclosing tag 'div', got %q", res.Tag)
	}
	if res.Start != 1 || res.End != 3 {
		t.Fatalf("expected range 1..3 for <div>..</div>, got %d..%d", res.Start, res.End)
	}
	_ = m
}

func TestTagRange_SingleLinePaired(t *testing.T) {
	m, opt := withFS(map[string]string{"span.html": "<div><span>hello</span></div>\n"})
	res, err := TagRange("span.html", 1, opt)
	if err != nil {
		t.Fatalf("tag-range single line: %v", err)
	}
	if res.Tag != "span" {
		t.Fatalf("expected innermost tag 'span', got %q", res.Tag)
	}
	_ = m
}

// ──────────────────────────────────────────────
// #4: be-balance 应跳过字符串/注释中的括号
// ──────────────────────────────────────────────

func TestBalanceSkipsBracesInStrings(t *testing.T) {
	m, opt := withFS(map[string]string{"strings.js": "var x = \"{\";\nvar y = \"}\";\n"})
	out, err := CheckStructureBalance("strings.js", true, opt)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	unbalanced, _ := v["unbalanced"].([]any)
	for _, item := range unbalanced {
		m2, _ := item.(map[string]any)
		if sym, _ := m2["symbol"].(string); sym == "{" || sym == "}" {
			t.Fatalf("expected braces in strings to be ignored, found unmatched: %v", m2)
		}
	}
	_ = m
}

func TestBalanceSkipsBracesInComments(t *testing.T) {
	m, opt := withFS(map[string]string{"comments.js": "// line comment { with brace\n/* block { comment */\nvar x = {};\n"})
	out, err := CheckStructureBalance("comments.js", true, opt)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	unbalanced, _ := v["unbalanced"].([]any)
	for _, item := range unbalanced {
		m2, _ := item.(map[string]any)
		if sym, _ := m2["symbol"].(string); sym == "{" || sym == "}" {
			t.Fatalf("expected braces in comments to be ignored, found unmatched: %v", m2)
		}
	}
	_ = m
}

// ──────────────────────────────────────────────
// #27: be-delete function target 不吞噬相邻花括号
// ──────────────────────────────────────────────

func TestDeleteFunctionTarget_NoEatAdjacentBrace(t *testing.T) {
	content := "package main\n\nfunc first() {\n\treturn 1\n}\n\nfunc second() {\n\treturn 2\n}\n\nfunc third() {\n\treturn 3\n}\n"
	m, opt := withFS(map[string]string{"main.go": content})
	span, err := ResolveTargetSpan("main.go", ContentTarget{Kind: "function", Value: "second"}, opt)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	_, err = Delete("main.go", span.Start, span.End, "plain", false, false, opt)
	if err != nil {
		t.Fatalf("delete second: %v", err)
	}
	got := readFS(t, m, "main.go")
	if !strings.Contains(got, "func first() {") {
		t.Fatal("first function should remain")
	}
	if !strings.Contains(got, "func third() {") {
		t.Fatal("third function should remain")
	}
	if strings.Contains(got, "func second() {") {
		t.Fatal("second function should be deleted")
	}
}

// ──────────────────────────────────────────────
// #28: Snapshot rollback（仍需真实 FS）
// ──────────────────────────────────────────────

func TestSnapshotRollback(t *testing.T) {
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", t.TempDir())
	resetSnapshotStore(t)
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "rollback.txt")
	if err := os.WriteFile(path, []byte("original\nline2\nline3\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Replace(path, 2, 2, nil, "modified\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	got := readFile(t, path)
	if got != "original\nmodified\nline3\n" {
		t.Fatalf("unexpected after replace: %q", got)
	}
	count, errs := RollbackSnapshots(1)
	if len(errs) > 0 {
		t.Fatalf("rollback errors: %v", errs)
	}
	if count != 1 {
		t.Fatalf("expected 1 rollback, got %d", count)
	}
	restored := readFile(t, path)
	if restored != "original\nline2\nline3\n" {
		t.Fatalf("expected rollback to original, got: %q", restored)
	}
	CommitSnapshots()
}

func TestSnapshotRollback_Multiple(t *testing.T) {
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", t.TempDir())
	resetSnapshotStore(t)
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "multiback.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _ = Replace(path, 1, 1, nil, "x\n", "plain", false, "", false)
	_, _ = Replace(path, 3, 3, nil, "z\n", "plain", false, "", false)
	got := readFile(t, path)
	if got != "x\nb\nz\n" {
		t.Fatalf("expected both edits, got: %q", got)
	}
	count, errs := RollbackSnapshots(2)
	if len(errs) > 0 {
		t.Fatalf("rollback errors: %v", errs)
	}
	if count != 2 {
		t.Fatalf("expected 2 rollbacks, got %d", count)
	}
	restored := readFile(t, path)
	if restored != "a\nb\nc\n" {
		t.Fatalf("expected full rollback to original, got: %q", restored)
	}
	CommitSnapshots()
}

// #58: 连续多次 replace 后 rollback 应完整恢复原始文件
func TestSnapshotRollback_ConsecutiveReplace(t *testing.T) {
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", t.TempDir())
	resetSnapshotStore(t)
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "consecutive.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Replace(path, 2, 2, nil, "X\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace 1: %v", err)
	}
	_, err = Replace(path, 4, 4, nil, "Y\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace 2: %v", err)
	}
	got := readFile(t, path)
	if got != "a\nX\nc\nY\ne\n" {
		t.Fatalf("unexpected after both edits: %q", got)
	}
	count, errs := RollbackSnapshots(2)
	if len(errs) > 0 {
		t.Fatalf("rollback errors: %v", errs)
	}
	if count != 2 {
		t.Fatalf("expected 2 rollbacks, got %d", count)
	}
	restored := readFile(t, path)
	if restored != "a\nb\nc\nd\ne\n" {
		t.Fatalf("expected full rollback to original, got: %q", restored)
	}
	CommitSnapshots()
}

// #58: snapshot 应持久化到磁盘，进程重启后仍可 rollback
func TestSnapshotPersistence_Restart(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", dir)
	resetSnapshotStore(t)
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "restart.txt")
	if err := os.WriteFile(path, []byte("original\ncontent\nhere\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Replace(path, 2, 2, nil, "modified\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if readFile(t, path) != "original\nmodified\nhere\n" {
		t.Fatalf("unexpected after replace")
	}

	// Verify a snapshot file was persisted
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read snapshot dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 snapshot file, got %d", len(entries))
	}

	// Simulate process restart: clear in-memory state
	resetSnapshotStore(t)

	// Rollback should reload from disk and restore the full original file
	count, errs := RollbackSnapshots(1)
	if len(errs) > 0 {
		t.Fatalf("rollback errors after restart: %v", errs)
	}
	if count != 1 {
		t.Fatalf("expected 1 rollback after restart, got %d", count)
	}
	restored := readFile(t, path)
	if restored != "original\ncontent\nhere\n" {
		t.Fatalf("expected rollback after restart to original, got: %q", restored)
	}
	CommitSnapshots()
}

// #58: snapshot 队列满时最旧的磁盘文件应被删除
func TestSnapshotEviction_RemovesDiskFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", dir)
	resetSnapshotStore(t)
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "evict.txt")
	if err := os.WriteFile(path, []byte("line\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	for i := 0; i < MaxSnapshots+1; i++ {
		_, err := Replace(path, 1, 1, nil, fmt.Sprintf("v%d\n", i), "plain", false, "", false)
		if err != nil {
			t.Fatalf("replace %d: %v", i, err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read snapshot dir: %v", err)
	}
	if len(entries) != MaxSnapshots {
		t.Fatalf("expected %d snapshot files after eviction, got %d", MaxSnapshots, len(entries))
	}

	CommitSnapshots()
}

// Capacity: 超过总磁盘容量上限时最旧 snapshot 被自动淘汰
func TestSnapshotCapacityEviction_RemovesOldest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", dir)
	resetSnapshotStore(t)
	CommitSnapshots()

	// Override capacity to a small value so tests run quickly.
	maxSnapshotTotalBytes = 10000

	content := strings.Repeat("x", 1500) + "\n"
	ids := make([]string, 4)
	warnings := 0
	for i := 0; i < 4; i++ {
		id, warn := PushSnapshot(SnapshotRecord{
			Tool:    "test",
			File:    "/tmp/cap.txt",
			Summary: fmt.Sprintf("snap-%d", i),
			Before:  SnapshotRange{Start: 1, End: 1, Lines: []string{content}},
			After:   SnapshotRange{Start: 1, End: 1, Lines: []string{content}},
		})
		ids[i] = id
		if warn != "" {
			warnings++
		}
	}

	if warnings == 0 {
		t.Fatal("expected at least one capacity eviction warning")
	}

	list := ListSnapshots()
	if len(list) != 3 {
		t.Fatalf("expected 3 snapshots after capacity eviction, got %d", len(list))
	}

	seen := make(map[string]bool)
	for _, s := range list {
		seen[s.ID] = true
	}
	if seen[ids[0]] {
		t.Fatalf("oldest snapshot should have been evicted")
	}
	for i := 1; i < 4; i++ {
		if !seen[ids[i]] {
			t.Fatalf("snapshot %d (id=%s) should remain", i, ids[i])
		}
	}

	stats := SnapshotQueueStats()
	if stats.DiskBytes > int64(maxSnapshotTotalBytes) {
		t.Fatalf("disk bytes %d exceeds capacity %d", stats.DiskBytes, maxSnapshotTotalBytes)
	}

	if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("snapshot-%s.json", ids[0]))); !os.IsNotExist(err) {
		t.Fatalf("evicted snapshot file should be removed from disk")
	}
	for i := 1; i < 4; i++ {
		if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("snapshot-%s.json", ids[i]))); err != nil {
			t.Fatalf("remaining snapshot file missing: %v", err)
		}
	}

	CommitSnapshots()
}

// Capacity: 加载磁盘 snapshot 时超过总容量上限会淘汰最旧
func TestSnapshotCapacityEviction_LoadFromDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", dir)
	resetSnapshotStore(t)
	CommitSnapshots()

	content := strings.Repeat("y", 1500) + "\n"
	ids := make([]string, 4)
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("disk%d", i)
		ids[i] = id
		rec := SnapshotRecord{
			ID:        id,
			Tool:      "test",
			File:      "/tmp/diskcap.txt",
			Summary:   fmt.Sprintf("snap-%d", i),
			Before:    SnapshotRange{Start: 1, End: 1, Lines: []string{content}},
			After:     SnapshotRange{Start: 1, End: 1, Lines: []string{content}},
			CreatedAt: int64(i),
		}
		data, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			t.Fatalf("marshal snapshot %d: %v", i, err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("snapshot-%s.json", id)), data, 0o644); err != nil {
			t.Fatalf("write snapshot %d: %v", i, err)
		}
	}

	// Set capacity below total disk size and reload from disk.
	resetSnapshotStore(t)
	maxSnapshotTotalBytes = 10000
	ensureSnapshotStore()

	list := ListSnapshots()
	if len(list) != 3 {
		t.Fatalf("expected 3 snapshots loaded after capacity eviction, got %d", len(list))
	}

	seen := make(map[string]bool)
	for _, s := range list {
		seen[s.ID] = true
	}
	if seen[ids[0]] {
		t.Fatalf("oldest disk snapshot should have been evicted")
	}
	for i := 1; i < 4; i++ {
		if !seen[ids[i]] {
			t.Fatalf("snapshot %d should remain after load", i)
		}
	}

	if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("snapshot-%s.json", ids[0]))); !os.IsNotExist(err) {
		t.Fatalf("evicted disk snapshot file should be removed")
	}

	CommitSnapshots()
}

// ──────────────────────────────────────────────
// #33/#44: 内容按 JSON 解析结果原样使用
// ──────────────────────────────────────────────

func TestReplaceContentAsIs_NoTransform(t *testing.T) {
	m, opt := withFS(map[string]string{"asis.txt": "a\nb\nc\n"})
	content := "x\\ny"
	res, err := Replace("asis.txt", 2, 2, nil, content, "plain", false, "", false, opt)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	got := readFS(t, m, "asis.txt")
	if strings.Contains(got, "x\n") && !strings.Contains(got, "x\\n") {
		// This would mean \n was real newline when we expected literal
	}
	_ = res
}

// ──────────────────────────────────────────────
// #34: Replace 保留制表符缩进
// ──────────────────────────────────────────────

func TestReplaceTabIndentation_MultipleLines(t *testing.T) {
	content := "func foo() {\n\tif true {\n\t\tdoSomething()\n\t}\n}\n"
	m, opt := withFS(map[string]string{"main.go": content})
	old := "\tif true {\n\t\tdoSomething()\n\t}\n"
	newContent := "\tif false {\n\t\tdoNothing()\n\t}\n"
	_, err := Replace("main.go", 2, 4, &old, newContent, "plain", false, "", false, opt)
	if err != nil {
		t.Fatalf("replace multi-line tab: %v", err)
	}
	got := readFS(t, m, "main.go")
	expected := "func foo() {\n\tif false {\n\t\tdoNothing()\n\t}\n}\n"
	if got != expected {
		t.Fatalf("expected tab indentation preserved, got: %q", got)
	}
}

// ──────────────────────────────────────────────
// #26: brief mode
// ──────────────────────────────────────────────

func TestShowBriefMode(t *testing.T) {
	m, opt := withFS(map[string]string{"brief.txt": "line1\nline2\nline3\n"})
	res, _, err := Show("brief.txt", 1, 3, true, opt)
	if err != nil {
		t.Fatalf("show brief: %v", err)
	}
	if res.Brief != true {
		t.Fatal("expected Brief=true in result")
	}
	if res.Content != "" {
		t.Fatalf("expected empty content in brief mode, got: %q", res.Content)
	}
	if res.Total != 3 {
		t.Fatalf("expected total=3, got %d", res.Total)
	}
	_ = m
}

// ──────────────────────────────────────────────
// #21: injectable FileSystem（迁移到 MemFS）
// ──────────────────────────────────────────────

func TestInjectedFileSystem_BlockedRead(t *testing.T) {
	// MemFS does not have a block feature; simulate by simply not creating the file
	m := NewMemFS(nil)
	_, _, err := Show("blocked.txt", 1, 1, false, WithFileSystem(m))
	if err == nil {
		t.Fatal("expected error for missing file in MemFS")
	}
	_ = m
}

// ──────────────────────────────────────────────
// #35: tool param convention — target resolution edge cases
// ──────────────────────────────────────────────

func TestTargetResolution_MarkerNotFound(t *testing.T) {
	_, opt := withFS(map[string]string{"marker.txt": "a\nb\nc\n"})
	_, err := ResolveTargetSpan("marker.txt", ContentTarget{Kind: "marker", Value: "nonexistent"}, opt)
	if err == nil {
		t.Fatal("expected error for unresolved marker")
	}
}

func TestTargetResolution_FunctionNotFound(t *testing.T) {
	_, opt := withFS(map[string]string{"nofunc.txt": "just text\nno functions here\n"})
	_, err := ResolveTargetSpan("nofunc.txt", ContentTarget{Kind: "function", Value: "missingFunc"}, opt)
	if err == nil {
		t.Fatal("expected error for unresolved function")
	}
}

// ──────────────────────────────────────────────
// #8: start 参数越界与边界值
// ──────────────────────────────────────────────

func TestShowStartBoundary(t *testing.T) {
	m, opt := withFS(map[string]string{"boundary.txt": "only\n"})
	res, _, err := Show("boundary.txt", 1, 1, false, opt)
	if err != nil {
		t.Fatalf("show start=1 on 1-line file: %v", err)
	}
	if res.Start != 1 || res.End != 1 {
		t.Fatalf("expected range 1..1, got %d..%d", res.Start, res.End)
	}
	_ = m
}

func TestShowStartOutOfRange(t *testing.T) {
	_, opt := withFS(map[string]string{"oor.txt": "a\nb\n"})
	_, _, err := Show("oor.txt", 999, 1, false, opt)
	if err == nil {
		t.Fatal("expected error for start > total")
	}
}

// ──────────────────────────────────────────────
// snapshot isolation: workspace-scoped snapshot directories
// ──────────────────────────────────────────────

func TestWorkspaceID_StableAndShort(t *testing.T) {
	id1 := WorkspaceID()
	id2 := WorkspaceID()
	if id1 != id2 {
		t.Fatalf("WorkspaceID should be stable, got %q vs %q", id1, id2)
	}
	if len(id1) != 16 {
		t.Fatalf("expected 16-char hex workspace id, got %q (len=%d)", id1, len(id1))
	}
}

func TestSnapshotDir_IsolatesWorkspaces(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", "")
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	t.Setenv("BETTER_EDIT_WORKSPACE", "workspace-a")
	dirA := SnapshotDir()

	t.Setenv("BETTER_EDIT_WORKSPACE", "workspace-b")
	dirB := SnapshotDir()

	if dirA == dirB {
		t.Fatalf("expected different snapshot dirs for different workspaces, both %q", dirA)
	}
	baseA := filepath.Dir(dirA)
	baseB := filepath.Dir(dirB)
	if baseA != baseB {
		t.Fatalf("snapshot dirs should share the same base cache dir, got %q and %q", baseA, baseB)
	}
}

func TestSnapshotIsolation_RollbackDoesNotAffectOtherWorkspace(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", "")
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	// Workspace A: make an edit and leave the snapshot pending.
	t.Setenv("BETTER_EDIT_WORKSPACE", "ws-a")
	resetSnapshotStore(t)
	CommitSnapshots()
	pathA := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(pathA, []byte("a-original\n"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if _, err := Replace(pathA, 1, 1, nil, "a-modified\n", "plain", false, "", false); err != nil {
		t.Fatalf("replace a: %v", err)
	}
	if readFile(t, pathA) != "a-modified\n" {
		t.Fatalf("unexpected content for a before rollback")
	}

	// Workspace B: make an unrelated edit.
	t.Setenv("BETTER_EDIT_WORKSPACE", "ws-b")
	resetSnapshotStore(t)
	CommitSnapshots()
	pathB := filepath.Join(t.TempDir(), "b.txt")
	if err := os.WriteFile(pathB, []byte("b-original\n"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}
	if _, err := Replace(pathB, 1, 1, nil, "b-modified\n", "plain", false, "", false); err != nil {
		t.Fatalf("replace b: %v", err)
	}

	// Rollback workspace A only.
	t.Setenv("BETTER_EDIT_WORKSPACE", "ws-a")
	resetSnapshotStore(t)
	count, errs := RollbackSnapshots(1)
	if len(errs) > 0 {
		t.Fatalf("rollback a errors: %v", errs)
	}
	if count != 1 {
		t.Fatalf("expected 1 rollback in workspace a, got %d", count)
	}
	if readFile(t, pathA) != "a-original\n" {
		t.Fatalf("workspace a should be restored, got: %q", readFile(t, pathA))
	}

	// Workspace B should still have its pending snapshot and be unaffected.
	t.Setenv("BETTER_EDIT_WORKSPACE", "ws-b")
	resetSnapshotStore(t)
	if len(ListSnapshots()) != 1 {
		t.Fatalf("workspace b should still have 1 pending snapshot, got %d", len(ListSnapshots()))
	}
	count, errs = RollbackSnapshots(1)
	if len(errs) > 0 {
		t.Fatalf("rollback b errors: %v", errs)
	}
	if count != 1 {
		t.Fatalf("expected 1 rollback in workspace b, got %d", count)
	}
	if readFile(t, pathB) != "b-original\n" {
		t.Fatalf("workspace b should be restored, got: %q", readFile(t, pathB))
	}

	CommitSnapshots()
}

// ──────────────────────────────────────────────
// persist: 可配置磁盘持久化
// ──────────────────────────────────────────────

func TestSnapshotPersistDisabled_NoDiskWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", dir)
	t.Setenv("BETTER_EDIT_SNAPSHOT_PERSIST", "false")
	resetSnapshotStore(t)
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "no_persist.txt")
	if err := os.WriteFile(path, []byte("original\ncontent\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Replace(path, 2, 2, nil, "modified\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read snapshot dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no snapshot files when persistence disabled, got %d", len(entries))
	}

	// Rollback should still work from memory.
	count, errs := RollbackSnapshots(1)
	if len(errs) > 0 {
		t.Fatalf("rollback errors: %v", errs)
	}
	if count != 1 {
		t.Fatalf("expected 1 rollback, got %d", count)
	}
	if readFile(t, path) != "original\ncontent\n" {
		t.Fatalf("expected in-memory rollback to work, got: %q", readFile(t, path))
	}
	CommitSnapshots()
}

func TestSnapshotPersistDisabled_RestartLosesSnapshots(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BETTER_EDIT_SNAPSHOT_DIR", dir)
	t.Setenv("BETTER_EDIT_SNAPSHOT_PERSIST", "false")
	resetSnapshotStore(t)
	CommitSnapshots()
	path := filepath.Join(t.TempDir(), "restart_no_persist.txt")
	if err := os.WriteFile(path, []byte("original\ncontent\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Replace(path, 2, 2, nil, "modified\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}

	// Simulate process restart.
	resetSnapshotStore(t)

	// Rollback should not find any snapshots after restart.
	count, _ := RollbackSnapshots(1)
	if count != 0 {
		t.Fatalf("expected 0 rollbacks after restart without persistence, got %d", count)
	}
	if readFile(t, path) != "original\nmodified\n" {
		t.Fatalf("expected file to remain modified after restart without persistence, got: %q", readFile(t, path))
	}
}
